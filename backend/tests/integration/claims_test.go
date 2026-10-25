package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/services"
	"github.com/eyho/eyho/tests/testutil"
)

func remaining(t *testing.T, e *testutil.Env, listingID string) int {
	t.Helper()
	return count(t, e, `SELECT portions_remaining FROM listings WHERE id=$1`, listingID)
}

// SC-003 / Scenario 3: 50 buyers race for the last portion; exactly one wins.
func TestClaims_ConcurrentLastPortion(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "racecook", "professional")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 1)

	const n = 50
	tokens := make([]string, n)
	for i := range tokens {
		tokens[i], _ = verifiedBuyer(e, fmt.Sprintf("racer%d", i))
		// Provision every account up front so the race is about claims, not sign-in.
		require.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", tokens[i], nil).Code)
	}

	var ok, conflict, other atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			<-start
			rec := e.Do(http.MethodPost, "/api/v1/claims", tok, map[string]any{"listing_id": listing, "quantity": 1})
			switch rec.Code {
			case http.StatusCreated:
				ok.Add(1)
			case http.StatusConflict:
				if testutil.JSON(t, rec)["error"] == "insufficient_portions" {
					conflict.Add(1)
				} else {
					other.Add(1)
				}
			default:
				other.Add(1)
			}
		}(tokens[i])
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), ok.Load(), "exactly one claim succeeds")
	assert.Equal(t, int32(n-1), conflict.Load(), "everyone else gets 409 insufficient_portions")
	assert.Zero(t, other.Load())
	assert.Equal(t, 0, remaining(t, e, listing))
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM claims WHERE listing_id=$1`, listing))
}

// Same race with multi-portion claims: never more portions claimed than the listing has.
func TestClaims_NoOverselling_MultiPortion(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "oversellcook", "professional")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 5)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		tok, _ := verifiedBuyer(e, fmt.Sprintf("multi%d", i))
		require.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", tok, nil).Code)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e.Do(http.MethodPost, "/api/v1/claims", tok, map[string]any{"listing_id": listing, "quantity": 2})
		}()
	}
	close(start)
	wg.Wait()
	claimed := count(t, e, `SELECT COALESCE(sum(quantity),0)::int FROM claims WHERE listing_id=$1`, listing)
	assert.Equal(t, 4, claimed, "two claims of 2 fit into 5 portions")
	assert.Equal(t, 5-claimed, remaining(t, e, listing))
}

// Scenario 2: claim, pay via webhook, impact record, history.
func TestClaims_PaidFlowCreatesImpactRecord(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "paidcook", "nonprofit")
	buyer, _ := verifiedBuyer(e, "paidbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 5)

	rec := e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 2})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	body := testutil.JSON(t, rec)
	claimID := body["claim_id"].(string)
	assert.EqualValues(t, 2, body["quantity"])
	assert.EqualValues(t, 1000, body["amount_due_cents"])
	assert.Equal(t, "pi_fake_"+claimID+"_secret", body["stripe_client_secret"])
	expires, err := time.Parse(time.RFC3339, body["payment_reservation_expires_at"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(services.PaymentReservationTimeout), expires, 30*time.Second)
	assert.Equal(t, 3, remaining(t, e, listing), "portions are reserved immediately")
	assert.Equal(t, "pending_payment", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claimID))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claimID))

	// An unsigned delivery is refused and changes nothing.
	rec = e.Webhook(testutil.SucceedBody(claimID, 1000), "forged")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "pending_payment", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claimID))

	// A payment for the wrong amount is not confirmed.
	require.Equal(t, http.StatusOK, e.Webhook(testutil.SucceedBody(claimID, 1), testutil.ValidSignature).Code)
	assert.Equal(t, "pending_payment", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claimID))

	// The real success event; redelivery is idempotent.
	payClaim(t, e, claimID, 1000)
	payClaim(t, e, claimID, 1000)
	assert.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claimID))
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM claims WHERE id=$1 AND confirmed_at IS NOT NULL`, claimID))
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claimID), "SC-005: one impact record per paid claim")
	assert.Equal(t, 2, count(t, e, `SELECT portions FROM impact_records WHERE claim_id=$1`, claimID))
	assert.Equal(t, 2*services.GramsPerPortion, count(t, e, `SELECT estimated_weight_grams FROM impact_records WHERE claim_id=$1`, claimID))
	assert.Equal(t, 3, remaining(t, e, listing))

	// Order history and detail show the claim with the listing block.
	rec = e.Do(http.MethodGet, "/api/v1/claims/"+claimID, buyer, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.JSON(t, rec)
	assert.Equal(t, "paid", got["payment_status"])
	assert.EqualValues(t, 1000, got["amount_paid_cents"])
	assert.Equal(t, "Teststraat 1", got["listing"].(map[string]any)["pickup_address_text"])
	assert.Equal(t, "Sanne's Kitchen", got["listing"].(map[string]any)["cook_display_name"])

	// The listing's cook may read it too; a stranger gets 404.
	assert.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/claims/"+claimID, cook.Token, nil).Code)
	stranger, _ := verifiedBuyer(e, "stranger")
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodGet, "/api/v1/claims/"+claimID, stranger, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodPost, "/api/v1/claims/"+claimID+"/confirm-pickup", stranger, nil).Code)
}

// Webhook payment failure releases the portions.
func TestClaims_PaymentFailedReleasesPortions(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "failcook", "professional")
	buyer, _ := verifiedBuyer(e, "failbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 3)

	id := claimOK(t, e, buyer, listing, 2)
	require.Equal(t, 1, remaining(t, e, listing))
	require.Equal(t, http.StatusOK, e.Webhook(testutil.FailBody(id, 1000), testutil.ValidSignature).Code)
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, id))
	assert.Equal(t, 3, remaining(t, e, listing))
	// Redelivery must not return the portions twice.
	require.Equal(t, http.StatusOK, e.Webhook(testutil.FailBody(id, 1000), testutil.ValidSignature).Code)
	assert.Equal(t, 3, remaining(t, e, listing))

	// Failed payments never appear in the order history.
	rec := e.Do(http.MethodGet, "/api/v1/claims", buyer, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.JSON(t, rec)["claims"])
}

// Scenario 4 / FR-008: unpaid reservations are released by the timeout job; a late payment is refunded.
func TestClaims_TimeoutRelease(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "timeoutcook", "professional")
	buyer, _ := verifiedBuyer(e, "timeoutbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 3)

	stale := claimOK(t, e, buyer, listing, 2)
	fresh := claimOK(t, e, buyer, listing, 1)
	require.Equal(t, 0, remaining(t, e, listing))
	exec(t, e, `UPDATE claims SET reserved_at = NOW() - INTERVAL '11 minutes' WHERE id=$1`, stale)

	require.NoError(t, e.Claims.PaymentTimeoutJob(context.Background()))
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, stale))
	assert.Equal(t, "pending_payment", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, fresh), "within the window")
	assert.Equal(t, 2, remaining(t, e, listing), "stale quantity is back")
	require.NoError(t, e.Claims.PaymentTimeoutJob(context.Background()))
	assert.Equal(t, 2, remaining(t, e, listing), "the job is idempotent")

	// The released portions can be claimed by someone else.
	other, _ := verifiedBuyer(e, "timeoutother")
	claimOK(t, e, other, listing, 2)

	// A payment arriving after the release is refunded, not confirmed, and creates no impact.
	before := e.Payments.RefundCount()
	payClaim(t, e, stale, 1000)
	assert.Equal(t, before+1, e.Payments.RefundCount())
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, stale))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, stale))
}

// Provider outage: 502, reservation released, nothing stays reserved.
func TestClaims_PaymentProviderDownReleases(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "downcook", "professional")
	buyer, _ := verifiedBuyer(e, "downbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 2)

	e.Payments.FailCreate = true
	rec := e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 1})
	e.Payments.FailCreate = false
	require.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, "payment_unavailable", testutil.JSON(t, rec)["error"])
	assert.Equal(t, 2, remaining(t, e, listing))
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM claims WHERE listing_id=$1 AND payment_status='payment_failed'`, listing))
}

// Scenario 5 / FR-007, FR-012: expiry and no-show without refund; confirm-pickup then conflicts.
func TestClaims_ExpiryAndNoShow(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "expirycook", "professional")
	buyer, _ := verifiedBuyer(e, "expirybuyer")
	collector, _ := verifiedBuyer(e, "expirycollector")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 3)
	noShow := paidClaim(t, e, buyer, listing)
	collected := paidClaim(t, e, collector, listing)
	collect(t, e, collector, collected)

	// Move the pickup window into the past (creation itself rejects past windows).
	exec(t, e, `UPDATE listings SET pickup_window_start = NOW() - INTERVAL '3 hours', pickup_window_end = NOW() - INTERVAL '1 minute' WHERE id=$1`, listing)
	refunds := e.Payments.RefundCount()
	require.NoError(t, e.Listings.ExpiryJob(context.Background()))

	assert.Equal(t, "expired", str(t, e, `SELECT status::text FROM listings WHERE id=$1`, listing))
	assert.Equal(t, "no_show", str(t, e, `SELECT pickup_status::text FROM claims WHERE id=$1`, noShow))
	assert.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, noShow), "no-shows are not refunded")
	assert.Equal(t, "collected", str(t, e, `SELECT pickup_status::text FROM claims WHERE id=$1`, collected))
	assert.Equal(t, refunds, e.Payments.RefundCount())

	rec := e.Do(http.MethodPost, "/api/v1/claims/"+noShow+"/confirm-pickup", buyer, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// An expired listing is gone from discovery and cannot be claimed.
	rec = e.Do(http.MethodGet, fmt.Sprintf("/api/v1/listings?lat=%f&lng=%f", lat, lng), "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.JSON(t, rec)["listings"])
	rec = e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 1})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestClaims_ValidationAndAuth(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "valcook", "professional")
	buyer, _ := verifiedBuyer(e, "valbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 2)

	// Not signed in / email not verified.
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodPost, "/api/v1/claims", "", map[string]any{"listing_id": listing, "quantity": 1}).Code)
	email, sub := testutil.Unique("unverifiedbuyer")
	unverified := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: false})
	rec := e.Do(http.MethodPost, "/api/v1/claims", unverified, map[string]any{"listing_id": listing, "quantity": 1})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "email_not_verified", testutil.JSON(t, rec)["error"])

	// Validation.
	for _, q := range []int{0, -1, 101} {
		rec = e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": q})
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "quantity %d", q)
	}
	assert.Equal(t, http.StatusUnprocessableEntity, e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"quantity": 1}).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": "nope", "quantity": 1}).Code)

	// Unknown listing, draft listing.
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": uuid.NewString(), "quantity": 1}).Code)
	b := listingBody(lat, lng)
	b["publish"] = false
	rec = e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
	require.Equal(t, http.StatusCreated, rec.Code)
	draft := testutil.JSON(t, rec)["id"].(string)
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": draft, "quantity": 1}).Code)

	// Too many portions: 409 with the remaining count.
	rec = e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 3})
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, map[string]any{"error": "insufficient_portions", "portions_remaining": float64(2)}, testutil.JSON(t, rec))
}

func TestClaims_ConfirmPickupByCookAndStates(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "pickcook", "professional")
	buyer, _ := verifiedBuyer(e, "pickbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 3)

	unpaid := claimOK(t, e, buyer, listing, 1)
	rec := e.Do(http.MethodPost, "/api/v1/claims/"+unpaid+"/confirm-pickup", buyer, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "an unpaid claim cannot be collected")

	paid := paidClaim(t, e, buyer, listing)
	collect(t, e, cook.Token, paid) // the listing's cook may confirm
	assert.Equal(t, "collected", str(t, e, `SELECT pickup_status::text FROM claims WHERE id=$1`, paid))
	assert.Equal(t, http.StatusConflict, e.Do(http.MethodPost, "/api/v1/claims/"+paid+"/confirm-pickup", buyer, nil).Code, "already collected")
}

func TestClaims_ListKeysetPagination(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "pagecook", "professional")
	buyer, _ := verifiedBuyer(e, "pagebuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 10)
	var ids []string
	for i := 0; i < 5; i++ {
		ids = append(ids, paidClaim(t, e, buyer, listing))
	}

	var seen []string
	cursor := ""
	for page := 0; page < 5; page++ {
		path := "/api/v1/claims?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		rec := e.Do(http.MethodGet, path, buyer, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := testutil.JSON(t, rec)
		for _, c := range body["claims"].([]any) {
			seen = append(seen, c.(map[string]any)["id"].(string))
		}
		if body["next_cursor"] == nil {
			break
		}
		cursor = body["next_cursor"].(string)
	}
	require.Len(t, seen, 5)
	// Newest first: the reverse of creation order.
	for i := range ids {
		assert.Equal(t, ids[len(ids)-1-i], seen[i])
	}
	assert.Equal(t, http.StatusUnprocessableEntity, e.Do(http.MethodGet, "/api/v1/claims?cursor=garbage", buyer, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodGet, "/api/v1/claims", "", nil).Code)
}

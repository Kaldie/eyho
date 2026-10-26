package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

func patch(e *testutil.Env, token, listing string, body map[string]any) (int, map[string]any) {
	rec := e.Do(http.MethodPatch, "/api/v1/listings/"+listing, token, body)
	return rec.Code, testutil.JSON(e.T, rec)
}

// Quickstart Scenario 6 (FR-010): cancelling refunds paid claims and queues notifications.
func TestCancelListing_RefundsPaidClaims(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "cancelcook", "professional")
	b1, _ := verifiedBuyer(e, "cancelb1")
	b2, _ := verifiedBuyer(e, "cancelb2")
	b3, _ := verifiedBuyer(e, "cancelb3")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 5)

	paid1 := paidClaim(t, e, b1, listing)
	paid2 := paidClaim(t, e, b2, listing)
	unpaid := claimOK(t, e, b3, listing, 1)
	collected := paidClaim(t, e, b1, listing)
	collect(t, e, b1, collected)

	// Only the owning cook may cancel.
	other := newCook(t, e, "cancelother", "nonprofit")
	code, body := patch(e, other.Token, listing, map[string]any{"status": "cancelled"})
	assert.Equal(t, http.StatusForbidden, code, "%v", body)
	assert.Equal(t, "published", str(t, e, `SELECT status::text FROM listings WHERE id=$1`, listing))
	code, _ = patch(e, b1, listing, map[string]any{"status": "cancelled"})
	assert.Equal(t, http.StatusForbidden, code, "a buyer without a cook profile")

	code, body = patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, "cancelled", body["status"])

	for _, id := range []string{paid1, paid2} {
		assert.Equal(t, "refunded", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, id))
		assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM notifications WHERE claim_id=$1 AND kind='listing_cancelled_refund' AND sent_at IS NULL`, id), "notification queued")
	}
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, unpaid), "unpaid reservation is dropped")
	assert.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, collected), "collected food is not refunded")
	assert.ElementsMatch(t, []string{"pi_fake_" + paid1, "pi_fake_" + paid2}, e.Payments.Refunds[len(e.Payments.Refunds)-2:])

	// A late payment on the dropped reservation is refunded too.
	before := e.Payments.RefundCount()
	payClaim(t, e, unpaid, 500)
	assert.Equal(t, before+1, e.Payments.RefundCount())

	// Gone from discovery, cannot be claimed, cannot be cancelled twice.
	rec := e.Do(http.MethodGet, "/api/v1/listings?lat="+ftoa(lat)+"&lng="+ftoa(lng), "", nil)
	assert.Empty(t, testutil.JSON(t, rec)["listings"])
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodPost, "/api/v1/claims", b2, map[string]any{"listing_id": listing, "quantity": 1}).Code)
	code, _ = patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	assert.Equal(t, http.StatusConflict, code)

	// Retrying the sweep does not refund or notify again.
	n := e.Payments.RefundCount()
	require.NoError(t, e.Listings.ExpiryJob(context.Background()))
	assert.Equal(t, n, e.Payments.RefundCount())
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM notifications WHERE claim_id=$1`, paid1))
}

func TestCancelListing_NoClaimsRemovesFromDiscovery(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "cancelempty", "professional")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 2)
	code, _ := patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	require.Equal(t, http.StatusOK, code)
	rec := e.Do(http.MethodGet, "/api/v1/listings?lat="+ftoa(lat)+"&lng="+ftoa(lng), "", nil)
	assert.Empty(t, testutil.JSON(t, rec)["listings"])
	assert.Zero(t, count(t, e, `SELECT count(*) FROM notifications n JOIN claims c ON c.id=n.claim_id WHERE c.listing_id=$1`, listing))
}

// A refund the provider rejects leaves the claim paid; the job retries it.
func TestCancelListing_RefundFailureIsRetriedByJob(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "refundfailcook", "professional")
	buyer, _ := verifiedBuyer(e, "refundfailbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 2)
	id := paidClaim(t, e, buyer, listing)

	e.Payments.FailRefund = true
	code, _ := patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	require.Equal(t, http.StatusOK, code, "the listing is cancelled even if a refund is pending")
	assert.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, id))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM notifications WHERE claim_id=$1`, id))

	e.Payments.FailRefund = false
	require.NoError(t, e.Listings.ExpiryJob(context.Background()))
	assert.Equal(t, "refunded", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, id))
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM notifications WHERE claim_id=$1`, id))
}

// Manual portion edits and window extension (US3).
func TestUpdateListing_PortionsAndWindow(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "editcook", "professional")
	buyer, _ := verifiedBuyer(e, "editbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 5)
	claimOK(t, e, buyer, listing, 2) // 3 remaining, 2 held

	code, body := patch(e, cook.Token, listing, map[string]any{"portions_remaining": 1})
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.EqualValues(t, 1, body["portions_remaining"])

	// Raising it above the unclaimed capacity (5 total - 2 held = 3) would oversell: 409.
	code, body = patch(e, cook.Token, listing, map[string]any{"portions_remaining": 4})
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "portions_conflict", body["error"])
	code, _ = patch(e, cook.Token, listing, map[string]any{"portions_remaining": 3})
	assert.Equal(t, http.StatusOK, code)
	code, _ = patch(e, cook.Token, listing, map[string]any{"portions_remaining": -1})
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = patch(e, cook.Token, listing, map[string]any{"portions_remaining": 6})
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	// Extend the window; shortening is refused.
	end := time.Now().Add(5 * time.Hour).UTC().Format(time.RFC3339)
	code, body = patch(e, cook.Token, listing, map[string]any{"pickup_window_end": end})
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, end, body["pickup_window_end"])
	code, body = patch(e, cook.Token, listing, map[string]any{"pickup_window_end": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.Equal(t, "window_not_extended", body["error"])

	// Empty or unsupported patches, and unknown listings.
	code, _ = patch(e, cook.Token, listing, map[string]any{})
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = patch(e, cook.Token, listing, map[string]any{"status": "expired"})
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = patch(e, cook.Token, "00000000-0000-4000-8000-000000000000", map[string]any{"portions_remaining": 1})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodPatch, "/api/v1/listings/"+listing, "", map[string]any{"portions_remaining": 1}).Code)
}

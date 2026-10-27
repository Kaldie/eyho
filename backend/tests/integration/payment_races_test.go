package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/services"
	"github.com/eyho/eyho/tests/testutil"
)

// raceSetup returns an env, a published listing of 3 portions and an unpaid claim of 2.
func raceSetup(t *testing.T, prefix string) (e *testutil.Env, cook testCook, buyer, listing, claim string) {
	t.Helper()
	e = testutil.NewEnv(t, false)
	cook = newCook(t, e, prefix+"cook", "professional")
	buyer, _ = verifiedBuyer(e, prefix+"buyer")
	lat, lng := uniquePoint()
	listing = publish(t, e, cook, lat, lng, 3)
	claim = claimOK(t, e, buyer, listing, 2)
	return
}

// webhookDuringRelease reproduces the B2 interleaving deterministically: a transaction that has
// already locked the listing and released the claim (what the timeout job or a cancel does) is
// still open when the success webhook arrives. The webhook must block on the listing lock, then see
// the release after the commit and refund instead of keeping the money.
func webhookDuringRelease(t *testing.T, e *testutil.Env, listing, claim string, release func(tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := e.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	_, err = tx.Exec(ctx, `SELECT 1 FROM listings WHERE id=$1 FOR UPDATE`, listing)
	require.NoError(t, err)
	release(tx)

	var wg sync.WaitGroup
	wg.Add(1)
	var code int
	go func() {
		defer wg.Done()
		code = e.Webhook(testutil.SucceedBody(claim, 1000), testutil.ValidSignature).Code
	}()
	// Wait until the webhook is blocked on a lock held by our transaction.
	require.Eventually(t, func() bool {
		var n int
		require.NoError(t, e.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query ILIKE '%FOR UPDATE%'`).Scan(&n))
		return n > 0
	}, 5*time.Second, 20*time.Millisecond)
	assert.Zero(t, e.Payments.RefundCount(), "nothing is decided while the release is uncommitted")
	require.NoError(t, tx.Commit(ctx))
	wg.Wait()
	require.Equal(t, http.StatusOK, code)
}

func TestPaymentRace_SuccessDuringTimeoutRelease(t *testing.T) {
	e, _, _, listing, claim := raceSetup(t, "racetimeout")
	webhookDuringRelease(t, e, listing, claim, func(tx pgx.Tx) {
		ctx := context.Background()
		_, err := tx.Exec(ctx, `UPDATE claims SET payment_status='payment_failed' WHERE id=$1 AND payment_status='pending_payment'`, claim)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `UPDATE listings SET portions_remaining = portions_remaining + 2 WHERE id=$1`, listing)
		require.NoError(t, err)
	})
	assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.Refunds, "the late payment is refunded")
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claim))
	assert.Equal(t, 3, remaining(t, e, listing))
}

func TestPaymentRace_SuccessDuringCancel(t *testing.T) {
	e, _, _, listing, claim := raceSetup(t, "racecancel")
	webhookDuringRelease(t, e, listing, claim, func(tx pgx.Tx) {
		ctx := context.Background()
		_, err := tx.Exec(ctx, `UPDATE listings SET status='cancelled' WHERE id=$1`, listing)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `UPDATE claims SET payment_status='payment_failed' WHERE id=$1`, claim)
		require.NoError(t, err)
	})
	assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.Refunds)
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claim))
}

// The success wins the race when it commits first: the claim is paid and the later timeout job
// finds nothing to release.
func TestPaymentRace_SuccessBeforeTimeoutWins(t *testing.T) {
	e, _, _, listing, claim := raceSetup(t, "racewin")
	exec(t, e, `UPDATE claims SET reserved_at = NOW() - INTERVAL '11 minutes' WHERE id=$1`, claim)
	payClaim(t, e, claim, 1000)
	require.NoError(t, e.Claims.PaymentTimeoutJob(context.Background()))
	assert.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
	assert.Equal(t, 1, remaining(t, e, listing))
	assert.Zero(t, e.Payments.RefundCount())
	assert.NotContains(t, e.Payments.CancelledIntents(), "pi_fake_"+claim)
}

// Concurrent success webhook, timeout job and cancel never keep money for a released claim and
// never leave a paid claim whose portions were returned.
func TestPaymentRace_ConcurrentStress(t *testing.T) {
	for i := 0; i < 10; i++ {
		e, cook, _, listing, claim := raceSetup(t, "racestress")
		exec(t, e, `UPDATE claims SET reserved_at = NOW() - INTERVAL '11 minutes' WHERE id=$1`, claim)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); e.Webhook(testutil.SucceedBody(claim, 1000), testutil.ValidSignature) }()
		go func() { defer wg.Done(); _ = e.Claims.PaymentTimeoutJob(context.Background()) }()
		go func() {
			defer wg.Done()
			patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
		}()
		wg.Wait()
		// A cancel after the payment refunds through the sweep; run it as the expiry job would.
		require.NoError(t, e.Listings.ExpiryJob(context.Background()))
		status := str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim)
		switch status {
		case "paid":
			t.Fatalf("a paid claim on a cancelled listing must have been refunded")
		case "refunded":
			assert.Equal(t, 1, refundsOf(e, claim))
		case "payment_failed":
			assert.Equal(t, 1, refundsOf(e, claim), "released claim: the payment is refunded")
		default:
			t.Fatalf("unexpected status %s", status)
		}
	}
}

func TestPayment_LateSuccessAfterCancelIsRefunded(t *testing.T) {
	e, cook, _, listing, claim := raceSetup(t, "latecancel")
	code, _ := patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.CancelledIntents(), "cancel closes the open intent")
	payClaim(t, e, claim, 1000)
	assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.Refunds)
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claim))
}

func TestPayment_AmountOrCurrencyMismatchIsRefunded(t *testing.T) {
	for name, body := range map[string]func(claim string) string{
		"amount":   func(c string) string { return testutil.SucceedBody(c, 1) },
		"currency": func(c string) string { return testutil.SucceedBodyCurrency(c, 1000, "usd") },
	} {
		t.Run(name, func(t *testing.T) {
			e, _, _, listing, claim := raceSetup(t, "mismatch"+name)
			require.Equal(t, 1, remaining(t, e, listing))
			require.Equal(t, http.StatusOK, e.Webhook(body(claim), testutil.ValidSignature).Code)
			assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.Refunds)
			assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
			assert.Equal(t, 3, remaining(t, e, listing), "the reservation is released")
			assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claim))
			// A redelivery refunds again with the same idempotency key (the provider dedupes).
			require.Equal(t, http.StatusOK, e.Webhook(body(claim), testutil.ValidSignature).Code)
			assert.Equal(t, 3, remaining(t, e, listing))
		})
	}
}

func TestPayment_SuccessAfterWindowEndedIsRefunded(t *testing.T) {
	e, _, _, listing, claim := raceSetup(t, "windowend")
	exec(t, e, `UPDATE listings SET pickup_window_start = NOW() - INTERVAL '3 hours', pickup_window_end = NOW() - INTERVAL '1 minute' WHERE id=$1`, listing)
	payClaim(t, e, claim, 1000)
	assert.Equal(t, []string{"pi_fake_" + claim}, e.Payments.Refunds)
	assert.Equal(t, "payment_failed", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, claim))
	assert.Zero(t, count(t, e, `SELECT count(*) FROM impact_records WHERE claim_id=$1`, claim))
}

func TestPayment_RefundFailureAnswers500AndRetryRefunds(t *testing.T) {
	e, cook, _, listing, claim := raceSetup(t, "refundfail")
	patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	e.Payments.FailRefund = true
	rec := e.Webhook(testutil.SucceedBody(claim, 1000), testutil.ValidSignature)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	e.Payments.FailRefund = false
	payClaim(t, e, claim, 1000)
	assert.Equal(t, 1, e.Payments.RefundCount())
}

func TestClaims_ConfirmPickupRefusedOnCancelledListing(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "cancelpickcook", "professional")
	buyer, _ := verifiedBuyer(e, "cancelpickbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 2)
	paid := paidClaim(t, e, buyer, listing)
	// Cancelled but the refund failed, so the claim is still paid.
	e.Payments.FailRefund = true
	code, _ := patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	e.Payments.FailRefund = false
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "paid", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, paid))
	for _, tok := range []string{buyer, cook.Token} {
		rec := e.Do(http.MethodPost, "/api/v1/claims/"+paid+"/confirm-pickup", tok, nil)
		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "claim_conflict", testutil.JSON(t, rec)["error"])
	}
	assert.Equal(t, "pending", str(t, e, `SELECT pickup_status::text FROM claims WHERE id=$1`, paid))

	// The retry sweep refunds it (and leaves no stuck claim behind for other tests' sweeps).
	require.NoError(t, e.Listings.ExpiryJob(context.Background()))
	assert.Equal(t, "refunded", str(t, e, `SELECT payment_status::text FROM claims WHERE id=$1`, paid))
}

func TestClaims_PerBuyerReservationCap(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "capcook", "professional")
	buyer, _ := verifiedBuyer(e, "capbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 10)
	var ids []string
	for i := 0; i < services.MaxOpenReservationsPerBuyer; i++ {
		ids = append(ids, claimOK(t, e, buyer, listing, 1))
	}
	rec := e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 1})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "too_many_reservations", testutil.JSON(t, rec)["error"])
	assert.Equal(t, 10-services.MaxOpenReservationsPerBuyer, remaining(t, e, listing), "a refused claim reserves nothing")

	// Paying one frees a slot; other buyers are not affected.
	payClaim(t, e, ids[0], 500)
	claimOK(t, e, buyer, listing, 1)
	other, _ := verifiedBuyer(e, "capother")
	claimOK(t, e, other, listing, 1)
}

func TestClaims_PerBuyerReservationCap_Concurrent(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "capccook", "professional")
	buyer, _ := verifiedBuyer(e, "capcbuyer")
	require.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", buyer, nil).Code)
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 20)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e.Do(http.MethodPost, "/api/v1/claims", buyer, map[string]any{"listing_id": listing, "quantity": 1})
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, services.MaxOpenReservationsPerBuyer, count(t, e, `SELECT count(*) FROM claims WHERE listing_id=$1 AND payment_status='pending_payment'`, listing))
}

// Principle II: impact totals exclude refunded claims.
func TestImpactTotals_ExcludeRefundedClaims(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "impactcook", "professional")
	buyer, _ := verifiedBuyer(e, "impactbuyer")
	lat, lng := uniquePoint()
	listing := publish(t, e, cook, lat, lng, 5)
	paidClaim(t, e, buyer, listing)
	paidClaim(t, e, buyer, listing)
	buyerID := accountBuyerProfile(t, e, buyer)
	tot, err := e.Q.ImpactTotals(context.Background(), buyerID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, tot.Portions)
	assert.EqualValues(t, 2*services.GramsPerPortion, tot.EstimatedWeightGrams)

	code, _ := patch(e, cook.Token, listing, map[string]any{"status": "cancelled"})
	require.Equal(t, http.StatusOK, code)
	tot, err = e.Q.ImpactTotals(context.Background(), buyerID)
	require.NoError(t, err)
	assert.Zero(t, tot.Portions, "refunded claims no longer count")
	assert.Zero(t, tot.EstimatedWeightGrams)
}

func accountBuyerProfile(t *testing.T, e *testutil.Env, token string) pgtype.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.Pool.QueryRow(context.Background(), `SELECT id FROM buyer_profiles WHERE account_id=$1`, accountOf(t, e, token)).Scan(&id))
	return pgtype.UUID{Bytes: id, Valid: true}
}

// refundsOf counts the refunds issued for the claim's intent (jobs are global on the shared test
// database, so the total can include other tests' claims).
func refundsOf(e *testutil.Env, claim string) int {
	n := 0
	for _, id := range e.Payments.Refunds {
		if id == "pi_fake_"+claim {
			n++
		}
	}
	return n
}

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/registry"
	"github.com/eyho/eyho/src/services"
	"github.com/eyho/eyho/tests/testutil"
)

// Quickstart Scenario 6 (FR-013, SC-004).
func TestAccountDeletion_Scenario6(t *testing.T) {
	e := testutil.NewEnv(t, false)
	ctx := context.Background()

	cookEmail, cookSub := testutil.Unique("delcook")
	cookTok := e.Issuer.Token(testutil.TokenOptions{Subject: cookSub, Email: cookEmail, EmailVerified: true, Name: "Katrien Privé"})
	buyerTok, buyerEmail := verifiedBuyer(e, "delbuyer")

	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", cookTok, validCook(registry.FakeActiveNumber1))
	require.Equal(t, http.StatusCreated, rec.Code)
	cookID := uuid.MustParse(testutil.JSON(t, rec)["id"].(string))
	cookAccount := accountOf(t, e, cookTok)
	buyerAccount := accountOf(t, e, buyerTok)

	// Fixtures: a published listing of the cook, a paid claim awaiting pickup by the buyer, and
	// verification history for the cook.
	var listingID, buyerProfile pgtype.UUID
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT id FROM buyer_profiles WHERE account_id=$1`, buyerAccount).Scan(&buyerProfile))
	require.NoError(t, e.Pool.QueryRow(ctx, `
		INSERT INTO listings (cook_id, food_description, allergen_info, total_portions, portions_remaining,
			reduced_price_cents, retail_reference_price_cents, pickup_location, pickup_address_text,
			pickup_window_start, pickup_window_end, status)
		VALUES ($1, 'stew', 'none', 4, 3, 300, 900, ST_SetSRID(ST_MakePoint(4.9, 52.3), 4326)::geography,
			'Keizersgracht 88', $2, $3, 'published') RETURNING id`,
		cookID, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)).Scan(&listingID))
	var claimID pgtype.UUID
	require.NoError(t, e.Pool.QueryRow(ctx, `
		INSERT INTO claims (listing_id, buyer_id, quantity, payment_status, pickup_status, confirmed_at)
		VALUES ($1, $2, 1, 'paid', 'pending', NOW()) RETURNING id`, listingID, buyerProfile).Scan(&claimID))
	_, err := e.Pool.Exec(ctx, `INSERT INTO verification_history (cook_id, from_status, to_status, triggering_claim_ids)
		VALUES ($1, 'unverified', 'verified', ARRAY[$2]::uuid[])`, cookID, claimID)
	require.NoError(t, err)

	// The buyer cannot delete while the claim awaits pickup.
	rec = e.Do(http.MethodDelete, "/api/v1/me", buyerTok, nil)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, map[string]any{"error": "active_claims", "count": float64(1)}, testutil.JSON(t, rec))

	// Step 1: the cook is blocked by the paid claim on their listing.
	rec = e.Do(http.MethodDelete, "/api/v1/me", cookTok, nil)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, map[string]any{"error": "active_listings", "count": float64(1)}, testutil.JSON(t, rec))

	// Step 2: collected and rated, then deletion succeeds.
	_, err = e.Pool.Exec(ctx, `UPDATE claims SET pickup_status='collected', rating=5, rated_at=NOW() WHERE id=$1`, claimID)
	require.NoError(t, err)
	histBefore := count(t, e, `SELECT count(*) FROM verification_history WHERE cook_id=$1`, cookID)
	require.Equal(t, 1, histBefore)

	rec = e.Do(http.MethodDelete, "/api/v1/me", cookTok, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// Step 3: no personal data remains, history stays.
	var email, name *string
	var deletedAt pgtype.Timestamptz
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT email, display_name, deleted_at FROM accounts WHERE id=$1`, cookAccount).Scan(&email, &name, &deletedAt))
	assert.Nil(t, email)
	assert.Nil(t, name)
	assert.True(t, deletedAt.Valid)
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM account_identities WHERE account_id=$1`, cookAccount))

	var cname string
	var phone, addr, kvk, trade *string
	var anonymised pgtype.Timestamptz
	var acct pgtype.UUID
	var locNull bool
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT display_name, phone, default_pickup_address_text, kvk_number, kvk_trade_name,
		anonymised_at, account_id, default_pickup_location IS NULL FROM cook_profiles WHERE id=$1`, cookID).
		Scan(&cname, &phone, &addr, &kvk, &trade, &anonymised, &acct, &locNull))
	assert.Equal(t, "Former cook", cname)
	assert.Nil(t, phone)
	assert.Nil(t, addr)
	assert.Nil(t, kvk)
	assert.Nil(t, trade)
	assert.True(t, anonymised.Valid)
	assert.False(t, acct.Valid)
	assert.True(t, locNull)
	// Existing queries still scan the anonymised profile (NULL geography).
	cp, err := e.Q.GetCookProfileByID(ctx, pgtype.UUID{Bytes: cookID, Valid: true})
	require.NoError(t, err)
	assert.Nil(t, cp.DefaultPickupLocation)

	assert.Equal(t, histBefore, count(t, e, `SELECT count(*) FROM verification_history WHERE cook_id=$1`, cookID))
	var rating int
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT rating FROM claims WHERE id=$1`, claimID).Scan(&rating))
	assert.Equal(t, 5, rating, "ratings stay")
	var status string
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT status FROM listings WHERE id=$1`, listingID).Scan(&status))
	assert.Equal(t, "cancelled", status, "no listing stays claimable under a Former cook")

	// No trace of the cook's personal values anywhere in the account/cook tables.
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM accounts WHERE id=$1 AND (email IS NOT NULL OR display_name IS NOT NULL)`, cookAccount))
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM accounts WHERE email=$1 AND deleted_at IS NOT NULL`, cookEmail))
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM cook_profiles WHERE phone='+31612345678' AND id=$1`, cookID))

	// Provider deletion is queued; failures back off, success removes the row.
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM idp_deletion_requests WHERE subject=$1`, cookSub))
	e.IdP.FailDelete = true
	w := &services.IdpDeletionWorker{Q: e.Q, Provider: e.IdP}
	// Other rows in the shared DB may be due as well; only this subject matters.
	_, err = w.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM idp_deletion_requests WHERE subject=$1 AND attempts=1 AND next_attempt_at > NOW()`, cookSub))
	e.IdP.FailDelete = false
	_, err = e.Pool.Exec(ctx, `UPDATE idp_deletion_requests SET next_attempt_at=NOW() WHERE subject=$1`, cookSub)
	require.NoError(t, err)
	_, err = w.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM idp_deletion_requests WHERE subject=$1`, cookSub))
	assert.Contains(t, e.IdP.Deleted, cookSub)

	// Step 4: signing in again with the same identity gives a new, empty account.
	rec = e.Do(http.MethodGet, "/api/v1/me", cookTok, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	me := testutil.JSON(t, rec)
	assert.NotEqual(t, cookAccount.String(), me["id"])
	assert.Nil(t, me["cook_profile"])

	// The buyer can delete after collecting.
	_, err = e.Pool.Exec(ctx, `UPDATE claims SET pickup_status='collected' WHERE id=$1`, claimID)
	require.NoError(t, err)
	rec = e.Do(http.MethodDelete, "/api/v1/me", buyerTok, nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 0, count(t, e, `SELECT count(*) FROM accounts WHERE email=$1`, buyerEmail))
	// The claim keeps pointing at the (personal-data-free) buyer profile.
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM claims WHERE id=$1`, claimID))
}

func TestAccountDeletion_RollsBackWhenBlocked(t *testing.T) {
	e := testutil.NewEnv(t, false)
	ctx := context.Background()
	tok, email := verifiedBuyer(e, "delblocked")
	acc := accountOf(t, e, tok)
	// A published listing without claims would be cancelled on success; a blocked deletion must
	// leave everything as it was. Block via an active buyer claim on someone else's listing.
	var buyerProfile, listing pgtype.UUID
	require.NoError(t, e.Pool.QueryRow(ctx, `SELECT id FROM buyer_profiles WHERE account_id=$1`, acc).Scan(&buyerProfile))
	seedCook := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0x40, 0, 0x80, 0, 0, 0, 0, 0xc, 0, 3}, Valid: true}
	require.NoError(t, e.Pool.QueryRow(ctx, `
		INSERT INTO listings (cook_id, food_description, allergen_info, total_portions, portions_remaining,
			reduced_price_cents, retail_reference_price_cents, pickup_location, pickup_address_text,
			pickup_window_start, pickup_window_end, status)
		VALUES ($1, 'soup', 'none', 2, 1, 300, 900, ST_SetSRID(ST_MakePoint(4.9, 52.3), 4326)::geography,
			'x', $2, $3, 'draft') RETURNING id`, seedCook, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)).Scan(&listing))
	_, err := e.Pool.Exec(ctx, `INSERT INTO claims (listing_id, buyer_id, quantity, payment_status) VALUES ($1,$2,1,'pending_payment')`, listing, buyerProfile)
	require.NoError(t, err)

	rec := e.Do(http.MethodDelete, "/api/v1/me", tok, nil)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM accounts WHERE email=$1 AND deleted_at IS NULL`, email))
	assert.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", tok, nil).Code)
}

func count(t *testing.T, e *testutil.Env, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.Pool.QueryRow(context.Background(), q, args...).Scan(&n))
	return n
}

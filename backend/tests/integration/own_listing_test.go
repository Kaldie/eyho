package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

// Feature 001 FR-011 / Principle I: a cook cannot claim (and so cannot rate) their own listing.
func TestOwnListing_CookCannotClaimOwn(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "owncook", "professional")
	other := newCook(t, e, "othercook", "nonprofit")
	lat, lng := uniquePoint()
	own := publish(t, e, cook, lat, lng, 3)
	theirs := publish(t, e, other, lat, lng, 3)

	rec := e.Do(http.MethodPost, "/api/v1/claims", cook.Token, map[string]any{"listing_id": own, "quantity": 1})
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, map[string]any{"error": "own_listing"}, testutil.JSON(t, rec))
	assert.Equal(t, 3, remaining(t, e, own), "nothing was reserved")
	assert.Zero(t, count(t, e, `SELECT count(*) FROM claims WHERE listing_id=$1`, own))

	// The same cook may buy from another cook, and the other cook's own-listing rule is separate.
	claimOK(t, e, cook.Token, theirs, 1)
	rec = e.Do(http.MethodPost, "/api/v1/claims", other.Token, map[string]any{"listing_id": theirs, "quantity": 1})
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Defence in depth: a collected claim of the cook's own listing (as if it predated the guard)
	// still cannot be rated by them.
	buyer, _ := verifiedBuyer(e, "ownbuyer")
	id := paidClaim(t, e, buyer, own)
	exec(t, e, `UPDATE claims SET buyer_id = (SELECT id FROM buyer_profiles WHERE account_id=$2), pickup_status='collected' WHERE id=$1`, id, cook.AccountID)
	rec = e.Do(http.MethodPost, "/api/v1/claims/"+id+"/rating", cook.Token, map[string]any{"stars": 5})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "own_listing", testutil.JSON(t, rec)["error"])
	assert.Zero(t, count(t, e, `SELECT count(*) FROM claims WHERE id=$1 AND rating IS NOT NULL`, id))
}

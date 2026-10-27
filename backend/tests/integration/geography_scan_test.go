package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/tests/testutil"
)

// Existing 000001 queries select cook_profiles.* / listings.*; the geography columns must scan
// at runtime (ewkb.Point), both for a seeded point and for NULL.
func TestGeographyColumnsScan(t *testing.T) {
	e := testutil.NewEnv(t, false)
	ctx := context.Background()

	seedCook := pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0x40, 0, 0x80, 0, 0, 0, 0, 0xc, 0, 2}, Valid: true}
	cp, err := e.Q.GetCookProfileByID(ctx, seedCook)
	require.NoError(t, err)
	require.NotNil(t, cp.DefaultPickupLocation)
	assert.InDelta(t, 4.8895, cp.DefaultPickupLocation.X(), 1e-6)
	assert.InDelta(t, 52.3702, cp.DefaultPickupLocation.Y(), 1e-6)

	// A listing (non-null geography) and a profile with NULL location.
	var listingID pgtype.UUID
	require.NoError(t, e.Pool.QueryRow(ctx, `
		INSERT INTO listings (cook_id, food_description, allergen_info, total_portions, portions_remaining,
			reduced_price_cents, retail_reference_price_cents, pickup_location, pickup_address_text,
			pickup_window_start, pickup_window_end, status)
		VALUES ($1, 'scan test', 'none', 2, 2, 300, 900, ST_SetSRID(ST_MakePoint(4.9, 52.3), 4326)::geography,
			'Somewhere 1', $2, $3, 'draft') RETURNING id`,
		seedCook, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)).Scan(&listingID))
	t.Cleanup(func() { e.Pool.Exec(ctx, `DELETE FROM listings WHERE id=$1`, listingID) })

	l, err := e.Q.GetListingByID(ctx, listingID)
	require.NoError(t, err)
	assert.InDelta(t, 4.9, l.PickupLocation.X(), 1e-6)
	assert.InDelta(t, 52.3, l.PickupLocation.Y(), 1e-6)

	var nullLoc db.CookProfile
	_ = nullLoc
}

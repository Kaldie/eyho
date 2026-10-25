-- Listings queries for sqlc code generation.
-- All queries follow the sqlc annotation convention:
--   -- name: <QueryName> :<one|many|exec|execresult>

-- ============================================================
-- CreateListing
-- ============================================================
-- Inserts a new listing in 'draft' status.
-- portions_remaining is set equal to total_portions at creation.
-- The DB CHECK (pickup_window_end > pickup_window_start) enforces the Edge Case from data-model.md.
-- The DB CHECK (reduced_price_cents < retail_reference_price_cents) enforces FR-003.

-- name: CreateListing :one
INSERT INTO listings (
    cook_id,
    food_description,
    allergen_info,
    total_portions,
    portions_remaining,
    reduced_price_cents,
    retail_reference_price_cents,
    pickup_location,
    pickup_address_text,
    pickup_window_start,
    pickup_window_end,
    status
) VALUES (
    $1,                                                -- cook_id
    $2,                                                -- food_description
    $3,                                                -- allergen_info
    $4,                                                -- total_portions
    $4,                                                -- portions_remaining = total_portions initially
    $5,                                                -- reduced_price_cents
    $6,                                                -- retail_reference_price_cents
    ST_SetSRID(ST_MakePoint($7, $8), 4326)::geography, -- lng, lat -> geography point
    $9,                                                -- pickup_address_text
    $10,                                               -- pickup_window_start
    $11,                                               -- pickup_window_end
    $12                                                -- status ('draft' or 'published')
)
RETURNING *;

-- ============================================================
-- GetListingByID
-- ============================================================
-- Returns a single listing row plus the cook's current verification_status so the caller
-- can apply the "unverified" flag without a second query (FR-004).

-- name: GetListingByID :one
SELECT
    l.*,
    cp.verification_status AS cook_verification_status
FROM listings l
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE l.id = $1;

-- ============================================================
-- ListNearbyListings
-- ============================================================
-- Returns published listings within radius_meters of the given point, ordered by distance.
-- Uses ST_DWithin on geography columns (unit: metres) which leverages the GiST index
-- (idx_listings_pickup_location_gist) for sub-second performance at pilot scale (FR-011).
-- ST_Distance returns the geodetic distance in metres; divide by 1000 to expose distance_km
-- to the application layer.
--
-- Parameters:
--   $1 lng          FLOAT8  — longitude of the buyer's search origin
--   $2 lat          FLOAT8  — latitude of the buyer's search origin
--   $3 radius_m     FLOAT8  — search radius in metres (caller converts km -> m)
--   $4 limit        INT
--   $5 offset       INT

-- name: ListNearbyListings :many
SELECT
    l.*,
    cp.verification_status                        AS cook_verification_status,
    cp.org_type                                   AS cook_org_type,
    cp.rating_avg                                 AS cook_rating_avg,
    cp.rating_count                               AS cook_rating_count,
    ST_Distance(
        l.pickup_location,
        ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
    ) / 1000.0                                    AS distance_km
FROM listings l
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE
    l.status = 'published'
    AND ST_DWithin(
        l.pickup_location,
        ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
        $3   -- radius in metres
    )
ORDER BY
    -- Non-profits surface first within radius (Principle III: equal or preferential visibility).
    CASE WHEN cp.org_type = 'nonprofit' THEN 0 ELSE 1 END,
    distance_km
LIMIT  $4
OFFSET $5;

-- ============================================================
-- UpdateListingStatus
-- ============================================================
-- Used by the cook (publish / cancel) and the background expiry job (expire).
-- Returns the updated row so the caller can confirm the new state.

-- name: UpdateListingStatus :one
UPDATE listings
SET
    status     = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- ============================================================
-- DecrementPortionsAtomic
-- ============================================================
-- Atomically decrements portions_remaining by qty, but only when there are enough
-- portions available. The conditional WHERE clause and the DB CHECK constraint together
-- guarantee zero overselling (FR-006, SC-003, research.md concurrency decision).
--
-- Call this inside the same transaction as CreateClaim. If rows_affected == 0, the
-- caller MUST abort the transaction and return an "insufficient portions" error.
--
-- Parameters:
--   $1 listing_id   UUID
--   $2 qty          INT   — number of portions to reserve

-- name: DecrementPortionsAtomic :execresult
UPDATE listings
SET
    portions_remaining = portions_remaining - $2,
    updated_at         = NOW()
WHERE
    id                 = $1
    AND status         = 'published'
    AND portions_remaining >= $2;

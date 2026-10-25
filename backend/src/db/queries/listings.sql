-- Listings queries for sqlc code generation.
-- All queries follow the sqlc annotation convention:
--   -- name: <QueryName> :<one|many|exec|execresult>

-- ============================================================
-- CreateListing
-- ============================================================
-- Inserts a new listing as 'draft' or 'published' (the service decides; publishing is gated by
-- feature 001's CookCanPublish, never by verification_status). portions_remaining starts equal to
-- total_portions. DB CHECKs back up FR-003 (price below retail) and the window order.

-- name: CreateListing :one
INSERT INTO listings (
    cook_id, food_description, allergen_info, photo_url,
    total_portions, portions_remaining,
    reduced_price_cents, retail_reference_price_cents,
    pickup_location, pickup_address_text,
    pickup_window_start, pickup_window_end, status
) VALUES (
    sqlc.arg(cook_id), sqlc.arg(food_description), sqlc.arg(allergen_info), sqlc.narg(photo_url),
    sqlc.arg(total_portions), sqlc.arg(total_portions),
    sqlc.arg(reduced_price_cents), sqlc.arg(retail_reference_price_cents),
    ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography,
    sqlc.arg(pickup_address_text),
    sqlc.arg(pickup_window_start), sqlc.arg(pickup_window_end), sqlc.arg(status)
)
RETURNING *;

-- ============================================================
-- GetListingByID
-- ============================================================
-- One listing plus the cook block shown to buyers (FR-004): the cook's CURRENT verification status,
-- rating and cycle count are read live, so a demoted cook's published listings are re-flagged
-- immediately (spec Edge Case).

-- name: GetListingByID :one
SELECT
    l.*,
    cp.display_name                AS cook_display_name,
    cp.org_type                    AS cook_org_type,
    cp.verification_status         AS cook_verification_status,
    cp.rating_avg                  AS cook_rating_avg,
    cp.rating_count                AS cook_rating_count,
    cp.verification_cycle_count    AS cook_verification_cycle_count
FROM listings l
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE l.id = $1;

-- Row lock for read-modify-write by the cook (PATCH). Claim creation takes the same row lock via
-- DecrementPortionsAtomic, so the two serialise.
-- name: GetListingForUpdate :one
SELECT * FROM listings WHERE id = $1 FOR UPDATE;

-- ============================================================
-- ListNearbyListings
-- ============================================================
-- Published listings within radius_m of the search point whose pickup window has not ended
-- (the expiry job may lag up to a minute). Non-profits first, then distance (Principle III,
-- research.md "Discovery sort order"). ST_DWithin on geography uses the GiST index.

-- name: ListNearbyListings :many
SELECT
    l.*,
    cp.display_name                               AS cook_display_name,
    cp.verification_status                        AS cook_verification_status,
    cp.org_type                                   AS cook_org_type,
    cp.rating_avg                                 AS cook_rating_avg,
    cp.rating_count                               AS cook_rating_count,
    cp.verification_cycle_count                   AS cook_verification_cycle_count,
    (ST_Distance(
        l.pickup_location,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography
    ) / 1000.0)::float8                           AS distance_km
FROM listings l
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE
    l.status = 'published'
    AND l.pickup_window_end > NOW()
    AND ST_DWithin(
        l.pickup_location,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography,
        sqlc.arg(radius_m)::float8
    )
ORDER BY
    CASE WHEN cp.org_type = 'nonprofit' THEN 0 ELSE 1 END,
    distance_km,
    l.id
LIMIT sqlc.arg(row_limit)::int
OFFSET sqlc.arg(row_offset)::int;

-- ============================================================
-- UpdateListingStatus (kept from the scaffold; the service uses the guarded queries below)
-- ============================================================

-- name: UpdateListingStatus :one
UPDATE listings
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- ============================================================
-- DecrementPortionsAtomic
-- ============================================================
-- The atomic conditional UPDATE from research.md (FR-006, SC-003). Rows affected = 0 means the
-- listing is not published, its window has ended, or fewer than qty portions remain; the caller
-- aborts the transaction.

-- name: DecrementPortionsAtomic :execresult
UPDATE listings
SET
    portions_remaining = portions_remaining - $2,
    updated_at         = NOW()
WHERE
    id                 = $1
    AND status         = 'published'
    AND pickup_window_end > NOW()
    AND portions_remaining >= $2;

-- Gives reserved portions back (payment failed / timed out / reservation released). Capped at
-- total_portions so a manual adjustment by the cook can never push it past the CHECK.
-- name: ReturnPortions :exec
UPDATE listings
SET portions_remaining = LEAST(portions_remaining + sqlc.arg(qty)::int, total_portions),
    updated_at         = NOW()
WHERE id = sqlc.arg(id);

-- Portions held by live claims (unpaid reservation or paid), used to bound manual adjustments.
-- name: SumLiveClaimedQuantity :one
SELECT COALESCE(SUM(quantity), 0)::int AS claimed
FROM claims
WHERE listing_id = $1
  AND payment_status IN ('pending_payment', 'paid');

-- PATCH portions_remaining / pickup_window_end (service holds the row lock from GetListingForUpdate).
-- name: UpdateListingPortions :exec
UPDATE listings SET portions_remaining = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateListingWindowEnd :exec
UPDATE listings SET pickup_window_end = $2, updated_at = NOW() WHERE id = $1;

-- published -> cancelled only (data-model state machine).
-- name: CancelListing :one
UPDATE listings
SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND status = 'published'
RETURNING *;

-- Expiry job (FR-007): published -> expired once the window has passed.
-- name: ExpireListings :many
UPDATE listings
SET status = 'expired', updated_at = NOW()
WHERE status = 'published' AND pickup_window_end <= NOW()
RETURNING id;

-- draft -> published (PATCH status "published"). The service has already applied the registration
-- gate (CookCanPublish) and checked the window; verification_status plays no part (Principle I).
-- name: PublishListing :one
UPDATE listings
SET status = 'published', updated_at = NOW()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- Distance for GET /listings/{id}?lat&lng, same geodesic as discovery.
-- name: ListingDistanceKm :one
SELECT (ST_Distance(
    pickup_location,
    ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography
) / 1000.0)::float8 AS distance_km
FROM listings
WHERE id = sqlc.arg(id);

-- The cook's own listings in every status, newest first (GET /me/listings). portions_claimed is the
-- quantity held by live claims: unpaid reservations plus paid claims (refunded, failed and released
-- claims no longer hold portions).
-- name: ListListingsByCook :many
SELECT
    l.*,
    cp.display_name                AS cook_display_name,
    cp.org_type                    AS cook_org_type,
    cp.verification_status         AS cook_verification_status,
    cp.rating_avg                  AS cook_rating_avg,
    cp.rating_count                AS cook_rating_count,
    cp.verification_cycle_count    AS cook_verification_cycle_count,
    COALESCE((
        SELECT SUM(c.quantity) FROM claims c
        WHERE c.listing_id = l.id AND c.payment_status IN ('pending_payment', 'paid')
    ), 0)::int                     AS portions_claimed
FROM listings l
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE l.cook_id = $1
ORDER BY l.created_at DESC, l.id DESC
LIMIT 200;

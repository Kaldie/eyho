-- Claims queries for sqlc code generation.

-- ============================================================
-- CreateClaim
-- ============================================================
-- Inserts a new claim row in the initial state: payment_status='pending_payment',
-- pickup_status='pending'. Must be called inside the same transaction as
-- DecrementPortionsAtomic (see listings.sql) to satisfy the atomicity requirement
-- from FR-006 and SC-003.

-- name: CreateClaim :one
INSERT INTO claims (
    listing_id,
    buyer_id,
    quantity,
    payment_status,
    pickup_status,
    stripe_payment_intent_id,
    reserved_at
) VALUES (
    $1,               -- listing_id
    $2,               -- buyer_id
    $3,               -- quantity
    'pending_payment',
    'pending',
    $4,               -- stripe_payment_intent_id (nullable; pass NULL if not yet created)
    NOW()
)
RETURNING *;

-- ============================================================
-- GetClaimByID
-- ============================================================
-- Returns the claim row together with the listing's cook_id so the caller
-- can authorise the request without a separate query.

-- name: GetClaimByID :one
SELECT
    c.*,
    l.cook_id AS listing_cook_id
FROM claims c
JOIN listings l ON l.id = c.listing_id
WHERE c.id = $1;

-- ============================================================
-- UpdateClaimPaymentStatus
-- ============================================================
-- Transitions the payment_status of a claim. The confirmed_at column is set when
-- the new status is 'paid'; otherwise it is left unchanged (NULL coalesce preserves
-- the existing value).
--
-- Parameters:
--   $1 id               UUID
--   $2 payment_status   claim_payment_status
--   $3 confirmed_at     TIMESTAMPTZ  — pass NOW() on 'paid', NULL on other transitions

-- name: UpdateClaimPaymentStatus :one
UPDATE claims
SET
    payment_status = $2,
    confirmed_at   = COALESCE($3, confirmed_at),
    updated_at     = NOW()
WHERE id = $1
RETURNING *;

-- ============================================================
-- ReleasePendingClaims
-- ============================================================
-- Bulk-updates all claims that are still in 'pending_payment' and whose reservation
-- window has expired (reserved_at + timeout_interval). Called by the background job
-- described in research.md (runs every minute, default timeout 10 minutes).
--
-- This query only updates the claims table; the caller is responsible for returning
-- the reserved quantity to listings.portions_remaining for each returned claim row.
-- Returning the claim rows (listing_id, quantity) gives the caller the data it needs
-- for the subsequent inventory-release updates without a second SELECT.
--
-- Parameters:
--   $1 timeout_interval  INTERVAL  — e.g. '10 minutes'::interval

-- name: ReleasePendingClaims :many
UPDATE claims
SET
    payment_status = 'payment_failed',
    updated_at     = NOW()
WHERE
    payment_status = 'pending_payment'
    AND reserved_at < NOW() - $1::interval
RETURNING id, listing_id, quantity;

-- ============================================================
-- ListClaimsByBuyer
-- ============================================================
-- Buyer order history for GET /api/v1/claims, newest first. Failed payments are
-- omitted because they never became orders. Keyset pagination on created_at.
--
-- Parameters:
--   $1 buyer_id        UUID
--   $2 before          TIMESTAMPTZ  — cursor; pass NOW() for the first page
--   $3 limit           INT

-- name: ListClaimsByBuyer :many
SELECT
    c.*,
    l.food_description,
    l.photo_url,
    l.pickup_address_text,
    l.pickup_window_start,
    l.pickup_window_end,
    cp.display_name                    AS cook_display_name,
    l.reduced_price_cents * c.quantity AS amount_paid_cents
FROM claims c
JOIN listings l ON l.id = c.listing_id
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE
    c.buyer_id = $1
    AND c.payment_status <> 'payment_failed'
    AND c.created_at < $2
ORDER BY c.created_at DESC
LIMIT $3;

-- ============================================================
-- RateClaim
-- ============================================================
-- Records the buyer's rating (FR-013). The WHERE clause makes this a no-op
-- (zero rows) unless the claim is collected and not yet rated, so the service
-- can map an empty result to 409 not_collected / already_rated. The caller must
-- then update the cook's rating aggregates and evaluate verification thresholds
-- in the same transaction (see cook_profiles.sql).
--
-- Parameters:
--   $1 id      UUID
--   $2 rating  SMALLINT (1-5)

-- name: RateClaim :one
UPDATE claims
SET
    rating     = $2,
    rated_at   = NOW(),
    updated_at = NOW()
WHERE
    id = $1
    AND pickup_status = 'collected'
    AND rating IS NULL
RETURNING *;

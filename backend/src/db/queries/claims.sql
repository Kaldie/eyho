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
-- Buyer order history for GET /api/v1/claims, newest first. Failed payments are omitted because
-- they never became orders. Keyset pagination on (created_at, id): pass a far-future timestamp and
-- the max uuid for the first page; the service fetches limit+1 rows to build next_cursor.

-- name: ListClaimsByBuyer :many
SELECT
    c.*,
    l.food_description,
    l.photo_url,
    l.pickup_address_text,
    l.pickup_window_start,
    l.pickup_window_end,
    cp.display_name                            AS cook_display_name,
    (l.reduced_price_cents * c.quantity)::int  AS amount_paid_cents
FROM claims c
JOIN listings l ON l.id = c.listing_id
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE
    c.buyer_id = sqlc.arg(buyer_id)
    AND c.payment_status <> 'payment_failed'
    AND (c.created_at, c.id) < (sqlc.arg(before_ts)::timestamptz, sqlc.arg(before_id)::uuid)
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- Single claim in the same shape as a history item, plus the listing's cook for authorisation.
-- name: GetClaimViewByID :one
SELECT
    c.*,
    l.cook_id                                  AS listing_cook_id,
    l.food_description,
    l.photo_url,
    l.pickup_address_text,
    l.pickup_window_start,
    l.pickup_window_end,
    cp.display_name                            AS cook_display_name,
    (l.reduced_price_cents * c.quantity)::int  AS amount_paid_cents
FROM claims c
JOIN listings l ON l.id = c.listing_id
JOIN cook_profiles cp ON cp.id = l.cook_id
WHERE c.id = $1;

-- Stores the PaymentIntent id once the provider created it (after the reservation committed).
-- name: SetClaimPaymentIntent :exec
UPDATE claims SET stripe_payment_intent_id = $2, updated_at = NOW() WHERE id = $1;

-- pending_payment -> paid (payment webhook). Zero rows = already processed or no longer pending.
-- name: MarkClaimPaid :one
UPDATE claims
SET payment_status = 'paid', confirmed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND payment_status = 'pending_payment'
RETURNING *;

-- pending_payment -> payment_failed for one claim (payment failed webhook, or the intent could not
-- be created). The caller returns the quantity to the listing in the same transaction.
-- name: FailPendingClaim :one
UPDATE claims
SET payment_status = 'payment_failed', updated_at = NOW()
WHERE id = $1 AND payment_status = 'pending_payment'
RETURNING id, listing_id, quantity;

-- paid + pending -> collected (buyer or the listing's cook confirms, FR-012 window not required).
-- name: MarkClaimCollected :one
UPDATE claims
SET pickup_status = 'collected', updated_at = NOW()
WHERE id = $1 AND payment_status = 'paid' AND pickup_status = 'pending'
RETURNING *;

-- paid -> refunded (cook cancelled the listing, FR-010).
-- name: MarkClaimRefunded :one
UPDATE claims
SET payment_status = 'refunded', updated_at = NOW()
WHERE id = $1 AND payment_status = 'paid' AND pickup_status = 'pending'
RETURNING *;

-- Paid claims still awaiting pickup on cancelled listings: these must be refunded (FR-010). With a
-- listing id it serves the cancel request; without one it serves the retry sweep in the expiry job.
-- name: ListClaimsToRefund :many
SELECT c.id, c.buyer_id, c.listing_id, c.stripe_payment_intent_id
FROM claims c
JOIN listings l ON l.id = c.listing_id
WHERE l.status = 'cancelled'
  AND c.payment_status = 'paid'
  AND c.pickup_status = 'pending'
  AND (sqlc.narg(listing_id)::uuid IS NULL OR c.listing_id = sqlc.narg(listing_id)::uuid)
ORDER BY c.created_at
LIMIT 200;

-- Unpaid reservations of a listing that is being cancelled.
-- name: FailPendingClaimsForListing :many
UPDATE claims
SET payment_status = 'payment_failed', updated_at = NOW()
WHERE listing_id = $1 AND payment_status = 'pending_payment'
RETURNING id, buyer_id, quantity;

-- No-show (FR-012): paid claims not collected once the pickup window ended. Not refunded.
-- Claims on cancelled listings are excluded: they are refunded instead.
-- name: MarkNoShows :execrows
UPDATE claims c
SET pickup_status = 'no_show', updated_at = NOW()
FROM listings l
WHERE l.id = c.listing_id
  AND c.payment_status = 'paid'
  AND c.pickup_status = 'pending'
  AND l.status <> 'cancelled'
  AND l.pickup_window_end <= NOW();

-- Impact record (FR-009, Principle II): one per paid claim, created in the payment transaction.
-- ON CONFLICT keeps webhook redelivery idempotent.
-- name: CreateImpactRecord :exec
INSERT INTO impact_records (claim_id, listing_id, portions, estimated_weight_grams)
VALUES ($1, $2, $3, $4)
ON CONFLICT (claim_id) DO NOTHING;

-- Outbox row for the buyer notification (FR-010). Delivery channel is a later task.
-- name: CreateNotification :exec
INSERT INTO notifications (buyer_profile_id, claim_id, kind) VALUES ($1, $2, $3)
ON CONFLICT (claim_id, kind) DO NOTHING;

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
    rated_at   = clock_timestamp(),
    updated_at = NOW()
WHERE
    id = $1
    AND pickup_status = 'collected'
    AND rating IS NULL
RETURNING *;

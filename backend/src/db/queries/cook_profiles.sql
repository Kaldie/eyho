-- Cook profile queries for sqlc code generation.

-- ============================================================
-- GetCookProfileByID
-- ============================================================
-- Returns a cook profile by primary key. Callers use this to read verification_status
-- and include it in listing/profile API responses (FR-004).

-- name: GetCookProfileByID :one
SELECT *
FROM cook_profiles
WHERE id = $1;

-- ============================================================
-- UpdateVerificationStatus
-- ============================================================
-- Applies a verification status transition produced by the service layer after it
-- evaluates the promotion/demotion thresholds (5 good ratings -> verified;
-- 3 bad ratings -> unverified per Constitution Principle I).
--
-- The caller is responsible for:
--   1. Writing the corresponding VerificationHistory row (append-only; no query here).
--   2. Resetting the appropriate since-transition rating counters and incrementing
--      verification_cycle_count — both handled in this single UPDATE to keep the
--      profile state consistent within one transaction.
--
-- Parameters:
--   $1 id                       UUID
--   $2 new_status               cook_verification_status
--   $3 good_ratings_since_transition INT   — value after reset (typically 0)
--   $4 bad_ratings_since_transition  INT   — value after reset (typically 0)
--   $5 verification_cycle_count INT   — incremented value (old + 1)

-- name: UpdateVerificationStatus :one
UPDATE cook_profiles
SET
    verification_status         = $2,
    good_ratings_since_transition    = $3,
    bad_ratings_since_transition     = $4,
    verification_cycle_count    = $5,
    updated_at                  = NOW()
WHERE id = $1
RETURNING *;

-- ============================================================
-- Rating -> verification loop (feature 002, FR-013 / FR-014)
-- ============================================================

-- Serialises all ratings of one cook so counters, thresholds and the triggering-claim list stay
-- exact. The rating transaction takes this lock BEFORE writing the rating.
-- name: LockCookProfile :one
SELECT * FROM cook_profiles WHERE id = $1 FOR UPDATE;

-- Recomputes the denormalised aggregate from the claims themselves.
-- name: AggregateCookRatings :one
SELECT
    COALESCE(ROUND(AVG(c.rating)::numeric, 2), 0)::numeric(3, 2) AS rating_avg,
    COUNT(c.rating)::int                                         AS rating_count
FROM claims c
JOIN listings l ON l.id = c.listing_id
WHERE l.cook_id = $1 AND c.rating IS NOT NULL;

-- name: UpdateCookRatings :exec
UPDATE cook_profiles
SET rating_avg                    = $2,
    rating_count                  = $3,
    good_ratings_since_transition = $4,
    bad_ratings_since_transition  = $5,
    updated_at                    = NOW()
WHERE id = $1;

-- name: InsertVerificationHistory :exec
INSERT INTO verification_history (cook_id, from_status, to_status, triggering_claim_ids)
VALUES ($1, $2, $3, $4);

-- The latest N ratings of >= 4 stars (promotion) / <= 2 stars (demotion). Because counters reset on
-- every transition and ratings of one cook are serialised, the latest N matching ratings are
-- exactly the N counted since the last transition.
-- name: ListLatestGoodRatedClaimIDs :many
SELECT c.id
FROM claims c JOIN listings l ON l.id = c.listing_id
WHERE l.cook_id = sqlc.arg(cook_id) AND c.rating >= 4
ORDER BY c.rated_at DESC, c.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: ListLatestBadRatedClaimIDs :many
SELECT c.id
FROM claims c JOIN listings l ON l.id = c.listing_id
WHERE l.cook_id = sqlc.arg(cook_id) AND c.rating <= 2
ORDER BY c.rated_at DESC, c.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: ListVerificationHistory :many
SELECT * FROM verification_history WHERE cook_id = $1 ORDER BY created_at, id;

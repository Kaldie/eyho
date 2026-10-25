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
--   2. Resetting the appropriate consecutive-ratings counter and incrementing
--      verification_cycle_count — both handled in this single UPDATE to keep the
--      profile state consistent within one transaction.
--
-- Parameters:
--   $1 id                       UUID
--   $2 new_status               cook_verification_status
--   $3 consecutive_good_ratings INT   — value after reset (typically 0)
--   $4 consecutive_bad_ratings  INT   — value after reset (typically 0)
--   $5 verification_cycle_count INT   — incremented value (old + 1)

-- name: UpdateVerificationStatus :one
UPDATE cook_profiles
SET
    verification_status         = $2,
    consecutive_good_ratings    = $3,
    consecutive_bad_ratings     = $4,
    verification_cycle_count    = $5,
    updated_at                  = NOW()
WHERE id = $1
RETURNING *;

-- Accounts & sign-in queries (feature 001) for sqlc code generation.
-- Schema: schema.sql. Data model: specs/001-accounts-sign-in/data-model.md.
--
-- Cook profile queries select explicit columns and expose the pickup point as
-- default_pickup_lat / default_pickup_lng, so the geography column never has to be scanned into
-- a Go type. Pickup coordinates are used only for discovery defaults (constitution, Product
-- Constraints).

-- ============================================================
-- Accounts and identities
-- ============================================================

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- Resolves a token's (iss, sub) to its account. Deleted accounts have no identities left, so a
-- match is always a live account; callers should still check deleted_at.
-- name: GetAccountByIdentity :one
SELECT a.*
FROM accounts a
JOIN account_identities i ON i.account_id = a.id
WHERE i.issuer = $1 AND i.subject = $2;

-- FR-007 linking: only a live account whose own email is verified can be linked to. The caller
-- must also require that the token's email_verified is true.
-- name: GetAccountByVerifiedEmail :one
SELECT *
FROM accounts
WHERE email = $1
  AND email_verified
  AND deleted_at IS NULL;

-- Same lookup inside a transaction, locking the owner's row FOR SHARE so a concurrent account
-- deletion (which locks the row FOR UPDATE) cannot complete between this read and the identity
-- insert that follows: the deletion either waits for the link, or the link sees deleted_at.
-- name: GetAccountByVerifiedEmailForShare :one
SELECT *
FROM accounts
WHERE email = $1
  AND email_verified
  AND deleted_at IS NULL
FOR SHARE;

-- Locks the account row for the duration of a deletion transaction.
-- name: GetAccountForUpdate :one
SELECT * FROM accounts WHERE id = $1 FOR UPDATE;

-- Locks the buyer profile row (claims reference it, so their inserts wait for the deletion).
-- name: LockBuyerProfileForAccount :one
SELECT id FROM buyer_profiles WHERE account_id = $1 FOR UPDATE;

-- name: CreateAccount :one
INSERT INTO accounts (email, email_verified, display_name, is_demo)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateIdentity :one
INSERT INTO account_identities (account_id, issuer, subject)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateAccountEmailVerified :exec
UPDATE accounts
SET email_verified = $2,
    updated_at     = NOW()
WHERE id = $1
  AND deleted_at IS NULL
  AND email_verified IS DISTINCT FROM $2;

-- PATCH /me
-- name: UpdateAccountDisplayName :one
UPDATE accounts
SET display_name = $2,
    updated_at   = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- ============================================================
-- Buyer profiles
-- ============================================================

-- name: CreateBuyerProfile :one
INSERT INTO buyer_profiles (account_id)
VALUES ($1)
RETURNING *;

-- name: GetBuyerProfileByAccount :one
SELECT * FROM buyer_profiles WHERE account_id = $1;

-- ============================================================
-- Cook profiles
-- ============================================================

-- name: GetCookProfileByAccount :one
SELECT
    id,
    account_id,
    display_name,
    org_type,
    verification_status,
    phone,
    default_pickup_address_text,
    ST_Y(default_pickup_location::geometry)::float8 AS default_pickup_lat,
    ST_X(default_pickup_location::geometry)::float8 AS default_pickup_lng,
    kvk_number,
    kvk_trade_name,
    registration_status,
    registration_checked_at,
    nvwa_declared_at,
    nvwa_declaration_version,
    anonymised_at,
    created_at,
    updated_at
FROM cook_profiles
WHERE account_id = $1;

-- POST /me/cook-profile. New profiles start unverified (column default). Pass the KvK check
-- outcome: 'active' with the trade name and checked_at on success, or 'pending' with NULLs when
-- the register was unreachable (202). Returns the id; read the full row with
-- GetCookProfileByAccount.
-- name: CreateCookProfile :one
INSERT INTO cook_profiles (
    account_id,
    display_name,
    org_type,
    phone,
    default_pickup_address_text,
    default_pickup_location,
    kvk_number,
    kvk_trade_name,
    registration_status,
    registration_checked_at,
    nvwa_declared_at,
    nvwa_declaration_version
) VALUES (
    sqlc.arg(account_id),
    sqlc.arg(display_name),
    sqlc.arg(org_type),
    sqlc.arg(phone),
    sqlc.arg(default_pickup_address_text),
    ST_SetSRID(ST_MakePoint(sqlc.arg(pickup_lng)::float8, sqlc.arg(pickup_lat)::float8), 4326)::geography,
    sqlc.arg(kvk_number),
    sqlc.narg(kvk_trade_name),
    sqlc.arg(registration_status),
    sqlc.narg(registration_checked_at),
    sqlc.arg(nvwa_declared_at),
    sqlc.arg(nvwa_declaration_version)
)
RETURNING id;

-- PATCH /me/cook-profile for everything except the KvK number (see UpdateCookKvkRegistration)
-- and the declaration (cannot be withdrawn). NULL arguments leave the column unchanged. The
-- latitude and longitude must be passed together. Never touches anonymised profiles.
-- name: UpdateCookProfile :execrows
UPDATE cook_profiles
SET display_name                = COALESCE(sqlc.narg(display_name), display_name),
    org_type                    = COALESCE(sqlc.narg(org_type), org_type),
    phone                       = COALESCE(sqlc.narg(phone), phone),
    default_pickup_address_text = COALESCE(sqlc.narg(default_pickup_address_text), default_pickup_address_text),
    default_pickup_location     = CASE
        WHEN sqlc.narg(pickup_lng)::float8 IS NOT NULL AND sqlc.narg(pickup_lat)::float8 IS NOT NULL
        THEN ST_SetSRID(ST_MakePoint(sqlc.narg(pickup_lng)::float8, sqlc.narg(pickup_lat)::float8), 4326)::geography
        ELSE default_pickup_location
    END,
    updated_at                  = NOW()
WHERE id = sqlc.arg(id)
  AND anonymised_at IS NULL;

-- A changed KvK number after a definite check result ('active' or 'inactive'), or 'pending'
-- with NULLs when the register was unreachable. Rejected numbers (not found / inactive on PATCH)
-- must not call this: the old number stays in effect.
-- name: UpdateCookKvkRegistration :execrows
UPDATE cook_profiles
SET kvk_number              = sqlc.arg(kvk_number),
    kvk_trade_name          = sqlc.narg(kvk_trade_name),
    registration_status     = sqlc.arg(registration_status),
    registration_checked_at = sqlc.narg(registration_checked_at),
    updated_at              = NOW()
WHERE id = sqlc.arg(id)
  AND anonymised_at IS NULL;

-- Result of a (re-)check by the background job (FR-021). Only call with a definite answer
-- ('active' or 'inactive'); a transport error must not call this, so an active profile is never
-- downgraded by an outage. A NULL trade name keeps the stored one. Guarded by the kvk_number
-- the check was run for, so a concurrent PATCH of the number is not overwritten.
-- name: UpdateCookRegistrationResult :execrows
UPDATE cook_profiles
SET registration_status     = sqlc.arg(registration_status),
    kvk_trade_name          = COALESCE(sqlc.narg(kvk_trade_name), kvk_trade_name),
    registration_checked_at = sqlc.arg(registration_checked_at),
    updated_at              = NOW()
WHERE id = sqlc.arg(id)
  AND kvk_number = sqlc.arg(kvk_number)
  AND anonymised_at IS NULL;

-- Profiles the re-check job must visit: all 'pending' ones (the job applies its own backoff),
-- 'inactive' ones never checked or last checked before inactive_stale_before (now - 1 day, so a
-- re-activated business is picked up quickly), and 'active' ones never checked or last checked
-- before stale_before (now - 30 days). Oldest check first.
-- name: ListCookProfilesDueForRecheck :many
SELECT id, kvk_number, registration_status, registration_checked_at
FROM cook_profiles
WHERE anonymised_at IS NULL
  AND kvk_number IS NOT NULL
  AND (
        registration_status = 'pending'
        OR registration_checked_at IS NULL
        OR (registration_status = 'inactive'
            AND registration_checked_at < sqlc.arg(inactive_stale_before)::timestamptz)
        OR registration_checked_at < sqlc.arg(stale_before)::timestamptz
      )
ORDER BY registration_checked_at ASC NULLS FIRST
LIMIT sqlc.arg(max_rows)::int;

-- AccountOwnsCookProfile (FR-011 own-listing guard).
-- name: AccountOwnsCookProfile :one
SELECT EXISTS (
    SELECT 1 FROM cook_profiles WHERE id = $1 AND account_id = $2
) AS owns;

-- CookCanPublish (FR-020): active registration and a recorded NVWA declaration. Independent of
-- verification_status.
-- name: CookCanPublish :one
SELECT EXISTS (
    SELECT 1
    FROM cook_profiles
    WHERE id = $1
      AND registration_status = 'active'
      AND nvwa_declared_at IS NOT NULL
      AND anonymised_at IS NULL
) AS can_publish;

-- ============================================================
-- Account deletion (FR-013) -- run these in one transaction
-- ============================================================

-- Blocking check: claims that are still live (unpaid reservation, or paid and not yet picked up).
-- name: CountActiveClaimsForBuyer :one
SELECT COUNT(*)::int AS active_claims
FROM claims
WHERE buyer_id = $1
  AND (
        payment_status = 'pending_payment'
        OR (payment_status = 'paid' AND pickup_status = 'pending')
      );

-- Blocking check: published listings of the account's cook profile.
-- name: CountActiveListingsForCook :one
SELECT COUNT(*)::int AS active_listings
FROM listings
WHERE cook_id = $1
  AND status = 'published';

-- Clears every personal field and keeps the row so history, ratings and listings stay linked.
-- account_id is cleared too (the profile is no longer reachable from any account).
-- name: AnonymiseCookProfile :execrows
UPDATE cook_profiles
SET display_name                = 'Former cook',
    phone                       = NULL,
    default_pickup_address_text = NULL,
    default_pickup_location     = NULL,
    kvk_number                  = NULL,
    kvk_trade_name              = NULL,
    account_id                  = NULL,
    anonymised_at               = NOW(),
    updated_at                  = NOW()
WHERE account_id = $1
  AND anonymised_at IS NULL;

-- Removes the identities and returns them so the caller can enqueue provider deletion
-- (skipped for the mock and demo issuers).
-- name: DeleteIdentitiesByAccount :many
DELETE FROM account_identities
WHERE account_id = $1
RETURNING issuer, subject;

-- Nulls the personal fields and marks the account deleted.
-- name: AnonymiseAccount :execrows
UPDATE accounts
SET email          = NULL,
    email_verified = FALSE,
    display_name   = NULL,
    deleted_at     = NOW(),
    updated_at     = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- ============================================================
-- Deleted identity tombstones
-- ============================================================

-- Remembers that this provider identity belonged to a deleted account. Re-deleting a later
-- account with the same identity moves deleted_at forward.
-- name: InsertDeletedIdentity :exec
INSERT INTO deleted_identities (issuer, subject)
VALUES ($1, $2)
ON CONFLICT (issuer, subject) DO UPDATE SET deleted_at = NOW();

-- name: GetDeletedIdentity :one
SELECT deleted_at FROM deleted_identities WHERE issuer = $1 AND subject = $2;

-- Drops tombstones older than the longest access token lifetime.
-- name: PurgeDeletedIdentities :execrows
DELETE FROM deleted_identities WHERE deleted_at < sqlc.arg(older_than)::timestamptz;

-- ============================================================
-- Provider deletion queue
-- ============================================================

-- name: EnqueueIdpDeletion :exec
INSERT INTO idp_deletion_requests (issuer, subject)
VALUES ($1, $2)
ON CONFLICT (issuer, subject) DO NOTHING;

-- Claims up to max_rows due requests for this worker. Rows are locked with SKIP LOCKED and their
-- next_attempt_at is pushed out by lease_seconds, so a crashed worker's rows come back later and
-- concurrent workers never take the same row. attempts is incremented on claim.
-- name: ClaimDueIdpDeletions :many
UPDATE idp_deletion_requests
SET attempts        = attempts + 1,
    next_attempt_at = NOW() + make_interval(secs => sqlc.arg(lease_seconds)::float8)
WHERE id IN (
    SELECT d.id
    FROM idp_deletion_requests d
    WHERE d.next_attempt_at <= NOW()
    ORDER BY d.next_attempt_at
    LIMIT sqlc.arg(max_rows)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- After a failed provider call: schedule the next try (the worker computes the backoff).
-- name: RescheduleIdpDeletion :exec
UPDATE idp_deletion_requests
SET next_attempt_at = NOW() + make_interval(secs => sqlc.arg(backoff_seconds)::float8)
WHERE id = sqlc.arg(id);

-- Provider confirmed deletion (or user already gone): drop the row.
-- name: CompleteIdpDeletion :exec
DELETE FROM idp_deletion_requests WHERE id = $1;

-- ============================================================
-- Demo accounts (FR-016)
-- ============================================================

-- Seeded demo accounts with the identity the demo issuer signs tokens for. subject is the demo
-- key ('buyer', 'cook_verified', 'cook_unverified'); see seed/demo_accounts.sql.
-- name: ListDemoAccounts :many
SELECT
    a.id           AS account_id,
    a.display_name,
    i.issuer,
    i.subject      AS demo_key
FROM accounts a
JOIN account_identities i ON i.account_id = a.id
WHERE a.is_demo
  AND a.deleted_at IS NULL
ORDER BY a.created_at, a.id;

-- ============================================================
-- Added by the backend engineer (feature 001, T009 / T028)
-- ============================================================

-- Blocking check for account deletion (FR-013): listings of the cook that still have live claims
-- (unpaid reservation, or paid and awaiting pickup), whatever the listing's own status.
-- name: CountListingsWithLiveClaimsForCook :one
SELECT COUNT(DISTINCT l.id)::int AS active_listings
FROM listings l
JOIN claims c ON c.listing_id = l.id
WHERE l.cook_id = $1
  AND (
        c.payment_status = 'pending_payment'
        OR (c.payment_status = 'paid' AND c.pickup_status = 'pending')
      );

-- On deletion, published listings without live claims are cancelled (published -> cancelled is a
-- documented transition) so no listing stays claimable under a "Former cook".
-- name: CancelPublishedListingsForCook :execrows
UPDATE listings
SET status     = 'cancelled',
    updated_at = NOW()
WHERE cook_id = $1
  AND status = 'published';

-- On deletion the cook's pickup addresses must not outlive the profile: every listing of the
-- cook (whatever its status) loses its address text and keeps only a ~1 km coarsened point
-- (coordinates rounded to 2 decimals) so historical listings stay on the map at area level.
-- name: ScrubListingPickupForCook :execrows
UPDATE listings
SET pickup_address_text = 'Address removed',
    pickup_location     = ST_SetSRID(ST_MakePoint(
                              ROUND(ST_X(pickup_location::geometry)::numeric, 2)::float8,
                              ROUND(ST_Y(pickup_location::geometry)::numeric, 2)::float8), 4326)::geography,
    updated_at          = NOW()
WHERE cook_id = $1;

-- An account with no cook profile and no claims can be merged away without losing history.
-- name: AccountIsEmpty :one
SELECT (
    NOT EXISTS (SELECT 1 FROM cook_profiles cp WHERE cp.account_id = sqlc.arg(acct_id))
    AND NOT EXISTS (
        SELECT 1
        FROM claims c
        JOIN buyer_profiles b ON b.id = c.buyer_id
        WHERE b.account_id = sqlc.arg(acct_id)
    )
) AS is_empty;

-- name: MoveIdentities :execrows
UPDATE account_identities
SET account_id = sqlc.arg(to_account)
WHERE account_id = sqlc.arg(from_account);

-- name: DeleteBuyerProfileByAccount :exec
DELETE FROM buyer_profiles WHERE account_id = $1;

-- name: DeleteAccountRow :exec
DELETE FROM accounts WHERE id = $1;

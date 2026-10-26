-- Feature 001: Accounts & Sign-in. See specs/001-accounts-sign-in/data-model.md.
--
-- DEPLOY NOTES
-- * Takes brief ACCESS EXCLUSIVE locks on buyer_profiles and cook_profiles (ALTER TABLE). Both
--   tables are tiny at pilot scale; no table rewrite happens (all new columns are nullable or have
--   constant defaults), so lock time is milliseconds.
-- * buyer_profiles.account_id is NOT NULL. Pre-account buyer profiles cannot be mapped to an
--   account, so the migration aborts if any exist (none are expected: the app had no sign-in).
-- * The cook "required while not anonymised" check is added NOT VALID, so pre-existing cook rows
--   (which lack the new onboarding fields) do not block the migration; every new or updated row
--   is checked.

CREATE EXTENSION IF NOT EXISTS citext;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM buyer_profiles) THEN
        RAISE EXCEPTION 'buyer_profiles contains rows without an account; clear them before migrating';
    END IF;
END $$;

CREATE TYPE cook_registration_status AS ENUM ('pending', 'active', 'inactive');

-- ============================================================
-- accounts
-- ============================================================

CREATE TABLE accounts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email           CITEXT,
    email_verified  BOOLEAN     NOT NULL DEFAULT FALSE,
    display_name    TEXT,
    is_demo         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,

    -- Email is required while the account exists; deletion nulls it.
    CONSTRAINT chk_account_email_required_unless_deleted
        CHECK (deleted_at IS NOT NULL OR email IS NOT NULL),
    -- Deleted accounts hold no personal data.
    CONSTRAINT chk_account_deleted_has_no_pii
        CHECK (deleted_at IS NULL OR (email IS NULL AND display_name IS NULL))
);

-- Unique email among non-deleted accounts (case-insensitive via citext).
CREATE UNIQUE INDEX uq_accounts_email_active
    ON accounts (email)
    WHERE deleted_at IS NULL;

-- ============================================================
-- account_identities
-- ============================================================

CREATE TABLE account_identities (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id  UUID        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    issuer      TEXT        NOT NULL,
    subject     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_account_identities_issuer_subject UNIQUE (issuer, subject)
);

CREATE INDEX idx_account_identities_account_id ON account_identities (account_id);

-- ============================================================
-- idp_deletion_requests
-- ============================================================
-- Queue of provider users to delete after local account deletion. Rows are removed once the
-- provider confirms. Holds the provider subject only until deletion completes.

CREATE TABLE idp_deletion_requests (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer           TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    attempts         INTEGER     NOT NULL DEFAULT 0
        CONSTRAINT chk_idp_deletion_attempts_non_negative CHECK (attempts >= 0),
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Enqueueing twice for the same provider user is a no-op.
    CONSTRAINT uq_idp_deletion_requests_issuer_subject UNIQUE (issuer, subject)
);

CREATE INDEX idx_idp_deletion_requests_next_attempt
    ON idp_deletion_requests (next_attempt_at);

-- ============================================================
-- buyer_profiles.account_id
-- ============================================================
-- RESTRICT: accounts are soft-deleted (deleted_at); claims keep pointing at the buyer profile.

ALTER TABLE buyer_profiles
    ADD COLUMN account_id UUID NOT NULL
        CONSTRAINT fk_buyer_profiles_account REFERENCES accounts (id) ON DELETE RESTRICT,
    ADD CONSTRAINT uq_buyer_profiles_account_id UNIQUE (account_id);

-- ============================================================
-- cook_profiles: account link, onboarding fields, registration check
-- ============================================================
-- account_id is nullable: SET NULL keeps the anonymised profile (and its history, ratings and
-- listings) if an account row is ever hard-deleted.

ALTER TABLE cook_profiles
    ADD COLUMN account_id                   UUID
        CONSTRAINT fk_cook_profiles_account REFERENCES accounts (id) ON DELETE SET NULL,
    ADD COLUMN phone                        TEXT,
    ADD COLUMN default_pickup_address_text  TEXT,
    ADD COLUMN default_pickup_location      GEOGRAPHY(Point, 4326),
    ADD COLUMN anonymised_at                TIMESTAMPTZ,
    ADD COLUMN kvk_number                   TEXT,
    ADD COLUMN kvk_trade_name               TEXT,
    ADD COLUMN registration_status          cook_registration_status NOT NULL DEFAULT 'pending',
    ADD COLUMN registration_checked_at      TIMESTAMPTZ,
    ADD COLUMN nvwa_declared_at             TIMESTAMPTZ,
    ADD COLUMN nvwa_declaration_version     TEXT,
    ADD CONSTRAINT uq_cook_profiles_account_id UNIQUE (account_id),
    ADD CONSTRAINT chk_cook_kvk_number_format
        CHECK (kvk_number IS NULL OR kvk_number ~ '^[0-9]{8}$'),
    -- Anonymised profiles hold none of the personal fields.
    ADD CONSTRAINT chk_cook_anonymised_has_no_pii
        CHECK (
            anonymised_at IS NULL
            OR (
                phone IS NULL
                AND default_pickup_address_text IS NULL
                AND default_pickup_location IS NULL
                AND kvk_number IS NULL
                AND kvk_trade_name IS NULL
            )
        );

-- NOT VALID: see deploy notes. Enforced for all new and updated rows.
ALTER TABLE cook_profiles
    ADD CONSTRAINT chk_cook_onboarding_fields_required
        CHECK (
            anonymised_at IS NOT NULL
            OR (
                phone IS NOT NULL
                AND default_pickup_address_text IS NOT NULL
                AND default_pickup_location IS NOT NULL
                AND kvk_number IS NOT NULL
                AND nvwa_declared_at IS NOT NULL
                AND nvwa_declaration_version IS NOT NULL
            )
        ) NOT VALID;

-- Supports the registration re-check job (FR-021): pending retries and 30-day re-checks.
CREATE INDEX idx_cook_profiles_registration_recheck
    ON cook_profiles (registration_status, registration_checked_at)
    WHERE anonymised_at IS NULL;

-- Feature 001: scope the unique email index to VERIFIED emails.
--
-- data-model.md said "unique among non-deleted accounts", but quickstart Scenario 4 step 4 requires
-- that a sign-in with the same email but an UNVERIFIED claim creates a separate account (an
-- unverified email must never link, FR-007) -- which a plain unique index forbids. The invariant
-- that matters is: at most one live account owns a given VERIFIED email.
DROP INDEX uq_accounts_email_active;

CREATE UNIQUE INDEX uq_accounts_email_verified_active
    ON accounts (email)
    WHERE deleted_at IS NULL AND email_verified;

CREATE INDEX idx_accounts_email_active
    ON accounts (email)
    WHERE deleted_at IS NULL;

-- Fails if two live accounts share an email (possible once unverified duplicates exist).
DROP INDEX IF EXISTS idx_accounts_email_active;
DROP INDEX IF EXISTS uq_accounts_email_verified_active;

CREATE UNIQUE INDEX uq_accounts_email_active
    ON accounts (email)
    WHERE deleted_at IS NULL;

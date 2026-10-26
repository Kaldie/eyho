-- Reverses 000002_accounts. Destroys all accounts, identities and cook onboarding data.
-- buyer_profiles rows cannot exist without an account, so they are removed (and with them
-- nothing else: claims reference buyer_profiles with ON DELETE RESTRICT, so this fails loudly if
-- any claims exist, which is intended).

DROP INDEX IF EXISTS idx_cook_profiles_registration_recheck;

ALTER TABLE cook_profiles
    DROP CONSTRAINT IF EXISTS chk_cook_onboarding_fields_required,
    DROP CONSTRAINT IF EXISTS chk_cook_anonymised_has_no_pii,
    DROP CONSTRAINT IF EXISTS chk_cook_kvk_number_format,
    DROP CONSTRAINT IF EXISTS uq_cook_profiles_account_id,
    DROP COLUMN IF EXISTS nvwa_declaration_version,
    DROP COLUMN IF EXISTS nvwa_declared_at,
    DROP COLUMN IF EXISTS registration_checked_at,
    DROP COLUMN IF EXISTS registration_status,
    DROP COLUMN IF EXISTS kvk_trade_name,
    DROP COLUMN IF EXISTS kvk_number,
    DROP COLUMN IF EXISTS anonymised_at,
    DROP COLUMN IF EXISTS default_pickup_location,
    DROP COLUMN IF EXISTS default_pickup_address_text,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS account_id;

DELETE FROM buyer_profiles;
ALTER TABLE buyer_profiles
    DROP CONSTRAINT IF EXISTS uq_buyer_profiles_account_id,
    DROP COLUMN IF EXISTS account_id;

DROP TABLE IF EXISTS idp_deletion_requests;
DROP TABLE IF EXISTS account_identities;
DROP TABLE IF EXISTS accounts;

DROP TYPE IF EXISTS cook_registration_status;
-- citext is left installed (shared extension).

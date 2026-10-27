-- Demo accounts (feature 001, T033, FR-016). Local and demo environments ONLY: never run this
-- against production (accounts are flagged is_demo = true and sign in without a password).
-- Loaded after schema.sql when the local database is first created; idempotent, so safe to
-- run again by hand.
--
-- FAKE BUSINESS REGISTER TEST NUMBERS (backend/src/registry/fake.go must match; KVK_MODE=fake)
--   Active   : 68750110   -> demo cook (verified)    trade name "Keuken Katrien B.V."
--              69599084   -> demo cook (unverified)  trade name "Test Eetcafe Jordaan"
--   Inactive : 90000002   -> registration ended; onboarding must answer kvk_inactive
--   Not found: any other 8-digit number, for example 12345678 -> kvk_not_found
-- 68750110 and 69599084 are recalled from KVK's public test set but NOT confirmed against KVK's
-- documentation or API (no network access to it when this was written). T038 / research.md
-- "to confirm in implementation" must verify them against the kvk-test environment (the
-- inactive number is fictitious to this project; KVK's test set may have its own). Seed rows
-- store these numbers and trade names directly, so if kvk-test disagrees only this header,
-- the fake register and the two cook rows below need to change together.
--
-- DEMO IDENTITIES: each account has one identity with issuer 'eyho-demo' and subject equal to
-- its demo key ('buyer', 'cook_verified', 'cook_unverified'). The backend's demo issuer (T032)
-- must mint tokens with iss = 'eyho-demo' and sub = <key>. Because identities are keyed this
-- way, ListDemoAccounts (queries/accounts.sql) returns the key as demo_key.

BEGIN;

-- ------------------------------------------------------------
-- Accounts
-- ------------------------------------------------------------
INSERT INTO accounts (id, email, email_verified, display_name, is_demo) VALUES
    ('00000000-0000-4000-8000-00000000d001', 'demo-buyer@eyho.example',           TRUE, 'Demo buyer',           TRUE),
    ('00000000-0000-4000-8000-00000000d002', 'demo-cook-verified@eyho.example',   TRUE, 'Demo cook (verified)',   TRUE),
    ('00000000-0000-4000-8000-00000000d003', 'demo-cook-unverified@eyho.example', TRUE, 'Demo cook (unverified)', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO account_identities (id, account_id, issuer, subject) VALUES
    ('00000000-0000-4000-8000-0000000d1001', '00000000-0000-4000-8000-00000000d001', 'eyho-demo', 'buyer'),
    ('00000000-0000-4000-8000-0000000d1002', '00000000-0000-4000-8000-00000000d002', 'eyho-demo', 'cook_verified'),
    ('00000000-0000-4000-8000-0000000d1003', '00000000-0000-4000-8000-00000000d003', 'eyho-demo', 'cook_unverified')
ON CONFLICT (issuer, subject) DO NOTHING;

-- Every account has a buyer profile (created on first sign-in in normal use). The cooks can buy too.
INSERT INTO buyer_profiles (id, account_id) VALUES
    ('00000000-0000-4000-8000-0000000b0001', '00000000-0000-4000-8000-00000000d001'),
    ('00000000-0000-4000-8000-0000000b0002', '00000000-0000-4000-8000-00000000d002'),
    ('00000000-0000-4000-8000-0000000b0003', '00000000-0000-4000-8000-00000000d003')
ON CONFLICT (account_id) DO NOTHING;

-- ------------------------------------------------------------
-- Cook profiles (Amsterdam). Registration is active with a recorded declaration, so both can
-- publish (FR-020). verification_status is independent of registration (Principle I).
-- ------------------------------------------------------------
INSERT INTO cook_profiles (
    id, account_id, display_name, org_type, verification_status, verification_cycle_count,
    phone, default_pickup_address_text, default_pickup_location,
    kvk_number, kvk_trade_name, registration_status, registration_checked_at,
    nvwa_declared_at, nvwa_declaration_version
) VALUES
    ('00000000-0000-4000-8000-0000000c0002', '00000000-0000-4000-8000-00000000d002',
     'Keuken Katrien', 'professional', 'verified', 1,
     '+31600000002', 'Keizersgracht 88, 1015 CT Amsterdam',
     ST_SetSRID(ST_MakePoint(4.8895, 52.3702), 4326)::geography,
     '68750110', 'Keuken Katrien B.V.', 'active', NOW(),
     NOW(), '2026-10-25'),
    ('00000000-0000-4000-8000-0000000c0003', '00000000-0000-4000-8000-00000000d003',
     'Buurtkeuken Jordaan', 'nonprofit', 'unverified', 0,
     '+31600000003', 'Egelantiersgracht 40, 1015 RE Amsterdam',
     ST_SetSRID(ST_MakePoint(4.8800, 52.3744), 4326)::geography,
     '69599084', 'Test Eetcafe Jordaan', 'active', NOW(),
     NOW(), '2026-10-25')
ON CONFLICT (account_id) DO NOTHING;

-- The verified cook earned its status: record the transition (append-only history, Principle I).
-- No real ratings back this demo transition, so triggering_claim_ids is empty.
INSERT INTO verification_history (id, cook_id, from_status, to_status, triggering_claim_ids)
VALUES ('00000000-0000-4000-8000-0000000e0002', '00000000-0000-4000-8000-0000000c0002',
        'unverified', 'verified', '{}')
ON CONFLICT (id) DO NOTHING;

COMMIT;

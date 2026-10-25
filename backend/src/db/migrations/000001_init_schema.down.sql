-- Drop tables in reverse dependency order to avoid FK violations.

DROP TABLE IF EXISTS impact_records;
DROP TABLE IF EXISTS claims;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS buyer_profiles;
DROP TABLE IF EXISTS verification_history;
DROP TABLE IF EXISTS cook_profiles;

-- Drop enums after the tables that reference them are gone.
DROP TYPE IF EXISTS claim_pickup_status;
DROP TYPE IF EXISTS claim_payment_status;
DROP TYPE IF EXISTS listing_status;
DROP TYPE IF EXISTS cook_verification_status;
DROP TYPE IF EXISTS cook_org_type;

-- Drop the PostGIS extension last. Use CASCADE only if no other objects depend on it.
-- In production, prefer leaving the extension in place and only dropping application objects.
-- DROP EXTENSION IF EXISTS postgis;

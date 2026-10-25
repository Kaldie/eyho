-- Enable PostGIS for geography/spatial types and functions.
CREATE EXTENSION IF NOT EXISTS postgis;

-- ============================================================
-- ENUMS
-- ============================================================

CREATE TYPE cook_org_type AS ENUM ('nonprofit', 'professional');

CREATE TYPE cook_verification_status AS ENUM ('unverified', 'verified');

CREATE TYPE listing_status AS ENUM ('draft', 'published', 'expired', 'cancelled');

CREATE TYPE claim_payment_status AS ENUM (
    'pending_payment',
    'paid',
    'payment_failed',
    'refunded'
);

CREATE TYPE claim_pickup_status AS ENUM ('pending', 'collected', 'no_show');

-- ============================================================
-- cook_profiles
-- ============================================================

CREATE TABLE cook_profiles (
    id                          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name                TEXT            NOT NULL,
    org_type                    cook_org_type   NOT NULL,
    verification_status         cook_verification_status NOT NULL DEFAULT 'unverified',

    -- Counts since the last verification status transition; reset on each transition.
    -- Reaching 5 promotes unverified -> verified (Constitution Principle I).
    -- Reaching 3 demotes verified -> unverified (Constitution Principle I).
    good_ratings_since_transition    INTEGER         NOT NULL DEFAULT 0
        CONSTRAINT chk_cook_good_ratings_since_transition_non_negative
            CHECK (good_ratings_since_transition >= 0),
    bad_ratings_since_transition     INTEGER         NOT NULL DEFAULT 0
        CONSTRAINT chk_cook_bad_ratings_since_transition_non_negative
            CHECK (bad_ratings_since_transition >= 0),

    -- Total transitions (up or down) over the lifetime of this cook.
    verification_cycle_count    INTEGER         NOT NULL DEFAULT 0
        CONSTRAINT chk_cook_verification_cycle_count_non_negative
            CHECK (verification_cycle_count >= 0),

    -- Denormalised aggregate rating; recomputed by the service layer on each new rating.
    rating_avg                  NUMERIC(3, 2)   NOT NULL DEFAULT 0.00
        CONSTRAINT chk_cook_rating_avg_range
            CHECK (rating_avg BETWEEN 0.00 AND 5.00),
    rating_count                INTEGER         NOT NULL DEFAULT 0
        CONSTRAINT chk_cook_rating_count_non_negative
            CHECK (rating_count >= 0),

    created_at                  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- ============================================================
-- verification_history  (append-only — no UPDATE paths)
-- ============================================================

CREATE TABLE verification_history (
    id                      UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),
    cook_id                 UUID                        NOT NULL
        REFERENCES cook_profiles (id) ON DELETE CASCADE,
    from_status             cook_verification_status    NOT NULL,
    to_status               cook_verification_status    NOT NULL,

    -- from_status and to_status must differ (every row represents an actual transition).
    CONSTRAINT chk_verification_history_different_statuses
        CHECK (from_status <> to_status),

    -- The claim UUIDs whose ratings triggered this transition, kept for auditability
    -- (Constitution Principle I: permanent, per-cook history of every transition).
    triggering_claim_ids    UUID[]                      NOT NULL DEFAULT '{}',

    created_at              TIMESTAMPTZ                 NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_verification_history_cook_id
    ON verification_history (cook_id);

-- ============================================================
-- buyer_profiles
-- ============================================================

CREATE TABLE buyer_profiles (
    id                              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Default radius used for the discovery view (FR-011). Reasonable default is 3 km.
    default_discovery_radius_km     NUMERIC         NOT NULL DEFAULT 3.0
        CONSTRAINT chk_buyer_discovery_radius_positive
            CHECK (default_discovery_radius_km > 0),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- listings
-- ============================================================

CREATE TABLE listings (
    id                          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    cook_id                     UUID            NOT NULL
        REFERENCES cook_profiles (id) ON DELETE RESTRICT,
    food_description            TEXT            NOT NULL,

    -- allergen_info is required; an empty string is valid and means "none".
    allergen_info               TEXT            NOT NULL,

    -- Optional photo shown on listing cards (feature 003). NULL renders a placeholder.
    photo_url                   TEXT,

    total_portions              INTEGER         NOT NULL
        CONSTRAINT chk_listing_total_portions_positive
            CHECK (total_portions > 0),
    portions_remaining          INTEGER         NOT NULL
        CONSTRAINT chk_listing_portions_remaining_bounds
            CHECK (portions_remaining BETWEEN 0 AND total_portions),

    -- FR-003: reduced_price_cents MUST be less than retail_reference_price_cents.
    reduced_price_cents         INTEGER         NOT NULL
        CONSTRAINT chk_listing_reduced_price_positive
            CHECK (reduced_price_cents > 0),
    retail_reference_price_cents INTEGER        NOT NULL
        CONSTRAINT chk_listing_retail_reference_price_positive
            CHECK (retail_reference_price_cents > 0),
    CONSTRAINT chk_listing_price_below_retail
        CHECK (reduced_price_cents < retail_reference_price_cents),

    -- PostGIS geography point (SRID 4326) for ST_DWithin radius discovery (FR-011).
    pickup_location             GEOGRAPHY(Point, 4326) NOT NULL,

    -- Human-readable address revealed to the buyer after a claim is confirmed.
    pickup_address_text         TEXT            NOT NULL,

    pickup_window_start         TIMESTAMPTZ     NOT NULL,
    pickup_window_end           TIMESTAMPTZ     NOT NULL,

    -- pickup_window_end must be after pickup_window_start (data-model Edge Case).
    CONSTRAINT chk_listing_window_order
        CHECK (pickup_window_end > pickup_window_start),

    status                      listing_status  NOT NULL DEFAULT 'draft',

    created_at                  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- GiST index enables efficient ST_DWithin radius queries (FR-011, plan.md performance goal).
CREATE INDEX idx_listings_pickup_location_gist
    ON listings USING GIST (pickup_location);

-- Supports filtering published listings by cook (cook dashboard).
CREATE INDEX idx_listings_cook_id
    ON listings (cook_id);

-- Supports the background expiry job filtering by status + pickup_window_end.
CREATE INDEX idx_listings_status_window_end
    ON listings (status, pickup_window_end)
    WHERE status = 'published';

-- ============================================================
-- claims
-- ============================================================

CREATE TABLE claims (
    id                          UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id                  UUID                    NOT NULL
        REFERENCES listings (id) ON DELETE RESTRICT,
    buyer_id                    UUID                    NOT NULL
        REFERENCES buyer_profiles (id) ON DELETE RESTRICT,

    quantity                    INTEGER                 NOT NULL
        CONSTRAINT chk_claim_quantity_positive
            CHECK (quantity > 0),

    payment_status              claim_payment_status    NOT NULL DEFAULT 'pending_payment',
    pickup_status               claim_pickup_status     NOT NULL DEFAULT 'pending',

    -- Nullable until a Stripe PaymentIntent is created.
    stripe_payment_intent_id    TEXT,

    -- Timestamp when the reservation was made; used to enforce FR-008 payment timeout.
    reserved_at                 TIMESTAMPTZ             NOT NULL DEFAULT NOW(),

    -- Set by the service layer when Stripe confirms payment.
    confirmed_at                TIMESTAMPTZ,

    -- Buyer rating of the listing, set once after pickup is collected (FR-013).
    rating                      SMALLINT
        CONSTRAINT chk_claim_rating_range
            CHECK (rating BETWEEN 1 AND 5),
    rated_at                    TIMESTAMPTZ,

    CONSTRAINT chk_claim_rating_requires_collected
        CHECK (rating IS NULL OR pickup_status = 'collected'),
    CONSTRAINT chk_claim_rated_at_matches_rating
        CHECK ((rating IS NULL) = (rated_at IS NULL)),

    -- confirmed_at is only meaningful once payment is successful.
    CONSTRAINT chk_claim_confirmed_at_requires_paid
        CHECK (
            confirmed_at IS NULL
            OR payment_status IN ('paid', 'refunded')
        ),

    created_at                  TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ             NOT NULL DEFAULT NOW()
);

-- Supports the payment-timeout release job (FR-008): find pending claims older than N minutes.
CREATE INDEX idx_claims_payment_status_reserved_at
    ON claims (payment_status, reserved_at)
    WHERE payment_status = 'pending_payment';

-- Supports buyer history and claim lookup by listing.
CREATE INDEX idx_claims_listing_id ON claims (listing_id);

-- Supports GET /api/v1/claims (buyer order history, newest first).
CREATE INDEX idx_claims_buyer_id_created_at ON claims (buyer_id, created_at DESC);
CREATE INDEX idx_claims_buyer_id   ON claims (buyer_id);

-- ============================================================
-- impact_records  (append-only — no UPDATE paths; UNIQUE on claim_id)
-- ============================================================
-- Principle II: derived from real transaction data; created once per completed (paid) claim;
-- immutable after creation.

CREATE TABLE impact_records (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- One ImpactRecord per completed claim, enforced by the UNIQUE constraint.
    claim_id                UUID        NOT NULL UNIQUE
        REFERENCES claims (id) ON DELETE RESTRICT,

    -- Denormalised for aggregate waste-reduction queries without joining back to claims/listings.
    listing_id              UUID        NOT NULL
        REFERENCES listings (id) ON DELETE RESTRICT,

    -- Copied from claim.quantity at the time of payment confirmation.
    portions                INTEGER     NOT NULL
        CONSTRAINT chk_impact_portions_positive
            CHECK (portions > 0),

    -- Estimated food weight; derived from a per-dish average (implementation detail, not spec).
    estimated_weight_grams  INTEGER     NOT NULL
        CONSTRAINT chk_impact_weight_non_negative
            CHECK (estimated_weight_grams >= 0),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- No updated_at: this table is append-only and rows are never modified.
);

CREATE INDEX idx_impact_records_listing_id ON impact_records (listing_id);

-- Feature 002: buyer notification outbox (FR-010).
--
-- Cancelling a listing with paid claims must "notify and refund" the buyers. The delivery channel
-- (email, push) is a later task, so the cancel flow queues one row per affected claim here; a
-- future sender marks sent_at. Rows hold ids only (no email or name), so account deletion needs no
-- scrubbing. New table, no locks on existing ones.

CREATE TABLE notifications (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_profile_id  UUID        NOT NULL REFERENCES buyer_profiles (id) ON DELETE RESTRICT,
    claim_id          UUID        NOT NULL REFERENCES claims (id) ON DELETE RESTRICT,
    kind              TEXT        NOT NULL
        CONSTRAINT chk_notification_kind CHECK (kind IN ('listing_cancelled_refund')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at           TIMESTAMPTZ,

    -- One notification per claim and kind, so a retried refund never notifies twice.
    CONSTRAINT uq_notifications_claim_kind UNIQUE (claim_id, kind)
);

CREATE INDEX idx_notifications_unsent ON notifications (created_at) WHERE sent_at IS NULL;

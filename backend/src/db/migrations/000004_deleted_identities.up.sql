-- Feature 001: tombstones for deleted identities (account deletion, FR-013 / US4).
--
-- Deleting an account removes its account_identities rows. Without a record of that, a still-valid
-- access token of the deleted user would provision a brand-new account on its next request. A
-- tombstone remembers (issuer, subject) and when it was deleted: a token issued at or before
-- deleted_at is answered with 403 account_deleted; a token issued after it (a fresh sign-in) gets
-- a new, empty account (US4 scenario 4).
--
-- Tombstones are only needed while tokens issued before the deletion can still be valid, so the
-- backend purges rows older than TOMBSTONE_RETENTION (default 24h, longer than the longest access
-- token lifetime). They hold only the pseudonymous provider identifiers, no email or name.

CREATE TABLE deleted_identities (
    issuer      TEXT        NOT NULL,
    subject     TEXT        NOT NULL,
    deleted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (issuer, subject)
);

CREATE INDEX idx_deleted_identities_deleted_at ON deleted_identities (deleted_at);

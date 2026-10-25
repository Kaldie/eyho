# Phase 1 Data Model: Accounts & Sign-in

Delivered as migration `000002_accounts`. Extends the existing `buyer_profiles` and
`cook_profiles` tables (migration 000001) so both belong to an account.

## Account (new)

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| email | citext, nullable | Unique among non-deleted accounts with a **verified** email (migration `000003`; an unverified sign-in with an already-used email gets its own account, quickstart Scenario 4 step 4); null after deletion |
| email_verified | boolean | Cached from the latest token claim |
| display_name | text, nullable | From the provider profile; editable; null after deletion |
| is_demo | boolean | Seeded demo accounts only (FR-016); never true outside demo/local |
| created_at / updated_at | timestamptz | |
| deleted_at | timestamptz, nullable | Set on deletion; deleted accounts can't sign in |

**Validation**: `email` is required while `deleted_at` is null.

## AccountIdentity (new)

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| account_id | UUID (FK → Account) | |
| issuer | text | Token `iss` (Auth0 tenant, mock server, or demo issuer) |
| subject | text | Token `sub` |
| created_at | timestamptz | |

**Validation**: unique `(issuer, subject)`. Rows are deleted, not anonymised, on account deletion,
so the same provider login creates a fresh account afterwards (spec US4, scenario 4).

**Linking rule (FR-007)**: a new identity attaches to an existing account only when both the
token's email and the account's email are verified and equal.

## BuyerProfile (changed; table from migration 000001)

| Field | Change |
|---|---|
| account_id | **New**. UUID FK → Account, unique, not null. Created on first sign-in |

## CookProfile (changed; table from migration 000001)

| Field | Change |
|---|---|
| account_id | **New**. UUID FK → Account, unique, **nullable** (null once anonymised) |
| phone | **New**. text, required at onboarding, null after deletion. Stored, not verified |
| default_pickup_address_text | **New**. text, required at onboarding, null after deletion |
| default_pickup_location | **New**. geography(Point, 4326), required at onboarding, null after deletion |
| anonymised_at | **New**. timestamptz, nullable |
| kvk_number | **New**. text (8 digits), required at onboarding, null after deletion. Not unique: volunteers of one kitchen may share it |
| kvk_trade_name | **New**. text, from the business register, null after deletion |
| registration_status | **New**. enum `pending` / `active` / `inactive`. Set by the KvK check (FR-018, FR-021) |
| registration_checked_at | **New**. timestamptz, nullable. Last successful check; drives the 30-day re-check |
| nvwa_declared_at | **New**. timestamptz, required at onboarding (FR-019) |
| nvwa_declaration_version | **New**. text, the version of the declaration wording shown (for example `2026-10-25`) |
| display_name | Unchanged, but set to `Former cook` on anonymisation |

**Validation**: `phone`, `default_pickup_address_text`, `default_pickup_location`, `kvk_number`,
`nvwa_declared_at` and `nvwa_declaration_version` are required while `anonymised_at` is null.
`kvk_number` matches `^[0-9]{8}$`.

**Can publish** (FR-020): `registration_status = 'active'` and `nvwa_declared_at` is not null.
Independent of `verification_status`. `verification_status` starts `unverified` (Principle I).

**On deletion**: personal fields (including `kvk_number` and `kvk_trade_name`) cleared, `anonymised_at` set; `verification_history`, ratings on
claims, and listings remain linked to the anonymised profile (FR-013, SC-004).

## Listing / Claim (tables from migration 000001)

No schema change. Rules this feature adds:
- `listings.cook_id` is set from the signed-in account's cook profile.
- `claims.buyer_id` is set from the signed-in account's buyer profile.
- A claim is rejected when the listing's cook profile belongs to the same account (FR-011).
- A listing can only be published by a cook profile that can publish (FR-020); enforced by
  feature 002's listing service using this feature's `CookCanPublish` check.

## IdpDeletionRequest (new)

Queue for deleting users at the identity provider after local deletion (research.md).

| Field | Type | Notes |
|---|---|---|
| id | UUID | Primary key |
| issuer | text | |
| subject | text | Provider user id to delete |
| attempts | integer | |
| next_attempt_at | timestamptz | Exponential backoff |
| created_at | timestamptz | |

Rows are deleted as soon as the provider confirms deletion. Not created for the mock or demo
issuer.

## Relationships

```text
Account 1---* AccountIdentity
Account 1---1 BuyerProfile
Account 1---0..1 CookProfile   (CookProfile survives deletion with account_id = null)
```

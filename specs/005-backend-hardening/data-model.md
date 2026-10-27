# Data Model: Backend Hardening

No new tables. One table gains three columns; four queries are added for ownership and caller
lookup and four for the re-check work queue. Schema changes are edits to
`backend/src/db/schema.sql` (no migrations: there is no production database). New columns are
appended at the end of the table, because `SELECT *` queries scan in table order. After the
edit, run `sqlc generate` in `backend/src/db`.

## cook_profiles: registration re-check work state (US4)

Appended after `nvwa_declaration_version`:

| Column | Type | Null | Default | Meaning |
|---|---|---|---|---|
| `recheck_next_attempt_at` | `TIMESTAMPTZ` | yes | `NOW()` | When a *pending* profile is next due. It also holds the persisted backoff (1 min doubling to 1 h after each transport error). NULL for anonymised profiles |
| `recheck_attempts` | `INTEGER` | no | `0` | Failed register calls since the last definite answer. `CHECK (recheck_attempts >= 0)` |
| `recheck_lease_until` | `TIMESTAMPTZ` | yes | NULL | Set while one server holds the profile. Expired or NULL means it is free |

**Invariants**

- Anonymised profiles hold no work state: `chk_cook_anonymised_has_no_pii` is extended with
  `recheck_next_attempt_at IS NULL AND recheck_lease_until IS NULL`.
- Which profiles are due is unchanged from 001 (FR-021): pending ones (now also gated by
  `recheck_next_attempt_at <= now()`), active ones last checked more than 30 days ago, and
  inactive ones last checked more than 1 day ago. Only the lease and the quota split are new.

**Indexes**

- Replace `idx_cook_profiles_registration_recheck` with two partial indexes that match the two
  claim queries:
  - `(recheck_next_attempt_at) WHERE registration_status = 'pending' AND anonymised_at IS NULL`;
  - `(registration_checked_at) WHERE registration_status <> 'pending' AND anonymised_at IS NULL`.

**Work-state transitions** (registration status transitions are unchanged from 001)

```text
free ──claim (lease_until = now+120s)──▶ leased
leased ──definite answer──▶ free; attempts=0; next_attempt_at=NULL (definite rows are due by checked_at)
leased ──transport error──▶ free; attempts+1; next_attempt_at=now+backoff(attempts)
leased ──server crash──▶ (lease expires) free
profile created / KvK number changed ──▶ attempts=0; next_attempt_at=now()
profile anonymised ──▶ next_attempt_at=NULL; lease_until=NULL
```

## Queries (backend/src/db/queries)

### accounts.sql

- **`GetCallerByIdentity`** `:one`, with params `issuer` and `subject`:
  - joins `accounts a`, `account_identities i` (on `i.account_id = a.id`), `LEFT JOIN
    buyer_profiles b ON b.account_id = a.id` and `LEFT JOIN cook_profiles c ON c.account_id =
    a.id`;
  - returns `a.*`, `b.id AS buyer_profile_id` and `c.id AS cook_profile_id`;
  - one statement, served by the unique indexes on `(issuer, subject)`,
    `buyer_profiles.account_id` and `cook_profiles.account_id`. It replaces the 3–4
    statements of today's `ResolveAccount` for a known caller.

### listings.sql

- **`GetListingForCook`** `:one`: `WHERE id = $1 AND cook_id = $2`. Used by
  `Access.OwnedListing`.

### claims.sql

- **`GetClaimForParticipant`** `:one`:
  - `FROM claims c JOIN listings l ON l.id = c.listing_id`;
  - `WHERE c.id = $1 AND (c.buyer_id = sqlc.narg(buyer_id) OR l.cook_id = sqlc.narg(cook_id))`;
  - also returns `l.cook_id`, so the caller's role (buyer or cook) can be derived.
- **`GetClaimForBuyer`** `:one`: `WHERE id = $1 AND buyer_id = $2`.

### cook_profiles.sql

- **`ClaimStaleRegistrationRechecks`** `:many`, with params `stale_before`,
  `inactive_stale_before`, `lease_seconds` and `max_rows`:
  - leases non-pending, non-anonymised profiles that have a KvK number and are due (active
    and `checked_at < stale_before`, or inactive and `checked_at < inactive_stale_before`,
    or `checked_at IS NULL`) and are not leased;
  - oldest `registration_checked_at` first;
  - `FOR UPDATE SKIP LOCKED`.
- **`ClaimPendingRegistrationRechecks`** `:many`, with params `lease_seconds` and
  `max_rows`: leases pending profiles with `recheck_next_attempt_at <= now()` that are not
  leased, ordered by `recheck_next_attempt_at`, with `FOR UPDATE SKIP LOCKED`.
- **`CompleteRegistrationRecheck`** `:execrows`: today's `UpdateCookRegistrationResult`
  (guarded by `kvk_number` and `anonymised_at IS NULL`), plus clearing the lease, attempts
  and next attempt.
- **`RescheduleRegistrationRecheck`** `:exec`: sets `recheck_attempts = recheck_attempts + 1`,
  `recheck_next_attempt_at = now() + backoff`, and `recheck_lease_until = NULL`. Status is
  never touched.
- `ListCookProfilesDueForRecheck` is removed.
- `CreateCookProfile`, `UpdateCookKvkRegistration` and `AnonymiseCookProfile` set the work
  columns as in the transitions above.

## In-memory types (not persisted)

### Caller (`services/caller.go`)

Today's `services.Account`, renamed. The field set is unchanged, so 002 code written against
`middleware.AccountFromContext` keeps compiling through an alias:

| Field | From |
|---|---|
| `ID`, `Email`, `EmailVerified`, `DisplayName`, `IsDemo` | `accounts` |
| `BuyerProfileID` | `buyer_profiles.id` (always present: created with the account) |
| `CookProfileID` (`*uuid.UUID`) | `cook_profiles.id`, NULL until onboarding |
| `Issuer`, `Subject` | the verified token |

### Capability values (`services/access.go`)

These structs have unexported fields and can only be constructed by the `Access` loaders
(research.md):

| Type | Carries | Proves |
|---|---|---|
| `OwnedListing` | the listing row | the caller's cook profile owns it |
| `VisibleListing` | the listing row | published, or owned by the caller |
| `VisibleClaim` | the claim row, `Role` (`buyer` or `cook`) | the caller is its buyer or the listing's cook |
| `BuyerClaim` | the claim row | the caller is its buyer |

## Ownership relation (spec Key Entity)

```text
accounts 1──1 buyer_profiles 1──* claims *──1 listings *──1 cook_profiles 1──1 accounts
            (account_id)        (buyer_id)  (listing_id)    (cook_id)          (account_id)
```

A caller owns a listing when `listings.cook_id = caller.CookProfileID`. A caller participates
in a claim as its buyer (`claims.buyer_id = caller.BuyerProfileID`) or as the listing's cook.
An anonymised cook profile has `account_id = NULL`, so no caller can own its listings (spec
edge case).

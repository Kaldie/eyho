# Phase 0 Research: Backend Hardening

All findings refer to the code on `main` + the 2026-10-26 maintenance branch (Go 1.27, single
`schema.sql`). No open NEEDS CLARIFICATION items remain.

## Where ownership is checked (US1, FR-001 to FR-003)

**Decision**: a resource-access step in the service layer, `services.Access`, loads the item
with a query scoped by the caller's own profile id and returns a typed capability value:

| Loader | Query (new, `sqlc`) | Returns | Used by |
|---|---|---|---|
| `Access.OwnedListing(ctx, caller, id)` | `GetListingForCook`: `WHERE l.id = $1 AND l.cook_id = $2` | `OwnedListing` | `PATCH /listings/{id}` |
| `Access.VisibleListing(ctx, caller, id)` | `GetListingByID`, then: published → anyone; otherwise only the owning cook | `VisibleListing` | `GET /listings/{id}` |
| `Access.VisibleClaim(ctx, caller, id)` | `GetClaimForParticipant`: `WHERE c.id = $1 AND (c.buyer_id = $2 OR l.cook_id = $3)` (join on listing) | `VisibleClaim` (carries the role: buyer or cook) | `GET /claims/{id}`, `POST /claims/{id}/confirm-pickup` |
| `Access.BuyerClaim(ctx, caller, id)` | `GetClaimForBuyer`: `WHERE c.id = $1 AND c.buyer_id = $2` | `BuyerClaim` | `POST /claims/{id}/rating` |

A miss, whether the row is absent or belongs to someone else, is `services.ErrNotFound`. The
handler maps it to `404 {"error":"not_found"}` (contracts/errors.md). The profile ids come from
the `Caller` that the middleware resolved from the verified token (FR-003). Nothing from the
request body or query string feeds the `WHERE` clause except the item id itself.

The capability types have unexported fields and no exported constructor. The 002 service
methods take them instead of raw ids:
- `ListingService.Update(ctx, OwnedListing, UpdateListingRequest)`;
- `ClaimService.ConfirmPickup(ctx, VisibleClaim)`;
- `ClaimService.Rate(ctx, BuyerClaim, stars)`.

A handler that skips the loader does not compile.

**Today's stubs**: the handlers call the loader first and only then reach the stub body
(`501 not_implemented`). Ownership is therefore testable now: the owner gets 501, anyone else
404. Feature 002 replaces the bodies behind an already-guarded entry.

**Rationale**:
- Scoping the query, rather than load-then-compare, makes "not yours" and "doesn't exist"
  identical by construction, so no existence leaks through timing or error text (FR-002).
- It is also one round trip.
- Service-layer placement covers callers that are not HTTP handlers, such as a future admin
  task or job.

**Alternatives considered**:
- Chi middleware per route that loads the resource. Rejected: it couples routing to
  persistence, and a service called from elsewhere would be unguarded.
- Row-level security in PostgreSQL. Rejected for now: it needs per-request `SET` of the
  caller on a pooled connection (transaction-scoped settings everywhere). Strong, but a large
  change for a pilot; worth revisiting when there are more resource types.
- 403 for "exists but not yours", as 002's contract says today. Rejected by spec FR-002 because
  it reveals existence. Two lines of 002's contracts change: see
  contracts/authorization-matrix.md.

**Generic `PATCH /claims/{id}`**: the router has a "general claim update (future use)" route
that calls the same handler as confirm-pickup. It is in no contract, so it is removed. Fewer
unguarded entry points.

## Proving ownership on every route (FR-004, SC-001)

**Decision**: `testutil.AssertOtherUserRefused(t, env, req, owner, other, snapshot)`:
1. Sends `req` as `other` and asserts `404 {"error":"not_found"}`.
2. Asserts `snapshot()` (a closure reading the item from the database) is unchanged.
3. Sends `req` as `owner` and asserts the response is not 404.

Coverage is enforced, not just encouraged: `TestEveryResourceRouteHasOwnershipCase` walks the
chi router (`chi.Walk`). It fails if any route containing `{id}` under `/listings` or
`/claims` is missing from the ownership test table.

**Rationale**: a new 002 route without the test fails CI, which is what SC-001 asks for.

**Alternatives considered**: a checklist in the PR template. Rejected: not enforceable.

## One-lookup caller identification (US3, FR-008)

**Today** (`middleware.Authenticate` → `AccountService.ResolveAccount`):
1. `GetAccountByIdentity`.
2. A possible `UpdateAccountEmailVerified` write.
3. `GetBuyerProfileByAccount`, which falls back to `CreateBuyerProfile` (a write) when missing.
4. `GetCookProfileByAccount`.

That is three or four statements per authenticated request, some of them writes.

**Decision**:
- **Lookup.** A new query, `GetCallerByIdentity`: `accounts JOIN account_identities` on
  `(issuer, subject)`, `LEFT JOIN buyer_profiles` and `LEFT JOIN cook_profiles` on
  `account_id`. It returns the account columns plus `buyer_profile_id` and `cook_profile_id`.
  Every join column is covered by an existing unique index, so this is one index lookup per
  table and one statement. `services.CallerResolver.Resolve` runs it and maps it to `Caller`
  (today's `Account` struct, renamed; the field set is unchanged).
- **Steady state.** A known identity whose stored `email_verified` equals the token's claim
  costs 1 statement and 0 writes (SC-003).
- **Verified flag changed.** When the token's `email_verified` differs from the stored value,
  the resolver calls `Provisioner.SyncEmailVerified`. That is today's `refreshEmailVerified`,
  including the "merge an empty account into the verified owner" rule. It is an explicit
  write that happens once per change, not once per request.
- **Unknown identity.** On a miss, the middleware calls `Provisioner.Provision(ctx, claims)`
  explicitly. Provisioning is today's `resolveOnce` creation half, unchanged:
  - the demo issuer → `ErrUnknownIdentity`;
  - the tombstone check against `iat`;
  - `ErrEmailMissing`;
  - link by verified email (row locked `FOR SHARE`);
  - otherwise create account + identity + buyer profile in one transaction;
  - retry on a unique violation.

  It is idempotent because every path ends in a unique `(issuer, subject)` identity: a
  concurrent second call hits the unique violation, retries, and finds the identity. The
  middleware then resolves once more.
- **No lazy buyer profile.** The lazy `CreateBuyerProfile` in `load()` is removed.
  Provisioning and the seed always create the buyer profile. If one is ever missing, the
  resolver answers 500 and logs it, instead of silently writing.
- **Unchanged.** `RequireAccount`, `RequireVerifiedEmail` and `RequireCook` keep their
  signatures and answers (401 / 403 `account_deleted` / 503). They read `Caller` from the
  context; only the type name changes. `AccountFromContext` stays as an alias for 002 code
  already written against it.

**Rationale**: keeps the 001 contract byte-for-byte (no new endpoint, no frontend change).
Provisioning is still triggered by the first request, but it is a named, separately tested
step that the read path never runs for a known identity.

**Alternatives considered**:
- An explicit `POST /me/session` that the frontend must call after sign-in, with every other
  route answering 401 for unknown identities. Rejected: it changes the 001 contract and the
  frontend (feature 006 territory), and every client must remember to call it.
- Caching the resolved caller in memory per token. Rejected: invalidation on deletion and
  on cook onboarding is subtle, and one indexed query is already cheap.

## Splitting `AccountService` (FR-009)

**Decision**: one Go package, four types, moved mostly verbatim from `account_service.go`:

| Type | Responsibilities | From today's methods |
|---|---|---|
| `CallerResolver` + `Provisioner` | caller lookup; create/link/tombstone; email-verified sync and empty-account merge | `ResolveAccount`, `resolveOnce`, `linkByVerifiedEmail`, `refreshEmailVerified`, `load` |
| `AccountSettings` | display name; verification email resend (1/min limiter) | `UpdateDisplayName`, `RequestVerificationEmail` |
| `CookOnboarding` | cook profile view/create/update, KvK checks, publish gate, own-profile guard | `GetCookProfile`, `CreateCookProfile`, `UpdateCookProfile`, `CookCanPublish`, `CookPublishGate`, `AccountOwnsCookProfile` |
| `AccountDeletion` | deletion transaction, tombstones, provider-deletion queue | `DeleteAccount` |

Shared helpers (`pgUUID`, `tsPtr`, `isUniqueViolation`, `PoolTx`) stay in a small `db_helpers.go`.

Interfaces are declared by their consumers:
- `handlers/ports.go`: `MeHandler` needs `AccountSettings`-like methods, `CookOnboarding`
  methods and `DeleteAccount`. Each is a 1–3 method interface.
- `middleware`: already declares `TokenVerifier` and gains `CallerResolver`.

`main.go` wires the four concrete types. `testutil.Env` exposes them individually.

**Rationale**: the current 840-line service mixes four reasons to change. Go's
consumer-side interfaces let handlers be unit-tested with fakes; today `MeHandler` takes the
concrete `*services.AccountService`, so it has no unit tests.

**Alternatives considered**: sub-packages (`services/accounts`, `services/cooks`, ...).
Rejected for now: shared transaction helpers and errors would need a third package, and 002
code that imports `services` would churn. Revisit when `services` grows past ~10 files.

## One error helper (FR-010)

**Today**: `handlers.writeJSON` / `writeError` (with `field`) and `middleware.WriteError` /
`writeJSON` duplicate each other. The 409 deletion body and `cook_not_registered` are built ad
hoc.

**Decision**: a new package, `backend/src/api/respond`:
- `JSON(w, status, v)`;
- `Error(w, status, code)`;
- `FieldError(w, status, code, field)`;
- `ErrorWith(w, status, code, extra map[string]any)` for `count` and `registration_status`;
- `Decode(w, r, v) bool` (the 64 KiB `MaxBytesReader` + 422 mapping, moved from `me.go`).

Middleware and handlers both use it. `contracts/errors.md` is the registry of codes. A unit
test asserts every `Error*` call site uses a code listed there (a simple AST scan of string
literals passed to `respond.*Error*`).

**Rationale**: one shape, one place to change it; the registry keeps 002 from inventing
near-duplicate codes.

## Job leases for the registration re-check (US4, FR-011, FR-012)

**Today**: `registry.RecheckJob` lists due profiles (`ListCookProfilesDueForRecheck`, limit
500) and keeps its backoff in an in-memory map. With two servers:
- both list the same rows and both call the business register;
- each server has its own backoff, so a down register is called by every instance.

Ordering is `registration_checked_at ASC NULLS FIRST`. Pending profiles (never checked: NULL)
always come first, so more than 500 pending profiles would starve overdue active and inactive
cooks forever.

**Decision**:
- **Persist the work state on `cook_profiles`**, appended columns (data-model.md):
  - `recheck_next_attempt_at`: when the next attempt is due; also holds the backoff;
  - `recheck_attempts`: failed attempts since the last definite answer;
  - `recheck_lease_until`: lease expiry.
- **Claim work** with an `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED LIMIT n)`
  that sets `recheck_lease_until = now() + 120 s` and returns the rows. This is the pattern
  `ClaimDueIdpDeletions` already uses. A crashed server's rows come back when the lease
  expires (spec edge case).
- **Two quotas per run, two claim queries**:
  1. `ClaimStaleRegistrationRechecks`: active profiles checked more than 30 days ago and
     inactive profiles checked more than 1 day ago, oldest first, up to 100.
  2. `ClaimPendingRegistrationRechecks`: pending profiles whose `recheck_next_attempt_at` is
     due, ordered by it, up to 400.

  Overdue definite profiles are always served first and never starved (FR-012). Pending
  retries keep exponential backoff (1 min → 1 h, as today), now persisted.
- **Results.** A definite result runs `CompleteRegistrationRecheck`, which writes the result
  as today's `UpdateCookRegistrationResult` does, guarded by `kvk_number`, and clears the
  lease, attempts and next-attempt. A transport error runs `RescheduleRegistrationRecheck`,
  which sets the next attempt from the backoff, increments attempts and clears the lease. As
  today, an outage never touches `registration_status`.
- **Profile writes reset the work state.** `CreateCookProfile`, `UpdateCookKvkRegistration`
  and `AnonymiseCookProfile` set `recheck_next_attempt_at = now()` (or NULL when anonymised)
  and `recheck_attempts = 0`, so a changed KvK number is picked up promptly.
- **Provider-deletion worker**: already leased (`ClaimDueIdpDeletions`). Its tombstone purge
  runs on every server, but it is an idempotent `DELETE ... WHERE deleted_at < cutoff`, so
  no change is needed. Documented.

**Rationale**: the same mechanism as the provider-deletion queue, so there is one pattern to
learn and no new infrastructure (no leader election, no external queue).

**Alternatives considered**:
- A PostgreSQL advisory lock so only one server runs the job. Rejected: it serialises all
  work on one server, and a stuck holder blocks everyone.
- A separate `registration_rechecks` table. Rejected: it must be kept in sync with every
  cook-profile write and anonymisation; three columns on the row they describe are simpler.
- A single query with a weighted `ORDER BY`. Rejected: harder to reason about than two
  quotas.

**Known limitation, out of scope**: the verification-email resend limiter
(`lastEmail` map) is per server, so with N servers a user can request N emails a minute. That
is rate limiting, which the spec defers until the demo is public.

## CI pipeline (US2, FR-005 to FR-007)

**Decision**: `.github/workflows/ci.yml` on `pull_request` and `push` to `main`. Third-party
actions are pinned by commit SHA. Three jobs:

1. **backend** (`ubuntu-latest`, service `postgis/postgis:17-3.5`, health-checked):
   1. `actions/setup-go` with `go-version-file: backend/go.mod`.
   2. `gofmt -l backend` must print nothing.
   3. `go vet ./...` and `go build ./...`.
   4. `sqlc diff` in `backend/src/db` (sqlc v1.31.1). Generated code must match `schema.sql`
      and the queries.
   5. `psql -v ON_ERROR_STOP=1 -f schema.sql -f seed/demo_accounts.sql`: the same files the
      compose `db` service loads.
   6. `go test -race ./...` with `DATABASE_URL` and `EYHO_REQUIRE_DB=1`.
   7. `govulncheck ./...`.
2. **frontend** (`actions/setup-node`, Node 24, npm cache): `npm ci`, `npx tsc --noEmit`,
   `npx vitest run`, `npm run build`, `npm audit --omit=dev --audit-level=low`.
3. **e2e** (needs 1 and 2): `docker compose --profile e2e up --build --abort-on-container-exit
   --exit-code-from e2e`. This runs the Playwright suite against the compose stack, which is
   what the constitution's Local Development Constraint asks of CI. It is allowed to fail
   (`continue-on-error`) until the 002 flows it exercises are real, then made required.

**Skip → fail**: `testutil.NewEnv` keeps skipping locally without a database, so contributors
can run `go test ./...` with no setup. When `EYHO_REQUIRE_DB=1` it calls `t.Fatal` on a
missing or unreachable database instead (FR-006).

**gofmt (FR-007)**: `gofmt -l backend/` lists these six files today, all scaffold code from
before 001; they are reformatted in this feature:
- `src/models/buyer_profile.go`;
- `src/models/claim.go`;
- `src/models/cook_profile.go`;
- `src/models/impact_record.go`;
- `src/models/listing.go`;
- `src/services/claim_service.go`.

**Stale scaffold comments (FR-007)** in `main.go`:
- "A nil pool is acceptable here ... during early scaffold testing" (false: the pool is
  required and pinged);
- "Wire stub services. Replace with real implementations once db/ queries are generated by
  sqlc" (move to a `TODO(002)`);
- `_ = pool // Suppress "pool declared but unused"` (the pool is used).

**Rationale**: these are exactly the checks CLAUDE.md asks contributors to run, plus `sqlc
diff` and govulncheck, which no one runs today. The PostGIS service container matches the
compose image, so CI tests what developers run.

**Alternatives considered**:
- Running the backend tests inside `docker compose`. Rejected for the unit and integration
  job: slower and harder to read on failure. The e2e job covers the compose path.
- Testcontainers from Go tests. Rejected: needs Docker-in-test on every developer machine.

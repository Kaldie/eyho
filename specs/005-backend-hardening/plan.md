# Implementation Plan: Backend Hardening

**Branch**: `005-backend-hardening` | **Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-backend-hardening/spec.md`

## Summary

Make the Go backend safe to build feature 002 on.

1. **Ownership (US1).** Listing and claim routes stop at "signed in" today. Every route that
   names a listing or claim now first loads it through a resource-access step that queries
   with the caller's own profile id in the `WHERE` clause. A miss answers
   `404 {"error":"not_found"}`. The step hands back a typed `OwnedListing` / `VisibleClaim`
   value, and the 002 service methods accept only those types, so an unchecked call does not
   compile. A route walk in the tests fails if any `/{id}` route has no "other signed-in user
   is refused" case.
2. **CI (US2).** A GitHub Actions workflow runs gofmt, vet, `sqlc diff`, unit and database
   tests against a PostGIS service container (schema + seed loaded), govulncheck, and the
   frontend type check, tests, build and `npm audit --omit=dev`. `EYHO_REQUIRE_DB=1` turns
   the integration tests' skip into a failure.
3. **Caller identification (US3).** One joined query returns account, buyer profile and cook
   profile for `(iss, sub)`. Provisioning of an unknown identity becomes an explicit,
   idempotent `Provisioner` that keeps all of 001's linking and tombstone rules.
   `AccountService` splits into four focused services behind handler-side interfaces. One
   `api/respond` package writes every JSON body and error.
4. **Background jobs (US4).** The registration re-check gets persisted, leased work rows
   (`FOR UPDATE SKIP LOCKED`, like the existing provider-deletion queue) and two quotas per
   run, so a pending backlog cannot starve overdue active and inactive cooks.

No product behaviour changes. Two documented status codes in 002's contracts move from 403
to 404 (see contracts/authorization-matrix.md).

## Technical Context

**Language/Version**: Go 1.27 (backend). CI also builds the frontend: TypeScript 7, React 19,
Vite 8, Vitest 5, Node 24.

**Primary Dependencies**: existing `chi` v5, `pgx` v5 + `sqlc` v1.31.1 (generated queries),
`go-oidc` v3. New in CI only: `golang.org/x/vuln/cmd/govulncheck`, GitHub Actions
(`actions/checkout`, `actions/setup-go`, `actions/setup-node`, pinned by commit SHA).

**Storage**: PostgreSQL 17 + PostGIS 3.5. Schema changes are edits to
`backend/src/db/schema.sql` (no migrations until there is a production database): three
columns appended to `cook_profiles` for the re-check lease (see data-model.md), and no new
tables.

**Testing**: Go `testing` + `testify`. Integration tests in `backend/tests/integration` run
against a real database, now mandatory in CI (`EYHO_REQUIRE_DB=1`). New helpers in
`backend/tests/testutil`:
- `AssertOtherUserRefused` for the ownership pattern;
- a `pgx` query tracer that counts statements (SC-003);
- `Env.Account(...)` for multi-account setups.

**Target Platform**: Linux container (backend). CI uses GitHub-hosted `ubuntu-latest`
runners with a `postgis/postgis:17-3.5` service container.

**Project Type**: web application (Go API + React SPA); this feature is backend-only apart
from the CI job for the frontend.

**Performance Goals**: caller identification is one indexed query, < 2 ms p95 locally
(SC-003). The re-check job handles 500 profiles per minute per server, as before.

**Constraints**:
- No behaviour change for buyers or cooks.
- 001's linking, tombstone and email-verification rules are preserved exactly; all 21
  existing integration tests stay green unchanged.
- Background work is correct with N ≥ 1 servers on one database.
- CI must finish in under 10 minutes.

**Scale/Scope**: pilot scale (low thousands of accounts). Touches `api/`, `api/middleware`,
`services/`, `registry/`, `db/` and `tests/`, and adds `.github/workflows/ci.yml`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.* Constitution v3.1.1.

| Principle | Check | Status |
|---|---|---|
| I. Crowd-Sourced Verification & Trust | Ownership checks only narrow who can act. The own-listing guard (403 `own_listing`) and rating rules are unchanged; ratings stay buyer-only, now enforced by the scoped query | PASS |
| II. Waste-Reduction Impact Tracking | Not affected | PASS (n/a) |
| III. Community Collaboration | Not affected | PASS (n/a) |
| IV. Gamified Engagement | Not affected | PASS (n/a) |
| V. Low Price, Low Friction | Browsing stays anonymous. Sign-in latency improves (one query instead of 3–4 per request) | PASS |
| Product Constraints (food-safety legal floor) | The re-check job keeps the same outcomes and cadence (FR-021 of 001). Only who does the work changes (leases) and in what order (quotas). An outage still never downgrades an active cook | PASS |
| Product Constraints (minimal personal data) | New lease columns hold timestamps and a counter, no personal data. Anonymisation clears them too (keeps the "anonymised = no PII" rule simple) | PASS |
| Local Development Constraint | `docker compose up` is unchanged. CI loads the same `schema.sql` and seed that the compose `db` service does, and the e2e job runs the compose stack itself ("CI ... MUST be executable against the Docker Compose stack") | PASS |
| Development Workflow | Ownership and rating rules touch Principle I. Reviewed by `code-reviewer` before merge (CLAUDE.md) | PASS |

Post-design re-check: PASS. One justified complexity item below.

## Project Structure

### Documentation (this feature)

```text
specs/005-backend-hardening/
├── plan.md                         # This file
├── research.md                     # Phase 0
├── data-model.md                   # Phase 1
├── quickstart.md                   # Phase 1
├── contracts/
│   ├── errors.md                   # the one error shape + code registry
│   └── authorization-matrix.md     # who may call which listing/claim route
└── tasks.md                        # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
.github/workflows/ci.yml            # NEW: backend, frontend, e2e jobs

backend/
├── main.go                         # wiring of the split services; stale scaffold comments removed
├── src/
│   ├── api/
│   │   ├── respond/                # NEW: JSON, Error, ErrorWith, Decode (one error helper)
│   │   ├── middleware/auth.go      # Authenticate = one lookup (+ Provisioner on a miss)
│   │   ├── handlers/
│   │   │   ├── me.go               # depends on small interfaces declared in handlers/ports.go
│   │   │   ├── ports.go            # NEW: AccountSettings, CookOnboarding, AccountDeleter
│   │   │   ├── listings.go         # loads OwnedListing via services.Access before the stub body
│   │   │   └── claims.go           # loads VisibleClaim / BuyerClaim before the stub body
│   │   └── router.go               # generic PATCH /claims/{id} removed (see research)
│   ├── services/
│   │   ├── caller.go               # NEW: Caller type, CallerResolver (one query)
│   │   ├── provisioner.go          # NEW: Provision (create / link / tombstone rules from 001)
│   │   ├── account_settings.go     # NEW: display name, verification email (from account_service.go)
│   │   ├── cook_onboarding.go      # NEW: cook profile create/update, KvK, publish gate
│   │   ├── account_deletion.go     # NEW: DeleteAccount (unchanged logic)
│   │   ├── access.go               # NEW: OwnedListing, VisibleClaim, BuyerClaim + loaders
│   │   ├── listing_service.go      # interface takes OwnedListing instead of (cookID, id)
│   │   ├── claim_service.go        # interface takes VisibleClaim / BuyerClaim; gofmt
│   │   └── account_service.go      # REMOVED (split above)
│   ├── registry/recheck_job.go     # leased claims, persisted backoff, two quotas
│   ├── models/*.go                 # gofmt only
│   └── db/
│       ├── schema.sql              # cook_profiles: + 3 re-check columns (appended)
│       └── queries/
│           ├── accounts.sql        # + GetCallerByIdentity
│           ├── cook_profiles.sql   # + ClaimDueRegistrationRechecks, Complete/Reschedule
│           ├── listings.sql        # + GetListingForCook
│           └── claims.sql          # + GetClaimForParticipant, GetClaimForBuyer
└── tests/
    ├── testutil/
    │   ├── env.go                  # EYHO_REQUIRE_DB, Env.Account helper, statement counter
    │   └── ownership.go            # NEW: AssertOtherUserRefused + route-coverage walk
    └── integration/
        ├── ownership_test.go       # NEW: US1
        ├── caller_lookup_test.go   # NEW: US3 (one query, no writes, concurrent first sign-in)
        └── recheck_lease_test.go   # NEW: US4 (two job instances, fairness)
```

**Structure Decision**: the existing layout stays. The services stay in one Go package
(`services`), split into focused types and files rather than sub-packages, so the change is
mostly moving code and does not cause import churn for 002. Interfaces are declared where they
are consumed (`handlers/ports.go`, `middleware`), the usual Go idiom, which keeps each handler's
dependency to the two or three methods it calls.

## Complexity Tracking

| Deviation | Why needed | Simpler alternative rejected because |
|---|---|---|
| Typed capability values (`OwnedListing`, `VisibleClaim`, `BuyerClaim`) that only the access loaders can construct | Makes "forgot the ownership check" a compile error for every 002 service method, not just a review comment | A convention ("always call `CheckOwner` first") is exactly what failed review: the router already relies on checks that don't exist yet |

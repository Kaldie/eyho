# Quickstart: Validating Backend Hardening

Runnable checks for each user story. Commands run from the repository root unless noted.

## Prerequisites

- Go 1.27 and Node 24.
- A database with the schema and seed loaded. Use the compose `db` service
  (`docker compose up db`; on a schema change, `docker compose down -v` first), or any
  PostgreSQL 17 + PostGIS with:

  ```sh
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
    -f backend/src/db/schema.sql -f backend/src/db/seed/demo_accounts.sql
  ```

- Environment:

  ```sh
  export DATABASE_URL='postgres://eyho:eyho@localhost:5432/eyho?sslmode=disable'
  export EYHO_REQUIRE_DB=1
  ```

## US1: only the owner can act (SC-001)

```sh
cd backend && go test -race -run 'TestOwnership|TestEveryResourceRouteHasOwnershipCase' ./tests/integration/
```

Expected:
- Every row of [contracts/authorization-matrix.md](contracts/authorization-matrix.md) passes:
  - another signed-in user gets `404 {"error":"not_found"}`;
  - the database row is unchanged;
  - the owner gets the stub's `501 not_implemented` until 002 lands, and the real answer
    after.
- Adding a new `/claims/{id}/…` route without a case makes
  `TestEveryResourceRouteHasOwnershipCase` fail and name the route.

Manual spot check with the demo accounts (compose stack running):
1. Sign in as `cook_verified` and `buyer` (POST `/api/v1/demo/sign-in`).
2. Create a listing row for the cook in the database.
3. `PATCH /api/v1/listings/<id>` with the buyer's token → `404 not_found`.
4. The same with the cook's token → `501 not_implemented` (stub).

## US2: CI runs everything, nothing skips (SC-002)

```sh
unset DATABASE_URL; EYHO_REQUIRE_DB=1 go -C backend test ./tests/integration/
```

Expected: FAIL with "DATABASE_URL not set", not "SKIP".

On a pull request, `.github/workflows/ci.yml` shows the `backend`, `frontend` and `e2e` jobs.
Three throwaway branches prove that each check bites:
- an un-gofmt'ed file → `backend` fails at the gofmt step;
- a broken assertion in `account_deletion_test.go` → `backend` fails at `go test`, and the log
  shows the test ran (not `--- SKIP`);
- a dependency with a known advisory → `npm audit` or `govulncheck` fails and names it.

Locally, the same checks:

```sh
test -z "$(gofmt -l backend)" && (cd backend && go vet ./... && go test -race ./... \
  && (cd src/db && sqlc diff) && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)
cd frontend && npm ci && npx tsc --noEmit && npx vitest run && npm run build && npm audit --omit=dev
```

## US3: one lookup, no writes (SC-003)

```sh
cd backend && go test -race -run 'TestCallerLookup' ./tests/integration/
```

Expected:
- `TestCallerLookup_KnownCallerIsOneStatement`: a signed-in `GET /api/v1/me` from a known,
  unchanged identity records exactly one statement from the middleware (counted by the
  test's `pgx` query tracer) and zero `INSERT`, `UPDATE` or `DELETE`.
- `TestCallerLookup_ConcurrentFirstSignIn`: 20 parallel first requests for one new identity →
  exactly one `accounts` row, one identity and one buyer profile.
- All 21 existing integration tests from 001 pass unchanged: linking, tombstones,
  email-verification merge and deletion behave as before.
- Every error body in the suite matches [contracts/errors.md](contracts/errors.md)
  (`TestErrorCodesAreRegistered`).

## US4: jobs with several servers (SC-004, SC-005)

```sh
cd backend && go test -race -run 'TestRecheckLease' ./tests/integration/
```

Expected:
- `TestRecheckLease_TwoInstances`: two `RecheckJob`s over one database and a counting fake
  register, with 50 due profiles, run concurrently → each KvK number is checked exactly once.
- `TestRecheckLease_CrashedHolder`: a lease taken and not completed → after
  `lease_until` passes, another instance processes the row.
- `TestRecheckLease_Fairness`: 1,000 pending profiles plus 1 active profile checked 31 days
  ago → the active one is re-checked in the first run.
- `TestRecheckLease_OutageKeepsStatus`: the register returns errors → no
  `registration_status` changes, and `recheck_next_attempt_at` backs off 1 min, 2 min,
  4 min, …, capped at 1 h.

Manual two-server check with compose: run a second backend on another port against the same
database (`docker compose run --rm -e PORT=8090 backend`). Onboard one cook with the fake
register's pending path, and watch the fake register's log: one check per due profile, not
two.

# Implementation Plan: Accounts & Sign-in

**Branch**: `001-accounts-sign-in` | **Date**: 2026-10-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-accounts-sign-in/spec.md`

## Summary

Give eyho real identities. People sign in through Auth0 (Google or email + password) using a
standards-based OIDC client in the React app; the Go backend verifies access tokens against the
issuer's keys and maps each identity to an `Account` that owns a buyer profile and, after
onboarding, a cook profile. Browsing stays public; claiming, listing, rating and order history
require a signed-in account with a verified email. Locally, a mock OIDC server in Docker Compose
replaces Auth0; in the demo and locally, a small demo issuer inside the backend provides one-tap
seeded accounts. Account deletion anonymises the cook's trust record instead of deleting it.

First feature in the roadmap: it depends on nothing later, and features 002–004 build on its
foundational layer (accounts, middleware, mock server).

## Technical Context

**Language/Version**: Go 1.22+ (backend); TypeScript 5.x + React 18 (frontend) — same as 002

**Primary Dependencies**:
- Backend: `github.com/coreos/go-oidc/v3` (token verification, JWKS caching),
  `github.com/go-jose/go-jose/v4` (demo issuer signing), Auth0 Management API over plain HTTP
  (deletion, verification-email jobs); existing `chi`, `pgx`, `sqlc`, `golang-migrate`
- Frontend: `oidc-client-ts` + `react-oidc-context`
- Local: `ghcr.io/navikt/mock-oauth2-server` container
- Address search: PDOK Locatieserver (free Dutch address API, no key) for cook onboarding

**Storage**: PostgreSQL 16 + PostGIS (existing); migration `000002_accounts` (see data-model.md)

**Testing**: Go `testing` + `testify` with tokens minted by a test issuer; Vitest for auth UI
states; Playwright against Docker Compose using the mock server's login page

**Target Platform**: responsive web (same as 002); Auth0 tenant in an EU region

**Project Type**: web application (frontend + backend)

**Performance Goals**: token verification adds < 5 ms p95 per request (local JWKS cache);
sign-in round trip from Claim back to the claim dialog < 60 s with Google (SC-001)

**Constraints**: no passwords stored by eyho (FR-002); `docker compose up` must work with no
provider credentials (FR-015); demo issuer and mock issuer must be impossible to enable outside
`demo`/`local` (FR-017); €0/month at pilot scale (SC-006)

**Scale/Scope**: pilot, low thousands of accounts, single metro area

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.* Constitution v3.0.0.

| Principle | Check | Status |
|---|---|---|
| I. Crowd-Sourced Verification & Trust | New cooks start `unverified` (FR-010); cooks can't claim or rate their own listings (FR-011, constitution v3.0.0); deletion keeps verification history in anonymised form (FR-013) | PASS |
| II. Waste-Reduction Impact Tracking | Not affected; impact records stay attached to claims after account deletion | PASS |
| III. Community Collaboration | Not affected | PASS |
| IV. Gamified Engagement | Not affected | PASS (n/a) |
| V. Low Price, Low Friction | Browsing never requires an account (FR-003); sign-in only at Claim, returning the buyer to the same claim (FR-005); Google one-tap path | PASS |
| Product Constraints (minimal personal data) | Buyers: email and display name only. Cooks: phone and default pickup address, both needed to coordinate pickups; phone not verified (no SMS provider). All erased on deletion | PASS |
| Local Development Constraint | Mock OIDC server added to `docker-compose.yml` with this feature; no provider keys needed locally | PASS |
| Development Workflow | Changes to Principle I behaviour (own-listing guard, anonymisation) reviewed before merge | PASS |

Post-design re-check: PASS. One justified complexity item below.

## Project Structure

### Documentation (this feature)

```text
specs/001-accounts-sign-in/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── accounts-api.md  # Phase 1
└── tasks.md             # Phase 2
```

### Source Code (repository root)

```text
backend/
├── src/
│   ├── auth/               # token verifier (go-oidc), demo issuer, env safeguards
│   ├── api/
│   │   ├── middleware/     # Authenticate, RequireAccount, RequireVerifiedEmail, RequireCook
│   │   └── handlers/       # me.go, demo_auth.go
│   ├── services/           # account_service.go (provisioning, linking, onboarding, deletion)
│   ├── idp/                # Auth0 Management API client (delete user, verification email)
│   └── db/
│       ├── migrations/     # 000002_accounts.{up,down}.sql
│       └── queries/        # accounts.sql
└── tests/
    └── integration/        # linking, deletion, demo safeguard

frontend/
├── src/
│   ├── auth/               # OIDC provider setup, useAccount(), RequireSignIn, sign-in page
│   ├── pages/              # AccountPage (settings, delete), CookOnboardingPage, AuthCallback
│   └── components/         # header account menu
└── tests/

docker-compose.yml          # + mock-oidc service
```

**Structure Decision**: same frontend/backend split as 002. Auth code lives in its own packages
(`backend/src/auth`, `frontend/src/auth`) so 002's handlers only depend on middleware and an
`AccountFromContext` helper.

## Complexity Tracking

| Deviation | Why needed | Simpler alternative rejected because |
|---|---|---|
| A second token issuer (demo issuer) inside the backend for `demo`/`local` | One-tap demo accounts on a single free backend instance (feature 004), without enabling Auth0's discouraged password grant | Seeding Auth0 users with shared passwords exposes a credential pattern in a public repo and still needs typing; a separate demo auth service costs a second hosted service |

# Implementation Plan: Post & Claim Surplus Food Listing

**Branch**: `001-surplus-food-listing` | **Date**: 2026-08-03 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-surplus-food-listing/spec.md`

## Summary

Enable cooks (non-profit or professional) to publish surplus-food listings at a reduced price —
flagged "unverified" until they earn crowd-sourced verified status — and enable buyers to discover
nearby listings and claim portions with in-app payment. The
technical approach is a standard web application: a Go backend service exposing a REST API backed
by PostgreSQL (with PostGIS for proximity/local-discovery queries), and a React web frontend.
Stripe handles payment capture and payout to cooks. Listing lifecycle (publish → expire/cancel)
and claim lifecycle (reserve → paid → collected/no-show) are modeled as explicit state machines to
satisfy the no-overselling and impact-tracking requirements from the spec.

## Technical Context

**Language/Version**: Go 1.22+ (backend API); TypeScript 5.x + React 18 (web frontend)

**Primary Dependencies**:
- Backend: `chi` (HTTP routing), `pgx` (PostgreSQL driver), `sqlc` (typed SQL codegen), official
  `stripe-go` SDK, `golang-migrate` for schema migrations
- Frontend: React 18 + Vite, React Router, a lightweight data-fetching layer (e.g. TanStack Query)

**Storage**: PostgreSQL 16 with the PostGIS extension (for radius-based "local discovery" queries
required by Principle III and FR-011)

**Testing**: Go `testing` + `testify` for backend unit/integration tests; Vitest for frontend unit
tests; Playwright for end-to-end claim-flow tests

**Target Platform**: Responsive web app (desktop + mobile browsers) for v1; native mobile apps are
an explicit future phase, not part of this feature

**Project Type**: Web application (frontend + backend)

**Performance Goals**: Discovery view returns results in under 1s at p95 for a typical local
radius query (supports SC-002: buyer claims within 90 seconds); claim/checkout API responds in
under 500ms at p95

**Constraints**: Claim decrement must be atomic under concurrent requests (FR-006, zero
overselling per SC-003); payment reservation timeout enforced server-side (FR-008); no offline
support required for v1

**Scale/Scope**: Pilot scope — single metro area, low thousands of listings/claims per week;
architecture should not block scaling to multiple cities later, but multi-region infra is out of
scope for v1

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Crowd-Sourced Verification & Trust | Listing-creation endpoint allows any cook to publish (FR-001); every listing/read response includes current verification status — with an explicit "unverified" flag — plus rating and review count (FR-004); `CookProfile`/`VerificationHistory` track promotion (5 good) and demotion (3 bad) transitions and cycle count per data-model.md | PASS |
| II. Waste-Reduction Impact Tracking | `ImpactRecord` is generated server-side on every completed claim (FR-009), derived from real transaction data, not client-supplied | PASS |
| III. Community Collaboration & Social Cohesion | PostGIS-backed radius query makes local discovery the default view (FR-011); non-profit cooks are not down-ranked by the discovery query | PASS |
| IV. Gamified Engagement | Not addressed by this feature — deferred to a later feature (impact badges/streaks). No violation: this feature does not preclude adding it. | PASS (deferred) |
| V. Low Price, Low Friction | Reduced-price validation against retail reference price enforced server-side (FR-003); single-flow claim+payment via Stripe minimizes checkout steps (FR-005) | PASS |
| Product Constraints | Stripe is an established payment processor (no custom payment handling); only minimal PII (contact + pickup location) collected; PostGIS location data used solely for discovery | PASS |

No violations requiring Complexity Tracking justification.

## Project Structure

### Documentation (this feature)

```text
specs/001-surplus-food-listing/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
backend/
├── src/
│   ├── models/          # Listing, Claim, CookProfile, VerificationHistory, BuyerProfile, ImpactRecord
│   ├── services/        # listing lifecycle, claim/reservation logic, impact aggregation, verification promotion/demotion
│   ├── api/              # HTTP handlers (chi routes), Stripe webhook handler
│   └── db/                # sqlc-generated queries, migrations (golang-migrate)
└── tests/
    ├── contract/         # API contract tests against /contracts/*
    ├── integration/      # claim-race / overselling / expiry scenarios
    └── unit/

frontend/
├── src/
│   ├── components/      # ListingCard, ClaimFlow, DiscoveryMap/List
│   ├── pages/             # Discovery, ListingDetail, CookDashboard
│   └── services/          # API client, Stripe Elements integration
└── tests/
    ├── unit/
    └── e2e/               # Playwright: publish → discover → claim → pay
```

**Structure Decision**: Standard web application split (Option 2: frontend + backend), matching
the "Both, web first" platform decision. `backend/` and `frontend/` are separate deployable units
communicating over the REST API defined in `contracts/`.

## Complexity Tracking

*No Constitution Check violations — table intentionally left empty.*

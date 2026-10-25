# Phase 0 Research: Post & Claim Surplus Food Listing

## Backend language: Go

**Decision**: Go 1.22+ with `chi` router, `pgx`/`sqlc` for data access.

**Rationale**: User-specified. Go gives predictable performance for concurrent claim requests
(goroutines make it straightforward to implement the atomic decrement/reservation logic safely),
a single static binary simplifies deployment for a small pilot team, and its standard library
covers most of the API surface needed without heavy framework overhead.

**Alternatives considered**: Node.js/TypeScript (rejected — user preferred Go); Python/FastAPI
(rejected for same reason; would have been a fine alternative for team familiarity/speed).

## Database: PostgreSQL + PostGIS

**Decision**: PostgreSQL 16 with the PostGIS extension.

**Rationale**: User-specified PostgreSQL. The feature's core discovery requirement (FR-011,
Principle III — local-radius discovery) needs efficient geospatial "find listings within N km"
queries. PostGIS is the standard, well-supported way to do this in Postgres (`ST_DWithin` on a
`geography` column, GiST index) without needing a separate search/geo service for a pilot-scale
app.

**Alternatives considered**: Plain lat/lng with bounding-box math (rejected — accurate radius
queries and future filtering by area are harder to get right and slower without spatial
indexing); a managed geo-search service like Elasticsearch/Algolia (rejected — unnecessary
operational overhead at pilot scale).

## Payments: Stripe (with Connect for cook payouts)

**Decision**: Stripe for buyer checkout, Stripe Connect (Express accounts) for paying out
non-profit/professional cooks.

**Rationale**: User-specified Stripe. Satisfies the constitution's "established payment
processor" constraint. Stripe Connect is purpose-built for marketplace models where a platform
facilitates payment between a buyer and many independent sellers (cooks), including split
payments/platform fees, which fits the low-fee requirement (Principle V) and avoids building
custom money-movement logic.

**Alternatives considered**: Building custom payment handling (explicitly disallowed by the
constitution's Product Constraints section); other processors (Adyen, Braintree) — viable but
Stripe has the most mature marketplace/Connect tooling and documentation, reducing integration
risk for a pilot.

## Concurrency control for claims (no overselling)

**Decision**: Decrement `portions_remaining` using a single atomic SQL statement
(`UPDATE listings SET portions_remaining = portions_remaining - $qty WHERE id = $id AND
portions_remaining >= $qty`), checking rows-affected to determine success, wrapped in the same
transaction as creating the `Claim` row.

**Rationale**: Satisfies FR-006 and SC-003 (zero overselling) without needing external locking
infrastructure (e.g., Redis locks). PostgreSQL's row-level locking on the `UPDATE` makes this
correct under concurrent claim attempts.

**Alternatives considered**: Application-level locking/mutex (rejected — doesn't work across
multiple backend instances); optimistic concurrency with version column and retry loop (viable
alternative, more complex than needed here since the conditional `UPDATE` already gives atomicity).

## Listing expiry and payment-timeout release

**Decision**: A scheduled background job (Go ticker or cron-triggered endpoint) runs every minute
to: (a) mark listings as `expired` once their pickup window has passed, and (b) release any
`Claim` still in `pending_payment` past its reservation timeout (default 10 minutes) back to
available inventory.

**Rationale**: Satisfies FR-007 and FR-008 with a simple, observable mechanism appropriate for
pilot scale; avoids needing a separate job-queue system.

**Alternatives considered**: Database-level `pg_cron` (viable, considered simpler for ops once
infra is finalized — noted as a future optimization, not required for v1).

## Frontend: React + Vite (web first)

**Decision**: React 18 + Vite for the web frontend, consumed via the REST API.

**Rationale**: Matches the "both, web first" platform decision. React is broadly supported,
integrates cleanly with Stripe's official Elements/Checkout components, and keeps the door open
to reusing components/patterns if a React Native mobile app follows later.

**Alternatives considered**: Server-rendered framework (e.g., Next.js) — reasonable alternative,
deferred since this feature doesn't yet require SSR/SEO; plain SPA with Vite is simpler to start.

## Discovery sort order

**Decision**: non-profit cooks first, then by distance (maintainer decision, 2026-10-25). The
existing `ListNearbyListings` query already orders this way; spec Acceptance Scenario 1 was
updated to match.

**Rationale**: Principle III requires equal or preferential visibility for non-profits.

**Alternatives considered**: pure distance (simplest, but gives non-profits no preference);
distance with non-profits winning ties (preference too weak to matter in practice).

## How ratings count towards verification

**Decision**: each buyer's rating of a collected claim counts separately (maintainer decision,
2026-10-25; constitution amended to v3.0.0). Promotion after 5 ratings of 4+ stars, demotion after
3 ratings of 2 or fewer, counters reset on every transition. `VerificationHistory` records the
triggering claim IDs.

**Rationale**: simplest rule to explain to cooks and evaluate immediately on each rating; no need
to decide when a multi-buyer listing's rating is "final".

**Risk**: a single popular listing can promote a cook in one evening, and a cook could ask friends
to claim and rate. Mitigations: cooks cannot claim (and so cannot rate) their own listings
(feature 001 FR-011); the reporting mechanism deferred in plan.md remains the backstop.

## Outcome

All unknowns from the Technical Context are resolved; no remaining `NEEDS CLARIFICATION` markers.

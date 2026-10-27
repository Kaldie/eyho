<!--
Sync Impact Report
- Version change: 3.1.0 → 3.1.1 (PATCH — wording of the Local Development Constraint)
  "schema migrations" → "schema loading": there is no production database yet, so the schema is a
  single file loaded by the db service on first start instead of a migration chain. Decided by the
  maintainer on 2026-10-26. Migrations return once a production database exists.
- Version change: 3.0.0 → 3.1.0 (MINOR — materially expanded Product Constraints)
  Added: cooks MUST pass an automatic business-registration check (active KvK registration plus a
  recorded NVWA registration declaration) before publishing. This is a legal floor, kept separate
  from the reputation-based "verified" status, and is instant so cooks still onboard immediately.
  Decided by the maintainer on 2026-10-25. Follow-up: spec 001 (FR-018 to FR-023), spec 002
  (FR-001, listings contract) and spec 004 demo data updated in the same change.
- Version change: 2.1.0 → 3.0.0 (MAJOR — redefinition of Principle I's verification thresholds)
  Modified: promotion/demotion now count individual buyer ratings rather than listings. Each
  buyer's rating of a collected claim counts separately, so one listing with several buyers can
  contribute several ratings. Decided by the maintainer on 2026-10-25 while planning feature 002.
  Follow-up: spec 002 FR-013, data-model and quickstart updated in the same change.
- Version change: 2.0.0 → 2.1.0 (MINOR — added Local Development Workflow constraint)
  Added: local Docker Compose stack is the required way to run and test the full stack locally.
  Every service (db, migrations, backend, frontend) must be startable via `docker compose up`.
- Version change: 1.0.0 → 2.0.0 (MAJOR — backward-incompatible redefinition of Principle I)
- Modified principles:
  - I. Trust & Verification (NON-NEGOTIABLE) → I. Crowd-Sourced Verification & Trust (NON-NEGOTIABLE)
    Previously required upfront verification before any listing could go public. Now allows
    unverified cooks to list (clearly flagged), with verification status earned/lost via a
    reputation loop, plus a permanent per-cook history requirement.
- Added sections: none (existing sections updated in place)
- Removed sections: none
- Follow-up TODOs:
  - Feature 002 (specs/002-surplus-food-listing) was written under the prior all-or-nothing
    verification model (FR-001, FR-004) and must be reconciled with this amendment before
    /speckit-tasks proceeds — see Next Actions below.
-->

# Eyho Constitution

## Core Principles

### I. Crowd-Sourced Verification & Trust (NON-NEGOTIABLE)
Cooks (non-profit or professional) MAY publish listings before earning verified status, but every
listing and profile from an unverified cook MUST be clearly flagged as "unverified" to buyers.
Verification status MUST be reputation-driven rather than a one-time gate:
- An unverified cook MUST become verified after accumulating **5** buyer ratings of 4 stars or
  higher. Each buyer's rating of a collected claim counts separately.
- A verified cook MUST revert to unverified — retriggering the verification process — after
  accumulating **3** buyer ratings of 2 stars or lower.
- A cook MUST NOT be able to rate their own listings, directly or by claiming them.
- The system MUST retain a permanent, per-cook history of every verification status transition
  (timestamp, direction, and the ratings that triggered it) and of how often a cook has cycled
  between verified and unverified. This history MUST be available to internal review and
  reflected in what buyers see (at minimum, current status; a transition summary SHOULD be
  visible).
Buyers MUST be able to see a cook's current verification status, rating history, and review
count before claiming a listing. A reporting/flagging mechanism for food-safety or trust
violations MUST exist independently of the rating-based mechanism, and flagged listings or
accounts MUST receive a response within a defined SLA.
Rationale: strangers are exchanging home- and kitchen-made food, so trust and safety remain
foundational — but gating every new non-profit/professional cook behind a manual verification
step before they can contribute conflicts with Principle V (low friction) and slows the platform's
core mission. A transparent, self-correcting, and fully auditable reputation loop lets good cooks
onboard immediately while still protecting buyers and catching bad actors quickly. The automatic
business-registration check in Product Constraints is a legal precondition, not a verification
step: it involves no human review and does not grant or affect "verified" status.

### II. Waste-Reduction Impact Tracking
Every completed transaction MUST be tied to a quantifiable waste-reduction metric (e.g., portions
saved, kilograms of food, estimated CO2e avoided). Impact data MUST be visible both at the
individual level (a user's personal impact) and aggregated publicly (community/platform impact).
Impact metrics MUST be derived from real transaction data and MUST NOT be presented as estimates
disguised as fact.
Rationale: reducing food waste is the platform's core mission; if impact is not measured and
surfaced, the value proposition is unproven and unmotivating.

### III. Community Collaboration & Social Cohesion
Features MUST support connection between neighbors and community members, not just anonymous
transactions (e.g., local pickup points, community cook profiles, shared events). The platform
SHOULD favor local, neighborhood-level discovery over broad marketplace browsing. Non-profit and
community organizations MUST receive equal or preferential visibility versus professional/
commercial cooks.
Rationale: the mission extends beyond moving surplus food — it includes strengthening ties within
communities.

### IV. Gamified Engagement
Recurring participation (posting, claiming, reviewing) SHOULD be reinforced through gamification
(streaks, badges, leaderboards, impact milestones). Gamification MUST reinforce the core mission
(waste reduction, community trust) and MUST NOT incentivize unsafe behavior, such as rushing
food-safety checks for points or rewards.
Rationale: sustained engagement is required for a two-sided marketplace to reach critical mass;
game mechanics help build the habit without undermining safety.

### V. Low Price, Low Friction
Listed prices MUST remain significantly below retail/menu price — the reduced price reflects
surplus/waste-prevention, not a general discount marketplace. The checkout and claiming flow MUST
be completable in a minimal number of steps, and buyer onboarding MUST NOT require lengthy
signup before browsing. Platform fees/commissions MUST be kept minimal to preserve affordability
for buyers and viability for non-profit cooks.
Rationale: the model only works if price and friction both stay low enough to beat "throw it
away" for the cook and "buy at full price" for the buyer.

## Product Constraints

All listings MUST comply with applicable local food-safety and food-handling regulations;
compliance is a legal floor, not a competitive feature. Before a cook can publish any listing, the
system MUST confirm automatically that the cook's KvK (Dutch Chamber of Commerce) number belongs
to an active registration, and MUST record the cook's declaration that the kitchen is registered
with the NVWA (Dutch food safety authority) and follows a hygiene code. Cooks without an active
registration MUST NOT be able to publish. The registration status MUST be re-checked periodically,
and MUST NOT be presented to buyers as, or confused with, "verified" status. Personal data (identity, location,
payment details) MUST be handled with minimal collection and clear purpose — only what is needed
to enable safe, trustworthy transactions. Payment flows MUST use an established payment
processor rather than custom-built payment handling. Location data MUST be used to support local
discovery (Principle III) and MUST NOT be sold or shared with third parties for unrelated
purposes.

## Development Workflow

This project follows Spec-Driven Development via Spec Kit: every feature begins with a spec
(`/speckit-specify`), followed by a plan (`/speckit-plan`), a task breakdown (`/speckit-tasks`),
and implementation (`/speckit-implement`). Specs and plans MUST be checked against this
constitution before implementation begins; a feature that conflicts with a Core Principle MUST
either be revised or the conflict explicitly justified and recorded in the plan's Complexity
Tracking section. Code changes affecting Trust & Verification (Principle I) or Product
Constraints MUST be reviewed before merge.

### Local Development Constraint (NON-NEGOTIABLE)

The project MUST be fully runnable locally via a single `docker compose up` command. Every
service — PostgreSQL + PostGIS (including schema loading), Go backend, React frontend — MUST be
represented as a service in `docker-compose.yml` at the repository root. This is the canonical
way to run, test, and demo the full stack locally; no manual environment setup beyond copying
`.env.example` and providing real Stripe test keys should be required. Any new service or
infrastructure dependency added to the project MUST be added to `docker-compose.yml` at the
same time it is introduced. CI and QA validation scenarios MUST be executable against the
Docker Compose stack.

## Governance

This constitution supersedes other project practices. Amendments require: (1) a documented
rationale for the change, (2) a version bump following semantic versioning — MAJOR for backward
incompatible principle removals/redefinitions, MINOR for new principles or materially expanded
guidance, PATCH for clarifications and wording fixes — and (3) an update to the Sync Impact
Report at the top of this file. All specs and plans MUST verify compliance with this constitution;
unjustified complexity or principle violations MUST be resolved before implementation proceeds.

**Version**: 3.1.1 | **Ratified**: 2026-08-01 | **Last Amended**: 2026-10-26

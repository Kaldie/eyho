# Feature Specification: Private Demo Deployment

**Feature Branch**: `004-demo-deployment`

**Created**: 2026-10-25

**Status**: Draft

**Input**: User description: "Add a feature that deploys the thing." Scope settled in a Q&A session:
private demo/staging only, near-zero hosting cost, manually triggered deploys, a single password
in front of the whole site, a free platform-provided web address, Amsterdam demo data reset on
every deploy, and simulated payments.

**Depends on**: features 001 (accounts & sign-in, including one-tap demo accounts) and 002 (post
& claim surplus food) being implemented in the backend. This feature MUST NOT be implemented until
their APIs are functional — deploying the current stub backend has no demo value (decision
recorded 2026-10-25).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer deploys the demo on demand (Priority: P1)

The maintainer wants to put the current state of the project online so others can try it. From
the repository's hosting page they start a deploy with one action, choosing which branch to
deploy. The deploy updates the database schema, the backend and the frontend, and reports success
or failure with a link to the live demo.

**Why this priority**: Without a repeatable, one-action deploy there is no demo at all. Everything
else in this feature builds on it.

**Independent Test**: Trigger a deploy of `main`, wait for it to finish, open the reported link and
confirm the discovery page loads data served by the deployed backend (not frontend mock data).

**Acceptance Scenarios**:

1. **Given** the deploy action has been started for a branch, **When** all steps succeed, **Then**
   the demo serves that branch's frontend and backend, the database schema matches that branch's
   migrations, and the run summary shows the demo's web address.
2. **Given** a deploy is started, **When** any step fails (schema migration, backend, frontend),
   **Then** the run is marked failed, later steps do not run, and the summary names the failed step.
3. **Given** a deploy is already running, **When** a second deploy is started, **Then** the second
   waits for the first to finish rather than running in parallel.
4. **Given** nobody starts a deploy, **When** commits are pushed to any branch, **Then** the demo is
   not changed (deploys are manual only).

---

### User Story 2 - Invited visitor tries the demo behind a password (Priority: P1)

Someone the maintainer shared the link and password with opens the demo. They are asked for the
password once, then can browse Amsterdam listings, view details and complete a claim with a
simulated payment, exactly as a real buyer would.

**Why this priority**: The demo exists to be shown to people. It has to be private (strangers
exchanging food is a trust-sensitive domain, and this is not a launch) and it has to work
end-to-end.

**Independent Test**: Open the demo in a fresh browser, enter the password, browse listings, claim
a portion and reach the order confirmation without any real payment details.

**Acceptance Scenarios**:

1. **Given** a visitor without the password, **When** they open any page or call any API path of the
   demo, **Then** they are refused access and see nothing of the app's content or data.
2. **Given** a visitor who enters the correct password, **When** they browse, **Then** they are not
   asked again for the rest of their browser session.
3. **Given** a visitor claiming portions, **When** they reach payment, **Then** the payment step
   clearly says it is simulated, completes without any card details, and the claim proceeds exactly
   as if payment had succeeded (impact record created, portions decremented).
4. **Given** a visitor who knows the backend's direct platform address, **When** they call it
   directly, bypassing the password-protected site, **Then** the request is refused.
5. **Given** the demo has been idle long enough for the backend to go to sleep, **When** the next
   visitor arrives, **Then** they see a friendly loading state rather than an error while the
   backend wakes.

---

### User Story 3 - Demo always starts from realistic Amsterdam data (Priority: P2)

Every deploy wipes the demo database and loads a fixed, realistic data set: Amsterdam cooks
(non-profit and professional, verified and unverified), active listings within walking distance of
each other with pickup windows relative to the time of deploy, and a few buyers with past orders.

**Why this priority**: An empty marketplace demos badly, and a demo full of leftover test clutter
demos worse. A known starting point also makes the demo repeatable for walkthroughs.

**Independent Test**: Add a listing in the demo, redeploy, and confirm the added listing is gone and
the standard demo data set is back, with pickup windows that are still in the future.

**Acceptance Scenarios**:

1. **Given** a successful deploy, **When** the demo is opened, **Then** it shows the standard demo
   listings, all currently claimable, around a default Amsterdam location.
2. **Given** visitors changed data in the demo, **When** the next deploy runs, **Then** all of that
   data is replaced by the standard demo data set.
3. **Given** the demo data set, **When** a visitor browses, **Then** they can see at least one
   unverified cook (flag visible), one non-profit cook, and listings with and without photos.

---

### Edge Cases

- What if the demo data's pickup windows have all passed since the last deploy? Listings expire
  normally; the demo simply shows fewer or no listings until the next deploy. The empty state from
  feature 003 MUST be shown, and the maintainer can redeploy to refresh.
- What if a schema migration fails half-way? The deploy MUST stop before the new backend is
  released, so the running backend never sees a schema it does not expect. Because demo data is
  reset every deploy, recovering by re-running the deploy is acceptable; no rollback tooling is
  required.
- What if a free-tier usage limit is hit (database compute hours, backend instance hours,
  bandwidth)? The demo may become unavailable until the limit resets. This is accepted for a
  private demo; the run summary or a documented check SHOULD make the cause discoverable.
- What if the simulated-payment setting is somehow enabled outside the demo? The backend MUST refuse
  to start (see FR-008).
- What if the password needs changing? Changing it in the deploy secrets and redeploying MUST be
  sufficient; no code change.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST provide a manually triggered deploy that the maintainer can start
  from the repository's hosting page, with the branch to deploy as its only input.
- **FR-002**: A deploy MUST, in order: apply database schema migrations, reset and load the demo
  data set, release the backend, then release the frontend. Any failing step MUST stop the deploy.
- **FR-003**: Only one deploy MAY run at a time; a newly started deploy MUST wait for a running one.
- **FR-004**: All secrets (database connection, password, proxy secret, platform credentials) MUST
  be stored in the repository's secret store, never in the repository's files or deploy logs.
- **FR-005**: Every page and API path of the demo MUST require a single shared password, prompted
  once per browser session.
- **FR-006**: The frontend and the API MUST be served from the same web address, so the password
  applies to both and the browser makes no cross-origin calls.
- **FR-007**: The backend MUST reject any request that did not come through the password-protected
  site, using a shared secret that only the site and the backend know.
- **FR-008**: The backend MUST support a simulated-payment mode that completes claims as paid
  without contacting a payment processor. The mode MUST be off by default, MUST only be honored
  when the backend is explicitly running as the demo environment, and the backend MUST refuse to
  start if the mode is on in any other environment.
- **FR-009**: When simulated payments are on, the checkout UI MUST state clearly that no real
  payment is taken.
- **FR-010**: The demo data set MUST be defined in the repository, MUST be loadable by the same
  mechanism locally (Docker Compose) and in the demo, and MUST compute pickup windows relative to
  the moment it is loaded.
- **FR-011**: The demo data set MUST include, around Amsterdam: at least 3 cooks covering
  non-profit/professional and verified/unverified, at least 8 active listings within a few
  kilometers of a default buyer location, at least 2 buyers with completed past orders and
  ratings, and at least one listing with no photo. Every seeded cook MUST have an active KvK
  registration from KVK's free test environment and a recorded NVWA declaration (feature 001,
  FR-023), and the demo backend MUST check new cooks against that test environment, so the
  registration gate can be shown with fictitious companies at no cost.
- **FR-012**: The frontend MUST show a loading state while the backend is waking from idle, rather
  than an error, for up to the platform's documented wake-up time.
- **FR-013**: Local development MUST be unaffected: `docker compose up` MUST keep working with no
  password, no proxy secret and real Stripe test-mode payments (Constitution, Local Development
  Constraint).
- **FR-014**: The deploy run summary MUST show the demo's web address and the deployed commit.

### Key Entities

- **Deploy run**: One manual execution of the deploy. Attributes: branch, commit, start/end time,
  per-step outcome, demo address.
- **Demo data set**: Version-controlled seed definition of cooks, buyers, listings, claims and
  ratings, with times expressed relative to load time.
- **Demo environment configuration**: The set of secrets and settings that make an environment
  "the demo" (environment name, simulated-payment flag, password, proxy secret, database
  connection).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The maintainer can go from "start deploy" to a working demo link in under 10 minutes,
  with no manual steps outside starting the deploy.
- **SC-002**: Ongoing hosting cost is €0 per month at demo usage levels.
- **SC-003**: 100% of requests without the password, and 100% of direct backend requests that
  bypass the site, are refused.
- **SC-004**: A first-time visitor with the password can complete a simulated claim in under 2
  minutes, including any backend wake-up time.
- **SC-005**: After every successful deploy, the demo shows the full standard demo data set with all
  listings claimable.

## Assumptions

- **Audience**: private demo/staging for invited people only; not a pilot or public launch. A real
  launch will need its own feature (proper accounts, real payments, always-on hosting).
- **Hosting (decided 2026-10-25, near-zero budget)**: frontend and password gate on Cloudflare
  Pages using its free `*.pages.dev` address; backend on a Render free web service built from the
  existing backend container definition; database on a Neon free project with PostGIS. Render's
  own free Postgres was rejected because it expires after 30 days. The plan phase MUST re-check
  current free-tier limits before committing.
- **Accepted trade-offs**: the backend sleeps after a period of inactivity and takes about a minute
  to wake; free-tier monthly limits can make the demo temporarily unavailable.
- **Payments**: simulated in the demo instead of using Stripe test mode. This deviates from the
  constitution's Product Constraint that payments use an established processor; it is acceptable
  only because no real money or real buyers are involved, and the plan MUST record it under
  Complexity Tracking with FR-008's safeguard.
- **Webhooks**: with simulated payments the demo receives no payment-processor webhooks, so FR-007
  needs no exception for them.
- **Access control**: a single shared password gates the whole site; no audit of who viewed the
  demo. Inside the demo, people sign in with one tap as seeded demo accounts (feature 001,
  FR-016), which are part of the demo data set reset on every deploy.
- **Data**: no demo data is precious; wiping on every deploy is intended.
- **Out of scope**: custom domains, automatic deploys on push, production or pilot environments,
  monitoring/alerting, backups, and rollback tooling.

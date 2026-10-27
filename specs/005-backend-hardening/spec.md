# Feature Specification: Backend Hardening

**Feature Branch**: `005-backend-hardening`

**Created**: 2026-10-26

**Status**: Draft

**Input**: User description: the server-side architectural concerns from the 2026-10-26
security and architecture review, captured as a feature instead of piecemeal fixes
(maintainer decision, 2026-10-26). Scope: authorization that lives with each resource; a
pipeline that runs every check, including the database tests, on every change; cheap,
side-effect-free identification of the caller on every request; focused account services; and
background jobs that are safe with more than one server. The browser-side and delivery
concerns from the same review are feature 006.

**Position in the roadmap**: builds on 001 (accounts and sign-in) and on the endpoints
scaffolded for 002. It adds no product behaviour. It sets the patterns feature 002 must follow
for every endpoint, so User Story 1 should land before 002's listing and claim endpoints do.
The review's directly exploitable findings (server timeouts, maximum token age, key
refetching, outdated software) were fixed on 2026-10-26 and are not part of this feature.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Only the owner can act on a listing or claim (Priority: P1)

A signed-in person can see and change only what is theirs: a cook changes or cancels only
their own listings; a buyer sees, confirms pickup for and rates only their own claims; a cook
sees the claims on their own listings. Anyone else who is signed in gets a "not found" answer
that does not reveal whether the item exists. Every new endpoint follows the same rule, and a
shared test pattern proves it.

**Why this priority**: today the server only checks that someone is signed in on these routes
and leaves ownership to services that are not written yet. The moment feature 002 fills them
in, any signed-in user could change another cook's listing or read another buyer's order
unless ownership is enforced. It must exist before 002's endpoints do.

**Independent Test**: With two signed-in accounts A and B, A creates a listing and a claim. B
tries to read, change, cancel, confirm or rate them; every attempt is refused and nothing
changes. A can do all of it.

**Acceptance Scenarios**:

1. **Given** cook A's listing, **When** signed-in user B tries to change or cancel it,
   **Then** B gets "not found" and the listing is unchanged.
2. **Given** buyer A's claim, **When** signed-in user B asks for it, confirms its pickup or
   rates it, **Then** B gets "not found" and nothing changes.
3. **Given** cook A's listing with a claim by buyer C, **When** A views that claim, **Then** A
   sees it (the cook may see claims on their own listings) but cannot rate it.
4. **Given** any endpoint that acts on a listing or claim, **When** its automated tests run,
   **Then** they include the "other signed-in user is refused" case.

---

### User Story 2 - Every change is checked before it can merge (Priority: P1)

Every change runs the same checks a contributor runs locally: server build, static checks,
formatting, unit and database tests against a real geo-enabled database, web app type checks,
unit tests and build, and known-vulnerability scans of both dependency sets. A database test
can no longer pass by silently skipping.

**Why this priority**: today there is no pipeline. The database tests (account linking,
deletion, the registration gate) skip when no database is configured, so the most
security-critical behaviour has not been tested on any shared run. Every later story,
including feature 006, relies on this pipeline to prove it works.

**Independent Test**: Open a change that breaks a database test, one that is badly
formatted, and one that adds a dependency with a known vulnerability. Each is marked failing
with the reason.

**Acceptance Scenarios**:

1. **Given** a change, **When** the pipeline runs, **Then** all the listed checks run and the
   database tests actually execute.
2. **Given** the pipeline environment, **When** the database is missing, **Then** the
   database tests fail instead of skipping.
3. **Given** a shipped dependency with a known vulnerability, **When** the pipeline runs,
   **Then** it fails and names the dependency.

---

### User Story 3 - Identifying the caller is cheap and never writes (Priority: P2)

Checking who is calling costs one database lookup and never writes. Creating an account for a
new identity happens once, through an explicit and idempotent step. The account rules are
split into focused parts (sign-in and linking, cook onboarding and the business register,
deletion) that handlers use through narrow interfaces, and all server errors use one shape.

**Why this priority**: every authenticated request currently costs three or four database
queries and can write, even when it only reads. Feature 002 adds many authenticated
endpoints, so this is cheaper to change before them than after.

**Independent Test**: Count database statements for a signed-in read request: one lookup, no
writes. Send two concurrent first requests for a brand-new identity: exactly one account
results.

**Acceptance Scenarios**:

1. **Given** a known signed-in person, **When** they make a read-only request, **Then**
   identifying them takes one database lookup and makes no writes.
2. **Given** a first-ever sign-in, **When** the identity is new, **Then** the account is
   created exactly once, even with concurrent first requests.
3. **Given** any error from any endpoint, **When** the response is read, **Then** it has the
   one documented error shape.

---

### User Story 4 - Background jobs are safe with several servers (Priority: P3)

Background jobs (registration re-checks, identity-provider deletions) do each piece of work
exactly once when two or more servers run. Work held by a server that crashes is picked up
again later. A large backlog of pending registrations can no longer stop active cooks from
being re-checked.

**Why this priority**: invisible at today's single-server scale, but the registration
re-check would call the business register once per server, and the backlog ordering can
starve re-checks of active cooks. The demo (004) may run more than one server.

**Independent Test**: Run two servers with a shared database and a due registration re-check:
the register is called once. Fill the database with more pending cooks than one run
processes, and confirm an overdue active cook is still re-checked in that run.

**Acceptance Scenarios**:

1. **Given** two servers running, **When** a registration re-check is due, **Then** the
   business register is asked once and the result is stored once.
2. **Given** a server that crashes while holding a re-check, **When** its lease expires,
   **Then** another server performs it.
3. **Given** more pending registrations than one run handles, **When** an active cook's
   re-check is overdue, **Then** it is re-checked in the next run.

---

### Edge Cases

- An owner check runs against an anonymised (deleted) cook's listing: nobody can change it.
- A cook who is also the buyer of a claim (not possible today, since own-listing claims are
  refused) would still see it only once and could rate it only as the buyer.
- A first request for a brand-new identity arrives concurrently on two servers: one account
  results.
- A deleted account's identity is used again after its tombstone was purged: identification
  treats it as new, and the explicit creation step decides (existing 001 rules).
- A background job's server crashes mid-run: the work it held is picked up by another server
  after its lease expires, and is not lost or done twice.
- The pipeline's database cannot start: the run fails visibly, not green with skipped tests.

## Requirements *(mandatory)*

### Functional Requirements

**Authorization (US1)**

- **FR-001**: Every operation on a listing or claim MUST check, on the server, that the
  signed-in person is allowed to act on that specific item (cook: own listings and the claims
  on them; buyer: own claims) before reading or changing it.
- **FR-002**: A signed-in person who is not allowed MUST get the same "not found" answer as
  for an item that does not exist.
- **FR-003**: Ownership MUST be decided from the server's own records of who owns the item,
  never from identifiers sent by the client.
- **FR-004**: The project MUST provide a reusable test pattern for the "another signed-in
  person is refused" case. Every endpoint that acts on a listing or claim MUST use it.

**Pipeline (US2)**

- **FR-005**: Every change MUST run: server build, static analysis, formatting check, unit
  tests and database tests against a real geo-enabled database built from the schema file;
  web app type check, unit tests and production build; and known-vulnerability scans for both
  the server's and the web app's shipped dependencies.
- **FR-006**: In the pipeline, a database test MUST fail, not skip, when the database is
  unavailable.
- **FR-007**: The code base MUST pass the formatting check, and stale placeholder comments
  from the initial scaffold MUST be removed.

**Caller identification and structure (US3)**

- **FR-008**: Identifying the caller of a request MUST take at most one database lookup and
  MUST NOT write. Account creation for a new identity MUST be a separate, idempotent step
  that keeps all of 001's linking, tombstone and email-verification rules.
- **FR-009**: Account rules MUST be split into focused parts (sign-in and linking, cook
  onboarding and the business register, deletion). Request handlers MUST depend on narrow
  interfaces, not on one concrete service.
- **FR-010**: The server MUST answer errors in one documented shape, produced by one shared
  helper.

**Background jobs (US4)**

- **FR-011**: Every background job MUST take a time-limited lease on each piece of work it
  processes, so that with several servers each piece is processed once, and work held by a
  crashed server is retried after the lease expires.
- **FR-012**: The registration re-check MUST give overdue active and inactive cooks a fair
  share of each run, so pending registrations cannot starve them.

### Key Entities

- **Ownership**: the relation between a signed-in account and a listing (through its cook
  profile) or a claim (through its buyer profile). Authorization reads it from the server's
  records only.
- **Job lease**: a time-limited claim by one server on one piece of background work, such as
  a registration re-check or a provider deletion. It expires so another server can take over.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of listing and claim endpoints have an automated test proving that another
  signed-in person is refused, and all such tests pass.
- **SC-002**: Every merged change has passed all pipeline checks, and zero database tests are
  skipped in the pipeline.
- **SC-003**: A signed-in read request performs exactly one database lookup to identify the
  caller and zero writes.
- **SC-004**: With two servers running for 24 hours, every due background task is performed
  exactly once.
- **SC-005**: An overdue active cook is re-checked within one run even when pending
  registrations exceed what a run processes.

## Assumptions

- The pipeline runs on the repository's hosting provider. Its database is built from the
  single schema file (there are no migrations until a production database exists).
- Feature 002's endpoints are still stubs. This feature adds the ownership checks and their
  tests to the stubs' routes, and 002 implements the bodies behind them.
- This feature changes no behaviour visible to buyers or cooks.
- The 2026-10-26 fixes are done and out of scope.
- Not in scope: browser session, example data, production delivery (feature 006); rate
  limiting beyond what exists (to be specified when the demo is public).

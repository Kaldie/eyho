# Feature Specification: Architecture Hardening

**Feature Branch**: `005-architecture-hardening`

**Created**: 2026-10-26

**Status**: Draft

**Input**: User description: the architectural concerns from the 2026-10-26 security and
architecture review, captured as one feature instead of piecemeal fixes (maintainer decision,
2026-10-26). Scope: authorization that lives with each resource; cheap, side-effect-free
sign-in checks on every request; focused account services; background jobs that are safe with
more than one server; one way to answer and call the API, with no demo data in the shipped
app; one signed-in state in the browser that survives a reload; a production-ready delivery
(security headers, non-root containers, local-only ports, self-hosted fonts, trusted photo
hosts); and a pipeline that runs every check, including the database tests, on every change.

**Position in the roadmap**: builds on 001 (accounts and sign-in), 002 (listings and claims,
scaffolded), 003 (frontend) and 004 (demo deployment). It adds no new product behaviour. It
sets the patterns feature 002 must follow for every new endpoint and screen, so its first user
story should land before 002's claim and listing endpoints do. The review's directly
exploitable findings (server timeouts, token age, key refetching, outdated software) were
already fixed on 2026-10-26 and are not part of this feature.

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

### User Story 2 - One signed-in state in the browser that survives a reload (Priority: P1)

A person who signs in stays signed in when they reload the page or open a link in the same
tab, without the app storing their access token where page scripts can read it later. Real
sign-in and demo sign-in are one state with a clear rule: signing in one way ends the other.
Signing out removes everything the app loaded for that person.

**Why this priority**: today a reload signs the person out, which breaks the "sign in, then
continue your claim" journey from feature 001. A demo session left in the tab silently
overrides a real sign-in. Data loaded for one person can stay visible after switching
accounts.

**Independent Test**: Sign in, reload the account page, and confirm the account is still
shown. Sign in with a demo account, then with a real account: the real account is shown.
Sign out and confirm no previous person's orders or profile are visible.

**Acceptance Scenarios**:

1. **Given** a signed-in person, **When** they reload any page, **Then** they are still signed
   in and see the same page without being sent to sign-in.
2. **Given** a demo session in the tab, **When** the person completes a real sign-in,
   **Then** the real account is used and the demo session is ended.
3. **Given** a signed-in person, **When** they sign out, **Then** no data loaded for them
   remains in the app, and signing in as someone else shows only that person's data.
4. **Given** a signed-in person, **When** the browser's storage is inspected, **Then** no
   access token is stored there.

---

### User Story 3 - The shipped app only shows real data (Priority: P1)

Every screen that shows listings, claims or orders gets its data from the server, signed in
as the current person. The example data used during frontend development exists only in
tests, never in the app people download. Address lookups go through the same service layer
as all other calls.

**Why this priority**: today several screens return built-in example data. Every signed-in
person would see the same "order", and payment would start with a fake payment secret. The
prepared replacement calls also forget to send the sign-in, so they would fail as soon as
they are switched on.

**Independent Test**: Build the app and search it for the example listing and order names:
none are present. Open discovery, a listing, the claim flow and order history against the
server and confirm every call carries the person's sign-in when one exists.

**Acceptance Scenarios**:

1. **Given** the production build, **When** it is searched for the development example data,
   **Then** none is found.
2. **Given** a signed-in buyer, **When** they open order history, **Then** they see only
   their own orders, as returned by the server.
3. **Given** any server error, **When** a screen shows it, **Then** the same error shape and
   handling is used everywhere in the app.

---

### User Story 4 - Production delivery is locked down (Priority: P2)

An operator can deploy the app knowing that browsers are told which sources may run code or
show content, that other sites cannot frame the claim and payment pages, and that neither the
web app nor the server runs with administrator rights in its container. A visitor's address is
not sent to a font provider or to a photo host chosen by a cook. Developers' local stacks are
reachable only from their own machine.

**Why this priority**: there is no production image for the web app yet, and no security
headers anywhere, so this is needed before the demo deployment (004) is public. It is P2
because no production deployment exists today.

**Independent Test**: Start the production web app image. Confirm the security headers on
every page, that framing by another origin is refused, that the containers run as a
non-administrator user, and that loading a page contacts no third-party font or photo host.
Start the local stack and confirm its services cannot be reached from another machine on the
network.

**Acceptance Scenarios**:

1. **Given** the production web app, **When** any page is requested, **Then** the response
   carries a content security policy, a framing ban, a referrer policy and a no-sniff
   instruction.
2. **Given** another website, **When** it embeds the claim or payment page in a frame,
   **Then** the browser refuses to show it.
3. **Given** a listing whose photo address points outside the allowed image host, **When** it
   is saved, **Then** it is rejected; existing ones render the placeholder instead.
4. **Given** the local stack is running, **When** another machine on the same network tries
   its database, sign-in, server or web ports, **Then** the connection is refused.

---

### User Story 5 - Every change is checked before it can merge (Priority: P2)

Every change runs the same checks a contributor runs locally: server build, static checks,
formatting, unit and database tests against a real geo-enabled database, web app type checks,
unit tests and build, and known-vulnerability scans of both dependency sets. A database test
can no longer pass by silently skipping.

**Why this priority**: today there is no pipeline. The database tests (account linking,
deletion, the registration gate) skip when no database is configured, so the most
security-critical behaviour has not been tested on any shared run.

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

### User Story 6 - The server stays correct and fast as it grows (Priority: P3)

Checking who is calling costs one database lookup and never writes. Creating an account for a
new identity happens once, through an explicit and idempotent step. Account rules are split
into focused parts (sign-in and linking, cook onboarding and the business register, deletion)
that handlers use through narrow interfaces. Background jobs (registration re-checks,
provider deletions) do their work exactly once when two or more servers run. A large backlog
of pending registrations can no longer stop active cooks from being re-checked.

**Why this priority**: none of this is visible to users at today's scale, but each issue
becomes a bug or an outage once the demo runs more than one server or the cook count grows.
Doing it before 002 adds more callers is cheaper than after.

**Independent Test**: Count database statements for a signed-in read request (one lookup, no
writes). Run two servers with a shared database and a due registration re-check: the register
is called once. Fill the database with more pending cooks than one run processes and confirm
an overdue active cook is still re-checked in that run.

**Acceptance Scenarios**:

1. **Given** a known signed-in person, **When** they make a read-only request, **Then**
   identifying them takes one database lookup and makes no writes.
2. **Given** a first-ever sign-in, **When** the identity is new, **Then** the account is
   created exactly once, even with concurrent first requests.
3. **Given** two servers running, **When** a registration re-check is due, **Then** the
   business register is asked once and the result is stored once.
4. **Given** more pending registrations than one run handles, **When** an active cook's
   re-check is overdue, **Then** it is re-checked in the next run.

---

### Edge Cases

- A person signs in on two tabs and signs out in one: the other tab is signed out on its next
  request and shows no stale data.
- Silent session renewal fails, for example because the provider session expired: the person
  is shown as signed out with a sign-in prompt. No error page is shown and no data is lost
  mid-claim.
- An owner check runs against an anonymised (deleted) cook's listing: nobody can change it.
- A first request for a brand-new identity arrives concurrently on two servers: one account
  results.
- A background job's server crashes mid-run: the work it held is picked up by another server
  after its lease expires, and is not lost or done twice.
- The content security policy blocks something the app needs, such as the payment provider:
  a test exercises the claim and payment flow under the production policy.
- A listing photo was saved before the allowed-host rule existed: it renders the placeholder.

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

**Browser session (US2)**

- **FR-005**: The web app MUST hold exactly one signed-in state. Completing a real sign-in
  MUST end any demo session, and vice versa.
- **FR-006**: A signed-in person MUST stay signed in across page reloads in the same browser
  session, renewed silently with the identity provider, without storing access tokens in
  browser storage readable by page scripts.
- **FR-007**: Signing out or switching account MUST discard all data loaded for the previous
  person.
- **FR-008**: The current access token MUST reach the API layer through the app's state, not
  through a global value set while the screen is drawn.

**Data and API consistency (US3)**

- **FR-009**: Every web app call to the eyho server MUST go through one client that attaches
  the sign-in when present and turns errors into one error shape.
- **FR-010**: Development example data MUST NOT be part of the production build. It MUST live
  only with the tests.
- **FR-011**: Calls to external lookups (address search) MUST go through the same service
  layer and be listed in the content security policy's allowed connections.
- **FR-012**: The server MUST answer errors in one documented shape, produced by one shared
  helper.

**Production delivery (US4)**

- **FR-013**: There MUST be a production web app image that serves the built app as static
  files, runs as a non-administrator user and sets on every response a content security policy
  (only the app's own origin, the payment provider, the identity provider and the address
  service), a ban on framing by other origins, a referrer policy and a no-sniff instruction.
- **FR-014**: The server image MUST run as a non-administrator user.
- **FR-015**: Fonts MUST be served by the app itself. No third-party font request may happen
  on page load.
- **FR-016**: Listing photos MUST be accepted only from the configured image host. Any other
  photo address MUST be rejected when saved and rendered as the placeholder.
- **FR-017**: The local stack MUST expose its database, sign-in server, API and web app only
  on the developer's own machine.

**Pipeline (US5)**

- **FR-018**: Every change MUST run: server build, static analysis, formatting check, unit
  tests and database tests against a real geo-enabled database built from the schema file;
  web app type check, unit tests and production build; and known-vulnerability scans for both
  the server's and the web app's shipped dependencies.
- **FR-019**: In the pipeline, a database test MUST fail, not skip, when the database is
  unavailable.
- **FR-020**: The code base MUST pass the formatting check, and stale placeholder comments
  from the initial scaffold MUST be removed.

**Server structure and jobs (US6)**

- **FR-021**: Identifying the caller of a request MUST take at most one database lookup and
  MUST NOT write. Account creation for a new identity MUST be a separate, idempotent step.
- **FR-022**: Account rules MUST be split into focused parts (sign-in and linking, cook
  onboarding and the business register, deletion). Request handlers MUST depend on narrow
  interfaces, not on one concrete service.
- **FR-023**: Every background job MUST take a time-limited lease on each piece of work it
  processes, so that with several servers each piece is processed once, and work held by a
  crashed server is retried after the lease expires.
- **FR-024**: The registration re-check MUST give overdue active and inactive cooks a fair
  share of each run, so pending registrations cannot starve them.

### Key Entities

- **Ownership**: the relation between a signed-in account and a listing (through its cook
  profile) or a claim (through its buyer profile). Authorization reads it from the server's
  records only.
- **Job lease**: a time-limited claim by one server on one piece of background work, such as
  a registration re-check or a provider deletion. It expires so another server can take over.
- **Browser session**: the one signed-in state of the web app: real or demo, the access token
  held in memory, renewed silently.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of listing and claim endpoints have an automated test proving that another
  signed-in person is refused, and all such tests pass.
- **SC-002**: A signed-in person who reloads any page is still signed in 100% of the time
  while their provider session is valid.
- **SC-003**: The production build contains none of the development example data.
- **SC-004**: The production web app scores the top grade on a standard security-header scan,
  and the full claim and payment flow works under its content security policy.
- **SC-005**: Loading any page contacts no third-party host other than the identity provider,
  the payment provider and the address service, and only when they are used.
- **SC-006**: Every merged change has passed all pipeline checks. Zero database tests are
  skipped in the pipeline.
- **SC-007**: A signed-in read request performs exactly one database lookup to identify the
  caller and zero writes.
- **SC-008**: With two servers running for 24 hours, every due background task is performed
  exactly once.

## Assumptions

- This feature changes no product behaviour visible to buyers or cooks, apart from the
  session surviving a reload and no longer showing example data.
- The identity provider supports silent renewal for a signed-in browser session. Auth0 and
  the local mock server both do.
- Listing photos will come from one configured image host chosen in feature 002 or 004. Until
  then the allowed-host list is empty and every listing shows the placeholder.
- The pipeline runs on the repository's hosting provider. The database in the pipeline is
  built from the single schema file (there are no migrations until a production database
  exists).
- The 2026-10-26 fixes (server timeouts and graceful shutdown, maximum token age, throttled
  key refetching, upgraded toolchain and dependencies, mock sign-in audience) are done and
  out of scope.
- Not in scope: new product features, implementing feature 002, and rate limiting beyond
  what already exists (to be specified when the demo is public).

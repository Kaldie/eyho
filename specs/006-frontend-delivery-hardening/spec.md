# Feature Specification: Frontend & Delivery Hardening

**Feature Branch**: `006-frontend-delivery-hardening`

**Created**: 2026-10-26

**Status**: Draft

**Input**: User description: the browser-side and delivery concerns from the 2026-10-26
security and architecture review (maintainer decision, 2026-10-26, to capture them as a
feature; split from the original "architecture hardening" spec, whose server-side half is
feature 005). Scope: one signed-in state in the browser that survives a reload; one way to
call the API, with no example data in the shipped app; and a production-ready delivery
(security headers, non-root containers, local-only ports, self-hosted fonts, trusted photo
hosts).

**Position in the roadmap**: builds on 001 (sign-in), 003 (frontend) and 005 (whose pipeline
checks this feature's work). It adds no new product behaviour. US2 overlaps with 002's
frontend tasks (T012, T021) that wire screens to the real API: whichever lands first
introduces the single API client, and the other uses it. US3 must be done before the demo
(004) is public.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One signed-in state that survives a reload (Priority: P1)

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

### User Story 2 - The shipped app only shows real data (Priority: P1)

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

### User Story 3 - Production delivery is locked down (Priority: P2)

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
   is shown, **Then** the placeholder is rendered and the outside host is never contacted.
4. **Given** the local stack is running, **When** another machine on the same network tries
   its database, sign-in, server or web ports, **Then** the connection is refused.

---

### Edge Cases

- A person signs in on two tabs and signs out in one: the other tab is signed out on its next
  request and shows no stale data.
- Silent session renewal fails, for example because the provider session expired: the person
  is shown as signed out with a sign-in prompt. No error page is shown and no data is lost
  mid-claim.
- A reload happens in the middle of the claim flow: the person returns to the same claim step
  (the 001 "resume the claim" behaviour still holds).
- The content security policy blocks something the app needs, such as the payment provider:
  a test exercises the claim and payment flow under the production policy.
- A listing photo was saved before the allowed-host rule existed: it renders the placeholder.
- The address service is down: address search shows its existing "cannot search right now"
  state, and the rest of the page works.

## Requirements *(mandatory)*

### Functional Requirements

**Browser session (US1)**

- **FR-001**: The web app MUST hold exactly one signed-in state. Completing a real sign-in
  MUST end any demo session, and vice versa.
- **FR-002**: A signed-in person MUST stay signed in across page reloads in the same browser
  session, renewed silently with the identity provider, without storing access tokens in
  browser storage readable by page scripts.
- **FR-003**: Signing out or switching account MUST discard all data loaded for the previous
  person.
- **FR-004**: The current access token MUST reach the API layer through the app's state, not
  through a global value set while the screen is drawn.

**Data and API consistency (US2)**

- **FR-005**: Every web app call to the eyho server MUST go through one client that attaches
  the sign-in when present and turns errors into one error shape (the server's shape from
  feature 005).
- **FR-006**: Development example data MUST NOT be part of the production build. It MUST live
  only with the tests.
- **FR-007**: Calls to external lookups (address search) MUST go through the same service
  layer and be listed in the content security policy's allowed connections.

**Production delivery (US3)**

- **FR-008**: There MUST be a production web app image that serves the built app as static
  files, runs as a non-administrator user and sets on every response a content security policy
  (only the app's own origin, the payment provider, the identity provider and the address
  service), a ban on framing by other origins, a referrer policy and a no-sniff instruction.
- **FR-009**: The server image MUST run as a non-administrator user.
- **FR-010**: Fonts MUST be served by the app itself. No third-party font request may happen
  on page load.
- **FR-011**: Listing photos MUST be shown only from the configured image host. Any other
  photo address MUST render the placeholder without contacting that address. (Rejecting such
  addresses when a listing is saved belongs to feature 002's listing validation.)
- **FR-012**: The local stack MUST expose its database, sign-in server, API and web app only
  on the developer's own machine.

### Key Entities

- **Browser session**: the one signed-in state of the web app: real or demo, the access token
  held in memory, renewed silently.
- **Allowed image host**: the one configured origin that listing photos may come from.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A signed-in person who reloads any page is still signed in 100% of the time
  while their provider session is valid.
- **SC-002**: After sign-out, 0 items of the previous person's data are visible or retained
  in the app.
- **SC-003**: The production build contains none of the development example data.
- **SC-004**: The production web app scores the top grade on a standard security-header scan,
  and the full claim and payment flow works under its content security policy.
- **SC-005**: Loading any page contacts no third-party host other than the identity provider,
  the payment provider and the address service, and only when they are used.

## Assumptions

- The identity provider supports silent renewal for a signed-in browser session. Auth0 and
  the local mock server both do.
- Listing photos will come from one configured image host chosen in feature 002 or 004. Until
  then the allowed-host list is empty and every listing shows the placeholder.
- The local development server keeps working as today. The production image is an additional
  target, not a replacement for local development.
- Feature 005 provides the pipeline and the server's single error shape.
- Not in scope: server-side ownership checks, caller identification and background jobs
  (feature 005); new product features.

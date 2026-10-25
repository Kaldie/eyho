# Feature Specification: Post & Claim Surplus Food Listing

**Feature Branch**: `002-surplus-food-listing`

**Created**: 2026-08-01

**Status**: Draft

**Input**: User description: "Post & claim surplus food listing — a cook (non-profit/pro) creates a listing for surplus food at a reduced price, and a buyer discovers and claims it."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cook posts a surplus food listing (Priority: P1)

A verified cook (non-profit or professional kitchen) has cooked more food than needed for the day
and wants to sell the surplus at a reduced price rather than throw it away. They create a listing
describing the food, set a quantity of portions, a reduced price, a pickup location, and a pickup
time window.

**Why this priority**: Without a way to post surplus, there is nothing for buyers to discover or
claim. This is the entry point of the entire marketplace and the only way waste-reduction impact
gets created.

**Independent Test**: Can be fully tested by having a verified cook create a listing and
confirming it appears correctly (with all required details) without any buyer-side functionality
existing yet.

**Acceptance Scenarios**:

1. **Given** a verified cook with an active account, **When** they submit a listing with food
   description, portion count, reduced price, allergen info, pickup location, and pickup window,
   **Then** the listing is published and visible to nearby buyers.
2. **Given** a cook who is not yet verified, **When** they submit and publish a listing, **Then**
   the listing is published and visible to buyers but clearly flagged as "unverified".
3. **Given** a published listing, **When** the pickup window ends without all portions being
   claimed, **Then** the listing is automatically marked expired and removed from active
   discovery.

---

### User Story 2 - Buyer discovers and claims a listing (Priority: P1)

A buyer opens the app to see surplus food available nearby. They browse or search listings, view
details (price, portions remaining, pickup location/time, allergen info, cook's verification
status and rating), and claim one or more portions from a listing they want.

**Why this priority**: Discovery and claiming is the other half of the core transaction; without
it, posted listings never convert into saved food. Equally critical to User Story 1 for a viable
MVP.

**Independent Test**: Can be fully tested by seeding one or more published listings and having a
buyer browse, view listing details, and successfully claim portions, independent of how the
listing was created.

**Acceptance Scenarios**:

1. **Given** one or more active listings near the buyer, **When** the buyer opens the discovery
   view, **Then** listings are shown with non-profit cooks first and then by proximity
   (Principle III), with price, remaining portions, and
   pickup window visible at a glance.
2. **Given** an active listing with available portions, **When** a buyer claims a quantity of
   portions and completes payment, **Then** the portions are reserved for that buyer, the
   listing's remaining count decreases, and the buyer receives pickup confirmation details.
3. **Given** a listing with only 1 portion remaining, **When** two buyers attempt to claim it at
   the same time, **Then** only one claim succeeds and the other buyer is informed the portion is
   no longer available.
4. **Given** a buyer has claimed portions, **When** they fail to collect by the end of the pickup
   window, **Then** the claim is marked as a no-show and the portions are not refunded by default.

---

### User Story 3 - Cook manages an active listing (Priority: P2)

A cook needs to adjust a listing after posting it — for example, reducing the remaining portion
count if some were given away in person, extending the pickup window, or cancelling the listing
entirely if the food is no longer available.

**Why this priority**: Improves reliability and trust once the core post/claim loop (P1) works,
but the marketplace still functions for a first release without it.

**Independent Test**: Can be tested by having a cook with an existing published listing edit its
remaining portions, pickup window, or cancel it, and confirming buyer-facing views update
accordingly.

**Acceptance Scenarios**:

1. **Given** a published listing with no claims yet, **When** the cook cancels it, **Then** the
   listing is removed from discovery immediately.
2. **Given** a published listing with existing claims, **When** the cook cancels it, **Then**
   affected buyers are notified and refunded automatically.

---

### Edge Cases

- What happens when a cook tries to publish a listing with a pickup window in the past? The
  system MUST reject it with a validation error.
- What happens when a buyer's payment fails after they've reserved a portion? The reservation
  MUST be released back to available inventory within a short, defined timeout.
- How does the system handle a cook who reverts from verified to unverified (per the
  crowd-sourced verification process) while they have active listings? Those listings MUST
  remain visible but MUST be immediately re-flagged as "unverified"; they are not removed.
- Who is the "authenticated cook/buyer"? Identity comes from feature 001 (accounts & sign-in);
  every write endpoint in this feature requires a signed-in account with a verified email.
- What happens when a listing's pickup location falls outside the buyer's local discovery range?
  It MUST NOT appear in that buyer's default discovery view (Principle III: local discovery).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow any cook (verified or unverified) to create and publish listings,
  provided the cook's business registration is confirmed active (feature 001, FR-020;
  Constitution v3.1.0 Product Constraints); listings from a cook whose current status is
  unverified MUST be clearly flagged "unverified" to buyers (Principle I).
- **FR-002**: A listing MUST include: food description, allergen information, portion count,
  reduced price per portion, pickup location, and pickup time window.
- **FR-003**: System MUST prevent a listing's price from being set at or above an unreduced/retail
  reference price supplied by the cook, enforcing "reduced price" as a hard rule (Principle V).
- **FR-004**: System MUST show buyers, for each listing, the cook's current verification status
  (including a clear "unverified" flag when applicable), aggregate rating, and review count
  before they claim.
- **FR-005**: System MUST let buyers claim one or more available portions and complete payment via
  an established payment processor in a single flow.
- **FR-006**: System MUST decrement a listing's available portion count atomically per claim, and
  MUST NOT allow claims to exceed the portions available (no overselling).
- **FR-007**: System MUST automatically expire a listing when its pickup window ends, whether or
  not all portions were claimed.
- **FR-008**: System MUST release a buyer's reserved portions back to available inventory if
  payment is not completed within a defined timeout.
- **FR-009**: System MUST record, for every completed claim, the waste-reduction impact data
  (portions, estimated weight) required by Principle II.
- **FR-010**: System MUST allow a cook to cancel an active listing, and MUST automatically notify
  and refund any buyers who had already claimed portions from it.
- **FR-011**: System MUST restrict a buyer's default discovery view to listings within a
  configurable local radius (Principle III), with an explicit action required to broaden it.
- **FR-012**: System MUST mark a claim as a no-show if the buyer does not confirm pickup by the
  end of the pickup window, and MUST NOT automatically refund no-show claims.
- **FR-013**: System MUST let a buyer rate a listing (1-5 stars) after their claim's pickup is
  confirmed as collected, and MUST use that rating to evaluate the cook's verification status:
  promote an unverified cook to verified after 5 buyer ratings of 4 stars or higher, and demote a
  verified cook back to unverified after 3 buyer ratings of 2 stars or lower; each buyer's rating
  counts separately (Principle I, constitution v3.0.0).
- **FR-014**: System MUST record every verification status transition (direction, timestamp, and
  the triggering listings/ratings) in a permanent, auditable history per cook, and MUST track how
  many times each cook has cycled between verified and unverified (Principle I).

### Key Entities

- **Listing**: A surplus food offer created by a cook. Attributes: food description, allergen
  info, total portions, portions remaining, reduced price per portion, pickup location, pickup
  time window, status (draft/published/expired/cancelled).
- **Claim**: A buyer's reservation of one or more portions from a listing. Attributes: quantity,
  payment status, pickup confirmation status (pending/collected/no-show), timestamps.
- **Cook Profile**: Represents a non-profit or professional cook. Attributes: verification status
  (unverified/verified), good/bad rating counters since the last status transition, verification cycle count,
  aggregate rating, active listings.
- **Verification History**: An append-only record of a cook's status transitions. Attributes:
  from/to status, triggering listings/ratings, timestamp.
- **Buyer Profile**: Represents a buyer. Attributes: default local discovery radius, claim history.
- **Impact Record**: A derived record per completed claim capturing portions saved and estimated
  weight, feeding aggregate waste-reduction metrics (Principle II).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A cook (verified or unverified) can publish a new listing in under 2 minutes.
- **SC-002**: A buyer can find and claim a nearby listing in under 90 seconds from opening
  discovery to completed payment.
- **SC-003**: Zero overselling incidents — the number of completed claims never exceeds a
  listing's stated portion count.
- **SC-004**: At least 90% of published listings result in at least one claim within their pickup
  window (pilot target, revisit after launch data).
- **SC-005**: 100% of completed claims produce a corresponding impact record used in aggregate
  waste-reduction reporting.

## Assumptions

- Verification is crowd-sourced, not a separate upfront onboarding gate (Constitution Principle I,
  v2.0.0): any cook may publish immediately, starting as "unverified." This feature owns the
  promotion/demotion mechanics themselves, since they are driven by ratings on this feature's own
  listings/claims — a cook is promoted to verified after 5 buyer ratings of 4+ stars, and demoted
  back to unverified after 3 buyer ratings of 2 or less, with every transition recorded in
  `VerificationHistory` (see data-model.md). The legal registration check (active KvK number plus
  NVWA declaration) is owned by feature 001; this feature only refuses to publish when 001 says
  the cook can't publish yet.
- Payment is handled in-app through an established third-party payment processor; no cash-on-
  pickup flow is in scope for v1.
- A listing represents one food item/dish offered in multiple identical portions; mixed-item
  listings (multiple different dishes in one listing) are out of scope for v1.
- "Local" discovery radius defaults to a reasonable walking/short-drive distance (a few
  kilometers); the exact default value is a product/config decision left to the planning phase.
- No-show claims are non-refundable by default to discourage casual over-claiming, consistent with
  waste-reduction goals; a dispute/appeal process is out of scope for v1.
- Multi-portion listings only; single-country/single-currency pricing assumed for v1 (no
  multi-currency support).

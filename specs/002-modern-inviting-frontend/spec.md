# Feature Specification: Modern, Inviting Marketplace Frontend

**Feature Branch**: `002-modern-inviting-frontend`

**Created**: 2026-08-14

**Status**: Draft

**Input**: User description: "create a good looking front end that is modern and invites to participate"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Visitor is drawn in and discovers nearby food (Priority: P1)

A first-time visitor lands on the site (not yet logged in). The landing/discovery page uses
strong visual design, real listing photos, clear value messaging ("save food, save money"), and
an obvious primary action, so within seconds they understand what eyho does and start browsing
nearby surplus listings without needing an account.

**Why this priority**: First impressions decide whether a visitor becomes a buyer at all. If the
landing/browse experience doesn't look trustworthy and inviting, the rest of the app never gets
used, regardless of how well checkout or the cook dashboard work.

**Independent Test**: Load the discovery page as a logged-out visitor on desktop and mobile
widths and confirm the value proposition, a visible listing grid/map, and a clear call-to-action
are understandable within one screen, with no functional account required to browse.

**Acceptance Scenarios**:

1. **Given** a logged-out visitor on the landing page, **When** the page loads, **Then** they see
   a hero section stating what eyho is, an immediate call-to-action to browse listings, and at
   least one real listing preview above the fold.
2. **Given** a visitor browsing the listing grid, **When** listings are displayed, **Then** each
   listing card shows a photo, food name, price, portions remaining, distance/pickup window, and
   cook verification badge, in a visually consistent card layout.
3. **Given** a visitor on a mobile-width screen, **When** they view the landing or discovery page,
   **Then** the layout reflows responsively with no horizontal scrolling and all primary actions
   remain reachable with one hand/thumb.
4. **Given** a visitor with a slow network connection, **When** listing images are loading,
   **Then** placeholder/skeleton states are shown instead of blank space or layout shift.

---

### User Story 2 - Buyer feels confident claiming and checking out (Priority: P1)

A buyer who has decided on a listing goes through viewing details, selecting a quantity, and
paying. The flow should feel modern and trustworthy (clear pricing, no surprise steps, visible
progress) so the buyer completes the claim instead of abandoning it.

**Why this priority**: This is the core revenue/impact-generating action of the app. A visually
confusing or dated checkout directly costs completed claims (and prevented food waste), even if
discovery worked perfectly.

**Independent Test**: From a listing detail page, select a quantity, proceed to checkout, and
confirm the flow can be completed end-to-end with a clear running total, clear next step at every
stage, and a confirmation screen at the end — independent of the discovery UI.

**Acceptance Scenarios**:

1. **Given** a buyer on a listing detail page, **When** they view the page, **Then** they see
   large food imagery, a clear price per portion, a quantity selector, remaining portions, pickup
   window/location, allergen info, and a prominent "Claim" call-to-action.
2. **Given** a buyer proceeding to checkout, **When** they move through payment steps, **Then**
   a visible step indicator shows where they are in the process and the total price updates live
   as quantity changes.
3. **Given** a buyer completes payment, **When** the claim succeeds, **Then** they see a clear
   confirmation screen with pickup details and a way to access this confirmation again later.
4. **Given** a buyer on a small screen, **When** completing checkout, **Then** form fields, buttons
   and the total are all visible without excessive zooming or scrolling per step.
5. **Given** a buyer submits payment, **When** the card is declined, **Then** an inline error
   explains the failure on the payment step and the buyer's quantity/step selection is preserved.
6. **Given** a buyer is mid-checkout, **When** the listing's pickup window expires before payment
   completes, **Then** the claim is blocked, a clear "no longer available" message is shown, and
   no charge occurs.

---

### User Story 3 - Cook feels proud presenting and managing listings (Priority: P2)

A cook (non-profit or professional kitchen) uses a dashboard to create new listings and monitor
active ones. The dashboard should look professional and make the cook feel their food is being
presented attractively to buyers, encouraging them to keep posting.

**Why this priority**: Supply (cooks posting) is as essential as demand (buyers claiming), but the
marketplace already functions for a first release on a plainer cook-facing UI as long as the
buyer-facing surfaces (P1) are strong. Improves retention and listing quality over time.

**Independent Test**: Log in as a cook, create a new listing through the dashboard form, and
confirm the in-progress and published listing states are shown with clear visual status
(draft/published/expired) and a preview of how buyers will see the listing.

**Acceptance Scenarios**:

1. **Given** a cook on their dashboard, **When** they view active listings, **Then** each listing
   shows a clear status badge (draft/published/claimed portions/expired) and key stats
   (views, portions claimed) in a scannable layout.
2. **Given** a cook creating a new listing, **When** they fill out the listing form, **Then** they
   see a live preview of how the listing card will appear to buyers.
3. **Given** a cook's verification status is pending, **When** they view their dashboard, **Then**
   a clear, non-alarming indicator explains their current verification state and what it means for
   buyers.

---

### Edge Cases

- What happens when a listing has no photo available? (Show a branded placeholder illustration,
  never a broken image or empty box.)
- How does the UI communicate a listing that just sold out while the buyer was viewing it?
  (Real-time or near-real-time update disabling the claim action with a clear "sold out" state.)
- What happens when a buyer's connection drops mid-checkout? (Preserve selected quantity/state and
  show a retry path rather than restarting the flow.)
- How does the UI handle an empty discovery result (no listings nearby)? (Show an inviting empty
  state with guidance, e.g. expand search radius, rather than a blank page.)
- How does the UI present an unverified cook's listing without discouraging buyers unnecessarily?
  (Neutral, informative badge rather than a warning-style treatment.)
- What happens when a buyer's payment is declined (not a dropped connection)? (Show an inline,
  recoverable error on the payment step, preserving selected quantity/state, distinct from the
  connection-drop retry path.)
- What happens when a listing's pickup window expires while a buyer is mid-checkout? (Block the
  claim, show a clear "no longer available" message, and return the buyer to discovery — do not
  charge for a listing whose pickup window has already ended.)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST present a public landing/discovery page, viewable without
  authentication, that communicates eyho's value proposition and surfaces real listings.
- **FR-002**: The system MUST display listings as visually consistent cards including photo (or
  branded placeholder), food name, price, portions remaining, pickup window/location, and cook
  verification status.
- **FR-003**: The system MUST provide a listing detail view with full listing information, a
  quantity selector, and a prominent primary action to claim/purchase.
- **FR-004**: The system MUST provide a checkout flow with a visible progress indicator and a
  live-updating total price as quantity or selections change.
- **FR-005**: The system MUST show a confirmation screen after a successful claim, including
  pickup details, and MUST make this confirmation retrievable again later (e.g., from an order
  history view).
- **FR-006**: The system MUST provide a cook dashboard showing all of a cook's listings with
  clear status indicators (draft, published, partially claimed, sold out, expired).
- **FR-007**: The system MUST provide a listing creation/edit form for cooks that includes a live
  preview of the buyer-facing listing card.
- **FR-008**: The system MUST be fully responsive, presenting usable layouts across common mobile,
  tablet, and desktop screen widths with no horizontal scrolling.
- **FR-009**: The system MUST show loading/skeleton states for images and content that has not yet
  finished loading, avoiding blank flashes or layout shift.
- **FR-010**: The system MUST show an inviting empty state (with next-step guidance) when no
  listings are available to display.
- **FR-011**: The system MUST disable or clearly mark a listing's claim action once it is sold out,
  and reflect this without requiring a full page reload.
- **FR-012**: The system MUST meet WCAG 2.1 AA color contrast and keyboard-navigability standards
  for all primary user flows (browse, view listing, checkout, cook dashboard).
- **FR-013**: The visual design system (colors, typography, spacing, components) MUST be applied
  consistently across all pages covered in User Stories 1-3.
- **FR-014**: The system MUST show a distinct, recoverable error state when a payment is declined
  or otherwise fails during checkout, preserving the buyer's selected quantity and current step
  rather than resetting the flow.
- **FR-015**: The system MUST block claim/checkout completion once a listing's pickup window has
  expired, even if the buyer began checkout before expiry, and MUST clearly explain why the claim
  can no longer proceed.
- **FR-016**: Checkout in this iteration is scoped to a single listing (one cook) per checkout
  session; multi-listing/multi-cook cart checkout is out of scope (see Assumptions).

### Key Entities

- **Listing Card (view)**: The visual, at-a-glance representation of a Listing used in grids and
  search results — photo, name, price, portions remaining, distance/pickup window, verification
  badge, status.
- **Listing Detail (view)**: The full-information presentation of a single Listing used for
  purchase decisions.
- **Checkout Flow (view)**: The multi-step presentation guiding a buyer from quantity selection
  through payment to confirmation.
- **Cook Dashboard (view)**: The cook-facing summary and management surface for their own
  listings, including status and performance indicators.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A first-time visitor can identify what eyho does and locate the browse/discovery
  action within 5 seconds of landing on the page (measured via usability testing).
- **SC-002**: At least 90% of test users successfully complete a full claim (browse → detail →
  checkout → confirmation) on their first attempt without assistance.
- **SC-003**: The discovery, listing detail, and checkout pages each score at or above 90 on
  standard automated performance/accessibility audits (e.g., Lighthouse) for mobile.
- **SC-004**: Listing grid, detail, and checkout pages render usably (no broken layout, no
  horizontal scroll) at 360px, 768px, and 1440px viewport widths.
- **SC-005**: Cart/checkout abandonment during the claim flow decreases relative to the prior
  interface, measured over the first month post-launch.
- **SC-006**: Cooks report increased confidence in listing presentation, measured via post-launch
  survey (target: 80% agree or strongly agree their listings "look professional").

## Assumptions

- This feature covers visual/UX redesign and frontend structure only; it assumes the underlying
  listing, claim, payment, and authentication backend capabilities from Feature 001 already exist
  or are being built in parallel and are consumed as-is.
- "Modern" is interpreted as: clean typography, generous whitespace, card-based layouts, smooth
  micro-interactions/transitions, mobile-first responsive design, and a cohesive color/branding
  system — not any single specific design trend.
- Target platform for this feature is responsive web (desktop and mobile browsers); native mobile
  apps are out of scope.
- Real listing photos are assumed to be supplied by cooks at listing-creation time; this feature is
  responsible for how photos are displayed, not for photo capture/upload tooling itself.
- English is the only language required for this iteration; localization is out of scope.
- Checkout is single-listing/single-cook per session; a shared cart across multiple cooks is
  deferred to a future iteration, consistent with the Low Price, Low Friction principle's
  preference for a minimal-step claim flow.

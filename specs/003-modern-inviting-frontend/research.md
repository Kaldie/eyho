# Research: Modern, Inviting Marketplace Frontend

## Styling approach

- **Decision**: Tailwind CSS 3, with a small `styles/tokens.css` layer defining eyho's color
  palette (warm/food-friendly primary + neutral grays), type scale, and spacing scale as CSS
  variables consumed by Tailwind's `theme.extend`.
- **Rationale**: Existing frontend has no CSS framework yet (plain component files). Tailwind
  gives utility-first velocity for the card/grid/responsive layouts required by FR-002/FR-008
  without introducing a large runtime CSS-in-JS dependency, keeps bundle size small (important
  for SC-003 Lighthouse mobile score), and is straightforward to theme consistently (FR-013).
- **Alternatives considered**: CSS Modules (more boilerplate per component, harder to keep a
  single consistent token system across pages); a full component library like MUI/Chakra
  (heavier bundle, harder to make it feel distinctively "eyho" rather than generic, fights
  FR-013's single cohesive system requirement); styled-components (runtime cost, team has no
  existing usage).

## Accessible interactive primitives

- **Decision**: Build the small set of interactive primitives (quantity stepper, step indicator,
  badge, skeleton, empty state) as plain accessible React components using native semantics
  (`<button>`, `aria-live`, `role="status"`) rather than pulling in a full headless-UI library.
- **Rationale**: The primitive set needed is small (5-6 components) and simple enough that native
  HTML + ARIA attributes meet WCAG 2.1 AA (FR-012) without an extra dependency. Keeps the
  component library lightweight and easy for the frontend-engineer agent to maintain.
- **Alternatives considered**: Radix UI primitives (good accessibility defaults, but adds a
  dependency for functionality achievable natively at this scale); Headless UI (React-Router-
  adjacent ecosystem fit, similar overkill for this component count).

## Real-time "sold out" update (FR-011)

- **Decision**: Poll the listing detail endpoint via TanStack Query's `refetchInterval` (e.g.
  every 10-15s) while a listing detail page is open, and optimistically disable the claim CTA
  the moment a claim response indicates zero portions remain.
- **Rationale**: Feature 002's contracts (existing) do not define a websocket/SSE channel;
  introducing one is out of scope for a frontend-only feature. Short-interval polling is a
  standard, low-complexity way to reflect near-real-time availability changes on a
  single-item detail page without backend contract changes.
- **Alternatives considered**: WebSocket/SSE push (more "real-time" but requires new backend
  contract work — out of scope per plan.md Constitution Check; deferred as a future enhancement
  once Feature 002's contracts are extended).

## Image loading / skeleton states (FR-009)

- **Decision**: Use native `loading="lazy"` + explicit `width`/`height` (or `aspect-ratio`) on all
  listing images to prevent layout shift, paired with a Tailwind-animated skeleton placeholder
  shown until the image's `onLoad` fires.
- **Rationale**: Meets FR-009 (no blank flash/layout shift) and contributes directly to the
  Lighthouke Performance/CLS score behind SC-003, with no additional library.
- **Alternatives considered**: Third-party lazy-load/image libraries (unnecessary given modern
  browser-native `loading="lazy"` support across target evergreen browsers).

## Order/claim history retrieval (FR-005)

- **Decision**: Add a new `OrderHistoryPage` route backed by a `GET /orders` (or equivalent
  existing "my claims" endpoint from Feature 002's backend) — assumed available or trivially
  addable in backend per plan.md Assumptions; if the endpoint does not yet exist, this plan's
  frontend work proceeds against a documented contract in `contracts/` and coordinates with
  backend-engineer to implement it.
- **Rationale**: FR-005 explicitly requires confirmation to be "retrievable again later," which
  needs a durable list view, not just a one-time confirmation screen.
- **Alternatives considered**: Store confirmation only in browser localStorage (fails across
  devices/sessions, does not meet "retrievable later" in a trustworthy way for a payment
  confirmation).

## Accessibility & performance validation approach

- **Decision**: Add automated Lighthouse CI (or `@lhci/cli`) checks for the three key pages, and
  `axe-core`-based assertions in the existing Playwright e2e suite for keyboard nav / contrast
  regressions.
- **Rationale**: SC-003 and FR-012 are measurable/testable requirements; automating them prevents
  regressions during future iteration, consistent with the project's Spec-Driven Development
  workflow (constitution Development Workflow section).
- **Alternatives considered**: Manual-only audits (not repeatable, easy to regress silently).

## Payment decline handling (FR-014)

- **Decision**: Handle Stripe payment-intent failure (`requires_payment_method` / declined) as a
  distinct `CheckoutState.paymentIntentStatus === "failed"` case, rendered as an inline error on
  the payment step with a retry action — separate code path from the connection-drop edge case
  (which preserves state and offers reconnect/retry at the network layer).
- **Rationale**: FR-014 requires this to be visibly distinct from a dropped connection so the
  buyer understands whether the problem is their card or their network. Stripe.js already
  surfaces a typed error on the confirm-payment call, so no new backend contract is needed.
- **Alternatives considered**: Treating decline and connection-drop identically as a generic
  "something went wrong" — rejected, fails FR-014's "distinct" requirement and is worse UX (buyer
  can't tell if retrying with the same card will help).

## Pickup-window expiry mid-checkout (FR-015)

- **Decision**: Re-validate `pickupWindow.end` against current time immediately before submitting
  payment (client-side check) and treat a 4xx "listing no longer available" response from the
  claim/payment endpoint as authoritative if the client-side check is stale. Block submission and
  show a blocking message; do not attempt payment.
- **Rationale**: Prevents charging for an expired listing (FR-015) with a minimal client-side
  guard, while still trusting the backend as source of truth since network latency could let a
  window expire between the client check and the actual request.
- **Alternatives considered**: Relying solely on backend rejection after payment attempt —
  rejected, risks a confusing "payment succeeded then refunded" experience; better to block before
  submission when detectable client-side.

All Technical Context items resolved — no remaining NEEDS CLARIFICATION markers.

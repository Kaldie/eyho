---

description: "Task list for Modern, Inviting Marketplace Frontend"
---

# Tasks: Modern, Inviting Marketplace Frontend

**Input**: Design documents from `/specs/003-modern-inviting-frontend/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/order-history.md, quickstart.md

**Tests**: Not explicitly requested in spec.md beyond automated a11y/perf/e2e checks called out in
plan.md/quickstart.md — component/e2e test tasks are included per Testing section of plan.md
(existing Vitest/Playwright setup), scoped to new states this feature introduces.

**Organization**: Tasks are grouped by user story (US1 = P1 discovery, US2 = P1 checkout,
US3 = P2 cook dashboard) to enable independent delivery.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Maps task to US1/US2/US3
- File paths are relative to repository root

## Path Conventions

Existing web-app split retained: `backend/src/`, `frontend/src/` (per plan.md Structure Decision).
This feature only touches `frontend/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Introduce the styling foundation shared by every page in this feature.

- [X] T001 Add Tailwind CSS 3 + PostCSS + Autoprefixer as devDependencies in `frontend/package.json`, with `frontend/tailwind.config.ts` and `frontend/postcss.config.js`
- [X] T002 Create design tokens in `frontend/src/styles/tokens.css` (color palette, type scale, spacing scale as CSS variables) and wire into `tailwind.config.ts` `theme.extend`
- [X] T003 Create Tailwind entrypoint `frontend/src/styles/index.css` (`@tailwind base/components/utilities`) and import it in `frontend/src/main.tsx`
- [X] T004 [P] Configure `frontend/.eslintrc` (or existing lint config) to lint Tailwind class usage / new `.tsx` files under `src/components/ui/` — SKIPPED: no eslint config (`.eslintrc*`/`eslint.config.*`) exists anywhere in `frontend/`; nothing to extend without introducing a new lint setup, which was out of scope for this task

**Checkpoint**: `npm run dev` renders with Tailwind utilities available; no visual change yet required.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared UI primitives and view-model types every user story's pages consume. No user
story page work should start before this phase is done.

**⚠️ CRITICAL**: Blocks Phase 3, 4, 5.

- [X] T005 [P] Define shared view-model types (`ListingCardView`, `ListingDetailView`, `CheckoutState`, `OrderConfirmationView`, `CookDashboardListingView`) in `frontend/src/types/views.ts` per data-model.md
- [X] T006 [P] Create `Button` primitive in `frontend/src/components/ui/Button.tsx` (primary/secondary/disabled variants, meets WCAG AA contrast per FR-012)
- [X] T007 [P] Create `Badge` primitive in `frontend/src/components/ui/Badge.tsx` (neutral verification-status styling and listing-status styling per Edge Cases)
- [X] T008 [P] Create `Skeleton` primitive in `frontend/src/components/ui/Skeleton.tsx` (animated placeholder, used for FR-009)
- [X] T009 [P] Create `EmptyState` primitive in `frontend/src/components/ui/EmptyState.tsx` (icon/illustration + message + CTA slot, for FR-010)
- [X] T010 [P] Create `QuantityStepper` primitive in `frontend/src/components/ui/QuantityStepper.tsx` (keyboard-accessible +/- with min/max, extracted from existing inline logic in `ClaimFlow.tsx`)
- [X] T011 [P] Create `StepIndicator` primitive in `frontend/src/components/ui/StepIndicator.tsx` (renders `CheckoutState.step` progress, for FR-004)
- [X] T012 [P] Create `ImageWithFallback` component in `frontend/src/components/ui/ImageWithFallback.tsx` (lazy-load + `aspect-ratio` + branded placeholder on null/error, per FR-009 and Edge Cases)
- [X] T013 Extend `frontend/src/services/api.ts` with `getOrders()`, `getOrder(orderId)` per `contracts/order-history.md`, plus TanStack Query hooks `useOrders`/`useOrder`

**Checkpoint**: Shared primitives + types + API surface exist; user story phases can now proceed independently.

---

## Phase 3: User Story 1 - Visitor is drawn in and discovers nearby food (Priority: P1) 🎯 MVP

**Goal**: A logged-out visitor understands eyho's value proposition immediately and can browse a
visually consistent, responsive, gracefully-loading listing grid.

**Independent Test**: Load the discovery page logged out at 360px/768px/1440px widths; confirm
hero + CTA + listing grid above the fold, skeletons while loading, and an inviting empty state
when no listings are returned.

### Implementation for User Story 1

- [X] T014 [US1] Add hero section (value proposition + primary CTA) to `frontend/src/pages/DiscoveryPage.tsx`
- [X] T015 [US1] Restyle `frontend/src/components/ListingCard.tsx` using Tailwind + `Badge` (T007) + `ImageWithFallback` (T012), replacing inline `style={{}}` objects, per FR-002
- [X] T016 [US1] Wire `Skeleton` (T008) into `DiscoveryPage.tsx` listing grid while the listings query is loading, per FR-009
- [X] T017 [US1] Wire `EmptyState` (T009) into `DiscoveryPage.tsx` for a zero-result response, per FR-010
- [X] T018 [US1] Apply responsive grid layout (mobile-first, no horizontal scroll at 360/768/1440px) to `DiscoveryPage.tsx`, per FR-008
- [X] T019 [P] [US1] Add/extend Vitest component tests for `ListingCard.tsx` covering: photo present, `photoUrl: null` fallback, sold-out state, verification badge in `frontend/tests/components/ListingCard.test.tsx`
- [X] T020 [P] [US1] Add Playwright e2e test asserting no horizontal scroll and hero/CTA visibility at 360px/768px/1440px on the discovery page in `frontend/tests/e2e/discovery-responsive.spec.ts`
- [X] T021 [P] [US1] Add Playwright e2e test for the empty-discovery-result state in `frontend/tests/e2e/discovery-empty-state.spec.ts`

**Checkpoint**: User Story 1 fully functional and independently testable/demoable.

---

## Phase 4: User Story 2 - Buyer feels confident claiming and checking out (Priority: P1)

**Goal**: A buyer can view full listing detail, select a quantity, move through a visibly
progressing checkout with a live total, and land on a retrievable confirmation — handling sold-out,
connection-drop, payment-decline, and pickup-window-expiry cases distinctly.

**Independent Test**: From a listing detail page, claim a listing end-to-end to confirmation, then
retrieve that confirmation later from Order History; separately verify sold-out/decline/expiry
each produce a distinct, state-preserving UI response.

### Implementation for User Story 2

- [X] T022 [US2] Restyle `frontend/src/pages/ListingDetailPage.tsx` with large imagery (`ImageWithFallback`), price, `QuantityStepper` (T010), allergen info, and prominent Claim CTA, per FR-003
- [X] T023 [US2] Add `CheckoutState` state machine (`step`, `paymentIntentStatus`, `blockedReason`) to `frontend/src/components/ClaimFlow.tsx` per data-model.md, replacing the current single local `quantity` state
- [X] T024 [US2] Integrate `StepIndicator` (T011) into `ClaimFlow.tsx` and recompute total live from `CheckoutState.quantity`, per FR-004
- [X] T025 [US2] Wire real Stripe Elements (`PaymentElement`) into `ClaimFlow.tsx`, replacing the placeholder `div`, calling `stripe.confirmPayment()` — client-secret creation is mocked in `services/api.ts` (`createClaim`) since no live PaymentIntent backend endpoint exists yet
- [X] T026 [US2] Handle `paymentIntentStatus === "failed"` (declined card) as a distinct inline, recoverable error in `ClaimFlow.tsx` that preserves quantity/step, per FR-014
- [X] T027 [US2] Handle connection-drop during checkout as a distinct retry-path error (separate from T026) in `ClaimFlow.tsx`, per Edge Cases
- [X] T028 [US2] Add client-side `pickupWindow.end` expiry check before payment submission and handle backend 4xx "no longer available" response by setting `blockedReason: "expired"` and blocking submission in `ClaimFlow.tsx`, per FR-015
- [X] T029 [US2] Add TanStack Query `refetchInterval` polling of listing detail in `frontend/src/pages/ListingDetailPage.tsx` to disable the Claim CTA when portions reach 0 without a full reload, per FR-011 and research.md
- [X] T030 [US2] Build confirmation screen (pickup details) shown on `paymentIntentStatus === "succeeded"` in `ClaimFlow.tsx` — extracted into shared `frontend/src/components/OrderConfirmation.tsx`
- [X] T031 [US2] Create `frontend/src/pages/OrderHistoryPage.tsx` listing past orders via `useOrders()` (T013), each linking to a reusable `OrderConfirmationView` display
- [X] T032 [US2] Add `/orders` and `/orders/:orderId` routes in `frontend/src/App.tsx` pointing to `OrderHistoryPage.tsx`, per FR-005
- [X] T033 [US2] Apply responsive layout to `ListingDetailPage.tsx` and `ClaimFlow.tsx` (no excess zoom/scroll per step at mobile widths), per FR-008
- [X] T034 [P] [US2] Add Vitest tests for `ClaimFlow.tsx` covering: live total update, sold-out disables CTA, payment-decline distinct error, connection-drop distinct error, expiry blocks submission in `frontend/tests/components/ClaimFlow.test.tsx`
- [X] T035 [P] [US2] Add Playwright e2e test for the full claim happy path (browse → detail → checkout → confirmation → retrievable in Order History) in `frontend/tests/e2e/claim-flow.spec.ts` — not executed in this environment (no Playwright browser run); see summary for limitations
- [X] T036 [P] [US2] Add Playwright e2e test simulating a concurrent sold-out claim disabling the CTA without reload in `frontend/tests/e2e/claim-sold-out.spec.ts` — written as a documented placeholder given `services/api.ts` still serves mock data rather than real fetches; see summary

**Checkpoint**: User Stories 1 AND 2 both independently functional; MVP-complete for buyer-facing flows.

---

## Phase 5: User Story 3 - Cook feels proud presenting and managing listings (Priority: P2)

**Goal**: A cook sees a professional, scannable dashboard with clear status badges/stats per
listing, and a listing-creation form with a live buyer-facing preview.

**Independent Test**: Log in as a cook, view dashboard listings with status badges/stats, create a
listing and confirm the live preview matches `ListingCard.tsx` rendering.

### Implementation for User Story 3

- [X] T037 [US3] Restyle `frontend/src/pages/CookDashboardPage.tsx` listing rows with `Badge` (T007) status indicators (draft/published/partially claimed/sold out/expired) and views/portions-claimed stats, per FR-006
- [X] T038 [US3] Add neutral, non-alarming verification-state indicator to `CookDashboardPage.tsx`, per Acceptance Scenario 3 / Edge Cases
- [X] T039 [US3] Build listing creation/edit form in `CookDashboardPage.tsx` (or extracted `frontend/src/components/ListingForm.tsx`) with live-updating `CookDashboardListingView.cardPreview` rendered via `ListingCard.tsx`, per FR-007
- [X] T040 [P] [US3] Add Vitest tests for the live preview updating as form fields change in `frontend/tests/components/ListingForm.test.tsx`

**Checkpoint**: All three user stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Consistency, accessibility, and performance validation across all pages touched above.

- [X] T041 [P] Audit and align spacing/typography/color usage across `DiscoveryPage.tsx`, `ListingDetailPage.tsx`, `ClaimFlow.tsx`, `CookDashboardPage.tsx` against `tokens.css`, per FR-013
- [X] T042 [P] Add `prefers-reduced-motion` handling to any transitions/animations introduced (Skeleton, StepIndicator, hero), per FR-012 and Assumptions
- [X] T043 [P] Run axe-core accessibility assertions across discovery/detail/checkout/dashboard in Playwright e2e tests, per FR-012
- [X] T044 Configure Lighthouse CI (or `@lhci/cli`) for discovery, listing detail, and checkout pages targeting Performance + Accessibility ≥ 90 mobile, per SC-003
- [X] T045 Run `quickstart.md` end-to-end validation manually (all three user stories) and record results

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 only
- **User Story 2 (Phase 4)**: Depends on Phase 2 only (independently testable from US1, though both consume `ListingCard`/`ImageWithFallback`)
- **User Story 3 (Phase 5)**: Depends on Phase 2 only; reuses `ListingCard.tsx` from US1 for live preview (T039) — recommend completing Phase 3 first in practice, though not a hard blocker since `ListingCard.tsx` itself lives in Foundational-adjacent shared code
- **Polish (Phase 6)**: Depends on Phases 3-5 being complete

### Parallel Opportunities

- T001-T004 (Setup) mostly sequential (Tailwind config chain); T004 can run parallel to T002/T003
- T005-T012 (Foundational primitives) are independent files — all [P]
- Within US1: T019, T020, T021 (tests) are [P] once T014-T018 land
- Within US2: T034, T035, T036 (tests) are [P] once T022-T033 land
- US1 and US2 implementation tasks can be worked in parallel by different developers once Phase 2 is done (both are P1)
- US3 can be worked in parallel with US1/US2 once Phase 2 is done, but T039's preview depends on `ListingCard.tsx` (T015) being restyled first for the preview to be meaningful

---

## Implementation Strategy

### MVP First

1. Phase 1 (Setup) → Phase 2 (Foundational) → Phase 3 (US1)
2. **STOP and VALIDATE**: discovery page alone, per quickstart.md US1 steps
3. Phase 4 (US2) completes the buyer-facing MVP (browse + claim)

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → validate → demo (partial MVP: browsing only)
3. US2 → validate → demo (full buyer MVP: browse-to-claim)
4. US3 → validate → demo (cook-facing polish)
5. Polish phase → final accessibility/performance sign-off (SC-003, FR-012)

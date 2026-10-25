# Implementation Plan: Modern, Inviting Marketplace Frontend

**Branch**: `003-modern-inviting-frontend` | **Date**: 2026-08-14 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-modern-inviting-frontend/spec.md`

## Summary

Redesign and restructure the existing React frontend (landing/discovery, listing detail, claim/
checkout, cook dashboard) into a modern, visually consistent, responsive design system that
invites first-time visitors to browse and buyers to complete claims with confidence. Approach:
introduce Tailwind CSS + a small shared component library (cards, buttons, badges, skeletons,
step indicator) on top of the existing React Router + TanStack Query structure, restyle the four
existing pages against this system, and add the missing UI states called out in the spec
(skeleton loading, empty state, sold-out live update, order history).

## Technical Context

**Language/Version**: TypeScript 5.5 (React 18.3), matches existing `frontend/`

**Primary Dependencies**: React 18, React Router 6, TanStack Query 5, Stripe.js/React Stripe.js
(existing) + Tailwind CSS 3 (new, for the design system) + Headless UI or Radix primitives (new,
for accessible interactive components: quantity stepper, step indicator, badges)

**Storage**: N/A (frontend consumes existing backend REST API from Feature 002; no new storage)

**Testing**: Vitest + Testing Library (existing, component/unit) and Playwright (existing, e2e) —
extend both suites to cover new visual states (skeleton, empty, sold-out, responsive breakpoints)

**Target Platform**: Responsive web (modern evergreen browsers), mobile-first per FR-008

**Project Type**: Web application — existing `frontend/` (React) + `backend/` (Go) split

**Performance Goals**: Lighthouse mobile score ≥90 for Performance and Accessibility on discovery,
detail, and checkout pages (SC-003)

**Constraints**: WCAG 2.1 AA contrast/keyboard nav on all primary flows (FR-012); no horizontal
scroll at 360px/768px/1440px (FR-008, SC-004); no layout shift from unloaded images (FR-009)

**Scale/Scope**: 4 existing pages restyled (Discovery, ListingDetail, ClaimFlow/Checkout, Cook
Dashboard) + 1 new page (Order/Claim History, required by FR-005) + a shared component library
(~8-10 components). Checkout scope is single-listing/single-cook per session (FR-016) — no cart.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Crowd-Sourced Verification & Trust**: PASS. Spec FR-002/FR-006 require verification-status
  badges on both listing cards and the cook dashboard, using a neutral (not warning-style)
  treatment per Edge Cases — consistent with the constitution's transparency requirement.
- **II. Waste-Reduction Impact Tracking**: N/A for this feature — impact display is not in scope
  of spec 003 (frontend visual/UX redesign only); no requirement here removes or hides existing/
  planned impact metrics. Flagged as a follow-up: impact display should be designed into the
  shared component library later, not blocked by this plan.
- **III. Community Collaboration**: PASS. Local pickup/distance is already a required card field
  (FR-002); no requirement in this feature reduces local-discovery emphasis.
- **IV. Gamified Engagement**: N/A — out of scope for this visual redesign feature.
- **V. Low Price, Low Friction**: PASS. FR-004 (visible progress + live total) and SC-002 (90%
  first-attempt completion) directly serve the low-friction checkout mandate. FR-016's
  single-listing checkout scope keeps the flow minimal-step by design rather than adding cart
  complexity.
- **Local Development Constraint**: PASS. All new frontend dependencies (Tailwind, component
  primitives) are build-time/static; no new service required. `frontend` already runs via
  existing `docker-compose.yml` entry — no compose changes needed.

No violations. Complexity Tracking section not required.

## Project Structure

### Documentation (this feature)

```text
specs/003-modern-inviting-frontend/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md         # Phase 1 output
├── contracts/             # Phase 1 output
└── tasks.md              # Phase 2 output (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
backend/
├── src/
└── ...                   # Unchanged by this feature

frontend/
├── src/
│   ├── components/
│   │   ├── ListingCard.tsx        # Restyled
│   │   ├── ClaimFlow.tsx          # Restyled + step indicator
│   │   └── ui/                    # NEW: shared design-system primitives
│   │       ├── Button.tsx
│   │       ├── Badge.tsx          # verification/status badges
│   │       ├── Skeleton.tsx       # loading placeholders
│   │       ├── EmptyState.tsx
│   │       ├── QuantityStepper.tsx
│   │       └── StepIndicator.tsx
│   ├── pages/
│   │   ├── DiscoveryPage.tsx      # Restyled: hero + responsive grid + empty state
│   │   ├── ListingDetailPage.tsx  # Restyled: gallery, quantity, claim CTA
│   │   ├── CookDashboardPage.tsx  # Restyled: status badges, listing form + live preview
│   │   └── OrderHistoryPage.tsx   # NEW: retrievable confirmation (FR-005)
│   ├── services/api.ts            # Extended: sold-out/live status polling, order history fetch
│   ├── styles/                    # NEW: Tailwind entrypoint + design tokens
│   └── App.tsx                    # Route for OrderHistoryPage added
└── tests/
    ├── components/                # Unit tests for ui/ primitives + card/flow states
    └── e2e/                       # Playwright: responsive breakpoints, empty/sold-out states
```

**Structure Decision**: Existing Option 2 (web application: `backend/` + `frontend/`) is retained
as-is. This feature only adds files under `frontend/src/components/ui/`, one new page
(`OrderHistoryPage.tsx`), a `styles/` entrypoint for Tailwind, and corresponding tests — no new
top-level projects or services.

## Complexity Tracking

*No violations — section not applicable.*

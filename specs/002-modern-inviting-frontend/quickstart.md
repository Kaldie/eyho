# Quickstart: Modern, Inviting Marketplace Frontend

## Prerequisites

- Docker Desktop running (constitution's Local Development Constraint)
- `.env` copied from `.env.example` with Stripe test keys set

## Run the full stack

```powershell
docker compose up
```

- Frontend: http://localhost:5173 (or the port mapped in `docker-compose.yml`)
- Backend: http://localhost:8080

## Validate User Story 1 — Visitor discovery (P1)

1. Open the frontend URL in an incognito/private window (logged out).
2. Confirm hero section + value proposition + at least one listing card are visible above the
   fold (FR-001, Acceptance Scenario 1).
3. Resize the browser to 360px width — confirm no horizontal scroll and primary actions remain
   reachable (FR-008, SC-004).
4. Throttle network to "Slow 3G" in devtools, reload — confirm skeleton placeholders appear before
   listing images load, with no layout shift (FR-009).
5. Filter/search to an empty result set (or use a test account with no nearby listings) — confirm
   an inviting empty state with next-step guidance is shown (FR-010).

## Validate User Story 2 — Buyer checkout (P1)

1. From the discovery grid, click into a listing with `portionsRemaining > 0`.
2. Confirm listing detail shows photo/gallery, price, quantity selector, allergen info, and a
   prominent claim CTA (FR-003).
3. Increase quantity and confirm the total price updates live before submitting (FR-004).
4. Proceed through checkout; confirm a step indicator is visible at each stage (FR-004).
5. Complete payment with a Stripe test card; confirm a confirmation screen with pickup details
   appears (FR-005).
6. Navigate away, then to Order History; confirm the same confirmation is retrievable (FR-005).
7. In a second browser/tab, claim the last remaining portion of a listing while the first tab's
   detail page is open; confirm the first tab's claim CTA becomes disabled/marked sold out without
   a manual reload (FR-011).

## Validate User Story 3 — Cook dashboard (P2)

1. Log in as a cook (or use a seeded test cook account).
2. Confirm each listing on the dashboard shows a status badge (draft/published/partially claimed/
   sold out/expired) and stats (views, portions claimed) (FR-006).
3. Start creating a new listing; confirm a live preview of the buyer-facing card updates as the
   form is filled in (FR-007).
4. As an unverified test cook, confirm the verification-state indicator is neutral/informative,
   not alarming (Edge Cases).

## Automated checks

```powershell
cd frontend
npm run test          # Vitest unit/component tests, incl. ui/ primitives and view states
npm run test:e2e       # Playwright, incl. responsive breakpoints (360/768/1440) and a11y checks
```

Lighthouse mobile audits (Performance + Accessibility ≥ 90, SC-003) should be run against the
Discovery, Listing Detail, and Checkout pages before considering this feature done.

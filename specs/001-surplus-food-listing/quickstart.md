# Quickstart: Post & Claim Surplus Food Listing

Validation guide proving the feature works end-to-end. Assumes the backend, PostgreSQL/PostGIS,
and Stripe test-mode keys are configured per the eventual implementation tasks (see tasks.md).

## Prerequisites

- Backend running locally with a seeded database containing:
  - One `CookProfile` with `verification_status = verified`.
  - One `BuyerProfile` with a `default_discovery_radius_km` covering the test listing's location.
- Stripe test-mode API keys configured; use Stripe's test card `4242 4242 4242 4242` for
  successful payments and `4000 0000 0000 0002` for forced declines.

## Scenario 1: Cook publishes a listing (User Story 1)

1. Authenticate as the verified cook.
2. `POST /api/v1/listings` with a valid payload (see contracts/listings-api.md), `publish: true`.
3. Expect `201 Created` with `status: "published"`.
4. Repeat with a cook whose `verification_status != verified` → expect `403 Forbidden`.

**Pass condition**: only verified cooks can reach `published` status (FR-001).

## Scenario 2: Buyer discovers and claims (User Story 2)

1. `GET /api/v1/listings?lat=...&lng=...` using the buyer's location, matching the seeded
   listing's proximity. Expect the listing in the response with cook verification/rating visible
   (FR-004).
2. `POST /api/v1/claims` with `listing_id` and `quantity: 1`. Expect `201 Created` with a Stripe
   client secret.
3. Complete payment client-side using Stripe test card `4242 4242 4242 4242`.
4. Confirm the Stripe webhook fires `payment_intent.succeeded`; verify `Claim.payment_status =
   paid` and a corresponding `ImpactRecord` row exists (FR-009).
5. Re-fetch the listing; `portions_remaining` decreased by 1 (FR-006).

**Pass condition**: full claim-to-paid flow completes and produces an impact record.

## Scenario 3: Race condition — last portion (Acceptance Scenario, User Story 2)

1. Seed a listing with `portions_remaining = 1`.
2. Fire two concurrent `POST /api/v1/claims` requests for quantity 1 from two different buyers.
3. Expect exactly one `201 Created` and one `409 Conflict` (`insufficient_portions`).

**Pass condition**: no overselling under concurrency (SC-003).

## Scenario 4: Payment timeout release (FR-008)

1. Create a claim, do not complete payment.
2. Advance time (or use a shortened test timeout) past the reservation window.
3. Run the background release job.
4. Verify `Claim.payment_status = payment_failed` and `Listing.portions_remaining` restored.

## Scenario 5: Listing expiry and no-show (FR-007, FR-012)

1. Seed a `published` listing with `pickup_window_end` in the near past and one `paid` claim with
   `pickup_status = pending`.
2. Run the background expiry job.
3. Verify listing `status = expired` and the claim's `pickup_status = no_show`, with no refund
   issued.

## Scenario 6: Cook cancels a listing with existing claims (User Story 3, FR-010)

1. Seed a `published` listing with one `paid` claim.
2. `PATCH /api/v1/listings/{id}` with `status: cancelled` as the owning cook.
3. Verify the claim's `payment_status` becomes `refunded` and a notification is triggered (exact
   notification channel is an implementation detail for a later task).

## Out of scope for this quickstart

- Cook verification/onboarding flow (separate feature).
- Gamification/impact badges (deferred, Principle IV).
- Native mobile clients (deferred per plan.md platform decision).

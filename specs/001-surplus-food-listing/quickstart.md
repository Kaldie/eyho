# Quickstart: Post & Claim Surplus Food Listing

Validation guide proving the feature works end-to-end. Assumes the backend, PostgreSQL/PostGIS,
and Stripe test-mode keys are configured per the eventual implementation tasks (see tasks.md).

## Prerequisites

- Backend running locally with a seeded database containing:
  - One `CookProfile` with `verification_status = verified`.
  - One `CookProfile` with `verification_status = unverified`.
  - One `BuyerProfile` with a `default_discovery_radius_km` covering the test listing's location.
- Stripe test-mode API keys configured; use Stripe's test card `4242 4242 4242 4242` for
  successful payments and `4000 0000 0000 0002` for forced declines.

## Scenario 1: Cook publishes a listing (User Story 1)

1. Authenticate as the verified cook.
2. `POST /api/v1/listings` with a valid payload (see contracts/listings-api.md), `publish: true`.
3. Expect `201 Created` with `status: "published"` and `cook.verification_status: "verified"`.
4. Repeat as the unverified cook → expect `201 Created` with `status: "published"` and
   `cook.verification_status: "unverified"`.
5. `GET /api/v1/listings` as a buyer → both listings appear; the unverified cook's listing is
   returned with `verification_status: "unverified"`, plus `rating_avg` and `rating_count`.

**Pass condition**: any cook can publish (FR-001), and buyers can always see each cook's
verification status, rating and review count (FR-004). No listing is rejected because of
verification status.

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

## Scenario 7: Rating drives verification (FR-013, FR-014)

1. Seed the unverified cook with 5 listings, each with one `paid` claim at
   `pickup_status = collected`.
2. As each claim's buyer, `POST /api/v1/claims/{id}/rating` with `stars: 5`.
3. After the fifth rating, verify the cook's `verification_status = verified`, the
   since-transition counters are reset to 0, `verification_cycle_count = 1`, and one
   `VerificationHistory` row exists listing all 5 triggering claims.
   (Ratings count per buyer, so 5 buyers rating one listing would promote the cook too.)
4. Rate the same claim again → expect `409` with `already_rated`.
5. Rate a claim whose `pickup_status = pending` → expect `409` with `not_collected`.
6. Repeat the pattern with 3 ratings of `stars: 1` on the now-verified cook → verify demotion to
   `unverified`, a second `VerificationHistory` row, `verification_cycle_count = 2`, and that the
   cook's published listings are still `published` but now return `"unverified"`.

## Out of scope for this quickstart

- Identity / food-handling registration checks, if any (separate feature; crowd-sourced
  verification itself is covered by Scenario 7).
- Food-safety reporting/flagging and its response SLA (deferred; see plan.md Complexity
  Tracking).
- Gamification/impact badges (deferred, Principle IV).
- Native mobile clients (deferred per plan.md platform decision).

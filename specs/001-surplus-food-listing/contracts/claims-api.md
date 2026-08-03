# API Contract: Claims

Base path: `/api/v1/claims`

## POST /api/v1/claims

Buyer claims a quantity of portions from a listing and initiates payment in a single flow
(FR-005).

**Request body**:

```json
{
  "listing_id": "uuid",
  "quantity": 1
}
```

**Behavior**:
1. Server atomically checks and decrements `Listing.portions_remaining` (FR-006; see
   data-model.md state transitions). If insufficient portions remain, no claim is created.
2. Server creates a `Claim` with `payment_status = pending_payment`, `reserved_at = now()`.
3. Server creates a Stripe PaymentIntent and returns its client secret for the frontend to
   complete payment.

**Responses**:
- `201 Created`:
  ```json
  {
    "claim_id": "uuid",
    "quantity": 1,
    "amount_due_cents": 500,
    "stripe_client_secret": "string",
    "payment_reservation_expires_at": "2026-08-04T18:10:00Z"
  }
  ```
- `409 Conflict` — not enough portions remaining; body: `{ "error": "insufficient_portions",
  "portions_remaining": 0 }` (covers the two-buyers-race Acceptance Scenario).
- `404 Not Found` — listing does not exist or is not `published`.

## POST /api/v1/claims/{id}/confirm-pickup

Buyer (or cook, scanning a code) confirms the buyer collected the food.

- `200 OK` — `pickup_status` set to `collected`.
- `409 Conflict` — claim is not in `paid` state, or pickup window has already elapsed and claim
  was already marked `no_show`.

## Stripe webhook: POST /api/v1/webhooks/stripe

Internal endpoint, not called by the frontend directly.

- `payment_intent.succeeded` → set `Claim.payment_status = paid`, `confirmed_at = now()`, create
  the corresponding `ImpactRecord` (FR-009).
- `payment_intent.payment_failed` → set `Claim.payment_status = payment_failed`, release
  reserved portions back to `Listing.portions_remaining` (FR-008).

## Background job contract (not an HTTP endpoint)

Runs every 60s:
- Any `Claim` with `payment_status = pending_payment` and `reserved_at` older than the
  reservation timeout (default 10 minutes) → release portions, set `payment_status =
  payment_failed` (FR-008).
- Any `Listing` with `status = published` and `pickup_window_end` in the past → set `status =
  expired` (FR-007); any of its claims still `paid` and `pickup_status = pending` → set
  `pickup_status = no_show` (FR-012, non-refundable).

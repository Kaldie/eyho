# API Contract: Claims

Base path: `/api/v1/claims`

## Authentication

All endpoints below follow feature 004's rules (`specs/004-accounts-sign-in/contracts/accounts-api.md`):
requests carry `Authorization: Bearer <access token>`; the backend derives the acting buyer or
cook from the token. Common errors on protected endpoints: `401 {"error": "unauthenticated"}`,
`403 {"error": "email_not_verified"}`, `403 {"error": "cook_profile_required"}`.

| Endpoint | Access |
|---|---|
| `POST /claims` | Signed in, verified email; `403 {"error": "own_listing"}` if the buyer is the listing's cook |
| `GET /claims`, `GET /claims/{id}` | Signed in (own claims; cooks may read claims on their listings) |
| `POST /claims/{id}/confirm-pickup` | Signed in, the claim's buyer or the listing's cook |
| `POST /claims/{id}/rating` | Signed in, the claim's buyer |
| `POST /webhooks/stripe` | Stripe signature only, no user token |

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

## GET /api/v1/claims

Returns the authenticated buyer's own claims, newest first. This is the buyer's order history
(feature 002, `contracts/order-history.md`).

**Query params**: `cursor` (optional, opaque), `limit` (optional, default 20, max 100).

**Response** `200 OK`:

```json
{
  "claims": [
    {
      "id": "uuid",
      "quantity": 2,
      "amount_paid_cents": 1000,
      "payment_status": "paid",
      "pickup_status": "pending",
      "rating": null,
      "created_at": "2026-08-04T17:05:00Z",
      "listing": {
        "id": "uuid",
        "food_description": "string",
        "photo_url": "string | null",
        "pickup_address_text": "string",
        "pickup_window_start": "2026-08-04T17:00:00Z",
        "pickup_window_end": "2026-08-04T19:00:00Z",
        "cook_display_name": "string"
      }
    }
  ],
  "next_cursor": "string | null"
}
```

Claims with `payment_status = payment_failed` are omitted (they never became orders).

- `401 Unauthorized` — not authenticated as a buyer.

## GET /api/v1/claims/{id}

Returns a single claim in the same shape as a `GET /api/v1/claims` item. Used to re-open the
order confirmation screen.

- `404 Not Found` — the claim does not exist, or the requester is neither its buyer nor the
  listing's cook.

## POST /api/v1/claims/{id}/confirm-pickup

Buyer (or cook, scanning a code) confirms the buyer collected the food.

- `200 OK` — `pickup_status` set to `collected`.
- `409 Conflict` — claim is not in `paid` state, or pickup window has already elapsed and claim
  was already marked `no_show`.

## POST /api/v1/claims/{id}/rating

Buyer rates a listing after collecting it (FR-013). One rating per claim.

**Request body**:

```json
{ "stars": 4 }
```

**Behavior**:
1. Store `stars` and `rated_at` on the claim.
2. Recompute the cook's `rating_avg` / `rating_count`.
3. Evaluate the cook's verification thresholds per data-model.md (5 good → verified, 3 bad →
   unverified). Any transition writes a `VerificationHistory` row in the same transaction.

**Responses**:
- `201 Created` — `{ "claim_id": "uuid", "stars": 4, "rated_at": "..." }`
- `409 Conflict` — claim is not `collected`, or has already been rated; body:
  `{ "error": "not_collected" | "already_rated" }`.
- `403 Forbidden` — requester is not the claim's buyer.
- `422 Unprocessable Entity` — `stars` is not an integer from 1 to 5.

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

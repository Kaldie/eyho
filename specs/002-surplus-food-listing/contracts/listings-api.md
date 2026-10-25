# API Contract: Listings

Base path: `/api/v1/listings`

## Authentication

All endpoints below follow feature 001's rules (`specs/001-accounts-sign-in/contracts/accounts-api.md`):
requests carry `Authorization: Bearer <access token>`; the backend derives the acting buyer or
cook from the token. Common errors on protected endpoints: `401 {"error": "unauthenticated"}`,
`403 {"error": "email_not_verified"}`, `403 {"error": "cook_profile_required"}`.

| Endpoint | Access |
|---|---|
| `GET /listings`, `GET /listings/{id}` | Public (signed out allowed) |
| `POST /listings` | Signed in, verified email, cook profile |
| `PATCH /listings/{id}` | Signed in, verified email, owning cook |
| `GET /me/listings` | Signed in, verified email not required, cook profile (`403 cook_profile_required` otherwise) |

`POST /listings` takes `cook_id` from the token's account, never from the body. If
`pickup_location` / `pickup_address_text` are omitted, the cook's default pickup address
(feature 001 cook onboarding) is used.

## POST /api/v1/listings

Create a new listing for the authenticated cook. Any registered cook may publish, verified or not
(FR-001, Constitution Principle I and Product Constraints v3.1.0). A listing from a cook whose current `verification_status` is
`unverified` is published normally but flagged "unverified" in every buyer-facing response.

**Request body**:

```json
{
  "food_description": "string",
  "allergen_info": "string",
  "photo_url": "https://... (optional)",
  "total_portions": 1,
  "reduced_price_cents": 500,
  "retail_reference_price_cents": 1500,
  "pickup_location": { "lat": 52.37, "lng": 4.90 },
  "pickup_address_text": "string",
  "pickup_window_start": "2026-08-04T17:00:00Z",
  "pickup_window_end": "2026-08-04T19:00:00Z",
  "publish": true
}
```

**Responses**:
- `201 Created` — returns the created listing: status `published` if `publish: true`, otherwise
  `draft`, regardless of the cook's verification status. The response includes the same `cook`
  block as `GET /api/v1/listings`, so the cook can see how the listing is flagged.
- `403 Forbidden` — the requester has no cook profile (buyers cannot create listings), or
  `{"error": "cook_not_registered", "registration_status": "pending" | "inactive"}` when the
  cook's KvK registration isn't confirmed active (feature 001 FR-020). Drafts (`publish: false`)
  are still saved for unregistered cooks. Never returned because of verification status.
- `422 Unprocessable Entity` — validation failure, e.g. `reduced_price_cents >=
  retail_reference_price_cents` (FR-003) or `pickup_window_end` in the past (Edge Case). Body
  includes `{ "error": "...", "field": "..." }`. Codes: `missing_field`, `invalid_field`,
  `price_not_reduced` (field `reduced_price_cents`), `window_invalid` (end not after start) and
  `window_in_past` (field `pickup_window_end`). `allergen_info` is required but may be `""`.

The 201 body (and the GET detail and PATCH bodies) is the detail shape below, which adds
`total_portions`, `retail_reference_price_cents` and `pickup_location {lat, lng}` to the fields the
contract lists. `distance_km` is absent from create and update responses.

## GET /api/v1/listings

Discover listings near the buyer. Default scope is the buyer's local radius (FR-011, Principle
III).

**Query params**: `lat`, `lng` (required; `422 missing_field` / `invalid_field`), `radius_km`
(optional, greater than 0 and at most 100; defaults to the signed-in buyer's `default_discovery_radius_km`, else 2 km),
`status` (optional; only `published` is accepted, other values are `422` because non-published
listings are never public). Results: published listings whose pickup window has not ended, at most
100, non-profits first, then by distance.

**Response** `200 OK`:

```json
{
  "listings": [
    {
      "id": "uuid",
      "food_description": "string",
      "photo_url": "string | null",
      "reduced_price_cents": 500,
      "portions_remaining": 3,
      "pickup_window_start": "...",
      "pickup_window_end": "...",
      "distance_km": 1.2,
      "cook": {
        "display_name": "string",
        "org_type": "nonprofit",
        "verification_status": "unverified",
        "rating_avg": 4.8,
        "rating_count": 12,
        "verification_cycle_count": 0
      }
    }
  ]
}
```

(Per FR-004: cook verification status, aggregate rating and review count (`rating_count`) are
always included. Clients MUST render an "unverified" flag whenever `verification_status` is
`unverified`. `verification_cycle_count` supports the buyer-visible transition summary that
Principle I recommends.)

## GET /api/v1/listings/{id}

Returns full listing detail (same shape as list item, plus `allergen_info`, full
`pickup_address_text`, `status`, `total_portions`, `retail_reference_price_cents`,
`pickup_location`).

**Optional query params**: `lat` and `lng` together (both or neither, else `422`). When given, the
response includes `distance_km` from that point; otherwise `distance_km` is absent (not `null`).

- `404 Not Found` if the listing does not exist or is not visible to the requester. Published
  listings are public; draft, expired and cancelled listings are visible only to the owning cook.

## PATCH /api/v1/listings/{id}

Owning cook only (signed in, verified email, cook profile). Body fields, all optional but at least
one required (`422 missing_field` otherwise):

| Field | Meaning |
|---|---|
| `portions_remaining` | Manual adjustment, integer `0..total_portions` (`422 invalid_field` otherwise). It may not exceed the unclaimed capacity, `total_portions` minus the quantity held by paid and unpaid-reserved claims; that would allow overselling: `409 {"error": "portions_conflict"}`. Allowed on `draft` and `published` listings. |
| `pickup_window_end` | Extend only: must be later than the current end (`422 window_not_extended`). Allowed on `draft` and `published` listings. |
| `status` | `"cancelled"` (from `published` only) or `"published"` (from `draft` only). Any other value is `422 invalid_field`. |

State machine (data-model.md): `draft -> published` and `published -> cancelled` are the only
transitions this endpoint performs.

**Publishing a draft** (`status: "published"`): owner only; the listing must be a draft
(`409 {"error": "listing_not_draft"}` otherwise); the pickup window, after applying any
`pickup_window_end` sent in the same request, must still end in the future
(`422 window_in_past`); and the same registration gate as `POST /listings` with `publish: true`
applies: `403 {"error": "cook_not_registered", "registration_status": "pending" | "inactive"}`.
Verification status plays no part (Principle I).

**Cancelling** (`status: "cancelled"`): see the refund rule below. A listing that is not
`published` answers `409 {"error": "listing_not_active"}`.

- `200 OK` - the updated listing (detail shape, no `distance_km`).
- `403 Forbidden` - requester is not the owning cook (`{"error": "not_owner"}`).
- `404 Not Found` - no such listing.
- `409 Conflict` - `portions_conflict`, `listing_not_draft` or `listing_not_active` as above.

> Deviation note: the original text said "attempting to reduce `portions_remaining` below the sum
> already claimed". Reducing the remaining count can never undercut existing claims, so the rule is
> implemented as the equivalent bound on the other side: the cook cannot raise `portions_remaining`
> beyond what is not held by claims.

## GET /api/v1/me/listings

The calling cook's own listings in every status (`draft`, `published`, `expired`, `cancelled`),
newest first (`created_at` desc), at most 200. Requires a cook profile (`401`, `403
cook_profile_required`).

**Response** `200 OK`: `{ "listings": [ ... ] }`, each item in the detail shape of
`GET /listings/{id}` (list-item fields, `allergen_info`, `pickup_address_text`, `status`,
`total_portions`, `retail_reference_price_cents`, `pickup_location`) plus:

```json
{ "portions_claimed": 3 }
```

`portions_claimed` is the quantity held by live claims: paid claims plus unpaid reservations.
Refunded, failed and released claims do not count. `distance_km` is absent.

A cook being demoted from `verified` to `unverified` does NOT change a published listing's
`status`; the listing simply starts being returned with `verification_status: "unverified"`
(spec Edge Case).

Cancelling a listing with existing `paid` claims MUST trigger refund + notification per FR-010
(handled server-side as a side effect, not a separate client call).

Cancelling refunds every `paid` claim that has not been collected through the payment provider,
marks it `refunded` and queues one buyer notification (outbox table `notifications`, delivery
channel is a later task). Unpaid reservations are dropped (`payment_failed`); a payment that still
arrives for one is refunded automatically. A refund the provider rejects leaves the claim `paid`
and is retried by the once-a-minute job; the cancel itself still answers `200`.

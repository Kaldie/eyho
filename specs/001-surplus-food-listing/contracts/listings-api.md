# API Contract: Listings

Base path: `/api/v1/listings`

## POST /api/v1/listings

Create a new listing (draft) for the authenticated cook. Requires `verification_status =
verified` (FR-001) to transition to `published`.

**Request body**:

```json
{
  "food_description": "string",
  "allergen_info": "string",
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
- `201 Created` — returns the created listing (status `published` if cook is verified and
  `publish: true`, otherwise `draft`).
- `403 Forbidden` — cook is not verified; body: `{ "error": "cook_not_verified" }` (FR-001).
- `422 Unprocessable Entity` — validation failure, e.g. `reduced_price_cents >=
  retail_reference_price_cents` (FR-003) or `pickup_window_end` in the past (Edge Case). Body
  includes `{ "error": "...", "field": "..." }`.

## GET /api/v1/listings

Discover listings near the buyer. Default scope is the buyer's local radius (FR-011, Principle
III).

**Query params**: `lat`, `lng` (required), `radius_km` (optional, defaults to buyer's
`default_discovery_radius_km`), `status` (optional, defaults to `published`).

**Response** `200 OK`:

```json
{
  "listings": [
    {
      "id": "uuid",
      "food_description": "string",
      "reduced_price_cents": 500,
      "portions_remaining": 3,
      "pickup_window_start": "...",
      "pickup_window_end": "...",
      "distance_km": 1.2,
      "cook": {
        "display_name": "string",
        "org_type": "nonprofit",
        "verification_status": "verified",
        "rating_avg": 4.8
      }
    }
  ]
}
```

(Per FR-004: cook verification status and rating are always included.)

## GET /api/v1/listings/{id}

Returns full listing detail (same shape as list item, plus `allergen_info`, full
`pickup_address_text`, `status`).

- `404 Not Found` if the listing does not exist or is not visible to the requester's role.

## PATCH /api/v1/listings/{id}

Cook-only. Update `portions_remaining` (manual adjustment), `pickup_window_end` (extend), or
`status` (`cancelled`).

- `200 OK` — updated listing.
- `403 Forbidden` — requester is not the owning cook.
- `409 Conflict` — attempting to reduce `portions_remaining` below the sum already claimed.

Cancelling a listing with existing `paid` claims MUST trigger refund + notification per FR-010
(handled server-side as a side effect, not a separate client call).

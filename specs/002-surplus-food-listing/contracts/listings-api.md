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
| `PATCH /listings/{id}` | Signed in, owning cook |

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
`pickup_address_text`, `status`).

- `404 Not Found` if the listing does not exist or is not visible to the requester's role.

## PATCH /api/v1/listings/{id}

Cook-only. Update `portions_remaining` (manual adjustment), `pickup_window_end` (extend), or
`status` (`cancelled`).

- `200 OK` — updated listing.
- `403 Forbidden` — requester is not the owning cook.
- `409 Conflict` — attempting to reduce `portions_remaining` below the sum already claimed.

A cook being demoted from `verified` to `unverified` does NOT change a published listing's
`status`; the listing simply starts being returned with `verification_status: "unverified"`
(spec Edge Case).

Cancelling a listing with existing `paid` claims MUST trigger refund + notification per FR-010
(handled server-side as a side effect, not a separate client call).

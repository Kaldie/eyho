# Contract: Order/Claim History (supports FR-005)

Order history is served by feature 001's claims API — there is no separate `/api/orders`
endpoint. The backend contract (paths, snake_case wire shape, pagination, error codes) lives in
`specs/001-surplus-food-listing/contracts/claims-api.md`:

- `GET /api/v1/claims` — the authenticated buyer's claims, newest first (failed payments
  excluded).
- `GET /api/v1/claims/{id}` — a single claim, used to re-open the confirmation screen.

This document only defines how the frontend maps that response into its `OrderSummary` view
model (see data-model.md). The mapping lives in `services/api.ts` (`toOrderSummary`).

| `OrderSummary` field | Source field on a claim item |
|---|---|
| `orderId` | `id` |
| `listingSummary.foodName` | `listing.food_description` |
| `listingSummary.photoUrl` | `listing.photo_url` |
| `quantity` | `quantity` |
| `totalPrice` | `amount_paid_cents` (cents, same unit as listing prices) |
| `pickupLocation.label` | `listing.cook_display_name` |
| `pickupLocation.address` | `listing.pickup_address_text` |
| `pickupWindow.start` / `.end` | `listing.pickup_window_start` / `listing.pickup_window_end` |
| `claimedAt` | `created_at` |

**Errors**: `401` when not authenticated; `404` from the detail endpoint when the claim doesn't
exist or isn't the buyer's.

## Frontend consumption notes

- `services/api.ts` gains `getOrders()` and `getOrder(orderId)`, both wrapped in TanStack Query
  hooks (`useOrders`, `useOrder`).
- `OrderHistoryPage` renders a list using the existing card visual language; selecting an order
  navigates to a confirmation view reusing the `OrderConfirmationView` component from the
  checkout flow (FR-005's "way to access this confirmation again later").

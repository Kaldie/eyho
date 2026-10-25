# Contract: Order/Claim History (supports FR-005)

This feature assumes the following endpoint exists or is added in coordination with
`backend-engineer`. It is a read contract only — no new write behavior. If Feature 001's backend
already exposes an equivalent "my claims" endpoint, this frontend feature consumes it as-is under
this shape; otherwise this doc is the requested contract.

## GET /api/orders (buyer-scoped, authenticated)

Returns the authenticated buyer's claim/order history, newest first.

**Response 200**:

```json
{
  "orders": [
    {
      "orderId": "string",
      "listingSummary": { "foodName": "string", "photoUrl": "string | null" },
      "quantity": 0,
      "totalPrice": 0,
      "pickupLocation": { "label": "string", "address": "string" },
      "pickupWindow": { "start": "ISO-8601", "end": "ISO-8601" },
      "claimedAt": "ISO-8601"
    }
  ]
}
```

**Response 401**: Buyer not authenticated.

## GET /api/orders/{orderId}

Returns a single order confirmation (used to re-open the confirmation screen, FR-005).

**Response 200**: Single object matching the shape above (without the `orders` wrapper).

**Response 404**: Order does not exist or does not belong to the authenticated buyer.

## Frontend consumption notes

- `services/api.ts` gains `getOrders()` and `getOrder(orderId)`, both wrapped in TanStack Query
  hooks (`useOrders`, `useOrder`).
- `OrderHistoryPage` renders a list using the existing card visual language; selecting an order
  navigates to a confirmation view reusing the `OrderConfirmationView` component from the
  checkout flow (FR-005's "way to access this confirmation again later").

# Data Model: Modern, Inviting Marketplace Frontend

This feature is a frontend visual/UX layer over existing backend entities (Feature 001). No new
persisted data entities are introduced. The entities below are **view models** — client-side
shapes the UI renders — derived from existing/assumed backend API responses.

## ListingCardView

Rendered in the discovery grid (FR-002).

| Field | Type | Notes |
|---|---|---|
| id | string | Listing identifier |
| photoUrl | string \| null | null → render branded placeholder (Edge Cases) |
| foodName | string | |
| price | number (cents) | Formatted as currency in UI |
| portionsRemaining | number | 0 → card shows "Sold out" state (FR-011) |
| pickupWindow | { start: string; end: string } (ISO 8601) | Formatted as relative/local time |
| pickupLocation | { label: string; distanceMeters?: number } | Distance shown when buyer location known |
| cookVerificationStatus | "verified" \| "unverified" | Neutral badge styling per Edge Cases |
| status | "published" \| "sold_out" \| "expired" | Drives card visual state |

## ListingDetailView

Extends `ListingCardView` for the single-listing page (FR-003).

| Field | Type | Notes |
|---|---|---|
| ...ListingCardView fields | | |
| description | string | |
| allergenInfo | string[] | |
| photos | string[] | Gallery; empty → single placeholder |
| cookName | string | |
| cookRatingAverage | number \| null | null when cook has no ratings yet |
| cookRatingCount | number | |
| maxClaimableQuantity | number | min(portionsRemaining, backend per-buyer cap) |

## CheckoutState (client-only, not persisted)

Tracks progress through the claim/checkout flow (FR-004).

| Field | Type | Notes |
|---|---|---|
| step | "quantity" \| "payment" \| "confirmation" | Drives StepIndicator |
| listingId | string | |
| quantity | number | |
| totalPrice | number (cents) | Derived: quantity × listing.price, recomputed live |
| paymentIntentStatus | "idle" \| "processing" \| "succeeded" \| "failed" | "failed" = declined/error (FR-014); rendered inline, distinct from connection-drop retry |
| blockedReason | "expired" \| null | Set when pickup window expires mid-checkout (FR-015); when set, submission is blocked regardless of paymentIntentStatus |

Note: `CheckoutState` is scoped to exactly one `listingId` per session (FR-016) — no array of
line items; a new checkout session is required to claim from a different listing.

## OrderConfirmationView

Shown at end of checkout and re-derivable later via Order History (FR-005).

| Field | Type | Notes |
|---|---|---|
| orderId | string | |
| listingSummary | { foodName: string; photoUrl: string \| null } | |
| quantity | number | |
| totalPrice | number (cents) | |
| pickupLocation | { label: string; address: string } | |
| pickupWindow | { start: string; end: string } | |
| claimedAt | string (ISO 8601) | |

## CookDashboardListingView

Rendered per-listing on the cook dashboard (FR-006, FR-007).

| Field | Type | Notes |
|---|---|---|
| id | string | |
| status | "draft" \| "published" \| "partially_claimed" \| "sold_out" \| "expired" | |
| foodName | string | |
| portionsTotal | number | |
| portionsClaimed | number | |
| views | number | |
| verificationStatus | "verified" \| "unverified" | Own status, neutral treatment |
| cardPreview | ListingCardView | Live preview shown while editing (FR-007) |

## Relationships

- `ListingCardView` and `ListingDetailView` are read projections of the same backend `Listing`
  entity (owned by Feature 001) — this feature does not alter that entity's schema.
- `CheckoutState` is ephemeral client state; on success it produces one `OrderConfirmationView`,
  persisted server-side as an order/claim record (Feature 001 scope) and retrieved later via the
  Order History contract (see `contracts/order-history.md`).
- `CookDashboardListingView` is a per-cook read projection plus the `cardPreview` computed
  client-side from in-progress form state (not yet submitted).

## Validation rules (client-side, mirroring FR requirements)

- `quantity` MUST be a positive integer ≤ `maxClaimableQuantity`; UI MUST disable increment past
  this bound and MUST disable the claim CTA entirely when `portionsRemaining === 0` (FR-011).
- `photoUrl` / `photos` absent or failing to load MUST fall back to the branded placeholder, never
  a broken-image icon or blank box (Edge Cases).
- `pickupWindow.end` in the past MUST render the listing as `expired` regardless of `status` value
  reported by the API, as a UI-side safety net.
- `CheckoutState.blockedReason` MUST be set (and payment submission MUST be prevented) if
  `pickupWindow.end` has passed at submit-time, even if the buyer began checkout before expiry
  (FR-015). A 4xx "no longer available" response from the backend at submit-time MUST be treated
  as authoritative and set `blockedReason` even if the client-side clock check passed.
- `paymentIntentStatus === "failed"` MUST render a distinct inline error from a network/connection
  failure (FR-014); both MUST preserve `quantity` and `step`.

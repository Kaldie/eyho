// Shared client-side view-model types per data-model.md.
// These are read/derived projections rendered by the UI — they do not represent new
// persisted backend entities (Feature 002 owns the underlying Listing/Order data).

export type VerificationStatus = 'verified' | 'unverified'

/** Rendered in the discovery grid (FR-002). */
export interface ListingCardView {
  id: string
  /** null → render branded placeholder (Edge Cases) via ImageWithFallback. */
  photoUrl: string | null
  foodName: string
  /** Price in cents; formatted as currency in the UI. */
  price: number
  /** 0 → card shows "Sold out" state (FR-011). */
  portionsRemaining: number
  pickupWindow: { start: string; end: string }
  pickupLocation: { label: string; distanceMeters?: number }
  /** Neutral badge styling per Edge Cases — never hidden, only labeled. */
  cookVerificationStatus: VerificationStatus
  status: 'published' | 'sold_out' | 'expired'
}

/** Extends ListingCardView for the single-listing detail page (FR-003). */
export interface ListingDetailView extends ListingCardView {
  description: string
  allergenInfo: string[]
  /** Gallery; empty → single placeholder. */
  photos: string[]
  cookName: string
  /** null when cook has no ratings yet. */
  cookRatingAverage: number | null
  cookRatingCount: number
  /** min(portionsRemaining, backend per-buyer cap). */
  maxClaimableQuantity: number
}

export type CheckoutStep = 'quantity' | 'payment' | 'confirmation'
export type PaymentIntentStatus = 'idle' | 'processing' | 'succeeded' | 'failed'
export type BlockedReason = 'expired' | null

/**
 * Client-only state machine tracking progress through the claim/checkout flow (FR-004).
 * Scoped to exactly one listingId per session (FR-016) — no cart/line-item array.
 */
export interface CheckoutState {
  step: CheckoutStep
  listingId: string
  quantity: number
  /** Derived: quantity * listing.price, recomputed live. */
  totalPrice: number
  /** "failed" = declined/error (FR-014); rendered inline, distinct from connection-drop retry. */
  paymentIntentStatus: PaymentIntentStatus
  /** Set when pickup window expires mid-checkout (FR-015); blocks submission when set. */
  blockedReason: BlockedReason
}

/** Shown at end of checkout and re-derivable later via Order History (FR-005). */
export interface OrderConfirmationView {
  orderId: string
  listingSummary: { foodName: string; photoUrl: string | null }
  quantity: number
  totalPrice: number
  pickupLocation: { label: string; address: string }
  pickupWindow: { start: string; end: string }
  claimedAt: string
}

/** Rendered per-listing on the cook dashboard (FR-006, FR-007). */
export interface CookDashboardListingView {
  id: string
  status: 'draft' | 'published' | 'partially_claimed' | 'sold_out' | 'expired'
  foodName: string
  portionsTotal: number
  portionsClaimed: number
  views: number
  /** Own verification status, neutral treatment. */
  verificationStatus: VerificationStatus
  /** Live preview shown while editing (FR-007). */
  cardPreview: ListingCardView
}

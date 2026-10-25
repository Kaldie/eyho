// Typed API client matching contracts/listings-api.md, contracts/claims-api.md, and
// contracts/order-history.md.
// All functions currently return hardcoded mock data so the UI renders without a running backend.
// Replace the mock bodies with real fetch calls once the backend is available.

import { useQuery } from '@tanstack/react-query'

export type OrgType = 'nonprofit' | 'professional'
export type VerificationStatus = 'verified' | 'unverified'

export interface CookSummary {
  display_name: string
  org_type: OrgType
  /** Always present on every listing response per FR-004 and Principle I. */
  verification_status: VerificationStatus
  rating_avg: number
}

/** Shape returned by GET /api/v1/listings and GET /api/v1/listings/{id}. */
export interface ListingItem {
  id: string
  food_description: string
  reduced_price_cents: number
  portions_remaining: number
  pickup_window_start: string
  pickup_window_end: string
  distance_km: number
  cook: CookSummary
  /** null/absent → UI renders a branded placeholder (Edge Cases, FR-009). */
  photo_url?: string | null
  /** Only present on single-listing detail responses. */
  allergen_info?: string
  pickup_address_text?: string
  status?: 'draft' | 'published' | 'expired' | 'cancelled'
}

/** Shape returned by POST /api/v1/claims (201 Created). */
export interface ClaimCreated {
  claim_id: string
  quantity: number
  amount_due_cents: number
  stripe_client_secret: string
  payment_reservation_expires_at: string
}

// ---------------------------------------------------------------------------
// Mock data — replace with real fetch calls once the backend is running.
// ---------------------------------------------------------------------------

/**
 * Mock pickup times are relative to page load, rounded to the half hour, so mock
 * listings are always claimable instead of expiring on a fixed date.
 */
function mockTime(hoursFromNow: number): string {
  const d = new Date()
  d.setMinutes(d.getMinutes() < 30 ? 0 : 30, 0, 0)
  d.setTime(d.getTime() + hoursFromNow * 3_600_000)
  return d.toISOString()
}

const MOCK_LISTINGS: ListingItem[] = [
  {
    id: 'mock-listing-001',
    food_description: 'Vegetable lasagne (3 portions)',
    reduced_price_cents: 350,
    portions_remaining: 3,
    pickup_window_start: mockTime(0.5),
    pickup_window_end: mockTime(2.5),
    distance_km: 0.8,
    photo_url: 'https://images.unsplash.com/photo-1547592166-23ac45744acd?w=600&auto=format&fit=crop&q=60',
    allergen_info: 'Gluten, Dairy',
    pickup_address_text: 'Hoofdstraat 12, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'De Buurtkok',
      org_type: 'nonprofit',
      verification_status: 'verified',
      rating_avg: 4.7,
    },
  },
  {
    id: 'mock-listing-002',
    food_description: 'Lentil soup with crusty bread',
    reduced_price_cents: 200,
    portions_remaining: 5,
    pickup_window_start: mockTime(1),
    pickup_window_end: mockTime(3),
    distance_km: 1.4,
    photo_url: null,
    allergen_info: 'Gluten',
    pickup_address_text: 'Keizersgracht 88, Amsterdam',
    status: 'published',
    cook: {
      display_name: "Sanne's Kitchen",
      org_type: 'professional',
      // Unverified cook — Principle I: this status is always surfaced, never hidden.
      verification_status: 'unverified',
      rating_avg: 3.9,
    },
  },
  {
    id: 'mock-listing-003',
    food_description: 'Chickpea curry with rice',
    reduced_price_cents: 300,
    portions_remaining: 6,
    pickup_window_start: mockTime(0),
    pickup_window_end: mockTime(2),
    distance_km: 0.5,
    photo_url: null,
    allergen_info: '',
    pickup_address_text: 'Javastraat 41, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'Buurthuis Oost',
      org_type: 'nonprofit',
      verification_status: 'verified',
      rating_avg: 4.8,
    },
  },
  {
    id: 'mock-listing-004',
    food_description: 'Stamppot boerenkool with smoked sausage',
    reduced_price_cents: 450,
    portions_remaining: 2,
    pickup_window_start: mockTime(1),
    pickup_window_end: mockTime(2.5),
    distance_km: 1.1,
    photo_url: null,
    allergen_info: 'Dairy',
    pickup_address_text: 'Ferdinand Bolstraat 120, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'Café de Pijp',
      org_type: 'professional',
      verification_status: 'verified',
      rating_avg: 4.4,
    },
  },
  {
    id: 'mock-listing-005',
    food_description: 'Shakshuka with flatbread',
    reduced_price_cents: 325,
    portions_remaining: 4,
    pickup_window_start: mockTime(0.5),
    pickup_window_end: mockTime(1.5),
    distance_km: 1.7,
    photo_url: null,
    allergen_info: 'Eggs, Gluten',
    pickup_address_text: 'Bos en Lommerweg 210, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'Yara kookt',
      org_type: 'professional',
      verification_status: 'unverified',
      rating_avg: 4.2,
    },
  },
  {
    id: 'mock-listing-006',
    food_description: 'Pumpkin and sage risotto',
    reduced_price_cents: 400,
    portions_remaining: 0,
    pickup_window_start: mockTime(0),
    pickup_window_end: mockTime(1.5),
    distance_km: 0.9,
    photo_url: null,
    allergen_info: 'Dairy',
    pickup_address_text: 'Haarlemmerstraat 64, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'Keuken van Jordaan',
      org_type: 'nonprofit',
      verification_status: 'verified',
      rating_avg: 4.6,
    },
  },
]

// ---------------------------------------------------------------------------
// API functions
// ---------------------------------------------------------------------------

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

/**
 * GET /api/v1/listings?lat=&lng=&radius_km=
 *
 * Returns listings near the buyer. Default discovery scope is the buyer's local
 * radius per FR-011 and Principle III.
 */
export async function getListings(
  _lat: number,
  _lng: number,
  radiusKm?: number,
): Promise<ListingItem[]> {
  // TODO: replace with real fetch once the backend is running:
  // const params = new URLSearchParams({ lat: String(lat), lng: String(lng) })
  // if (radiusKm !== undefined) params.set('radius_km', String(radiusKm))
  // const res = await fetch(`${API_BASE}/api/v1/listings?${params}`)
  // if (!res.ok) throw new Error(`getListings failed: ${res.status}`)
  // const data = await res.json() as { listings: ListingItem[] }
  // return data.listings

  void API_BASE // suppress unused-var warning until real fetch is wired up
  // Mock honours the radius so the distance selector visibly works.
  return Promise.resolve(MOCK_LISTINGS.filter((l) => l.distance_km <= (radiusKm ?? 2)))
}

/**
 * GET /api/v1/listings/{id}
 *
 * Returns full listing detail including allergen_info, pickup_address_text, status.
 * Returns 404 if the listing does not exist or is not visible to the requester.
 */
export async function getListing(id: string): Promise<ListingItem> {
  // TODO: replace with real fetch:
  // const res = await fetch(`${API_BASE}/api/v1/listings/${id}`)
  // if (!res.ok) throw new Error(`getListing failed: ${res.status}`)
  // return res.json() as Promise<ListingItem>

  const found = MOCK_LISTINGS.find((l) => l.id === id)
  if (!found) return Promise.reject(new Error(`Listing ${id} not found`))
  return Promise.resolve(found)
}

/**
 * Error thrown by `createClaim` mock to simulate a backend 4xx rejection (e.g. the
 * listing/pickup window is no longer available). Per research.md, a 4xx response is
 * treated as authoritative even if the client-side expiry check passed.
 */
export class ClaimApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ClaimApiError'
  }
}

/**
 * POST /api/v1/claims
 *
 * Atomically reserves portions and initiates payment in a single step (FR-005, FR-006).
 * Returns a Stripe client secret for the frontend to complete payment with Stripe Elements.
 *
 * Mock-testing hooks (no real backend yet): passing a `listingId` of
 * `'network-error-listing'` simulates a dropped connection (rejects with a generic
 * `Error`), and `'expired-listing'` simulates a backend 409 "no longer available"
 * response (rejects with `ClaimApiError`). These let component/e2e tests exercise the
 * distinct error paths described in research.md without a running backend.
 */
export async function createClaim(
  listingId: string,
  quantity: number,
  unitPriceCents = 0,
): Promise<ClaimCreated> {
  // TODO: replace with real fetch:
  // const res = await fetch(`${API_BASE}/api/v1/claims`, {
  //   method: 'POST',
  //   headers: { 'Content-Type': 'application/json' },
  //   body: JSON.stringify({ listing_id: listingId, quantity }),
  // })
  // if (res.status >= 400 && res.status < 500) {
  //   const body = await res.json().catch(() => ({}))
  //   throw new ClaimApiError(res.status, body.message ?? 'Listing no longer available')
  // }
  // if (!res.ok) throw new Error(`createClaim failed: ${res.status}`)
  // return res.json() as Promise<ClaimCreated>

  if (listingId === 'network-error-listing') {
    return Promise.reject(new Error('Network request failed'))
  }
  if (listingId === 'expired-listing') {
    return Promise.reject(new ClaimApiError(409, 'This listing is no longer available.'))
  }

  return Promise.resolve({
    claim_id: `mock-claim-${listingId}`,
    quantity,
    amount_due_cents: quantity * unitPriceCents,
    stripe_client_secret: `pi_mock_secret_${listingId}`,
    payment_reservation_expires_at: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
  })
}

// ---------------------------------------------------------------------------
// Order history (contracts/order-history.md, supports FR-005)
// ---------------------------------------------------------------------------

/**
 * Wire shape of one item from GET /api/v1/claims and GET /api/v1/claims/{id}
 * (specs/002-surplus-food-listing/contracts/claims-api.md).
 */
export interface ClaimItem {
  id: string
  quantity: number
  amount_paid_cents: number
  payment_status: 'pending_payment' | 'paid' | 'refunded'
  pickup_status: 'pending' | 'collected' | 'no_show'
  rating: number | null
  created_at: string
  listing: {
    id: string
    food_description: string
    photo_url: string | null
    pickup_address_text: string
    pickup_window_start: string
    pickup_window_end: string
    cook_display_name: string
  }
}

/** Frontend view model for an order, mapped from a ClaimItem (see contracts/order-history.md). */
export interface OrderSummary {
  orderId: string
  listingSummary: { foodName: string; photoUrl: string | null }
  quantity: number
  totalPrice: number
  pickupLocation: { label: string; address: string }
  pickupWindow: { start: string; end: string }
  claimedAt: string
}

export function toOrderSummary(claim: ClaimItem): OrderSummary {
  return {
    orderId: claim.id,
    listingSummary: { foodName: claim.listing.food_description, photoUrl: claim.listing.photo_url },
    quantity: claim.quantity,
    totalPrice: claim.amount_paid_cents,
    pickupLocation: {
      label: claim.listing.cook_display_name,
      address: claim.listing.pickup_address_text,
    },
    pickupWindow: { start: claim.listing.pickup_window_start, end: claim.listing.pickup_window_end },
    claimedAt: claim.created_at,
  }
}

const MOCK_ORDERS: OrderSummary[] = [
  {
    orderId: 'mock-order-001',
    listingSummary: { foodName: 'Vegetable lasagne (3 portions)', photoUrl: null },
    quantity: 2,
    totalPrice: 700,
    pickupLocation: { label: 'De Buurtkok', address: 'Hoofdstraat 12, Amsterdam' },
    pickupWindow: { start: '2026-08-14T17:00:00Z', end: '2026-08-14T19:00:00Z' },
    claimedAt: '2026-08-14T15:32:00Z',
  },
]

/**
 * GET /api/v1/claims (buyer-scoped, authenticated)
 *
 * Returns the authenticated buyer's claim/order history, newest first (FR-005).
 */
export async function getOrders(): Promise<OrderSummary[]> {
  // TODO: replace with real fetch once the backend is running:
  // const res = await fetch(`${API_BASE}/api/v1/claims`)
  // if (!res.ok) throw new Error(`getOrders failed: ${res.status}`)
  // const data = (await res.json()) as { claims: ClaimItem[]; next_cursor: string | null }
  // return data.claims.map(toOrderSummary)

  return Promise.resolve(MOCK_ORDERS)
}

/**
 * GET /api/v1/claims/{id}
 *
 * Returns a single order confirmation, used to re-open the confirmation screen (FR-005).
 * Rejects if the order does not exist or does not belong to the authenticated buyer (404).
 */
export async function getOrder(orderId: string): Promise<OrderSummary> {
  // TODO: replace with real fetch:
  // const res = await fetch(`${API_BASE}/api/v1/claims/${orderId}`)
  // if (!res.ok) throw new Error(`getOrder failed: ${res.status}`)
  // return toOrderSummary((await res.json()) as ClaimItem)

  const found = MOCK_ORDERS.find((o) => o.orderId === orderId)
  if (!found) return Promise.reject(new Error(`Order ${orderId} not found`))
  return Promise.resolve(found)
}

// ---------------------------------------------------------------------------
// TanStack Query hooks
// ---------------------------------------------------------------------------

export function useOrders() {
  return useQuery({
    queryKey: ['orders'],
    queryFn: getOrders,
  })
}

export function useOrder(orderId: string) {
  return useQuery({
    queryKey: ['orders', orderId],
    queryFn: () => getOrder(orderId),
    enabled: Boolean(orderId),
  })
}

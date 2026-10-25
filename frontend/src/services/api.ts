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

const MOCK_LISTINGS: ListingItem[] = [
  {
    id: 'mock-listing-001',
    food_description: 'Vegetable lasagne (3 portions)',
    reduced_price_cents: 350,
    portions_remaining: 3,
    pickup_window_start: '2026-08-14T17:00:00Z',
    pickup_window_end: '2026-08-14T19:00:00Z',
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
    pickup_window_start: '2026-08-14T18:00:00Z',
    pickup_window_end: '2026-08-14T20:00:00Z',
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
  _radiusKm?: number,
): Promise<ListingItem[]> {
  // TODO: replace with real fetch once the backend is running:
  // const params = new URLSearchParams({ lat: String(lat), lng: String(lng) })
  // if (radiusKm !== undefined) params.set('radius_km', String(radiusKm))
  // const res = await fetch(`${API_BASE}/api/v1/listings?${params}`)
  // if (!res.ok) throw new Error(`getListings failed: ${res.status}`)
  // const data = await res.json() as { listings: ListingItem[] }
  // return data.listings

  void API_BASE // suppress unused-var warning until real fetch is wired up
  return Promise.resolve(MOCK_LISTINGS)
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

/** Shape returned by GET /api/orders and GET /api/orders/{orderId}. */
export interface OrderSummary {
  orderId: string
  listingSummary: { foodName: string; photoUrl: string | null }
  quantity: number
  totalPrice: number
  pickupLocation: { label: string; address: string }
  pickupWindow: { start: string; end: string }
  claimedAt: string
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
 * GET /api/orders (buyer-scoped, authenticated)
 *
 * Returns the authenticated buyer's claim/order history, newest first (FR-005).
 */
export async function getOrders(): Promise<OrderSummary[]> {
  // TODO: replace with real fetch once the backend is running:
  // const res = await fetch(`${API_BASE}/api/orders`)
  // if (!res.ok) throw new Error(`getOrders failed: ${res.status}`)
  // const data = await res.json() as { orders: OrderSummary[] }
  // return data.orders

  return Promise.resolve(MOCK_ORDERS)
}

/**
 * GET /api/orders/{orderId}
 *
 * Returns a single order confirmation, used to re-open the confirmation screen (FR-005).
 * Rejects if the order does not exist or does not belong to the authenticated buyer (404).
 */
export async function getOrder(orderId: string): Promise<OrderSummary> {
  // TODO: replace with real fetch:
  // const res = await fetch(`${API_BASE}/api/orders/${orderId}`)
  // if (!res.ok) throw new Error(`getOrder failed: ${res.status}`)
  // return res.json() as Promise<OrderSummary>

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

// Typed API client matching contracts/listings-api.md, contracts/claims-api.md, and
// contracts/order-history.md. Every function performs a real request through `apiFetch`
// (bearer token attached when signed in) and throws `ApiError` on a non-2xx response.

import { useQuery } from '@tanstack/react-query'

export type OrgType = 'nonprofit' | 'professional'
export type VerificationStatus = 'verified' | 'unverified'
export type ListingStatus = 'draft' | 'published' | 'expired' | 'cancelled'

export interface CookSummary {
  display_name: string
  org_type: OrgType
  /** Always present on every listing response per FR-004 and Principle I. */
  verification_status: VerificationStatus
  rating_avg: number
  /** Review count — always shown next to the rating (FR-004). */
  rating_count: number
  /** How often the cook's status has changed; feeds the transition summary (Principle I). */
  verification_cycle_count: number
}

/** Shape returned by GET /api/v1/listings and GET /api/v1/listings/{id}. */
export interface ListingItem {
  id: string
  food_description: string
  reduced_price_cents: number
  portions_remaining: number
  pickup_window_start: string
  pickup_window_end: string
  /** Absent on detail/create/update responses (no buyer position), so never assume it. */
  distance_km?: number
  cook: CookSummary
  /** null/absent → UI renders a branded placeholder (Edge Cases, FR-009). */
  photo_url?: string | null
  /** Only present on single-listing detail responses. */
  allergen_info?: string
  pickup_address_text?: string
  status?: ListingStatus
  /** Detail shape (detail, create, update and /me/listings responses). */
  total_portions?: number
  retail_reference_price_cents?: number
  pickup_location?: { lat: number; lng: number }
  /** Only on GET /me/listings: quantity held by paid claims and unpaid reservations. */
  portions_claimed?: number
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
// API functions
// ---------------------------------------------------------------------------

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

// ---------------------------------------------------------------------------
// Authenticated HTTP helper (feature 001, FR-012): every request to the backend carries
// `Authorization: Bearer <access token>` when a session exists. The token lives in memory
// only; `AuthProvider` registers the getter below.
// ---------------------------------------------------------------------------

let accessTokenGetter: () => string | null = () => null

/** Called by `AuthProvider` so `apiFetch` can attach the current access token. */
export function setAccessTokenGetter(getter: () => string | null): void {
  accessTokenGetter = getter
}

/**
 * Error for a non-2xx API response. `code` is the contract's `error` string
 * (e.g. `email_not_verified`, `active_claims`); `body` is the parsed JSON, if any.
 */
export class ApiError extends Error {
  status: number
  code: string | null
  body: Record<string, unknown>
  constructor(status: number, body: Record<string, unknown> = {}) {
    const code = typeof body.error === 'string' ? body.error : null
    super(code ?? `Request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.body = body
  }
}

/**
 * Readable text for a 422 `{ error, field }` body. The contract leaves `error` free-form
 * (a code or a sentence), so codes like `price_not_below_retail` are turned into a sentence.
 */
export function validationMessage(err: ApiError): string {
  const raw = err.code ?? 'invalid value'
  if (raw.includes(' ')) return raw
  const text = raw.replace(/_/g, ' ')
  return `${text.charAt(0).toUpperCase()}${text.slice(1)}.`
}

/** fetch() against the API base with the bearer token attached. Does not throw on non-2xx. */
export function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  const token = accessTokenGetter()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  return fetch(`${API_BASE}${path}`, { ...init, headers })
}

/** Reads a JSON body without throwing on empty or invalid bodies. */
export async function readJson(res: Response): Promise<Record<string, unknown>> {
  try {
    return (await res.json()) as Record<string, unknown>
  } catch {
    return {}
  }
}

/** Throws `ApiError` (with the parsed contract body) for any non-2xx response. */
export async function expectOk(res: Response): Promise<Response> {
  if (!res.ok) throw new ApiError(res.status, await readJson(res))
  return res
}

/**
 * GET /api/v1/listings?lat=&lng=&radius_km=
 *
 * Returns listings near the buyer. Default discovery scope is the buyer's local
 * radius per FR-011 and Principle III; `radiusKm` is only sent when the caller passes it
 * (the discovery page always does, starting at its local default).
 */
export async function getListings(
  lat: number,
  lng: number,
  radiusKm?: number,
  status?: ListingStatus,
): Promise<ListingItem[]> {
  const params = new URLSearchParams({ lat: String(lat), lng: String(lng) })
  if (radiusKm !== undefined) params.set('radius_km', String(radiusKm))
  if (status) params.set('status', status)
  const res = await expectOk(await apiFetch(`/api/v1/listings?${params}`))
  const data = (await res.json()) as { listings?: ListingItem[] }
  return data.listings ?? []
}

/**
 * GET /api/v1/listings/{id}
 *
 * Full listing detail including allergen_info, pickup_address_text, status.
 * 404 when the listing does not exist or is not visible to the requester.
 */
export async function getListing(
  id: string,
  at?: { lat: number; lng: number },
): Promise<ListingItem> {
  // lat and lng go together; with them the response carries distance_km.
  const query = at ? `?${new URLSearchParams({ lat: String(at.lat), lng: String(at.lng) })}` : ''
  const res = await expectOk(await apiFetch(`/api/v1/listings/${encodeURIComponent(id)}${query}`))
  return (await res.json()) as ListingItem
}

/** Body of POST /api/v1/listings (contracts/listings-api.md). `cook_id` is never sent. */
export interface CreateListingInput {
  food_description: string
  allergen_info: string
  photo_url?: string
  total_portions: number
  reduced_price_cents: number
  retail_reference_price_cents: number
  /** Omit both pickup fields to use the cook's default pickup address. */
  pickup_location?: { lat: number; lng: number }
  pickup_address_text?: string
  pickup_window_start: string
  pickup_window_end: string
  publish: boolean
}

/**
 * POST /api/v1/listings — 201 with the created listing (status `published` or `draft`).
 * Errors: 403 `cook_not_registered` (+ `registration_status`), 403 `email_not_verified`,
 * 403 `cook_profile_required`, 422 `{ error, field }`.
 */
export async function createListing(input: CreateListingInput): Promise<ListingItem> {
  const res = await expectOk(
    await apiFetch('/api/v1/listings', { method: 'POST', body: JSON.stringify(input) }),
  )
  return (await res.json()) as ListingItem
}

/** Body of PATCH /api/v1/listings/{id}: one of the three cook adjustments. */
export type UpdateListingInput =
  | { portions_remaining: number }
  | { pickup_window_end: string }
  | { status: 'cancelled' }
  /** Publishes a draft; `pickup_window_end` may extend an elapsed window in the same request. */
  | { status: 'published'; pickup_window_end?: string }

/**
 * PATCH /api/v1/listings/{id} — owning cook only. Errors: 409 `portions_conflict`,
 * `listing_not_draft`, `listing_not_active`; 422 `window_not_extended`, `window_in_past`;
 * 403 `not_owner` / `cook_not_registered`. Cancelling refunds paid claims server-side (FR-010).
 */
export async function updateListing(id: string, input: UpdateListingInput): Promise<ListingItem> {
  const res = await expectOk(
    await apiFetch(`/api/v1/listings/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    }),
  )
  return (await res.json()) as ListingItem
}

/**
 * GET /api/v1/me/listings — the signed-in cook's own listings in every status, newest
 * first (max 200), in the detail shape plus `portions_claimed`. Needs a cook profile
 * (403 `cook_profile_required`) but not a verified email.
 */
export async function getCookListings(): Promise<ListingItem[]> {
  const res = await expectOk(await apiFetch('/api/v1/me/listings'))
  const data = (await res.json()) as { listings?: ListingItem[] }
  return data.listings ?? []
}

/**
 * POST /api/v1/claims
 *
 * Atomically reserves portions and initiates payment in a single step (FR-005, FR-006).
 * Returns the Stripe client secret for the frontend to complete payment with Stripe Elements.
 * Errors: 409 `insufficient_portions` (+ `portions_remaining`), 403 `own_listing`,
 * 403 `email_not_verified`, 404 (listing missing or not published).
 */
export async function createClaim(listingId: string, quantity: number): Promise<ClaimCreated> {
  const res = await expectOk(
    await apiFetch('/api/v1/claims', {
      method: 'POST',
      body: JSON.stringify({ listing_id: listingId, quantity }),
    }),
  )
  return (await res.json()) as ClaimCreated
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
  payment_status: 'pending_payment' | 'paid' | 'refunded' | 'payment_failed'
  pickup_status: 'pending' | 'collected' | 'no_show'
  /** Stars 1-5 once the buyer has rated; null before. */
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

/**
 * Frontend view model for an order, mapped from a ClaimItem (see contracts/order-history.md).
 * `pickupStatus` and `rating` are additive fields for the rating prompt (T025); they are
 * optional so checkout, which has no claim item yet, can still build an order view.
 */
export interface OrderSummary {
  orderId: string
  listingSummary: { foodName: string; photoUrl: string | null }
  quantity: number
  totalPrice: number
  pickupLocation: { label: string; address: string }
  pickupWindow: { start: string; end: string }
  claimedAt: string
  pickupStatus?: ClaimItem['pickup_status']
  rating?: number | null
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
    pickupStatus: claim.pickup_status,
    rating: claim.rating,
  }
}

/**
 * GET /api/v1/claims (buyer-scoped, authenticated)
 *
 * Returns the authenticated buyer's claim/order history, newest first (FR-005). Only the
 * first page is requested; the history is short in v1 (`next_cursor` is not followed yet).
 */
export async function getOrders(): Promise<OrderSummary[]> {
  const res = await expectOk(await apiFetch('/api/v1/claims'))
  const data = (await res.json()) as { claims?: ClaimItem[]; next_cursor?: string | null }
  return (data.claims ?? []).map(toOrderSummary)
}

/**
 * GET /api/v1/claims/{id}
 *
 * A single order confirmation, used to re-open the confirmation screen (FR-005).
 * 404 when the order does not exist or does not belong to the authenticated buyer.
 */
export async function getOrder(orderId: string): Promise<OrderSummary> {
  const res = await expectOk(await apiFetch(`/api/v1/claims/${encodeURIComponent(orderId)}`))
  return toOrderSummary((await res.json()) as ClaimItem)
}

export interface RatingCreated {
  claim_id: string
  stars: number
  rated_at: string
}

/**
 * POST /api/v1/claims/{id}/rating (FR-013). Errors: 409 `already_rated` / `not_collected`,
 * 403 (not the claim's buyer, e.g. the cook's own listing), 422 (stars not 1-5).
 */
export async function rateClaim(claimId: string, stars: number): Promise<RatingCreated> {
  const res = await expectOk(
    await apiFetch(`/api/v1/claims/${encodeURIComponent(claimId)}/rating`, {
      method: 'POST',
      body: JSON.stringify({ stars }),
    }),
  )
  return (await res.json()) as RatingCreated
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

export const COOK_LISTINGS_QUERY_KEY = ['cook-listings'] as const

export function useCookListings(cookId: string) {
  return useQuery({
    queryKey: [...COOK_LISTINGS_QUERY_KEY, cookId],
    queryFn: getCookListings,
  })
}

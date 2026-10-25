/**
 * Route mocks that serve the real API shapes (specs/002-surplus-food-listing/contracts) so
 * the structural e2e specs run against `npx vite` without a backend.
 */
import type { Page, Route } from '@playwright/test'

export const LISTING_ID = 'listing-e2e-1'
export const UNVERIFIED_LISTING_ID = 'listing-e2e-2'

const inHours = (h: number) => new Date(Date.now() + h * 3_600_000).toISOString()

export function makeListing(over: Record<string, unknown> = {}) {
  return {
    id: LISTING_ID,
    food_description: 'Vegetable lasagne (3 portions)',
    photo_url: null,
    reduced_price_cents: 350,
    portions_remaining: 3,
    pickup_window_start: inHours(-0.5),
    pickup_window_end: inHours(2),
    distance_km: 0.8,
    allergen_info: 'Gluten, Dairy',
    pickup_address_text: 'Hoofdstraat 12, Amsterdam',
    status: 'published',
    cook: {
      display_name: 'De Buurtkok',
      org_type: 'nonprofit',
      verification_status: 'verified',
      rating_avg: 4.7,
      rating_count: 12,
      verification_cycle_count: 0,
    },
    ...over,
  }
}

export function makeUnverifiedListing() {
  return makeListing({
    id: UNVERIFIED_LISTING_ID,
    food_description: 'Lentil soup with crusty bread',
    reduced_price_cents: 200,
    portions_remaining: 5,
    cook: {
      display_name: "Sanne's Kitchen",
      org_type: 'professional',
      verification_status: 'unverified',
      rating_avg: 0,
      rating_count: 0,
      verification_cycle_count: 0,
    },
  })
}

const json = (route: Route, body: unknown, status = 200) =>
  route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })

/**
 * GET /api/v1/listings (list) and GET /api/v1/listings/{id} (detail). `detail` may be a
 * function to vary the answer per request (e.g. sold out on the second poll).
 */
export async function mockListingsApi(
  page: Page,
  listings = [makeListing(), makeUnverifiedListing()],
  detail?: (id: string, requestCount: number) => Record<string, unknown> | undefined,
) {
  const counts = new Map<string, number>()
  await page.route('**/api/v1/listings**', async (route) => {
    const url = new URL(route.request().url())
    const id = url.pathname.split('/api/v1/listings/')[1]
    if (route.request().method() !== 'GET') return route.fallback()
    if (!id) {
      return json(route, { listings })
    }
    const n = (counts.get(id) ?? 0) + 1
    counts.set(id, n)
    const found = detail?.(id, n) ?? listings.find((l) => l.id === id)
    return found ? json(route, found) : json(route, { error: 'not_found' }, 404)
  })
}

export const ORDER = {
  id: 'claim-e2e-1',
  quantity: 2,
  amount_paid_cents: 700,
  payment_status: 'paid',
  pickup_status: 'pending',
  rating: null,
  created_at: new Date().toISOString(),
  listing: {
    id: LISTING_ID,
    food_description: 'Vegetable lasagne (3 portions)',
    photo_url: null,
    pickup_address_text: 'Hoofdstraat 12, Amsterdam',
    pickup_window_start: inHours(-0.5),
    pickup_window_end: inHours(2),
    cook_display_name: 'De Buurtkok',
  },
}

export async function mockClaimsApi(page: Page, orders = [ORDER]) {
  await page.route('**/api/v1/claims**', async (route) => {
    const req = route.request()
    const id = new URL(req.url()).pathname.split('/api/v1/claims/')[1]
    if (req.method() === 'POST') {
      return json(
        route,
        {
          claim_id: 'claim-e2e-1',
          quantity: 1,
          amount_due_cents: 350,
          stripe_client_secret: 'pi_e2e_secret_x',
          payment_reservation_expires_at: inHours(0.17),
        },
        201,
      )
    }
    if (!id) return json(route, { claims: orders, next_cursor: null })
    const found = orders.find((o) => o.id === id)
    return found ? json(route, found) : json(route, { error: 'not_found' }, 404)
  })
}

/** GET /api/v1/me/listings: the signed-in cook's own listings (detail shape + portions_claimed). */
export async function mockCookListingsApi(page: Page, listings: Record<string, unknown>[] = []) {
  await page.route('**/api/v1/me/listings', (route) => json(route, { listings }))
}

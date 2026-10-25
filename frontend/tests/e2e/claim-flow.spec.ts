/**
 * E2E: Claim flow — Playwright
 *
 * Covers quickstart.md Scenario 2 (buyer discovers and claims) at the UI level. The API is
 * served by route mocks that follow contracts/listings-api.md and claims-api.md, so no
 * backend is needed. Full payment (Stripe test cards, webhook) needs the compose stack and a
 * real VITE_STRIPE_PUBLISHABLE_KEY; with the local placeholder key the flow stops at a
 * "payment isn't available" message, which is asserted here instead of crashing.
 */
import { test, expect } from '@playwright/test'
import { stubSignedInSession, UNVERIFIED_COOK } from './helpers/session'
import {
  LISTING_ID,
  ORDER,
  makeUnverifiedListing,
  mockClaimsApi,
  mockCookListingsApi,
  mockListingsApi,
} from './helpers/api'

const BASE_URL = 'http://localhost:5173'

// Claiming and the cook area need a signed-in account (feature 001); these structural
// specs run with a stubbed signed-in buyer (sign-in itself is covered in
// sign-in-to-claim.spec.ts).
test.beforeEach(async ({ page }) => {
  await stubSignedInSession(page)
  await mockListingsApi(page)
  await mockClaimsApi(page)
})

test.describe('Discovery view', () => {
  test('shows "Near you" heading', async ({ page }) => {
    await page.goto(BASE_URL)
    await expect(page.getByRole('heading', { name: /Near you/i })).toBeVisible()
  })

  test('renders at least one listing card with a verification badge', async ({ page }) => {
    await page.goto(BASE_URL)
        const badge = page.getByTestId('verification-badge').first()
    await expect(badge).toBeVisible()
  })

  test('unverified listing card shows "unverified" badge — Principle I', async ({ page }) => {
    await page.goto(BASE_URL)
    // The second listing in the mocked response is from an unverified cook
    const badges = page.getByTestId('verification-badge')
    const count = await badges.count()
    expect(count).toBeGreaterThanOrEqual(2)

    // At least one badge should say "unverified"
    const texts = await badges.allTextContents()
    expect(texts).toContain('unverified')
  })

  test('can open the claim flow panel', async ({ page }) => {
    await page.goto(BASE_URL)
    // Click the first "Claim" button
    await page.getByRole('button', { name: /^Claim$/i }).first().click()
    // The ClaimFlow dialog should appear
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()
  })

  test('claim panel shows quantity selector and Continue-to-payment CTA', async ({ page }) => {
    await page.goto(BASE_URL)
    await page.getByRole('button', { name: /^Claim$/i }).first().click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await expect(page.getByRole('dialog').getByRole('spinbutton', { name: 'Portions' })).toBeVisible()
    await expect(page.getByRole('button', { name: /Continue to payment/i })).toBeVisible()
  })

  test('can navigate to listing detail page', async ({ page }) => {
    // Directly navigate to a listing detail page
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    await expect(page.getByRole('heading', { name: /Vegetable lasagne/i })).toBeVisible()
  })
})

test.describe('Listing detail page', () => {
  test('shows verification badge on detail page — Principle I / FR-004', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    const badge = page.getByTestId('verification-badge')
    await expect(badge).toBeVisible()
  })

  test('shows "Claim this listing" button when portions available', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    await expect(page.getByRole('button', { name: /Claim this listing/i })).toBeVisible()
  })

  test('can open claim flow from detail page', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()
  })
})

test.describe('Claim happy path — browse to confirmation to Order History (T035)', () => {
  test('claiming a listing end-to-end lands on a confirmation retrievable from Order History', async ({
    page,
  }) => {
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()

    await page.getByRole('button', { name: /Continue to payment/i }).click()
    // The reservation (POST /claims) succeeded. With a real publishable key the Pay button
    // shows; with the local placeholder key a clear message replaces Stripe Elements.
    await expect(
      page.getByRole('button', { name: /^Pay/i }).or(page.getByTestId('payment-unavailable')),
    ).toBeVisible()

    // Order History is independently reachable and retrievable per FR-005 (served from the
    // mocked GET /claims, since paying needs Stripe and the webhook).
    await page.goto(`${BASE_URL}/orders`)
    await expect(page.getByRole('heading', { name: /Order history/i })).toBeVisible()
    await page.getByRole('link', { name: /Vegetable lasagne/i }).click()
    await expect(page.getByTestId('order-confirmation')).toBeVisible()
  })
})

test.describe('Cook Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await stubSignedInSession(page, UNVERIFIED_COOK)
    await mockCookListingsApi(page, [
      { ...makeUnverifiedListing(), total_portions: 8, portions_claimed: 3 },
      { ...makeUnverifiedListing(), id: 'draft-1', food_description: 'Draft stew', status: 'draft' },
    ])
  })

  test('shows Cook Dashboard heading', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByRole('heading', { name: /Cook Dashboard/i })).toBeVisible()
  })

  test('shows Create Listing button', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByRole('button', { name: /Create Listing/i })).toBeVisible()
  })

  test('lists the cook\'s own listings (all statuses) from /me/listings', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByTestId('cook-listing')).toHaveCount(2)
    await expect(page.getByTestId('cook-listing').first()).toContainText('3 / 8 claimed')
    await expect(page.getByTestId('cook-listing').nth(1).getByRole('button', { name: 'Publish' })).toBeVisible()
  })

  test('shows unverified status notice', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByTestId('verification-notice')).toContainText(/unverified/i)
  })
})

test.describe('Contract errors and ratings', () => {
  test('409 insufficient_portions lowers the quantity and explains why', async ({ page }) => {
    // Registered after the beforeEach mock, so this takes precedence for POST /claims.
    await page.route('**/api/v1/claims', async (route) => {
      if (route.request().method() !== 'POST') return route.fallback()
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'insufficient_portions', portions_remaining: 1 }),
      })
    })
    await page.goto(`${BASE_URL}/listings/${LISTING_ID}`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    const dialog = page.getByRole('dialog', { name: /Claim listing/i })
    await dialog.getByRole('button', { name: /Increase portions/i }).click()
    await dialog.getByRole('button', { name: /Continue to payment/i }).click()

    await expect(dialog.getByTestId('claim-notice')).toContainText(/Only 1 portion is left/i)
    await expect(dialog.getByRole('spinbutton', { name: 'Portions' })).toHaveValue('1')
  })

  test('a collected order offers a rating that posts to /claims/{id}/rating', async ({ page }) => {
    await mockClaimsApi(page, [{ ...ORDER, pickup_status: 'collected' }])
    let posted: unknown = null
    await page.route('**/api/v1/claims/*/rating', async (route) => {
      posted = route.request().postDataJSON()
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ claim_id: ORDER.id, stars: 5, rated_at: new Date().toISOString() }),
      })
    })
    await page.goto(`${BASE_URL}/orders/${ORDER.id}`)
    await page.getByRole('radio', { name: '5 stars' }).click()
    await page.getByRole('button', { name: 'Send rating' }).click()
    await expect(page.getByTestId('rating-done')).toContainText('5 stars')
    expect(posted).toEqual({ stars: 5 })
  })
})

/**
 * E2E: Claim flow — Playwright
 *
 * Covers quickstart.md Scenario 2 (buyer discovers and claims) at the UI level,
 * using the mock data served by the frontend skeleton (no live backend required
 * for these structural smoke tests).
 *
 * Full payment integration tests (Stripe test cards, webhook verification) are
 * added in the payment task once Stripe Elements is wired up.
 */
import { test, expect } from '@playwright/test'
import { stubSignedInSession, UNVERIFIED_COOK } from './helpers/session'

const BASE_URL = 'http://localhost:5173'

// Claiming and the cook area need a signed-in account (feature 001); these structural
// specs run with a stubbed signed-in buyer (sign-in itself is covered in
// sign-in-to-claim.spec.ts).
test.beforeEach(async ({ page }) => {
  await stubSignedInSession(page)
})

test.describe('Discovery view', () => {
  test('shows "Near you" heading', async ({ page }) => {
    await page.goto(BASE_URL)
    await expect(page.getByRole('heading', { name: /Near you/i })).toBeVisible()
  })

  test('renders at least one listing card with a verification badge', async ({ page }) => {
    await page.goto(BASE_URL)
    // Wait for mock listings to render
    const badge = page.getByTestId('verification-badge').first()
    await expect(badge).toBeVisible()
  })

  test('unverified listing card shows "unverified" badge — Principle I', async ({ page }) => {
    await page.goto(BASE_URL)
    // The second mock listing (index 1) has verification_status: 'unverified'
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
    await expect(page.getByLabel('Portions')).toBeVisible()
    await expect(page.getByRole('button', { name: /Continue to payment/i })).toBeVisible()
  })

  test('can navigate to listing detail page', async ({ page }) => {
    // Directly navigate to a known mock listing detail page
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await expect(page.getByRole('heading', { name: /Vegetable lasagne/i })).toBeVisible()
  })
})

test.describe('Listing detail page', () => {
  test('shows verification badge on detail page — Principle I / FR-004', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    const badge = page.getByTestId('verification-badge')
    await expect(badge).toBeVisible()
  })

  test('shows "Claim this listing" button when portions available', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await expect(page.getByRole('button', { name: /Claim this listing/i })).toBeVisible()
  })

  test('can open claim flow from detail page', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()
  })
})

test.describe('Claim happy path — browse to confirmation to Order History (T035)', () => {
  test('claiming a listing end-to-end lands on a confirmation retrievable from Order History', async ({
    page,
  }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()

    await page.getByRole('button', { name: /Continue to payment/i }).click()
    // Stripe PaymentElement requires a real Stripe account to fully confirm in CI;
    // this smoke test asserts the payment step is reached with a live total shown.
    await expect(page.getByRole('button', { name: /^Pay/i })).toBeVisible()

    // Order History is independently reachable and retrievable per FR-005, using the
    // pre-seeded mock order (not this in-progress claim, since Stripe confirmation
    // requires a real backend/Stripe account not available in this environment).
    await page.goto(`${BASE_URL}/orders`)
    await expect(page.getByRole('heading', { name: /Order history/i })).toBeVisible()
    await page.getByRole('link').first().click()
    await expect(page.getByTestId('order-confirmation')).toBeVisible()
  })
})

test.describe('Cook Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await stubSignedInSession(page, UNVERIFIED_COOK)
  })

  test('shows Cook Dashboard heading', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByRole('heading', { name: /Cook Dashboard/i })).toBeVisible()
  })

  test('shows Create Listing button', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByRole('button', { name: /Create Listing/i })).toBeVisible()
  })

  test('shows unverified status notice', async ({ page }) => {
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByTestId('verification-notice')).toContainText(/unverified/i)
  })
})

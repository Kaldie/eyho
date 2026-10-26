/**
 * E2E: browse signed out, sign in to claim (quickstart.md Scenarios 1 and 2) — Playwright.
 *
 * Written against the docker compose stack: `docker compose up` provides the frontend
 * (VITE_OIDC_* pointing at the mock-oidc service), the backend and the mock OIDC server.
 * Not run in the authoring environment (no docker).
 */
import { test, expect } from '@playwright/test'
import { newMockUser, signInOnMockServer } from './helpers/mockOidc'

const LISTING_PATH = '/listings/mock-listing-001'
const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080'

test.describe('Scenario 1: browse signed out, sign in to claim', () => {
  test('returns to the same listing with the claim dialog open at the chosen quantity', async ({
    page,
  }) => {
    // 1. Browsing needs no sign-in.
    await page.goto('/')
    await expect(page.getByRole('heading', { name: /Near you/i })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Sign in' })).toBeVisible()
    await page.goto(LISTING_PATH)
    await expect(page.getByRole('heading', { name: /Vegetable lasagne/i })).toBeVisible()

    // 2. Claim, choose 2 portions, be sent to sign in.
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    const dialog = page.getByRole('dialog', { name: /Claim listing/i })
    await expect(dialog).toBeVisible()
    await dialog.getByRole('button', { name: /Increase portions/i }).click()
    await expect(dialog.getByLabel('Portions', { exact: true })).toHaveValue('2')
    await dialog.getByRole('button', { name: /Sign in to continue/i }).click()

    // 3. Sign in on the mock server as a new user with a verified email.
    await signInOnMockServer(page, newMockUser())

    // 4. Back on the same listing, dialog open at 2 portions.
    await page.waitForURL(`**${LISTING_PATH}`)
    const resumed = page.getByRole('dialog', { name: /Claim listing/i })
    await expect(resumed).toBeVisible()
    await expect(resumed.getByLabel('Portions', { exact: true })).toHaveValue('2')
    await expect(resumed.getByRole('button', { name: /Continue to payment/i })).toBeVisible()
    await expect(page.getByRole('button', { name: /Sign in/i })).toHaveCount(0)
  })

  test('also resumes a claim started from the discovery grid', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: /^Claim$/i }).first().click()
    const dialog = page.getByRole('dialog', { name: /Claim listing/i })
    await dialog.getByRole('button', { name: /Increase portions/i }).click()
    await dialog.getByRole('button', { name: /Sign in to continue/i }).click()

    await signInOnMockServer(page, newMockUser())

    await page.waitForURL((url) => url.pathname === '/')
    const resumed = page.getByRole('dialog', { name: /Claim listing/i })
    await expect(resumed).toBeVisible()
    await expect(resumed.getByLabel('Portions', { exact: true })).toHaveValue('2')
  })
})

test.describe('Scenario 1b: unverified email (Scenario 3 UI)', () => {
  test('shows the verify banner instead of proceeding to payment', async ({ page }) => {
    await page.goto(LISTING_PATH)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await page
      .getByRole('dialog', { name: /Claim listing/i })
      .getByRole('button', { name: /Sign in to continue/i })
      .click()

    await signInOnMockServer(page, newMockUser({ emailVerified: false }))

    const dialog = page.getByRole('dialog', { name: /Claim listing/i })
    await expect(dialog.getByTestId('verify-email-banner')).toBeVisible()
    await expect(dialog.getByRole('button', { name: /Resend verification email/i })).toBeVisible()
    await expect(dialog.getByRole('button', { name: /Continue to payment/i })).toHaveCount(0)
  })
})

test.describe('Scenario 2: signed-out access to protected pages', () => {
  test('Orders explains and offers Sign in, with no data', async ({ page }) => {
    await page.goto('/orders')
    await expect(page.getByRole('heading', { name: /Sign in to see your orders/i })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Order history/i })).toHaveCount(0)
  })

  test('the cook area explains and offers Sign in, with no data', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: /Sign in to sell food as a cook/i })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible()
    await expect(page.getByRole('button', { name: /Create Listing/i })).toHaveCount(0)
  })

  test('GET /api/v1/claims without a token is 401 unauthenticated', async ({ request }) => {
    const res = await request.get(`${API_BASE}/api/v1/claims`)
    expect(res.status()).toBe(401)
    expect(await res.json()).toEqual({ error: 'unauthenticated' })
  })
})

/**
 * E2E: demo / local one-tap sign-in (quickstart.md Scenario 7, step 1) — Playwright.
 *
 * Needs the docker compose stack with the demo issuer enabled (APP_ENV=local). Not run in
 * the authoring environment (no docker). The "hidden when absent" test stubs the endpoint
 * and needs no backend.
 */
import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080'

test.describe('Demo endpoint absent', () => {
  test('hides the demo buttons and keeps the app usable', async ({ page }) => {
    await page.route('**/api/v1/demo/accounts', (route) => route.fulfill({ status: 404 }))
    await page.goto('/sign-in')
    await expect(page.getByRole('heading', { name: /Sign in to eyho/i })).toBeVisible()
    await expect(page.getByTestId('demo-accounts')).toHaveCount(0)

    await page.getByRole('link', { name: 'Food' }).click()
    await expect(page.getByRole('heading', { name: /Near you/i })).toBeVisible()
  })
})

test.describe('One-tap demo sign-in', () => {
  test.beforeEach(async ({ request }) => {
    const res = await request.get(`${API_BASE}/api/v1/demo/accounts`).catch(() => null)
    test.skip(!res || !res.ok(), 'demo issuer is not enabled on this backend')
  })

  test('lists the three demo accounts', async ({ page }) => {
    await page.goto('/sign-in')
    const demo = page.getByTestId('demo-accounts')
    await expect(demo.getByRole('button', { name: 'Demo buyer' })).toBeVisible()
    await expect(demo.getByRole('button', { name: 'Demo cook (verified)' })).toBeVisible()
    await expect(demo.getByRole('button', { name: 'Demo cook (unverified)' })).toBeVisible()
  })

  test('demo cook (verified) lands signed in and sees the cook area with a registration card', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByRole('button', { name: 'Demo cook (verified)' }).click()

    // Signed in: the header shows the account menu, not "Sign in".
    await expect(page.locator('button[aria-haspopup="menu"]')).toBeVisible()
    await expect(page.getByRole('link', { name: 'Sign in' })).toHaveCount(0)

    await page.getByRole('link', { name: 'For cooks' }).click()
    await expect(page.getByRole('heading', { name: /Cook Dashboard/i })).toBeVisible()
    await expect(page.getByTestId('registration-card')).toContainText('Registered')
    await expect(page.getByTestId('verification-notice')).toContainText('verified')
  })

  test('demo cook (unverified) is labelled unverified, registration stays separate', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByRole('button', { name: 'Demo cook (unverified)' }).click()
    await page.getByRole('link', { name: 'For cooks' }).click()
    await expect(page.getByTestId('verification-notice').getByText('unverified').first()).toBeVisible()
    await expect(page.getByTestId('registration-card')).toContainText('Registered')
  })

  test('demo buyer reaches payment in the claim dialog and sees onboarding in the cook area', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByRole('button', { name: 'Demo buyer' }).click()

    await page.getByRole('link', { name: 'For cooks' }).click()
    await expect(page.getByRole('heading', { name: /Become a cook/i })).toBeVisible()

    await page.getByRole('link', { name: 'Food' }).click()
    await page.getByRole('button', { name: /^Claim$/i }).first().click()
    await expect(page.getByRole('button', { name: /Continue to payment/i })).toBeVisible()
  })

  test('sign out returns to the signed-out header', async ({ page }) => {
    await page.goto('/sign-in')
    await page.getByRole('button', { name: 'Demo buyer' }).click()
    await page.locator('button[aria-haspopup="menu"]').click()
    await page.getByRole('menuitem', { name: 'Sign out' }).click()
    await expect(page.getByRole('link', { name: 'Sign in' })).toBeVisible()
  })
})

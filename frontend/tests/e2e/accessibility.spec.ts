/**
 * E2E: Automated accessibility audits — Playwright + axe-core
 *
 * Per FR-012 (WCAG 2.1 AA color contrast and keyboard-navigability across all primary
 * user flows) and the Polish-phase task T043, this runs axe-core against the discovery,
 * listing detail, checkout (claim flow dialog), and cook dashboard surfaces.
 *
 * NOTE: not executed in this environment (requires a running dev server / Playwright
 * browser binaries); written to match the conventions in claim-flow.spec.ts. Run with
 * `npm run test:e2e` once a dev server and browsers are available.
 */
import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { stubSignedInSession, UNVERIFIED_COOK } from './helpers/session'

const BASE_URL = 'http://localhost:5173'

test.describe('Accessibility — automated axe-core audits (FR-012)', () => {
  test('discovery page has no detectable accessibility violations', async ({ page }) => {
    await page.goto(BASE_URL)
    await expect(page.getByRole('heading', { name: /Near you/i })).toBeVisible()

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })

  test('listing detail page has no detectable accessibility violations', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await expect(page.getByRole('button', { name: /Claim this listing/i })).toBeVisible()

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })

  test('claim flow (checkout) dialog has no detectable accessibility violations', async ({ page }) => {
    await page.goto(`${BASE_URL}/listings/mock-listing-001`)
    await page.getByRole('button', { name: /Claim this listing/i }).click()
    await expect(page.getByRole('dialog', { name: /Claim listing/i })).toBeVisible()

    const results = await new AxeBuilder({ page })
      .include('[role="dialog"]')
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })

  test('cook dashboard has no detectable accessibility violations', async ({ page }) => {
    // The cook area requires a signed-in cook (feature 001); sign-in is stubbed here.
    await stubSignedInSession(page, UNVERIFIED_COOK)
    await page.goto(`${BASE_URL}/dashboard`)
    await expect(page.getByRole('heading', { name: /Cook Dashboard/i })).toBeVisible()

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    expect(results.violations).toEqual([])
  })
})

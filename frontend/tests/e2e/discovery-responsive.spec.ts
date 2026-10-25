/**
 * E2E: Discovery page responsiveness — Playwright
 *
 * Per FR-008, the discovery page must render without horizontal scroll at
 * 360px (mobile), 768px (tablet), and 1440px (desktop) widths, with the hero
 * value proposition and primary CTA visible above the fold at each width.
 *
 * NOTE: not executed in this environment (requires a running dev server /
 * browser binaries); written to match the conventions in claim-flow.spec.ts.
 */
import { test, expect } from '@playwright/test'

const WIDTHS = [360, 768, 1440]

test.describe('Discovery page — responsive layout', () => {
  for (const width of WIDTHS) {
    test(`renders without horizontal scroll at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 })
      await page.goto('/')

      const hasHorizontalScroll = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      )
      expect(hasHorizontalScroll).toBe(false)
    })

    test(`shows hero value proposition and primary CTA at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 })
      await page.goto('/')

      await expect(page.getByRole('heading', { name: /Save food, save money/i })).toBeVisible()
      await expect(page.getByRole('button', { name: /Browse nearby food/i })).toBeVisible()
    })
  }
})

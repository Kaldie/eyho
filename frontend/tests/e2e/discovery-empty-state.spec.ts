/**
 * E2E: Discovery page empty state — Playwright
 *
 * Per FR-010, a zero-result discovery query must show an inviting empty state
 * with next-step guidance (expand the search radius), never a bare "no
 * results" message.
 *
 * NOTE: not executed in this environment (requires a running dev server /
 * browser binaries, and a way to force a zero-result response — e.g. a very
 * small radius or a mocked route). Written to match the conventions in
 * claim-flow.spec.ts, using route mocking so it does not depend on live data.
 */
import { test, expect } from '@playwright/test'

test.describe('Discovery page — empty state', () => {
  test('shows an inviting empty state with next-step guidance when no listings are found', async ({
    page,
  }) => {
    // Mock the listings API to return zero results so the empty state is deterministic.
    await page.route('**/api/v1/listings**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ listings: [] }),
      })
    })

    await page.goto('/')

    await expect(page.getByText(/No listings nearby yet/i)).toBeVisible()
    await expect(
      page.getByText(/expanding your search radius/i),
    ).toBeVisible()
    await expect(page.getByRole('button', { name: /Expand search radius/i })).toBeVisible()
  })

  test('expanding the search radius from the empty state broadens the query (explicit action, Principle III)', async ({
    page,
  }) => {
    await page.route('**/api/v1/listings**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ listings: [] }),
      })
    })

    await page.goto('/')
    await expect(page.getByRole('button', { name: /Expand search radius/i })).toBeVisible()

    const radiusSelect = page.getByLabel('Distance')
    const before = await radiusSelect.inputValue()

    await page.getByRole('button', { name: /Expand search radius/i }).click()

    const after = await radiusSelect.inputValue()
    expect(after).not.toBe(before)
  })
})

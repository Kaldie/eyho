/**
 * Helpers for e2e tests that need a signed-in session without the identity provider.
 * `stubSignedInSession` adopts a fake demo-issuer session (the app keeps those in
 * sessionStorage under `eyho.demo-session`, see src/auth/AuthProvider.tsx) and stubs GET /me,
 * so structural specs keep running without a backend.
 */
import type { Page } from '@playwright/test'

export interface StubAccount {
  id: string
  email: string
  email_verified: boolean
  display_name: string
  is_demo: boolean
  cook_profile: Record<string, unknown> | null
}

export const VERIFIED_BUYER: StubAccount = {
  id: 'acc-e2e-buyer',
  email: 'buyer@example.nl',
  email_verified: true,
  display_name: 'E2E Buyer',
  is_demo: true,
  cook_profile: null,
}

export const UNVERIFIED_COOK: StubAccount = {
  ...VERIFIED_BUYER,
  id: 'acc-e2e-cook',
  display_name: 'E2E Cook',
  cook_profile: {
    id: 'cook-e2e',
    display_name: "Sanne's Kitchen",
    org_type: 'professional',
    phone: '+31612345678',
    default_pickup_address_text: 'Keizersgracht 88, Amsterdam',
    default_pickup_location: { lat: 52.37, lng: 4.89 },
    verification_status: 'unverified',
    kvk_number: '68750110',
    kvk_trade_name: "Sanne's Kitchen B.V.",
    registration_status: 'active',
    registration_checked_at: '2026-10-25T10:00:00Z',
    nvwa_declared_at: '2026-10-25T10:00:00Z',
    can_publish: true,
  },
}

export async function stubSignedInSession(page: Page, account: StubAccount = VERIFIED_BUYER) {
  await page.addInitScript(() => {
    window.sessionStorage.setItem(
      'eyho.demo-session',
      JSON.stringify({ token: 'e2e-token', expiresAt: Date.now() + 60 * 60 * 1000 }),
    )
  })
  await page.route('**/api/v1/me', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(account),
    }),
  )
}

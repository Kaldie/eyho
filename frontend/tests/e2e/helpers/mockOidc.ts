/**
 * Sign-in on the `mock-oidc` service from docker-compose.yml (navikt/mock-oauth2-server,
 * interactive login). The login page has a username field and a free-form claims textarea;
 * the server's claim mappings (infra/mock-oidc/config.json) expose the namespaced email
 * claims eyho reads.
 */
import type { Page } from '@playwright/test'

export interface MockUser {
  /** Becomes the token `sub`. */
  sub: string
  email: string
  emailVerified: boolean
}

export async function signInOnMockServer(page: Page, user: MockUser) {
  await page.locator('input[name="username"]').fill(user.sub)
  await page.locator('textarea[name="claims"]').fill(
    JSON.stringify({
      'https://eyho.app/email': user.email,
      'https://eyho.app/email_verified': user.emailVerified,
    }),
  )
  await page.locator('input[type="submit"], button[type="submit"]').first().click()
}

/** A user that is unique per run so repeated runs create fresh accounts. */
export function newMockUser(overrides: Partial<MockUser> = {}): MockUser {
  const id = `e2e-${Date.now()}-${Math.floor(Math.random() * 1e6)}`
  return { sub: id, email: `${id}@example.nl`, emailVerified: true, ...overrides }
}

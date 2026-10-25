# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: sign-in-to-claim.spec.ts >> Scenario 1: browse signed out, sign in to claim >> returns to the same listing with the claim dialog open at the chosen quantity
- Location: tests/e2e/sign-in-to-claim.spec.ts:22:3

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.fill: Test timeout of 30000ms exceeded.
Call log:
  - waiting for locator('input[name="username"]')

```

# Page snapshot

```yaml
- generic [ref=f1e2]:
  - banner [ref=f1e3]:
    - navigation "Main" [ref=f1e4]:
      - link "eyho" [ref=f1e5] [cursor=pointer]:
        - /url: /
      - link "Food" [ref=f1e6] [cursor=pointer]:
        - /url: /
      - link "Orders" [ref=f1e7] [cursor=pointer]:
        - /url: /orders
      - link "For cooks" [ref=f1e8] [cursor=pointer]:
        - /url: /dashboard
      - link "Sign in" [ref=f1e9] [cursor=pointer]:
        - /url: /sign-in
  - main [ref=f1e10]:
    - heading "Sign in to eyho" [level=1] [ref=f1e11]
    - paragraph [ref=f1e12]: Browsing is open to everyone. You only need an account to claim food, see your orders or cook.
    - paragraph [ref=f1e13]: Sign-in isn't set up in this environment yet.
```

# Test source

```ts
  1  | /**
  2  |  * Sign-in on the `mock-oidc` service from docker-compose.yml (navikt/mock-oauth2-server,
  3  |  * interactive login). The login page has a username field and a free-form claims textarea;
  4  |  * the server's claim mappings (infra/mock-oidc/config.json) expose the namespaced email
  5  |  * claims eyho reads.
  6  |  */
  7  | import type { Page } from '@playwright/test'
  8  | 
  9  | export interface MockUser {
  10 |   /** Becomes the token `sub`. */
  11 |   sub: string
  12 |   email: string
  13 |   emailVerified: boolean
  14 | }
  15 | 
  16 | export async function signInOnMockServer(page: Page, user: MockUser) {
> 17 |   await page.locator('input[name="username"]').fill(user.sub)
     |                                                ^ Error: locator.fill: Test timeout of 30000ms exceeded.
  18 |   await page.locator('textarea[name="claims"]').fill(
  19 |     JSON.stringify({
  20 |       'https://eyho.app/email': user.email,
  21 |       'https://eyho.app/email_verified': user.emailVerified,
  22 |     }),
  23 |   )
  24 |   await page.locator('input[type="submit"], button[type="submit"]').first().click()
  25 | }
  26 | 
  27 | /** A user that is unique per run so repeated runs create fresh accounts. */
  28 | export function newMockUser(overrides: Partial<MockUser> = {}): MockUser {
  29 |   const id = `e2e-${Date.now()}-${Math.floor(Math.random() * 1e6)}`
  30 |   return { sub: id, email: `${id}@example.nl`, emailVerified: true, ...overrides }
  31 | }
  32 | 
```
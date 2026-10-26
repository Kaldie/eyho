import type { ReactElement } from 'react'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SessionContext, SIGNED_OUT_SESSION, type Session } from '../../src/auth/session'
import { ME_QUERY_KEY, type Account } from '../../src/services/accounts'

export const VERIFIED_BUYER: Account = {
  id: 'acc-1',
  email: 'sanne@example.nl',
  email_verified: true,
  display_name: 'Sanne',
  is_demo: false,
  cook_profile: null,
}

export const UNVERIFIED_BUYER: Account = { ...VERIFIED_BUYER, email_verified: false }

interface Options {
  /** Omit for a signed-out visitor. */
  account?: Account
  session?: Partial<Session>
  route?: string
}

/**
 * Renders with a router, query client and a fixed session. A signed-in `account` is seeded
 * into the query cache so GET /me is not fetched.
 */
export function renderWithAuth(ui: ReactElement, { account, session, route = '/' }: Options = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  if (account) client.setQueryData(ME_QUERY_KEY, account)
  const value: Session = {
    ...SIGNED_OUT_SESSION,
    status: account ? 'signedIn' : 'signedOut',
    ...session,
  }
  return {
    client,
    session: value,
    ...render(
      <QueryClientProvider client={client}>
        <SessionContext.Provider value={value}>
          <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
        </SessionContext.Provider>
      </QueryClientProvider>,
    ),
  }
}

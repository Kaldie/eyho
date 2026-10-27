import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AccountMenu } from '../../src/components/AccountMenu'
import { RequireSignIn } from '../../src/auth/RequireSignIn'
import { SignInPage } from '../../src/pages/SignInPage'
import * as accounts from '../../src/services/accounts'

const demoAccounts = vi.hoisted(() => ({ list: [] as { key: string; label: string }[] }))
vi.mock('../../src/services/accounts', async (orig) => ({
  ...(await orig<typeof import('../../src/services/accounts')>()),
  useDemoAccounts: () => ({ data: demoAccounts.list }),
  demoSignIn: vi.fn(),
}))
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'

describe('AccountMenu', () => {
  it('shows "Sign in" when signed out', () => {
    renderWithAuth(<AccountMenu />)
    expect(screen.getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/sign-in')
  })

  it('shows the name with Account and Sign out when signed in', async () => {
    const signOut = vi.fn().mockResolvedValue(undefined)
    const user = userEvent.setup()
    renderWithAuth(<AccountMenu />, { account: VERIFIED_BUYER, session: { signOut } })

    expect(screen.queryByRole('link', { name: 'Sign in' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Sanne' }))
    expect(screen.getByRole('menuitem', { name: 'Account' })).toHaveAttribute('href', '/account')
    await user.click(screen.getByRole('menuitem', { name: 'Sign out' }))
    expect(signOut).toHaveBeenCalled()
  })
})

describe('RequireSignIn', () => {
  it('explains and offers Sign in instead of rendering protected content', () => {
    renderWithAuth(
      <RequireSignIn reason="to see your orders">
        <p>secret orders</p>
      </RequireSignIn>,
    )
    expect(screen.getByRole('heading', { name: /Sign in to see your orders/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByText('secret orders')).not.toBeInTheDocument()
  })

  it('renders children when signed in', () => {
    renderWithAuth(
      <RequireSignIn>
        <p>my orders</p>
      </RequireSignIn>,
      { account: VERIFIED_BUYER },
    )
    expect(screen.getByText('my orders')).toBeInTheDocument()
  })
})

describe('SignInPage', () => {
  it('hides demo buttons when the demo endpoint does not respond', async () => {
    demoAccounts.list = []
    renderWithAuth(<SignInPage />, { session: { oidcAvailable: true } })
    expect(screen.getByRole('button', { name: 'Continue' })).toBeInTheDocument()
    expect(screen.queryByTestId('demo-accounts')).not.toBeInTheDocument()
  })

  it('shows one-tap demo buttons and adopts the minted token', async () => {
    demoAccounts.list = [
      { key: 'buyer', label: 'Demo buyer' },
      { key: 'cook_verified', label: 'Demo cook (verified)' },
    ]
    vi.mocked(accounts.demoSignIn).mockResolvedValue({
      access_token: 'tok',
      expires_at: '2030-01-01T00:00:00Z',
    })
    const signInWithDemoToken = vi.fn()
    const user = userEvent.setup()
    renderWithAuth(<SignInPage />, { session: { signInWithDemoToken } })

    await user.click(await screen.findByRole('button', { name: 'Demo buyer' }))
    expect(accounts.demoSignIn).toHaveBeenCalledWith('buyer')
    expect(signInWithDemoToken).toHaveBeenCalledWith('tok', '2030-01-01T00:00:00Z')
  })

  it('does not offer Continue when OIDC is not configured', () => {
    demoAccounts.list = []
    renderWithAuth(<SignInPage />)
    expect(screen.queryByRole('button', { name: 'Continue' })).not.toBeInTheDocument()
  })
})

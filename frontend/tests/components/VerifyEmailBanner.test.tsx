import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { VerifyEmailBanner } from '../../src/components/VerifyEmailBanner'
import { ApiError } from '../../src/services/api'
import * as accounts from '../../src/services/accounts'
import { renderWithAuth, UNVERIFIED_BUYER, VERIFIED_BUYER } from '../helpers/renderWithAuth'

vi.mock('../../src/services/accounts', async (orig) => ({
  ...(await orig<typeof import('../../src/services/accounts')>()),
  resendVerificationEmail: vi.fn(),
}))

const resend = vi.mocked(accounts.resendVerificationEmail)

beforeEach(() => resend.mockReset())

describe('VerifyEmailBanner', () => {
  it('renders nothing when signed out', () => {
    renderWithAuth(<VerifyEmailBanner />)
    expect(screen.queryByTestId('verify-email-banner')).not.toBeInTheDocument()
  })

  it('renders nothing when the email is verified', () => {
    renderWithAuth(<VerifyEmailBanner />, { account: VERIFIED_BUYER })
    expect(screen.queryByTestId('verify-email-banner')).not.toBeInTheDocument()
  })

  it('shows the address and confirms a resend', async () => {
    resend.mockResolvedValue(undefined)
    const user = userEvent.setup()
    renderWithAuth(<VerifyEmailBanner />, { account: UNVERIFIED_BUYER })

    expect(screen.getByText('sanne@example.nl')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Resend verification email/i }))
    expect(await screen.findByText(/Verification email sent/i)).toBeInTheDocument()
  })

  it('explains the one-per-minute limit on 429', async () => {
    resend.mockRejectedValue(new ApiError(429))
    const user = userEvent.setup()
    renderWithAuth(<VerifyEmailBanner />, { account: UNVERIFIED_BUYER })

    await user.click(screen.getByRole('button', { name: /Resend verification email/i }))
    expect(await screen.findByText(/one email per minute/i)).toBeInTheDocument()
  })
})

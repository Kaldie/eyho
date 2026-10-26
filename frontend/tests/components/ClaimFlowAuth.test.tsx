/**
 * ClaimFlow auth states: signed out starts sign-in carrying the claim (FR-005); an
 * unverified email sees the verify banner instead of proceeding (FR-006).
 */
import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ClaimFlow } from '../../src/components/ClaimFlow'
import type { ListingItem } from '../../src/services/api'
import { renderWithAuth, UNVERIFIED_BUYER } from '../helpers/renderWithAuth'

vi.mock('@stripe/stripe-js', () => ({ loadStripe: () => Promise.resolve(null) }))
vi.mock('@stripe/react-stripe-js', () => ({
  Elements: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  PaymentElement: () => <div data-testid="payment-element" />,
  useStripe: () => ({}),
  useElements: () => ({}),
}))

const LISTING: ListingItem = {
  id: 'listing-9',
  food_description: 'Test food',
  reduced_price_cents: 300,
  portions_remaining: 5,
  pickup_window_start: new Date(Date.now() - 3_600_000).toISOString(),
  pickup_window_end: new Date(Date.now() + 3_600_000).toISOString(),
  distance_km: 0.5,
  cook: { display_name: 'Test Cook', org_type: 'nonprofit', verification_status: 'unverified', rating_avg: 4, rating_count: 3, verification_cycle_count: 0 },
}

describe('ClaimFlow signed out', () => {
  it('offers sign-in instead of payment and passes returnTo, listing and quantity', async () => {
    const signIn = vi.fn().mockResolvedValue(undefined)
    const user = userEvent.setup()
    renderWithAuth(<ClaimFlow listing={LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />, {
      session: { signIn },
      route: '/listings/listing-9?x=1',
    })

    expect(screen.queryByRole('button', { name: /Continue to payment/i })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Increase portions/i }))
    await user.click(screen.getByRole('button', { name: /Sign in to continue/i }))

    expect(signIn).toHaveBeenCalledWith({
      returnTo: '/listings/listing-9?x=1',
      listingId: 'listing-9',
      quantity: 2,
    })
  })

  it('still labels an unverified cook', () => {
    renderWithAuth(<ClaimFlow listing={LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)
    expect(screen.getByTestId('claim-flow-unverified-badge')).toHaveTextContent('unverified')
  })
})

describe('ClaimFlow with an unverified email', () => {
  it('shows the verify banner and no way to continue to payment', () => {
    renderWithAuth(<ClaimFlow listing={LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />, {
      account: UNVERIFIED_BUYER,
    })
    expect(screen.getByTestId('verify-email-banner')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Continue to payment/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Sign in to continue/i })).not.toBeInTheDocument()
  })
})

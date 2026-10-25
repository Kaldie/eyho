/**
 * Component tests for ClaimFlow — the CheckoutState machine introduced in T023-T028.
 *
 * Stripe Elements is mocked so tests can control `confirmPayment` deterministically:
 * `useStripe().confirmPayment` resolves per-test to simulate a decline vs. a successful
 * charge. Connection-drop and expiry are exercised via `services/api.ts`'s mock-testing
 * hooks (`network-error-listing`, `expired-listing`) and a past `pickup_window_end`.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ClaimFlow } from '../../src/components/ClaimFlow'
import type { ListingItem } from '../../src/services/api'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'

// The claim flow is for signed-in buyers with a verified email; auth states are covered
// in ClaimFlowAuth.test.tsx.
const render = (ui: React.ReactElement) => renderWithAuth(ui, { account: VERIFIED_BUYER })

const confirmPaymentMock = vi.fn()

vi.mock('@stripe/stripe-js', () => ({
  loadStripe: () => Promise.resolve(null),
}))

vi.mock('@stripe/react-stripe-js', () => ({
  Elements: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  PaymentElement: () => <div data-testid="payment-element" />,
  useStripe: () => ({ confirmPayment: confirmPaymentMock }),
  useElements: () => ({}),
}))

const FUTURE_WINDOW_END = new Date(Date.now() + 60 * 60 * 1000).toISOString()

const BASE_LISTING: ListingItem = {
  id: 'test-001',
  food_description: 'Test food',
  reduced_price_cents: 300,
  portions_remaining: 5,
  pickup_window_start: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
  pickup_window_end: FUTURE_WINDOW_END,
  distance_km: 0.5,
  photo_url: null,
  cook: {
    display_name: 'Test Cook',
    org_type: 'nonprofit',
    verification_status: 'verified',
    rating_avg: 4.5,
  },
}

async function goToPaymentStep() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: /Continue to payment/i }))
  await screen.findByTestId('payment-element')
  return user
}

beforeEach(() => {
  confirmPaymentMock.mockReset()
})

describe('ClaimFlow — live total', () => {
  it('recomputes the total as quantity changes', async () => {
    const user = userEvent.setup()
    render(<ClaimFlow listing={BASE_LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)

    expect(screen.getByText(/Total: €\s?3,00/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Increase portions/i }))
    expect(screen.getByText(/Total: €\s?6,00/)).toBeInTheDocument()
  })
})

describe('ClaimFlow — sold out', () => {
  it('disables the Continue CTA when there are no portions left', () => {
    const soldOut: ListingItem = { ...BASE_LISTING, portions_remaining: 0 }
    render(<ClaimFlow listing={soldOut} onClose={vi.fn()} onClaimed={vi.fn()} />)
    expect(screen.getByRole('button', { name: /Continue to payment/i })).toBeDisabled()
  })
})

describe('ClaimFlow — payment decline vs connection drop', () => {
  it('shows a distinct decline error and preserves quantity/step on a declined card', async () => {
    confirmPaymentMock.mockResolvedValue({ error: { message: 'Your card was declined.' } })
    render(<ClaimFlow listing={BASE_LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)
    const user = await goToPaymentStep()

    await user.click(screen.getByRole('button', { name: /^Pay/i }))

    const declineError = await screen.findByTestId('decline-error')
    expect(declineError).toHaveTextContent('Your card was declined.')
    expect(screen.queryByTestId('connection-error')).not.toBeInTheDocument()
    // Step/quantity preserved — payment step still showing, not bounced back to quantity.
    expect(screen.getByTestId('payment-element')).toBeInTheDocument()
  })

  it('shows a distinct connection-drop error with retry when reservation fails on network error', async () => {
    const user = userEvent.setup()
    const listing: ListingItem = { ...BASE_LISTING, id: 'network-error-listing' }
    render(<ClaimFlow listing={listing} onClose={vi.fn()} onClaimed={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    const connectionError = await screen.findByTestId('connection-error')
    expect(connectionError).toHaveTextContent(/couldn't reach eyho/i)
    expect(screen.queryByTestId('decline-error')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Retry/i })).toBeInTheDocument()
    // Preserved quantity step — never advanced to payment.
    expect(screen.queryByTestId('payment-element')).not.toBeInTheDocument()
  })
})

describe('ClaimFlow — pickup window expiry', () => {
  it('blocks submission with a clear message when the pickup window has passed', async () => {
    const user = userEvent.setup()
    const expiredListing: ListingItem = {
      ...BASE_LISTING,
      pickup_window_end: new Date(Date.now() - 1000).toISOString(),
    }
    render(<ClaimFlow listing={expiredListing} onClose={vi.fn()} onClaimed={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    const expiryError = await screen.findByTestId('expiry-error')
    expect(expiryError).toHaveTextContent(/no longer available/i)
    expect(screen.queryByTestId('payment-element')).not.toBeInTheDocument()
  })

  it('blocks submission when the backend authoritatively reports the listing expired (4xx)', async () => {
    const user = userEvent.setup()
    const listing: ListingItem = { ...BASE_LISTING, id: 'expired-listing' }
    render(<ClaimFlow listing={listing} onClose={vi.fn()} onClaimed={vi.fn()} />)

    await waitFor(() => expect(screen.getByRole('button', { name: /Continue to payment/i })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    const expiryError = await screen.findByTestId('expiry-error')
    expect(expiryError).toBeInTheDocument()
  })
})

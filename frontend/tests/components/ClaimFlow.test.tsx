/**
 * Component tests for ClaimFlow — the CheckoutState machine introduced in T023-T028.
 *
 * Stripe Elements is mocked so tests can control `confirmPayment` deterministically:
 * `useStripe().confirmPayment` resolves per-test to simulate a decline vs. a successful
 * charge. Connection-drop and expiry are exercised by making the mocked `createClaim` reject
 * (network error / 404) and with a past `pickup_window_end`.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ClaimFlow } from '../../src/components/ClaimFlow'
import { ApiError, createClaim, type ClaimCreated, type ListingItem } from '../../src/services/api'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'

// The claim flow is for signed-in buyers with a verified email; auth states are covered
// in ClaimFlowAuth.test.tsx.
const render = (ui: React.ReactElement) => renderWithAuth(ui, { account: VERIFIED_BUYER })

const confirmPaymentMock = vi.fn()

// Only the network call is replaced; ApiError and the rest stay real.
vi.mock('../../src/services/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../src/services/api')>()),
  createClaim: vi.fn(),
}))
const createClaimMock = vi.mocked(createClaim)

const CLAIM: ClaimCreated = {
  claim_id: 'claim-1',
  quantity: 1,
  amount_due_cents: 300,
  stripe_client_secret: 'pi_secret_1',
  payment_reservation_expires_at: new Date(Date.now() + 600_000).toISOString(),
}

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
    rating_count: 12,
    verification_cycle_count: 0,
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
  createClaimMock.mockReset()
  createClaimMock.mockResolvedValue(CLAIM)
  vi.stubEnv('VITE_STRIPE_PUBLISHABLE_KEY', 'pk_test_abcdefghijklmnop1234')
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
    createClaimMock.mockRejectedValue(new TypeError('Failed to fetch'))
    const listing: ListingItem = { ...BASE_LISTING }
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

  it('blocks submission when the backend authoritatively reports the listing expired (404)', async () => {
    const user = userEvent.setup()
    createClaimMock.mockRejectedValue(new ApiError(404, { error: 'not_found' }))
    const listing: ListingItem = { ...BASE_LISTING }
    render(<ClaimFlow listing={listing} onClose={vi.fn()} onClaimed={vi.fn()} />)

    await waitFor(() => expect(screen.getByRole('button', { name: /Continue to payment/i })).toBeEnabled())
    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    const expiryError = await screen.findByTestId('expiry-error')
    expect(expiryError).toBeInTheDocument()
  })
})

describe('ClaimFlow — real reservation', () => {
  it('sends the listing and quantity, and uses the server amount and client secret', async () => {
    createClaimMock.mockResolvedValue({ ...CLAIM, quantity: 2, amount_due_cents: 590 })
    const user = userEvent.setup()
    render(<ClaimFlow listing={BASE_LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: /Increase portions/i }))
    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    await screen.findByTestId('payment-element')
    expect(createClaimMock).toHaveBeenCalledWith('test-001', 2)
    expect(screen.getByRole('button', { name: /Pay €\s?5,90/ })).toBeInTheDocument()
  })

  it('degrades with a clear message instead of Stripe Elements when the key is a placeholder', async () => {
    vi.stubEnv('VITE_STRIPE_PUBLISHABLE_KEY', 'pk_test_placeholder')
    const user = userEvent.setup()
    render(<ClaimFlow listing={BASE_LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))

    expect(await screen.findByTestId('payment-unavailable')).toHaveTextContent(/isn't available/i)
    expect(screen.queryByTestId('payment-element')).not.toBeInTheDocument()
    await user.click(within(screen.getByTestId('payment-unavailable')).getByRole('button', { name: 'Back' }))
    expect(screen.getByRole('button', { name: /Continue to payment/i })).toBeInTheDocument()
  })
})

describe('ClaimFlow — contract errors', () => {
  async function submitAndFail(error: ApiError) {
    createClaimMock.mockRejectedValue(error)
    const user = userEvent.setup()
    render(<ClaimFlow listing={BASE_LISTING} onClose={vi.fn()} onClaimed={vi.fn()} />)
    // Pick 3 portions so a lowered selection is visible.
    await user.click(screen.getByRole('button', { name: /Increase portions/i }))
    await user.click(screen.getByRole('button', { name: /Increase portions/i }))
    await user.click(screen.getByRole('button', { name: /Continue to payment/i }))
    return user
  }

  it('409 insufficient_portions lowers the selection to what is left and stays on the quantity step', async () => {
    await submitAndFail(new ApiError(409, { error: 'insufficient_portions', portions_remaining: 2 }))

    expect(await screen.findByTestId('claim-notice')).toHaveTextContent(/Only 2 portions are left/i)
    expect(screen.getByRole('spinbutton', { name: 'Portions' })).toHaveValue(2)
    expect(screen.queryByTestId('payment-element')).not.toBeInTheDocument()
    expect(screen.queryByTestId('expiry-error')).not.toBeInTheDocument()
  })

  it('409 insufficient_portions with none left disables claiming', async () => {
    await submitAndFail(new ApiError(409, { error: 'insufficient_portions', portions_remaining: 0 }))

    expect(await screen.findByTestId('claim-notice')).toHaveTextContent(/sold out/i)
    expect(screen.getByRole('button', { name: /Continue to payment/i })).toBeDisabled()
  })

  it('403 own_listing explains the cook cannot claim their own food', async () => {
    await submitAndFail(new ApiError(403, { error: 'own_listing' }))
    expect(await screen.findByTestId('claim-notice')).toHaveTextContent(/your own listing/i)
  })

  it('403 email_not_verified shows the verify banner even when the cached account says verified', async () => {
    await submitAndFail(new ApiError(403, { error: 'email_not_verified' }))
    expect(await screen.findByTestId('verify-email-banner')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Continue to payment/i })).not.toBeInTheDocument()
  })

  it('422 shows the validation message', async () => {
    await submitAndFail(new ApiError(422, { error: 'quantity_too_low', field: 'quantity' }))
    expect(await screen.findByTestId('claim-notice')).toHaveTextContent('Quantity too low.')
  })

  it('5xx offers a retry without claiming the listing is gone', async () => {
    await submitAndFail(new ApiError(500, { error: 'internal' }))
    expect(await screen.findByTestId('connection-error')).toHaveTextContent(/went wrong/i)
    expect(screen.queryByTestId('expiry-error')).not.toBeInTheDocument()
  })
})

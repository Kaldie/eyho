import { useState, type FormEvent, type ReactNode } from 'react'
import { loadStripe } from '@stripe/stripe-js'
import { Elements, PaymentElement, useElements, useStripe } from '@stripe/react-stripe-js'
import type { ClaimCreated, ListingItem } from '@/services/api'
import { createClaim } from '@/services/api'
import { QuantityStepper } from '@/components/ui/QuantityStepper'
import { StepIndicator } from '@/components/ui/StepIndicator'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { OrderConfirmation } from '@/components/OrderConfirmation'
import type { CheckoutState } from '@/types/views'

interface ClaimFlowProps {
  listing: ListingItem
  onClose: () => void
  /** Called once the buyer dismisses the confirmation screen after a successful payment. */
  onClaimed: (quantity: number) => void
  /** Seeds the quantity step, e.g. from a pre-selector on ListingDetailPage. */
  initialQuantity?: number
}

// No real backend PaymentIntent endpoint yet (T025) — `createClaim` (services/api.ts)
// mocks the reservation + client-secret creation. Stripe.js is loaded with a
// placeholder/env publishable key so PaymentElement mounts correctly; wiring this to a
// live PaymentIntent is future work once the backend contract exists.
const stripePromise = loadStripe(
  (import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as string | undefined) ??
    'pk_test_mock_placeholder',
)

function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(
    cents / 100,
  )
}

/** Per data-model.md's validation rules: a pickup window in the past blocks submission. */
function isPickupWindowExpired(listing: ListingItem): boolean {
  return new Date(listing.pickup_window_end).getTime() <= Date.now()
}

/**
 * ClaimFlow — single-panel claim + payment flow (Constitution Principle V: low friction).
 *
 * Drives a `CheckoutState` machine (quantity -> payment -> confirmation) per
 * data-model.md, reserving portions via `createClaim`, collecting payment with Stripe
 * Elements, and distinguishing payment decline (FR-014) from connection-drop and
 * pickup-window-expiry (FR-015) as separate, state-preserving error paths.
 */
export function ClaimFlow({ listing, onClose, onClaimed, initialQuantity = 1 }: ClaimFlowProps) {
  const maxQty = Math.min(listing.portions_remaining, 10)
  const startQuantity = Math.min(Math.max(1, initialQuantity), Math.max(1, maxQty))

  const [checkout, setCheckout] = useState<CheckoutState>({
    step: 'quantity',
    listingId: listing.id,
    quantity: startQuantity,
    totalPrice: startQuantity * listing.reduced_price_cents,
    paymentIntentStatus: 'idle',
    blockedReason: null,
  })
  const [reservation, setReservation] = useState<ClaimCreated | null>(null)
  const [isReserving, setIsReserving] = useState(false)
  /** Connection-drop error — distinct from a card decline (T027), offers a retry action. */
  const [connectionError, setConnectionError] = useState<string | null>(null)
  /** Card-decline error — distinct from connection-drop (T026), inline + recoverable. */
  const [declineMessage, setDeclineMessage] = useState<string | null>(null)

  function setQuantity(quantity: number) {
    setCheckout((c) => ({ ...c, quantity, totalPrice: quantity * listing.reduced_price_cents }))
  }

  async function reserveAndAdvance() {
    setConnectionError(null)

    // Client-side expiry guard (FR-015) — checked immediately before submission.
    if (isPickupWindowExpired(listing)) {
      setCheckout((c) => ({ ...c, blockedReason: 'expired' }))
      return
    }

    setIsReserving(true)
    try {
      const claim = await createClaim(listing.id, checkout.quantity, listing.reduced_price_cents)
      setReservation(claim)
      setCheckout((c) => ({ ...c, step: 'payment', blockedReason: null }))
    } catch (err) {
      // Backend 4xx ("no longer available") is authoritative even if the client-side
      // clock check above passed (research.md).
      if (err && typeof err === 'object' && 'status' in err) {
        setCheckout((c) => ({ ...c, blockedReason: 'expired' }))
      } else {
        setConnectionError(
          "We couldn't reach eyho. Check your connection and try again — your selection is still saved.",
        )
      }
    } finally {
      setIsReserving(false)
    }
  }

  function handlePaymentSuccess() {
    setDeclineMessage(null)
    setCheckout((c) => ({ ...c, step: 'confirmation', paymentIntentStatus: 'succeeded' }))
  }

  function handlePaymentDeclined(message: string) {
    setDeclineMessage(message)
    setCheckout((c) => ({ ...c, paymentIntentStatus: 'failed' }))
  }

  function handleBackToQuantity() {
    setDeclineMessage(null)
    setCheckout((c) => ({ ...c, step: 'quantity', paymentIntentStatus: 'idle' }))
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Claim listing"
      className="fixed inset-0 z-50 flex items-end justify-center bg-neutral-900/45 sm:items-center sm:p-lg"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className="max-h-[92vh] w-full max-w-lg overflow-y-auto rounded-t-card bg-white p-lg sm:rounded-card">
        {checkout.step !== 'confirmation' && (
          <StepIndicator step={checkout.step} className="mb-lg" />
        )}

        <h2 className="mb-xs text-lg font-bold text-neutral-900">{listing.food_description}</h2>
        <p className="mb-lg flex items-center gap-sm text-sm text-neutral-600">
          Cook: {listing.cook.display_name}
          {listing.cook.verification_status === 'unverified' && (
            <Badge data-testid="claim-flow-unverified-badge" tone="neutral">
              unverified
            </Badge>
          )}
        </p>

        {checkout.blockedReason === 'expired' && (
          <div
            role="alert"
            data-testid="expiry-error"
            className="mb-lg rounded-card border border-neutral-300 bg-neutral-100 px-md py-sm text-sm text-neutral-800"
          >
            <p className="mb-sm font-semibold">This listing is no longer available</p>
            <p className="mb-sm">
              Its pickup window has ended, so it can&apos;t be claimed anymore. Your selection
              hasn&apos;t been charged.
            </p>
            <Button type="button" variant="secondary" onClick={onClose}>
              Back to listings
            </Button>
          </div>
        )}

        {checkout.blockedReason === null && checkout.step === 'quantity' && (
          <>
            <QuantityStepper
              value={checkout.quantity}
              max={maxQty}
              onChange={setQuantity}
              label="Portions"
            />
            <p className="my-lg text-xl font-bold text-primary-700">
              Total: {formatPrice(checkout.totalPrice)}
            </p>

            {connectionError && (
              <div
                role="alert"
                data-testid="connection-error"
                className="mb-lg rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-700"
              >
                <p className="mb-sm">{connectionError}</p>
                <Button type="button" variant="secondary" onClick={reserveAndAdvance}>
                  Retry
                </Button>
              </div>
            )}

            <Button
              type="button"
              className="w-full"
              disabled={isReserving || maxQty === 0}
              onClick={reserveAndAdvance}
            >
              {isReserving ? 'Reserving…' : `Continue to payment — ${formatPrice(checkout.totalPrice)}`}
            </Button>
          </>
        )}

        {checkout.blockedReason === null && checkout.step === 'payment' && reservation && (
          <Elements stripe={stripePromise} options={{ clientSecret: reservation.stripe_client_secret }}>
            <PaymentStep
              listing={listing}
              checkout={checkout}
              declineMessage={declineMessage}
              onSuccess={handlePaymentSuccess}
              onDecline={handlePaymentDeclined}
              onBack={handleBackToQuantity}
            />
          </Elements>
        )}

        {checkout.step === 'confirmation' && (
          <OrderConfirmation
            order={{
              orderId: reservation?.claim_id ?? 'unknown',
              listingSummary: {
                foodName: listing.food_description,
                photoUrl: listing.photo_url ?? null,
              },
              quantity: checkout.quantity,
              totalPrice: checkout.totalPrice,
              pickupLocation: {
                label: listing.cook.display_name,
                address: listing.pickup_address_text ?? '',
              },
              pickupWindow: {
                start: listing.pickup_window_start,
                end: listing.pickup_window_end,
              },
              claimedAt: new Date().toISOString(),
            }}
            action={
              <Button type="button" className="w-full" onClick={() => onClaimed(checkout.quantity)}>
                Done
              </Button>
            }
          />
        )}

        {checkout.step !== 'confirmation' && (
          <Button type="button" variant="secondary" className="mt-lg w-full" onClick={onClose}>
            Cancel
          </Button>
        )}
      </div>
    </div>
  )
}

interface PaymentStepProps {
  listing: ListingItem
  checkout: CheckoutState
  declineMessage: string | null
  onSuccess: () => void
  onDecline: (message: string) => void
  onBack: () => void
}

function PaymentStep({ listing, checkout, declineMessage, onSuccess, onDecline, onBack }: PaymentStepProps) {
  const stripe = useStripe()
  const elements = useElements()
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!stripe || !elements) return

    // Re-check expiry immediately before charging (FR-015) — the window may have
    // lapsed while the buyer was filling in payment details.
    if (isPickupWindowExpired(listing)) {
      onDecline(
        'This listing is no longer available — its pickup window has just ended. You have not been charged.',
      )
      return
    }

    setSubmitting(true)
    const { error } = await stripe.confirmPayment({ elements, redirect: 'if_required' })
    setSubmitting(false)

    if (error) {
      onDecline(error.message ?? 'Your card was declined. Please try a different payment method.')
    } else {
      onSuccess()
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <p className="mb-md text-lg font-bold text-primary-700">
        Total: {formatPrice(checkout.totalPrice)}
      </p>

      <div className="mb-lg rounded-card border border-neutral-200 p-md">
        <PaymentElement />
      </div>

      {declineMessage && (
        <DeclineNotice message={declineMessage} />
      )}

      <Button type="submit" className="w-full" disabled={!stripe || submitting}>
        {submitting ? 'Processing…' : `Pay ${formatPrice(checkout.totalPrice)}`}
      </Button>
      <Button type="button" variant="secondary" className="mt-sm w-full" onClick={onBack}>
        Back
      </Button>
    </form>
  )
}

function DeclineNotice({ message }: { message: string }): ReactNode {
  // Distinct styling/copy from `connection-error` (T027) so buyers can tell a card
  // problem apart from a network problem (FR-014).
  return (
    <div
      role="alert"
      data-testid="decline-error"
      className="mb-lg rounded-card border border-warning bg-neutral-50 px-md py-sm text-sm text-warning"
    >
      <p className="font-semibold">Payment declined</p>
      <p>{message}</p>
    </div>
  )
}

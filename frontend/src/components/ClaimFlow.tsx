import { useLocation } from 'react-router-dom'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Elements, PaymentElement, useElements, useStripe } from '@stripe/react-stripe-js'
import type { ClaimCreated, ListingItem } from '@/services/api'
import { ApiError, createClaim, validationMessage } from '@/services/api'
import { getStripe } from '@/services/stripe'
import { ME_QUERY_KEY } from '@/services/accounts'
import { QuantityStepper } from '@/components/ui/QuantityStepper'
import { StepIndicator } from '@/components/ui/StepIndicator'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { OrderConfirmation } from '@/components/OrderConfirmation'
import { VerifyEmailBanner } from '@/components/VerifyEmailBanner'
import { DemoAccountButtons } from '@/components/DemoAccountButtons'
import { useAccount } from '@/auth/useAccount'
import { useSession } from '@/auth/session'
import type { CheckoutState } from '@/types/views'

interface ClaimFlowProps {
  listing: ListingItem
  onClose: () => void
  /** Called once the buyer dismisses the confirmation screen after a successful payment. */
  onClaimed: (quantity: number) => void
  /** Seeds the quantity step, e.g. from a pre-selector on ListingDetailPage. */
  initialQuantity?: number
}

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
  const session = useSession()
  const { account, isLoading: accountLoading } = useAccount()
  const location = useLocation()
  const queryClient = useQueryClient()
  /** Starts from the listing and drops to what the server reports on a 409 (FR-006). */
  const [portionsLeft, setPortionsLeft] = useState(listing.portions_remaining)
  const maxQty = Math.min(portionsLeft, 10)
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
  /** Contract rejections of the reservation (409/403/422), shown on the quantity step. */
  const [claimNotice, setClaimNotice] = useState<string | null>(null)
  /** The server said the email is unverified even if the cached account says otherwise. */
  const [serverUnverified, setServerUnverified] = useState(false)

  const stripePromise = getStripe()
  const needsVerification = serverUnverified || Boolean(account && !account.email_verified)

  function setQuantity(quantity: number) {
    setCheckout((c) => ({ ...c, quantity, totalPrice: quantity * listing.reduced_price_cents }))
  }

  /**
   * Signed out: start sign-in carrying the page, listing and quantity in the OIDC state, so
   * the buyer lands back here with this dialog open at the same quantity (FR-005).
   */
  function startSignIn() {
    void session.signIn({
      returnTo: `${location.pathname}${location.search}`,
      listingId: listing.id,
      quantity: checkout.quantity,
    })
  }

  async function reserveAndAdvance() {
    setConnectionError(null)
    setClaimNotice(null)

    // Client-side expiry guard (FR-015) — checked immediately before submission.
    if (isPickupWindowExpired(listing)) {
      setCheckout((c) => ({ ...c, blockedReason: 'expired' }))
      return
    }

    setIsReserving(true)
    try {
      const claim = await createClaim(listing.id, checkout.quantity)
      setReservation(claim)
      setCheckout((c) => ({
        ...c,
        step: 'payment',
        blockedReason: null,
        totalPrice: claim.amount_due_cents,
      }))
    } catch (err) {
      handleReserveError(err)
    } finally {
      setIsReserving(false)
    }
  }

  /** Maps the claims-api.md error bodies to UI states; a 4xx is authoritative (research.md). */
  function handleReserveError(err: unknown) {
    if (!(err instanceof ApiError)) {
      setConnectionError(
        "We couldn't reach eyho. Check your connection and try again — your selection is still saved.",
      )
      return
    }
    if (err.status === 409 && err.code === 'insufficient_portions') {
      const left = typeof err.body.portions_remaining === 'number' ? err.body.portions_remaining : 0
      setPortionsLeft(left)
      setCheckout((c) => {
        const quantity = Math.max(1, Math.min(c.quantity, left))
        return { ...c, quantity, totalPrice: quantity * listing.reduced_price_cents }
      })
      setClaimNotice(
        left === 0
          ? 'Someone just claimed the last portions, so this listing is sold out. You have not been charged.'
          : `Only ${left} portion${left === 1 ? ' is' : 's are'} left now. We lowered your selection, so check it and continue.`,
      )
      void queryClient.invalidateQueries({ queryKey: ['listings'] })
      void queryClient.invalidateQueries({ queryKey: ['listing', listing.id] })
    } else if (err.status === 403 && err.code === 'own_listing') {
      setClaimNotice("This is your own listing, so you can't claim it.")
    } else if (err.status === 403 && err.code === 'email_not_verified') {
      setServerUnverified(true)
      void queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY })
    } else if (err.status === 401) {
      setClaimNotice('Your session has expired. Sign in again to claim — your selection is still here.')
    } else if (err.status === 404) {
      setCheckout((c) => ({ ...c, blockedReason: 'expired' }))
    } else if (err.status === 422) {
      setClaimNotice(validationMessage(err))
    } else if (err.status === 502 && err.code === 'payment_unavailable') {
      setClaimNotice(
        "Payments aren't configured on this server yet, so nothing was reserved or charged. Try again once payments are set up.",
      )
    } else if (err.status === 502 && err.code === 'payment_unavailable') {
      setClaimNotice(
        "Payments aren't configured on this server yet, so nothing was reserved or charged. Try again once payments are set up.",
      )
    } else {
      setConnectionError(
        'Something went wrong on our side. Try again — you have not been charged.',
      )
    }
  }

  function handlePaymentSuccess() {
    setDeclineMessage(null)
    setCheckout((c) => ({ ...c, step: 'confirmation', paymentIntentStatus: 'succeeded' }))
    void queryClient.invalidateQueries({ queryKey: ['orders'] })
    void queryClient.invalidateQueries({ queryKey: ['listings'] })
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
      className="fixed inset-0 z-50 flex items-end justify-center bg-[rgb(22_26_21/0.5)] sm:items-center sm:p-lg"
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
              Its pickup window has ended or it was withdrawn, so it can&apos;t be claimed anymore.
              You haven&apos;t been charged.
            </p>
            <Button type="button" variant="secondary" onClick={onClose}>
              Back to listings
            </Button>
          </div>
        )}

        {checkout.blockedReason === null && checkout.step === 'quantity' && needsVerification && (
          <VerifyEmailBanner className="mb-lg" force={serverUnverified} />
        )}

        {checkout.blockedReason === null && checkout.step === 'quantity' && !needsVerification && (
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

            {claimNotice && (
              <div
                role="alert"
                data-testid="claim-notice"
                className="mb-lg rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800"
              >
                {claimNotice}
              </div>
            )}

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

            {session.status === 'signedOut' ? (
              <>
                <p className="mb-sm text-sm text-neutral-600">
                  Sign in to claim. You&apos;ll come straight back to this claim.
                </p>
                <Button
                  type="button"
                  className="w-full"
                  disabled={maxQty === 0}
                  onClick={startSignIn}
                >
                  Sign in to continue
                </Button>
                <div className="mt-md">
                  <DemoAccountButtons />
                </div>
              </>
            ) : (
              <Button
                type="button"
                className="w-full"
                disabled={isReserving || maxQty === 0 || session.status === 'loading' || accountLoading}
                onClick={reserveAndAdvance}
              >
                {isReserving ? 'Reserving…' : `Continue to payment — ${formatPrice(checkout.totalPrice)}`}
              </Button>
            )}
          </>
        )}

        {checkout.blockedReason === null && checkout.step === 'payment' && reservation && (
          stripePromise ? (
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
          ) : (
            <PaymentUnavailable
              expiresAt={reservation.payment_reservation_expires_at}
              onBack={handleBackToQuantity}
            />
          )
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

/**
 * Shown instead of Stripe Elements when no usable publishable key is configured (local
 * placeholder). The reservation itself succeeded, so say so and when it lapses.
 */
function PaymentUnavailable({ expiresAt, onBack }: { expiresAt: string; onBack: () => void }) {
  const until = new Date(expiresAt).toLocaleTimeString('nl-NL', {
    hour: '2-digit',
    minute: '2-digit',
  })
  return (
    <div
      role="alert"
      data-testid="payment-unavailable"
      className="rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800"
    >
      <p className="mb-xs font-semibold">Card payment isn&apos;t available here</p>
      <p className="mb-sm">
        This environment has no Stripe publishable key configured (<code>VITE_STRIPE_PUBLISHABLE_KEY</code>
        ), so the payment form can&apos;t load. Your portions are held until {until} and released
        automatically if unpaid. You haven&apos;t been charged.
      </p>
      <Button type="button" variant="secondary" onClick={onBack}>
        Back
      </Button>
    </div>
  )
}

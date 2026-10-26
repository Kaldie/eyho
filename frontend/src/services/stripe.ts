import { loadStripe, type Stripe } from '@stripe/stripe-js'

/**
 * Stripe.js loading for the claim flow. Locally the publishable key is a placeholder
 * (`pk_test_placeholder`, `pk_test_...`) that Stripe Elements cannot use, so the key is
 * validated first and the claim flow shows a clear "payments unavailable" message instead
 * of mounting Elements. The key is read on each call (not at import) so tests can stub it.
 */
const REAL_KEY = /^pk_(test|live)_[A-Za-z0-9]{16,}$/

let cached: { key: string; promise: Promise<Stripe | null> } | null = null

export function getStripePublishableKey(): string | null {
  const key = (import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as string | undefined)?.trim()
  return key && REAL_KEY.test(key) ? key : null
}

/** null when no usable publishable key is configured. */
export function getStripe(): Promise<Stripe | null> | null {
  const key = getStripePublishableKey()
  if (!key) return null
  if (!cached || cached.key !== key) cached = { key, promise: loadStripe(key) }
  return cached.promise
}

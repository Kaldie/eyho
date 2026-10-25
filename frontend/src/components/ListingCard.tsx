import type { ListingItem } from '@/services/api'
import { Badge } from '@/components/ui/Badge'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'

interface ListingCardProps {
  listing: ListingItem
}

/**
 * Formats a cent amount as a locale currency string (EUR by default).
 * e.g. 350 -> "€3.50"
 */
function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', {
    style: 'currency',
    currency: 'EUR',
  }).format(cents / 100)
}

/**
 * ListingCard displays a single surplus-food listing in the discovery view.
 *
 * Per FR-004 and Constitution Principle I, the cook's verification_status badge
 * is ALWAYS rendered — the "unverified" label is never hidden or suppressed,
 * regardless of any other display logic.
 */
export function ListingCard({ listing }: ListingCardProps) {
  const { cook } = listing
  const isVerified = cook.verification_status === 'verified'
  const isSoldOut = listing.portions_remaining === 0

  return (
    <article className="flex h-full flex-col overflow-hidden rounded-card border border-neutral-200 bg-white p-md shadow-sm transition-shadow hover:shadow-md motion-reduce:transition-none">
      {/* Photo — always rendered via ImageWithFallback; null/failed → branded placeholder */}
      <div className="relative -mx-md -mt-md mb-md">
        <ImageWithFallback
          src={listing.photo_url ?? null}
          alt={listing.food_description}
          className="rounded-none rounded-t-card"
        />
        {isSoldOut && (
          <div className="absolute inset-0 flex items-center justify-center bg-neutral-900/40">
            <Badge tone="neutral" className="bg-white text-neutral-900">
              Sold out
            </Badge>
          </div>
        )}
      </div>

      {/* Food description */}
      <h3 className="mb-xs text-lg font-semibold text-neutral-900">{listing.food_description}</h3>

      {/* Price + portions */}
      <div className="mb-xs flex items-center gap-md text-sm text-neutral-700">
        <span className="text-base font-bold text-primary-700">
          {formatPrice(listing.reduced_price_cents)}
        </span>
        <span>{isSoldOut ? 'Sold out' : `${listing.portions_remaining} portions left`}</span>
      </div>

      {/* Pickup window */}
      <div className="mb-xs text-sm text-neutral-500">
        Pickup:{' '}
        {new Date(listing.pickup_window_start).toLocaleTimeString('nl-NL', {
          hour: '2-digit',
          minute: '2-digit',
        })}{' '}
        &ndash;{' '}
        {new Date(listing.pickup_window_end).toLocaleTimeString('nl-NL', {
          hour: '2-digit',
          minute: '2-digit',
        })}
      </div>

      {/* Distance */}
      <div className="mb-sm text-sm text-neutral-500">{listing.distance_km.toFixed(1)} km away</div>

      {/* Cook info — verification badge is always shown (Principle I / FR-004) */}
      <div className="mt-auto flex flex-wrap items-center gap-sm border-t border-neutral-100 pt-sm">
        <span className="font-semibold text-neutral-900">{cook.display_name}</span>

        {/* Verification badge — NON-NEGOTIABLE per Constitution Principle I.
            Always render; label "unverified" when status is not verified. */}
        <Badge
          data-testid="verification-badge"
          tone={isVerified ? 'success' : 'neutral'}
        >
          {isVerified ? 'verified' : 'unverified'}
        </Badge>

        {/* Org type pill */}
        <Badge tone="neutral">{cook.org_type === 'nonprofit' ? 'non-profit' : 'professional'}</Badge>

        {/* Rating */}
        <span className="ml-auto text-sm text-neutral-600">{cook.rating_avg.toFixed(1)} / 5</span>
      </div>
    </article>
  )
}

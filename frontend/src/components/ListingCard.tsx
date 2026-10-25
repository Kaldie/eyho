import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import type { ListingItem } from '@/services/api'
import { Badge } from '@/components/ui/Badge'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'

interface ListingCardProps {
  listing: ListingItem
  /** Optional action rendered in the card footer (e.g. the Claim button on discovery). */
  action?: ReactNode
  /** When set, the title links to the listing's detail page. Omitted in previews. */
  href?: string
}

/**
 * Formats a cent amount as a locale currency string (EUR by default).
 * e.g. 350 -> "€ 3,50"
 */
function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', {
    style: 'currency',
    currency: 'EUR',
  }).format(cents / 100)
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('nl-NL', { hour: '2-digit', minute: '2-digit' })
}

/**
 * ListingCard displays a single surplus-food listing in the discovery view.
 *
 * Per FR-004 and Constitution Principle I, the cook's verification_status badge
 * is ALWAYS rendered — the "unverified" label is never hidden or suppressed,
 * regardless of any other display logic.
 */
export function ListingCard({ listing, action, href }: ListingCardProps) {
  const { cook } = listing
  const isVerified = cook.verification_status === 'verified'
  const isSoldOut = listing.portions_remaining === 0

  return (
    <article className="flex h-full flex-col overflow-hidden rounded-card border border-neutral-200 bg-white">
      {/* Photo — always rendered via ImageWithFallback; null/failed → branded placeholder */}
      <div className="relative">
        <ImageWithFallback
          src={listing.photo_url ?? null}
          alt={listing.food_description}
          className="rounded-none"
          aspectRatio="16 / 9"
        />
        {/* The price sticker — the one loud element in the design */}
        <span className="price-sticker absolute bottom-sm right-md text-lg">
          {formatPrice(listing.reduced_price_cents)}
        </span>
        {isSoldOut && (
          <div className="absolute inset-0 flex items-center justify-center bg-[rgb(22_26_21/0.45)]">
            <Badge tone="neutral" className="text-sm">
              Sold out
            </Badge>
          </div>
        )}
      </div>

      <div className="flex flex-1 flex-col p-md">
        <h3 className="mb-xs text-lg font-bold leading-snug text-neutral-900">
          {href ? (
            <Link to={href} className="no-underline hover:underline focus-visible:underline">
              {listing.food_description}
            </Link>
          ) : (
            listing.food_description
          )}
        </h3>

        {/* What buyers decide on: when, how far, how many */}
        <p className="mb-md text-sm tabular-nums text-neutral-600">
          Pickup {formatTime(listing.pickup_window_start)}&ndash;
          {formatTime(listing.pickup_window_end)}
          {listing.distance_km !== undefined && `, ${listing.distance_km.toFixed(1)} km away`}
          <br />
          <span className={isSoldOut ? 'text-neutral-500' : 'font-semibold text-primary-700'}>
            {isSoldOut ? 'Sold out' : `${listing.portions_remaining} portions left`}
          </span>
        </p>

        {/* Cook info — verification badge is always shown (Principle I / FR-004) */}
        <div className="mt-auto flex flex-wrap items-center gap-xs border-t border-neutral-100 pt-sm">
          <span className="mr-xs text-sm font-semibold text-neutral-900">{cook.display_name}</span>

          {/* Verification badge — NON-NEGOTIABLE per Constitution Principle I.
              Always render; label "unverified" when status is not verified. */}
          <Badge data-testid="verification-badge" tone={isVerified ? 'success' : 'neutral'}>
            {isVerified ? 'verified' : 'unverified'}
          </Badge>

          {/* Org type pill */}
          <Badge tone="neutral">{cook.org_type === 'nonprofit' ? 'non-profit' : 'professional'}</Badge>

          {/* Rating */}
          <span className="ml-auto text-sm tabular-nums text-neutral-600">
            {cook.rating_count > 0
              ? `${cook.rating_avg.toFixed(1)} / 5 (${cook.rating_count} ${cook.rating_count === 1 ? 'review' : 'reviews'})`
              : 'No reviews yet'}
          </span>
        </div>

        {action && <div className="mt-md">{action}</div>}
      </div>
    </article>
  )
}

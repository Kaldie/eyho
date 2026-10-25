import { useState } from 'react'
import { useParams, Link, useLocation } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getListing } from '@/services/api'
import { ClaimFlow } from '@/components/ClaimFlow'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'
import { QuantityStepper } from '@/components/ui/QuantityStepper'
import { Skeleton } from '@/components/ui/Skeleton'
import type { ResumeClaimState } from '@/auth/session'

// Poll every 12s while the detail page is open so the Claim CTA disables itself as soon
// as portions run out, without requiring a full page reload (FR-011, research.md).
const POLL_INTERVAL_MS = 12_000

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('nl-NL', { hour: '2-digit', minute: '2-digit' })
}

function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(
    cents / 100,
  )
}

/**
 * ListingDetailPage — full detail view for a single listing (FR-003).
 *
 * Renders all fields from the GET /api/v1/listings/{id} response including
 * allergen_info and pickup_address_text (available on detail but not list).
 * The verification badge is always shown per FR-004 / Principle I.
 */
export function ListingDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  // After sign-in at Claim the router state asks us to reopen the same claim (FR-005).
  const resume = (useLocation().state as ResumeClaimState | null)?.resumeClaim
  const resumeHere = resume?.listingId === id ? resume : undefined
  const [claimOpen, setClaimOpen] = useState(Boolean(resumeHere))
  const [claimMessage, setClaimMessage] = useState<string | null>(null)
  const [quantity, setQuantity] = useState(resumeHere?.quantity ?? 1)

  const { data: listing, isLoading } = useQuery({
    queryKey: ['listing', id],
    queryFn: () => getListing(id),
    enabled: Boolean(id),
    refetchInterval: POLL_INTERVAL_MS,
  })

  function handleClaimed(claimedQuantity: number) {
    setClaimMessage(
      `Reserved ${claimedQuantity} portion(s). Find this order any time in Order History.`,
    )
    setClaimOpen(false)
    // Reflect the just-claimed portions immediately without waiting for the next poll.
    void queryClient.invalidateQueries({ queryKey: ['listing', id] })
  }

  if (isLoading) {
    return (
      <main className="mx-auto max-w-2xl px-md py-lg sm:px-lg">
        <Skeleton className="mb-lg h-72 w-full" label="Loading listing" />
        <Skeleton className="mb-sm h-8 w-2/3" label="Loading" />
        <Skeleton className="h-4 w-1/2" label="Loading" />
      </main>
    )
  }

  if (!listing) {
    return (
      <main className="mx-auto max-w-2xl px-md py-lg text-center sm:px-lg">
        <p className="text-neutral-700">
          Listing not found.{' '}
          <Link to="/" className="font-semibold text-primary-700">
            Back to discovery
          </Link>
        </p>
      </main>
    )
  }

  const { cook } = listing
  const isVerified = cook.verification_status === 'verified'
  const isSoldOut = listing.portions_remaining === 0
  const maxQty = Math.max(1, Math.min(listing.portions_remaining, 10))

  const pickupDay = new Date(listing.pickup_window_start).toLocaleDateString('en-GB', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  })
  const pickupTimes = `${formatTime(listing.pickup_window_start)}–${formatTime(listing.pickup_window_end)}`

  return (
    <main className="mx-auto max-w-2xl px-md pb-3xl pt-lg sm:px-lg">
      <Link to="/" className="mb-md inline-block text-sm font-semibold text-primary-700">
        &larr; All food nearby
      </Link>

      <div className="relative mb-lg">
        <ImageWithFallback
          src={listing.photo_url ?? null}
          alt={listing.food_description}
          className="w-full"
          aspectRatio="16 / 9"
        />
        <span className="price-sticker absolute -bottom-sm right-md text-2xl">
          {formatPrice(listing.reduced_price_cents)}
        </span>
      </div>

      <h1 className="mb-xs text-3xl font-extrabold leading-tight text-neutral-900">
        {listing.food_description}
      </h1>
      <p className="mb-lg text-base font-semibold text-primary-700">
        {isSoldOut ? 'Sold out' : `${listing.portions_remaining} portions left`}
      </p>

      {claimMessage && (
        <div
          role="status"
          className="mb-lg rounded-card border border-primary-200 bg-primary-50 px-lg py-sm text-primary-800"
        >
          {claimMessage}
        </div>
      )}

      <dl className="mb-lg grid grid-cols-[auto_1fr] gap-x-lg gap-y-sm rounded-card border border-neutral-200 bg-white p-md text-sm">
        <dt className="font-semibold text-neutral-900">Pickup</dt>
        <dd className="tabular-nums text-neutral-700">
          {pickupDay}, {pickupTimes}
        </dd>
        {listing.pickup_address_text && (
          <>
            <dt className="font-semibold text-neutral-900">Where</dt>
            <dd className="text-neutral-700">
              {listing.pickup_address_text}
              {listing.distance_km !== undefined && ` (${listing.distance_km.toFixed(1)} km away)`}
            </dd>
          </>
        )}
        {listing.allergen_info !== undefined && (
          <>
            <dt className="font-semibold text-neutral-900">Allergens</dt>
            <dd className="text-neutral-700">{listing.allergen_info || 'None declared'}</dd>
          </>
        )}
      </dl>

      {/* Cook card — verification badge always rendered per FR-004 / Principle I */}
      <section className="mb-xl rounded-card border border-neutral-200 bg-white p-md">
        <h2 className="mb-sm text-sm font-semibold text-neutral-600">Cooked by</h2>
        <div className="flex flex-wrap items-center gap-sm">
          <span className="text-lg font-bold text-neutral-900">{cook.display_name}</span>
          {/* Verification badge — NON-NEGOTIABLE per Principle I. Never hidden. */}
          <Badge data-testid="verification-badge" tone={isVerified ? 'success' : 'neutral'}>
            {isVerified ? 'verified' : 'unverified'}
          </Badge>
          <span className="ml-auto text-sm tabular-nums text-neutral-600">
            {cook.rating_count > 0
              ? `Rating: ${cook.rating_avg.toFixed(1)} / 5 (${cook.rating_count} ${cook.rating_count === 1 ? 'review' : 'reviews'})`
              : 'No reviews yet'}
          </span>
        </div>
        <p className="mt-xs text-sm text-neutral-600">
          {cook.org_type === 'nonprofit' ? 'Non-profit organisation' : 'Professional cook'}
          {!isVerified &&
            '. Not verified yet: cooks get verified after 5 ratings of 4 stars or more.'}
          {cook.verification_cycle_count > 0 &&
            ` Their status has changed ${cook.verification_cycle_count} ${cook.verification_cycle_count === 1 ? 'time' : 'times'} based on buyer ratings.`}
        </p>
      </section>

      {!isSoldOut && (
        <div className="mb-lg">
          <QuantityStepper value={quantity} max={maxQty} onChange={setQuantity} label="Portions" />
        </div>
      )}

      {/* Claim button — single step per Principle V */}
      <Button
        type="button"
        className="w-full sm:w-auto"
        disabled={isSoldOut}
        onClick={() => {
          setClaimMessage(null)
          setClaimOpen(true)
        }}
      >
        {isSoldOut ? 'Sold out' : 'Claim this listing'}
      </Button>

      {claimOpen && (
        <ClaimFlow
          listing={listing}
          initialQuantity={quantity}
          onClose={() => setClaimOpen(false)}
          onClaimed={handleClaimed}
        />
      )}
    </main>
  )
}

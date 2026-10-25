import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getListing } from '@/services/api'
import { ClaimFlow } from '@/components/ClaimFlow'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'
import { QuantityStepper } from '@/components/ui/QuantityStepper'
import { Skeleton } from '@/components/ui/Skeleton'

// Poll every 12s while the detail page is open so the Claim CTA disables itself as soon
// as portions run out, without requiring a full page reload (FR-011, research.md).
const POLL_INTERVAL_MS = 12_000

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
  const [claimOpen, setClaimOpen] = useState(false)
  const [claimMessage, setClaimMessage] = useState<string | null>(null)
  const [quantity, setQuantity] = useState(1)

  const { data: listing, isLoading, isError } = useQuery({
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

  if (isError || !listing) {
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

  return (
    <main className="mx-auto max-w-2xl px-md py-lg sm:px-lg">
      <Link to="/" className="mb-md inline-block text-sm font-semibold text-primary-700">
        &larr; Back to discovery
      </Link>

      <ImageWithFallback
        src={listing.photo_url ?? null}
        alt={listing.food_description}
        className="mb-lg w-full"
        aspectRatio="16 / 9"
      />

      <h1 className="mb-xs text-2xl font-bold text-neutral-900">{listing.food_description}</h1>

      {claimMessage && (
        <div
          role="status"
          className="mb-lg rounded-card border border-primary-200 bg-primary-50 px-lg py-sm text-primary-800"
        >
          {claimMessage}
        </div>
      )}

      <div className="mb-lg flex flex-wrap items-center gap-md">
        <span className="text-xl font-bold text-primary-700">
          {formatPrice(listing.reduced_price_cents)}
        </span>
        <Badge tone={isSoldOut ? 'neutral' : 'success'}>
          {isSoldOut ? 'Sold out' : `${listing.portions_remaining} portions left`}
        </Badge>
      </div>

      <section className="mb-lg grid grid-cols-1 gap-sm text-sm text-neutral-700 sm:grid-cols-2">
        {listing.allergen_info && (
          <p>
            <span className="font-semibold text-neutral-900">Allergens:</span>{' '}
            {listing.allergen_info}
          </p>
        )}
        {listing.pickup_address_text && (
          <p>
            <span className="font-semibold text-neutral-900">Pickup location:</span>{' '}
            {listing.pickup_address_text}
          </p>
        )}
        <p>
          <span className="font-semibold text-neutral-900">Pickup window:</span>{' '}
          {new Date(listing.pickup_window_start).toLocaleString('nl-NL')} &ndash;{' '}
          {new Date(listing.pickup_window_end).toLocaleString('nl-NL')}
        </p>
        <p>
          <span className="font-semibold text-neutral-900">Distance:</span>{' '}
          {listing.distance_km.toFixed(1)} km
        </p>
      </section>

      {/* Cook card — verification badge always rendered per FR-004 / Principle I */}
      <section className="mb-lg rounded-card border border-neutral-200 p-md">
        <h2 className="mb-sm text-base font-semibold text-neutral-900">Cook</h2>
        <div className="flex flex-wrap items-center gap-sm">
          <span className="font-semibold text-neutral-900">{cook.display_name}</span>
          {/* Verification badge — NON-NEGOTIABLE per Principle I. Never hidden. */}
          <Badge data-testid="verification-badge" tone={isVerified ? 'success' : 'neutral'}>
            {isVerified ? 'verified' : 'unverified'}
          </Badge>
          <span className="ml-auto text-sm text-neutral-600">
            Rating: {cook.rating_avg.toFixed(1)} / 5
          </span>
        </div>
        <p className="mt-sm text-sm text-neutral-500">
          {cook.org_type === 'nonprofit' ? 'Non-profit organisation' : 'Professional cook'}
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

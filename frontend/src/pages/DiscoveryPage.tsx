import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { ListingCard } from '@/components/ListingCard'
import { ClaimFlow } from '@/components/ClaimFlow'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
import { getListings } from '@/services/api'
import type { ListingItem } from '@/services/api'
import { useQuery } from '@tanstack/react-query'
import type { ResumeClaimState } from '@/auth/session'

const SKELETON_COUNT = 6

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('nl-NL', { hour: '2-digit', minute: '2-digit' })
}

/**
 * DiscoveryPage — the default view for buyers.
 *
 * Per Constitution Principle III (Community Collaboration & Local Discovery),
 * the default scope is always the buyer's local radius. Broadening the radius
 * must be an explicit user action (the radius selector below).
 */
export function DiscoveryPage() {
  // In v1 the buyer's coordinates come from the browser Geolocation API.
  // For the skeleton we use a fixed Amsterdam coordinate so the page renders.
  const lat = 52.37
  const lng = 4.9
  const [radiusKm, setRadiusKm] = useState<number>(2) // local default
  const [activeListing, setActiveListing] = useState<ListingItem | null>(null)
  const [claimMessage, setClaimMessage] = useState<string | null>(null)
  const [initialQuantity, setInitialQuantity] = useState(1)
  const resume = (useLocation().state as ResumeClaimState | null)?.resumeClaim
  const resumed = useRef(false)

  const { data: listings = [], isLoading, isError } = useQuery({
    queryKey: ['listings', lat, lng, radiusKm],
    queryFn: () => getListings(lat, lng, radiusKm),
  })

  // Back from sign-in at Claim: reopen that listing's claim dialog at the same quantity (FR-005).
  useEffect(() => {
    if (!resume || resumed.current) return
    const listing = listings.find((l) => l.id === resume.listingId)
    if (!listing) return
    resumed.current = true
    setInitialQuantity(resume.quantity)
    setActiveListing(listing)
  }, [resume, listings])

  function handleClaimed(quantity: number) {
    if (!activeListing) return
    setClaimMessage(
      `Reserved ${quantity} portion(s) of "${activeListing.food_description}". Find it any time in Order History.`,
    )
    setActiveListing(null)
  }

  function expandRadius() {
    setRadiusKm((current) => (current >= 10 ? current : current === 1 ? 2 : current === 2 ? 5 : 10))
  }

  const latestPickup = listings.reduce<string | null>(
    (latest, l) => (latest && latest > l.pickup_window_end ? latest : l.pickup_window_end),
    null,
  )
  const dishCount = listings.filter((l) => l.portions_remaining > 0).length

  return (
    <main className="mx-auto max-w-6xl px-md pb-3xl pt-lg sm:px-lg sm:pt-2xl">
      {/* Hero — value proposition + primary CTA per FR-001, Constitution Principle V (low friction) */}
      <section className="mb-xl max-w-[42rem] sm:mb-2xl">
        <h1 className="mb-sm text-[2.375rem] font-extrabold leading-[1.02] text-neutral-900 sm:mb-md sm:text-[3.5rem]">
          Save food, save money.
        </h1>
        <p className="mb-md text-base text-neutral-600 sm:mb-lg sm:text-lg">
          {isLoading || dishCount === 0
            ? 'Neighbourhood kitchens put their leftovers up for a fraction of the price. Claim a portion and pick it up today.'
            : `${dishCount} ${dishCount === 1 ? 'dish' : 'dishes'} from neighbourhood kitchens within ${radiusKm} km, at a fraction of the price${
                latestPickup ? `. Pickup until ${formatTime(latestPickup)}` : ''
              }.`}
        </p>
        <Button
          type="button"
          onClick={() => {
            const prefersReducedMotion = window.matchMedia?.(
              '(prefers-reduced-motion: reduce)',
            ).matches
            document
              .getElementById('listing-grid')
              ?.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth' })
          }}
        >
          Browse nearby food
        </Button>
      </section>

      <div className="mb-lg flex flex-wrap items-end justify-between gap-md">
        <div>
          <h2 className="text-2xl font-bold text-neutral-900">Near you</h2>
          <p className="text-sm text-neutral-600">Amsterdam, within {radiusKm} km</p>
        </div>

        {/* Radius selector — explicit user action to broaden per Principle III */}
        <div className="flex items-center gap-sm">
          <label htmlFor="radius-select" className="text-sm font-semibold text-neutral-700">
            Distance
          </label>
          <select
            id="radius-select"
            value={radiusKm}
            onChange={(e) => setRadiusKm(Number(e.target.value))}
            className="min-h-11 rounded-control border border-neutral-300 bg-white px-md text-sm font-semibold text-neutral-900"
          >
            <option value={1}>1 km</option>
            <option value={2}>2 km</option>
            <option value={5}>5 km</option>
            <option value={10}>10 km</option>
          </select>
        </div>
      </div>

      {claimMessage && (
        <div
          role="status"
          className="mb-lg rounded-card border border-primary-200 bg-primary-50 px-lg py-sm text-primary-800"
        >
          {claimMessage}
        </div>
      )}

      {isError && (
        <p className="mb-lg text-sm font-medium text-neutral-700">
          Couldn&rsquo;t load listings. Check your connection and refresh the page.
        </p>
      )}

      {isLoading && (
        <div
          id="listing-grid"
          className="grid grid-cols-1 gap-lg sm:grid-cols-2 lg:grid-cols-3"
          aria-busy="true"
        >
          {Array.from({ length: SKELETON_COUNT }).map((_, i) => (
            <Skeleton key={i} className="h-80 w-full" label="Loading listing" />
          ))}
        </div>
      )}

      {!isLoading && !isError && listings.length === 0 && (
        <EmptyState
          icon="🍲"
          title="No listings nearby yet"
          description="Try expanding your search radius to see more surplus food near you."
          action={
            <Button type="button" variant="secondary" onClick={expandRadius}>
              Expand search radius
            </Button>
          }
        />
      )}

      {!isLoading && !isError && listings.length > 0 && (
        <div id="listing-grid" className="grid grid-cols-1 gap-lg sm:grid-cols-2 lg:grid-cols-3">
          {listings.map((listing) => (
            <ListingCard
              key={listing.id}
              listing={listing}
              href={`/listings/${listing.id}`}
              action={
                <Button
                  type="button"
                  className="w-full"
                  disabled={listing.portions_remaining === 0}
                  onClick={() => {
                    setClaimMessage(null)
                    setInitialQuantity(1)
                    setActiveListing(listing)
                  }}
                >
                  {listing.portions_remaining === 0 ? 'Sold out' : 'Claim'}
                </Button>
              }
            />
          ))}
        </div>
      )}

      {activeListing && (
        <ClaimFlow
          listing={activeListing}
          initialQuantity={initialQuantity}
          onClose={() => setActiveListing(null)}
          onClaimed={handleClaimed}
        />
      )}
    </main>
  )
}

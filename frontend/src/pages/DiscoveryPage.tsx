import { useState } from 'react'
import { ListingCard } from '@/components/ListingCard'
import { ClaimFlow } from '@/components/ClaimFlow'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
import { getListings } from '@/services/api'
import type { ListingItem } from '@/services/api'
import { useQuery } from '@tanstack/react-query'

const SKELETON_COUNT = 6

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

  const { data: listings = [], isLoading, isError } = useQuery({
    queryKey: ['listings', lat, lng, radiusKm],
    queryFn: () => getListings(lat, lng, radiusKm),
  })

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

  return (
    <main className="mx-auto max-w-6xl px-md py-lg sm:px-lg">
      {/* Hero — value proposition + primary CTA per FR-001, Constitution Principle V (low friction) */}
      <section className="mb-xl rounded-card bg-primary-50 px-lg py-2xl text-center sm:px-2xl">
        <h1 className="mb-sm text-3xl font-bold text-neutral-900 sm:text-4xl">
          Save food, save money.
        </h1>
        <p className="mx-auto mb-lg max-w-xl text-base text-neutral-600 sm:text-lg">
          Discover surplus meals from verified and up-and-coming local cooks, at a fraction of the
          price — pick a portion nearby and claim it in seconds.
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

      <h2 className="mb-xs text-xl font-semibold text-neutral-900">Discovery — nearby listings</h2>
      <p className="mb-lg text-sm text-neutral-500">
        Showing surplus food within {radiusKm} km of your location.
      </p>

      {/* Radius selector — explicit user action to broaden per Principle III */}
      <div className="mb-lg flex flex-wrap items-center gap-sm">
        <label htmlFor="radius-select" className="text-sm font-semibold text-neutral-700">
          Search radius:
        </label>
        <select
          id="radius-select"
          value={radiusKm}
          onChange={(e) => setRadiusKm(Number(e.target.value))}
          className="rounded-card border border-neutral-300 px-sm py-xs text-sm text-neutral-900"
        >
          <option value={1}>1 km</option>
          <option value={2}>2 km (default)</option>
          <option value={5}>5 km</option>
          <option value={10}>10 km</option>
        </select>
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
        <p className="mb-lg text-sm font-medium text-neutral-700">Failed to load listings.</p>
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
            <div key={listing.id} className="flex flex-col">
              <ListingCard listing={listing} />
              <Button
                type="button"
                className="mt-sm"
                onClick={() => { setClaimMessage(null); setActiveListing(listing) }}
              >
                Claim
              </Button>
            </div>
          ))}
        </div>
      )}

      {activeListing && (
        <ClaimFlow
          listing={activeListing}
          onClose={() => setActiveListing(null)}
          onClaimed={handleClaimed}
        />
      )}
    </main>
  )
}

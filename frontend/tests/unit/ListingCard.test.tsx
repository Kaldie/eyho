/**
 * Unit tests for ListingCard — focused on the verification badge requirement.
 *
 * Constitution Principle I (NON-NEGOTIABLE): every listing rendered for an
 * unverified cook MUST display an "unverified" badge. These tests guard against
 * regressions where a UI clean-up accidentally drops the badge.
 */
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ListingCard } from '../../src/components/ListingCard'
import type { ListingItem } from '../../src/services/api'

const BASE_LISTING: ListingItem = {
  id: 'test-001',
  food_description: 'Test food',
  reduced_price_cents: 300,
  portions_remaining: 2,
  pickup_window_start: '2026-08-14T17:00:00Z',
  pickup_window_end: '2026-08-14T19:00:00Z',
  distance_km: 0.5,
  cook: {
    display_name: 'Test Cook',
    org_type: 'nonprofit',
    verification_status: 'verified',
    rating_avg: 4.5,
    rating_count: 12,
    verification_cycle_count: 0,
  },
}

describe('ListingCard — verification badge (Principle I)', () => {
  it('shows "verified" badge when cook is verified', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    const badge = screen.getByTestId('verification-badge')
    expect(badge.textContent).toBe('verified')
  })

  it('shows "unverified" badge when cook is unverified', () => {
    const listing: ListingItem = {
      ...BASE_LISTING,
      cook: { ...BASE_LISTING.cook, verification_status: 'unverified' },
    }
    render(<ListingCard listing={listing} />)
    const badge = screen.getByTestId('verification-badge')
    expect(badge.textContent).toBe('unverified')
  })

  it('always renders the badge — never absent', () => {
    // Renders both variants and confirms the badge element is always in the DOM.
    const { rerender } = render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByTestId('verification-badge')).toBeTruthy()

    const unverifiedListing: ListingItem = {
      ...BASE_LISTING,
      cook: { ...BASE_LISTING.cook, verification_status: 'unverified' },
    }
    rerender(<ListingCard listing={unverifiedListing} />)
    expect(screen.getByTestId('verification-badge')).toBeTruthy()
  })

  it('renders food description', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText('Test food')).toBeTruthy()
  })

  it('renders portions remaining', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText(/2 portions left/)).toBeTruthy()
  })

  it('renders cook display name', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText('Test Cook')).toBeTruthy()
  })

  it('renders rating_avg', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText(/4\.5 \/ 5/)).toBeTruthy()
  })
})

describe('ListingCard review count (FR-004)', () => {
  it('shows the number of reviews next to the rating', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText(/4\.5 \/ 5 \(12 reviews\)/)).toBeTruthy()
  })

  it('says there are no reviews yet instead of a 0.0 rating', () => {
    render(
      <ListingCard
        listing={{ ...BASE_LISTING, cook: { ...BASE_LISTING.cook, rating_avg: 0, rating_count: 0 } }}
      />,
    )
    expect(screen.getByText('No reviews yet')).toBeTruthy()
    expect(screen.getByTestId('verification-badge')).toBeTruthy()
  })
})

describe('ListingCard without a distance', () => {
  it('omits the distance instead of crashing (detail and create responses have none)', () => {
    const { distance_km: _unused, ...withoutDistance } = BASE_LISTING
    void _unused
    render(<ListingCard listing={withoutDistance} />)
    expect(screen.queryByText(/km away/)).toBeNull()
    expect(screen.getByTestId('verification-badge')).toBeTruthy()
  })
})

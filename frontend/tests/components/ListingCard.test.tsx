/**
 * Component tests for ListingCard — photo, fallback, sold-out, and verification states.
 *
 * Complements tests/unit/ListingCard.test.tsx (verification badge focus) with the
 * additional states introduced by the Tailwind/primitive restyle (T015).
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
  photo_url: 'https://example.com/photo.jpg',
  cook: {
    display_name: 'Test Cook',
    org_type: 'nonprofit',
    verification_status: 'verified',
    rating_avg: 4.5,
  },
}

describe('ListingCard — photo', () => {
  it('renders an <img> when photo_url is present', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    const img = screen.getByAltText('Test food')
    expect(img.tagName).toBe('IMG')
    expect(img.getAttribute('src')).toBe('https://example.com/photo.jpg')
  })

  it('renders the branded placeholder when photo_url is null', () => {
    const listing: ListingItem = { ...BASE_LISTING, photo_url: null }
    render(<ListingCard listing={listing} />)
    // ImageWithFallback renders a role="img" placeholder (not an <img> element) when src is null.
    const placeholder = screen.getByRole('img', { name: 'Test food' })
    expect(placeholder.tagName).not.toBe('IMG')
    expect(screen.getByText('eyho')).toBeTruthy()
  })

  it('renders the branded placeholder when photo_url is absent', () => {
    const { photo_url: _omit, ...rest } = BASE_LISTING
    const listing = rest as ListingItem
    render(<ListingCard listing={listing} />)
    expect(screen.getByRole('img', { name: 'Test food' })).toBeTruthy()
  })
})

describe('ListingCard — sold out state', () => {
  it('shows "Sold out" when portions_remaining is 0', () => {
    const listing: ListingItem = { ...BASE_LISTING, portions_remaining: 0 }
    render(<ListingCard listing={listing} />)
    const soldOutLabels = screen.getAllByText('Sold out')
    expect(soldOutLabels.length).toBeGreaterThan(0)
  })

  it('shows portions remaining count when not sold out', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByText(/2 portions left/)).toBeTruthy()
    expect(screen.queryByText('Sold out')).toBeNull()
  })
})

describe('ListingCard — verification badge', () => {
  it('shows "verified" badge when cook is verified', () => {
    render(<ListingCard listing={BASE_LISTING} />)
    expect(screen.getByTestId('verification-badge').textContent).toBe('verified')
  })

  it('shows "unverified" badge when cook is unverified — never hidden (Principle I)', () => {
    const listing: ListingItem = {
      ...BASE_LISTING,
      cook: { ...BASE_LISTING.cook, verification_status: 'unverified' },
    }
    render(<ListingCard listing={listing} />)
    expect(screen.getByTestId('verification-badge').textContent).toBe('unverified')
  })
})

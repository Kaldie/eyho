package models

import (
	"time"

	"github.com/google/uuid"
)

// ListingStatus represents the lifecycle state of a surplus food listing.
// Transitions are strictly defined in data-model.md and must not be bypassed.
type ListingStatus string

const (
	// ListingStatusDraft is the initial state; not visible to buyers.
	ListingStatusDraft ListingStatus = "draft"
	// ListingStatusPublished means the listing is visible and claimable.
	// Listings from unverified cooks remain in this state but carry the
	// "unverified" flag (Principle I); they are never auto-cancelled on
	// cook demotion.
	ListingStatusPublished ListingStatus = "published"
	// ListingStatusExpired is set automatically when pickup_window_end passes.
	ListingStatusExpired ListingStatus = "expired"
	// ListingStatusCancelled is set by the cook; triggers refunds for all
	// paid claims (FR-010).
	ListingStatusCancelled ListingStatus = "cancelled"
)

// PickupLocation is the geographic point of a listing's pickup spot.
// Stored as PostGIS geography(Point,4326) in the database; surfaced here as
// lat/lng for application-layer use.
type PickupLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Listing represents a surplus food offering published by a cook.
type Listing struct {
	ID          uuid.UUID `json:"id"`
	CookID      uuid.UUID `json:"cook_id"`
	FoodDescription string `json:"food_description"`
	// AllergenInfo is required; may be an empty string only when the cook
	// explicitly declares "none".
	AllergenInfo string `json:"allergen_info"`
	// PhotoURL is optional; nil renders a placeholder on listing cards.
	PhotoURL *string `json:"photo_url,omitempty"`
	TotalPortions     int `json:"total_portions"`
	// PortionsRemaining must never go below 0 or above TotalPortions (FR-006).
	// Decrements and increments are atomic conditional-UPDATEs in the DB layer.
	PortionsRemaining int `json:"portions_remaining"`
	// ReducedPriceCents must be strictly less than RetailReferencePriceCents
	// (FR-003), enforced server-side.
	ReducedPriceCents         int `json:"reduced_price_cents"`
	RetailReferencePriceCents int `json:"retail_reference_price_cents"`
	// PickupLocation is stored as a PostGIS point; used for radius queries
	// (FR-011, Principle III).
	PickupLocation  PickupLocation `json:"pickup_location"`
	PickupAddressText string       `json:"pickup_address_text"`
	// PickupWindowEnd must be in the future at creation time and must be after
	// PickupWindowStart.
	PickupWindowStart time.Time `json:"pickup_window_start"`
	PickupWindowEnd   time.Time `json:"pickup_window_end"`
	Status            ListingStatus `json:"status"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

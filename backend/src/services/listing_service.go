package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/eyho/eyho/src/models"
)

// CreateListingRequest carries the validated input for creating a new listing.
// Server-side enforcement: reduced_price_cents < retail_reference_price_cents
// (FR-003); pickup_window_end in the future (Edge Case); publish flag vs.
// cook verification status (FR-001 / Principle I).
type CreateListingRequest struct {
	FoodDescription           string
	AllergenInfo              string
	TotalPortions             int
	ReducedPriceCents         int
	RetailReferencePriceCents int
	PickupLocation            models.PickupLocation
	PickupAddressText         string
	PickupWindowStart         string // RFC3339
	PickupWindowEnd           string // RFC3339
	// Publish, if true, attempts to move the listing straight to "published".
	// Cook verification_status determines whether the published listing is
	// flagged "unverified" (Principle I) — it does NOT gate publishing.
	Publish bool
}

// UpdateListingRequest carries the fields a cook may PATCH on an existing
// listing. All fields are optional; only non-zero values are applied.
type UpdateListingRequest struct {
	// PortionsRemaining may be adjusted manually; must not drop below the
	// sum already claimed (409 Conflict if it does).
	PortionsRemaining *int
	// PickupWindowEnd may be extended.
	PickupWindowEnd *string // RFC3339
	// Status may only be set to "cancelled", triggering refunds for paid
	// claims (FR-010).
	Status *models.ListingStatus
}

// ListingService defines the business operations on Listing entities.
// Business rules live here; the api/ layer only does request/response mapping.
type ListingService interface {
	// Create validates the request, enforces pricing rules (FR-003), and
	// persists the listing. If Publish is true and the cook is verified it
	// immediately transitions to "published"; otherwise it lands in "draft".
	// Unverified cooks may still publish — the listing is flagged accordingly
	// (Principle I).
	Create(ctx context.Context, cookID uuid.UUID, req CreateListingRequest) (*models.Listing, error)

	// List returns published listings within the given radius ordered by
	// distance, using PostGIS (FR-011, Principle III).
	List(ctx context.Context, lat, lng, radiusKm float64) ([]*models.Listing, error)

	// Get returns a single listing by ID. Returns ErrNotFound if the listing
	// does not exist or is not visible to the caller.
	Get(ctx context.Context, id uuid.UUID) (*models.Listing, error)

	// Update applies the allowed PATCH fields to a listing.
	// Returns ErrForbidden if the caller is not the owning cook.
	// Returns ErrConflict if PortionsRemaining would be set below already-claimed sum.
	// Cancelling a listing triggers server-side refunds for all paid claims (FR-010).
	Update(ctx context.Context, cookID uuid.UUID, id uuid.UUID, req UpdateListingRequest) (*models.Listing, error)
}

// Sentinel errors returned by ListingService; handlers map these to HTTP codes.
var (
	ErrListingNotFound      = errors.New("listing not found")
	ErrListingForbidden     = errors.New("caller is not the owning cook")
	ErrListingConflict      = errors.New("portions_remaining would drop below claimed sum")
	ErrListingPricingRule   = errors.New("reduced_price_cents must be less than retail_reference_price_cents")
	ErrListingWindowInPast  = errors.New("pickup_window_end must be in the future")
	ErrListingWindowInvalid = errors.New("pickup_window_end must be after pickup_window_start")
)

// stubListingService is a no-op implementation used until the real service is
// wired. Every method returns an explicit "not implemented" error so that
// calling a stub handler fails loudly rather than silently.
type stubListingService struct{}

// NewStubListingService returns the stub implementation for use during
// scaffolding. Replace with the real implementation once db/ queries exist.
func NewStubListingService() ListingService {
	return &stubListingService{}
}

func (s *stubListingService) Create(_ context.Context, _ uuid.UUID, _ CreateListingRequest) (*models.Listing, error) {
	return nil, errors.New("listing_service: Create not implemented")
}

func (s *stubListingService) List(_ context.Context, _, _, _ float64) ([]*models.Listing, error) {
	return nil, errors.New("listing_service: List not implemented")
}

func (s *stubListingService) Get(_ context.Context, _ uuid.UUID) (*models.Listing, error) {
	return nil, errors.New("listing_service: Get not implemented")
}

func (s *stubListingService) Update(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ UpdateListingRequest) (*models.Listing, error) {
	return nil, errors.New("listing_service: Update not implemented")
}

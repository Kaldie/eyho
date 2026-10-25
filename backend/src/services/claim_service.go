package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/eyho/eyho/src/models"
)

// CreateClaimResult holds the data returned to the buyer after a successful
// claim creation (matches claims-api.md 201 response shape).
type CreateClaimResult struct {
	Claim                      *models.Claim
	AmountDueCents             int
	// StripeClientSecret is forwarded to the frontend so Stripe Elements can
	// complete payment without routing card data through our servers.
	StripeClientSecret         string
	PaymentReservationExpiresAt string // RFC3339
}

// UpdateClaimRequest carries the fields that may be patched on a claim.
type UpdateClaimRequest struct {
	// PickupStatus may be set to "collected" via the confirm-pickup endpoint.
	PickupStatus *models.PickupStatus
}

// ClaimService defines the business operations on Claim entities.
// The atomic decrement of Listing.portions_remaining is the responsibility of
// this service layer, enforced via a conditional-UPDATE in the DB (FR-006,
// SC-003).
type ClaimService interface {
	// Create atomically checks and decrements Listing.portions_remaining,
	// creates the Claim with payment_status=pending_payment, and creates a
	// Stripe PaymentIntent. Returns ErrInsufficientPortions if the listing
	// does not have enough remaining. Returns ErrListingNotFound if the
	// listing is not published.
	Create(ctx context.Context, listingID, buyerID uuid.UUID, qty int) (*CreateClaimResult, error)

	// Get returns a single claim by ID. Returns ErrClaimNotFound if it does
	// not exist or is not visible to the caller.
	Get(ctx context.Context, id uuid.UUID) (*models.Claim, error)

	// Update applies allowed patch fields to a claim (currently only
	// pickup_status via the confirm-pickup path). Returns ErrClaimConflict if
	// the claim is not in a state that allows the transition.
	Update(ctx context.Context, id uuid.UUID, req UpdateClaimRequest) (*models.Claim, error)
}

// Sentinel errors returned by ClaimService; handlers map these to HTTP codes.
var (
	ErrInsufficientPortions = errors.New("insufficient portions remaining")
	ErrClaimNotFound        = errors.New("claim not found")
	ErrClaimConflict        = errors.New("claim is not in the required state for this operation")
)

// stubClaimService is a no-op implementation used until the real service is
// wired. Returns explicit "not implemented" errors.
type stubClaimService struct{}

// NewStubClaimService returns the stub implementation for use during
// scaffolding. Replace with the real implementation once db/ queries exist.
func NewStubClaimService() ClaimService {
	return &stubClaimService{}
}

func (s *stubClaimService) Create(_ context.Context, _, _ uuid.UUID, _ int) (*CreateClaimResult, error) {
	return nil, errors.New("claim_service: Create not implemented")
}

func (s *stubClaimService) Get(_ context.Context, _ uuid.UUID) (*models.Claim, error) {
	return nil, errors.New("claim_service: Get not implemented")
}

func (s *stubClaimService) Update(_ context.Context, _ uuid.UUID, _ UpdateClaimRequest) (*models.Claim, error) {
	return nil, errors.New("claim_service: Update not implemented")
}

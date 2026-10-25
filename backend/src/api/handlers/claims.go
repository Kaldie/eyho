package handlers

import (
	"net/http"

	"github.com/eyho/eyho/src/services"
)

// ClaimHandler holds the dependencies for claim HTTP handlers.
// Keep handlers thin: validation and request/response mapping live here;
// business rules live in services.ClaimService.
type ClaimHandler struct {
	svc services.ClaimService
}

// NewClaimHandler constructs a ClaimHandler with the given service.
func NewClaimHandler(svc services.ClaimService) *ClaimHandler {
	return &ClaimHandler{svc: svc}
}

// CreateClaim handles POST /api/v1/claims.
// Contract: claims-api.md §POST /api/v1/claims
// Atomically checks and decrements Listing.portions_remaining (FR-006),
// creates the Claim with payment_status=pending_payment, and returns a
// Stripe client secret for the frontend to complete payment (FR-005).
// Returns 201, 409 (insufficient_portions), or 404 (listing not published).
func (h *ClaimHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

// GetClaim handles GET /api/v1/claims/{id}.
// Contract: claims-api.md (implicit — claim detail for buyer/cook view).
// Returns 200 with claim detail, or 404 if not found / not visible.
func (h *ClaimHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

// UpdateClaim handles PATCH /api/v1/claims/{id}.
// Covers the confirm-pickup path: POST /api/v1/claims/{id}/confirm-pickup
// sets pickup_status = collected.
// Contract: claims-api.md §POST /api/v1/claims/{id}/confirm-pickup
// Returns 200, or 409 if the claim is not in paid state (or already no_show).
func (h *ClaimHandler) UpdateClaim(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

package handlers

import (
	"net/http"

	"github.com/eyho/eyho/src/services"
)

// ListingHandler holds the dependencies for listing HTTP handlers.
// Keep handlers thin: validation and request/response mapping live here;
// business rules live in services.ListingService.
type ListingHandler struct {
	svc services.ListingService
}

// NewListingHandler constructs a ListingHandler with the given service.
func NewListingHandler(svc services.ListingService) *ListingHandler {
	return &ListingHandler{svc: svc}
}

// CreateListing handles POST /api/v1/listings.
// Contract: listings-api.md §POST /api/v1/listings
// Any cook may publish, verified or not (FR-001, Principle I): verification
// status never blocks creation, it only drives the "unverified" flag in
// buyer-facing responses. Returns 201 on success, 403 if the requester has no
// cook profile, 422 on validation failure (FR-003 pricing rule, Edge Case
// pickup window).
func (h *ListingHandler) CreateListing(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

// ListListings handles GET /api/v1/listings.
// Contract: listings-api.md §GET /api/v1/listings
// Query params: lat, lng (required), radius_km (optional), status (optional).
// Returns 200 with listing array; cook verification_status, rating_avg and
// rating_count always included (FR-004, Principle I).
func (h *ListingHandler) ListListings(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

// GetListing handles GET /api/v1/listings/{id}.
// Contract: listings-api.md §GET /api/v1/listings/{id}
// Returns 200 with full listing detail (includes allergen_info,
// pickup_address_text, status). Returns 404 if not found or not visible.
func (h *ListingHandler) GetListing(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

// UpdateListing handles PATCH /api/v1/listings/{id}.
// Contract: listings-api.md §PATCH /api/v1/listings/{id}
// Cook-only. Allowed fields: portions_remaining, pickup_window_end, status
// (cancelled only). Returns 200, 403 (not owner), 409 (portions below claimed
// sum). Cancellation triggers server-side refunds (FR-010).
func (h *ListingHandler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/services"
)

// ClaimHandler maps claim requests and responses; rules live in services.ClaimService.
type ClaimHandler struct {
	svc *services.ClaimService
}

// NewClaimHandler constructs a ClaimHandler.
func NewClaimHandler(svc *services.ClaimService) *ClaimHandler { return &ClaimHandler{svc: svc} }

func claimID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_not_found", "")
		return uuid.Nil, false
	}
	return id, true
}

// CreateClaim handles POST /api/v1/claims (behind RequireVerifiedEmail). The buyer comes from the
// token; a cook claiming their own listing gets 403 own_listing.
func (h *ClaimHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	acct, _ := middleware.AccountFromContext(r.Context())
	var b struct {
		ListingID *string `json:"listing_id"`
		Quantity  *int    `json:"quantity"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.ListingID == nil {
		writeError(w, http.StatusUnprocessableEntity, "missing_field", "listing_id")
		return
	}
	lid, err := uuid.Parse(*b.ListingID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_field", "listing_id")
		return
	}
	if b.Quantity == nil {
		writeError(w, http.StatusUnprocessableEntity, "missing_field", "quantity")
		return
	}
	res, err := h.svc.Create(r.Context(), acct, lid, *b.Quantity)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// ListClaims handles GET /api/v1/claims: the buyer's claims (order history), newest first.
func (h *ClaimHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	acct, _ := middleware.AccountFromContext(r.Context())
	limit := 0
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			writeError(w, http.StatusUnprocessableEntity, "invalid_field", "limit")
			return
		}
		limit = n
	}
	page, err := h.svc.List(r.Context(), acct, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// GetClaim handles GET /api/v1/claims/{id} for the claim's buyer or the listing's cook.
func (h *ClaimHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	id, ok := claimID(w, r)
	if !ok {
		return
	}
	acct, _ := middleware.AccountFromContext(r.Context())
	v, err := h.svc.Get(r.Context(), acct, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// ConfirmPickup handles POST /api/v1/claims/{id}/confirm-pickup (buyer or the listing's cook).
func (h *ClaimHandler) ConfirmPickup(w http.ResponseWriter, r *http.Request) {
	id, ok := claimID(w, r)
	if !ok {
		return
	}
	acct, _ := middleware.AccountFromContext(r.Context())
	v, err := h.svc.ConfirmPickup(r.Context(), acct, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// RateClaim handles POST /api/v1/claims/{id}/rating (the claim's buyer).
func (h *ClaimHandler) RateClaim(w http.ResponseWriter, r *http.Request) {
	id, ok := claimID(w, r)
	if !ok {
		return
	}
	acct, _ := middleware.AccountFromContext(r.Context())
	var b struct {
		Stars *int `json:"stars"`
	}
	if !decode(w, r, &b) {
		return
	}
	stars := 0 // out of range: the service answers 403/404 first, then 422
	if b.Stars != nil {
		stars = *b.Stars
	}
	res, err := h.svc.Rate(r.Context(), acct, id, stars)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

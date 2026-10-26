package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/services"
)

// writeServiceError maps listing/claim service errors to the contract's error bodies (T006):
//
//	422 {error, field}                      validation
//	403 {error: cook_not_registered, registration_status}
//	403 {error: own_listing}
//	409 {error: insufficient_portions, portions_remaining}
//	409 {error: not_collected | already_rated}
//	429 {error: too_many_reservations}
//	404 / 403 / 409 plain {error}
//
// Anything unexpected is logged and answered 500 internal_error without details.
func writeServiceError(w http.ResponseWriter, err error) {
	var ve *services.ValidationError
	var ip *services.InsufficientPortionsError
	var nr *services.CookNotRegisteredError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, ve.Code, ve.Field)
	case errors.As(err, &nr):
		middleware.WriteCookNotRegistered(w, nr.RegistrationStatus)
	case errors.As(err, &ip):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "insufficient_portions", "portions_remaining": ip.Remaining})
	case errors.Is(err, services.ErrOwnListing):
		writeError(w, http.StatusForbidden, "own_listing", "")
	case errors.Is(err, services.ErrCookProfileMissing):
		writeError(w, http.StatusForbidden, "cook_profile_required", "")
	case errors.Is(err, services.ErrListingForbidden):
		writeError(w, http.StatusForbidden, "not_owner", "")
	case errors.Is(err, services.ErrNotClaimBuyer):
		writeError(w, http.StatusForbidden, "not_claim_buyer", "")
	case errors.Is(err, services.ErrListingNotFound):
		writeError(w, http.StatusNotFound, "listing_not_found", "")
	case errors.Is(err, services.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found", "")
	case errors.Is(err, services.ErrNotCollected):
		writeError(w, http.StatusConflict, "not_collected", "")
	case errors.Is(err, services.ErrAlreadyRated):
		writeError(w, http.StatusConflict, "already_rated", "")
	case errors.Is(err, services.ErrClaimConflict):
		writeError(w, http.StatusConflict, "claim_conflict", "")
	case errors.Is(err, services.ErrListingNotActive):
		writeError(w, http.StatusConflict, "listing_not_active", "")
	case errors.Is(err, services.ErrListingNotDraft):
		writeError(w, http.StatusConflict, "listing_not_draft", "")
	case errors.Is(err, services.ErrPortionsConflict):
		writeError(w, http.StatusConflict, "portions_conflict", "")
	case errors.Is(err, services.ErrTooManyReservations):
		writeError(w, http.StatusTooManyRequests, "too_many_reservations", "")
	case errors.Is(err, services.ErrPaymentUnavailable):
		writeError(w, http.StatusBadGateway, "payment_unavailable", "")
	default:
		log.Printf("handler error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
	}
}

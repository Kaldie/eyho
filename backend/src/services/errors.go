package services

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
)

// Errors of the listing and claim services; handlers map them to the contract's status codes
// (api/handlers/errors.go). Validation failures reuse *ValidationError (422).
var (
	ErrListingNotFound  = errors.New("listing not found")
	ErrListingForbidden = errors.New("caller is not the owning cook")
	// ErrListingNotActive: PATCH on a listing that is not published (data-model state machine).
	ErrListingNotActive = errors.New("listing is not published")
	// ErrListingNotDraft: publishing a listing that is not a draft.
	ErrListingNotDraft = errors.New("listing is not a draft")
	// ErrPortionsConflict: the requested portions_remaining would oversell live claims.
	ErrPortionsConflict = errors.New("portions_remaining conflicts with live claims")

	ErrOwnListing         = errors.New("cook cannot claim their own listing")
	ErrClaimNotFound      = errors.New("claim not found")
	ErrClaimConflict      = errors.New("claim is not in the required state")
	ErrNotClaimBuyer      = errors.New("requester is not the claim's buyer")
	ErrNotCollected       = errors.New("claim is not collected")
	ErrAlreadyRated       = errors.New("claim already rated")
	ErrPaymentUnavailable = errors.New("payment provider unavailable")
)

// InsufficientPortionsError maps to 409 {"error":"insufficient_portions","portions_remaining":n}.
type InsufficientPortionsError struct{ Remaining int }

func (e *InsufficientPortionsError) Error() string {
	return fmt.Sprintf("insufficient portions (%d remaining)", e.Remaining)
}

// CookNotRegisteredError maps to 403 {"error":"cook_not_registered","registration_status":...}.
type CookNotRegisteredError struct{ RegistrationStatus string }

func (e *CookNotRegisteredError) Error() string {
	return "cook not registered: " + e.RegistrationStatus
}

// numericFloat converts a NUMERIC to float64 (0 when NULL or unrepresentable).
func numericFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// numericFromFloat builds a NUMERIC from a float64 (used for the default radius in tests).
func numericFromFloat(f float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(f * 100)), Exp: -2, Valid: true}
}

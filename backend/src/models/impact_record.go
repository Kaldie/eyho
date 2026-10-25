package models

import (
	"time"

	"github.com/google/uuid"
)

// ImpactRecord is an append-only, immutable record created server-side the
// moment a Claim reaches payment_status = paid (FR-009, Principle II).
//
// Impact metrics MUST be derived from real transaction data — they are never
// client-supplied or estimated without disclosure (Principle II).
type ImpactRecord struct {
	ID        uuid.UUID `json:"id"`
	// ClaimID has a unique constraint; exactly one ImpactRecord per completed claim.
	ClaimID   uuid.UUID `json:"claim_id"`
	// ListingID is denormalized from the claim for efficient aggregate queries.
	ListingID uuid.UUID `json:"listing_id"`
	// Portions is copied from Claim.Quantity at completion time.
	Portions  int `json:"portions"`
	// EstimatedWeightGrams is derived from a per-listing or per-dish average.
	// The estimation method is an implementation detail; it must not be
	// presented as a measured fact to end users (Principle II).
	EstimatedWeightGrams int       `json:"estimated_weight_grams"`
	CreatedAt            time.Time `json:"created_at"`
}

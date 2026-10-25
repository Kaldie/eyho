package models

import (
	"time"

	"github.com/google/uuid"
)

// OrgType represents the type of cook organisation.
type OrgType string

const (
	OrgTypeNonprofit    OrgType = "nonprofit"
	OrgTypeProfessional OrgType = "professional"
)

// VerificationStatus reflects the crowd-sourced reputation state of a cook.
// Per Constitution Principle I: this status does NOT gate publishing — it only
// controls whether listings are flagged "unverified" to buyers.
type VerificationStatus string

const (
	VerificationStatusUnverified VerificationStatus = "unverified"
	VerificationStatusVerified   VerificationStatus = "verified"
)

// CookProfile represents a non-profit or professional cook who may publish
// listings. Verification is reputation-driven, not a manual upfront gate.
type CookProfile struct {
	ID                   uuid.UUID          `json:"id"`
	DisplayName          string             `json:"display_name"`
	OrgType              OrgType            `json:"org_type"`
	VerificationStatus   VerificationStatus `json:"verification_status"`
	// GoodRatingsSinceTransition counts buyer ratings >=4 stars since the last
	// status change. Reaching 5 promotes unverified -> verified.
	GoodRatingsSinceTransition int `json:"good_ratings_since_transition"`
	// BadRatingsSinceTransition counts buyer ratings <=2 stars since the last
	// status change. Reaching 3 demotes verified -> unverified.
	BadRatingsSinceTransition int `json:"bad_ratings_since_transition"`
	// VerificationCycleCount is the total number of status transitions in
	// either direction, kept for auditability (Principle I).
	VerificationCycleCount int     `json:"verification_cycle_count"`
	RatingAvg              float64 `json:"rating_avg"`
	RatingCount            int     `json:"rating_count"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// VerificationHistory is an append-only record of every verification status
// transition for a cook. Required by Constitution Principle I.
type VerificationHistory struct {
	ID                  uuid.UUID          `json:"id"`
	CookID              uuid.UUID          `json:"cook_id"`
	FromStatus          VerificationStatus `json:"from_status"`
	ToStatus            VerificationStatus `json:"to_status"`
	// TriggeringListingIDs holds the listing/rating IDs that caused this
	// transition, for full auditability.
	TriggeringClaimIDs []uuid.UUID `json:"triggering_claim_ids"`
	CreatedAt            time.Time   `json:"created_at"`
}

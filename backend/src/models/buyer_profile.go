package models

import (
	"time"

	"github.com/google/uuid"
)

// BuyerProfile holds buyer-specific settings, notably the default discovery
// radius used for the local listing discovery view (FR-011, Principle III).
type BuyerProfile struct {
	ID                       uuid.UUID `json:"id"`
	// DefaultDiscoveryRadiusKm is the buyer's preferred local search radius.
	// Defaults to ~3 km on first creation.
	DefaultDiscoveryRadiusKm float64   `json:"default_discovery_radius_km"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

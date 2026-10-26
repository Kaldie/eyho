// Package registry checks cook KvK numbers against the Dutch business register (feature 001,
// FR-018, FR-021, FR-023). The registration status is a legal precondition for publishing and is
// never the same thing as the reputation-based "verified" status (Constitution Principle I).
package registry

import "context"

// Status is a definite answer from the register.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusNotFound Status = "not_found"
)

// Result is a definite answer. TradeName may be empty for not_found.
type Result struct {
	Status    Status
	TradeName string
}

// BusinessRegister looks up a KvK number. A non-nil error means NO answer was obtained
// (transport failure, timeout, unexpected response, rejected API key): callers must treat it as
// "unknown" and never as inactive or not found.
type BusinessRegister interface {
	Check(ctx context.Context, kvkNumber string) (Result, error)
}

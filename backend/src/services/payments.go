package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// PaymentProvider is the seam between claim logic and the payment processor (T003). Claim and
// listing services only talk to this interface, so tests use a fake and feature 004 can add a
// simulated provider without touching claim logic. No card data ever reaches the backend: the
// frontend confirms the intent with the client secret.
type PaymentProvider interface {
	// CreateIntent creates a payment for a reserved claim. It must be idempotent per ClaimID.
	CreateIntent(ctx context.Context, req IntentRequest) (*Intent, error)
	// Refund refunds the full amount of a paid intent. The idempotency key (the claim id) makes a
	// retry after a crash safe.
	Refund(ctx context.Context, intentID, idempotencyKey string) error
	// CancelIntent cancels an unpaid payment so it can no longer succeed (used when a reservation is
	// released). Cancelling an intent that is already cancelled or paid is not an error worth
	// retrying: a payment that still succeeds is refunded by the webhook handler.
	CancelIntent(ctx context.Context, intentID string) error
	// ParseWebhook verifies the signature of a webhook delivery and decodes the event. It returns
	// ErrInvalidWebhookSignature for a bad signature.
	ParseWebhook(payload []byte, signatureHeader string) (*PaymentEvent, error)
}

// IntentRequest describes the payment to create.
type IntentRequest struct {
	ClaimID     uuid.UUID
	AmountCents int
	Currency    string // lower-case ISO code, "eur"
}

// Intent is the created payment; ClientSecret is handed to the frontend.
type Intent struct {
	ID           string
	ClientSecret string
}

// PaymentEventType is the subset of provider events the backend acts on.
type PaymentEventType string

const (
	PaymentSucceeded PaymentEventType = "succeeded"
	PaymentFailed    PaymentEventType = "failed"
	PaymentIgnored   PaymentEventType = "ignored"
)

// PaymentEvent is a verified, provider-neutral webhook event.
type PaymentEvent struct {
	Type        PaymentEventType
	IntentID    string
	ClaimID     uuid.UUID // from the intent metadata set in CreateIntent; uuid.Nil if absent
	AmountCents int
	Currency    string // lower-case ISO code as reported by the provider
}

// ErrInvalidWebhookSignature is returned by ParseWebhook for unverifiable deliveries, including every delivery when no webhook secret is configured.
var ErrInvalidWebhookSignature = errors.New("invalid webhook signature")

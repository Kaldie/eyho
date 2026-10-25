package handlers

import (
	"net/http"

	"github.com/eyho/eyho/src/services"
)

// WebhookHandler holds dependencies for inbound webhook handlers.
type WebhookHandler struct {
	claimSvc services.ClaimService
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(claimSvc services.ClaimService) *WebhookHandler {
	return &WebhookHandler{claimSvc: claimSvc}
}

// StripeWebhook handles POST /api/v1/webhooks/stripe.
// Contract: claims-api.md §Stripe webhook
//
// Relevant Stripe events:
//   - payment_intent.succeeded  -> set Claim.payment_status = paid,
//     confirmed_at = now(), create ImpactRecord (FR-009, Principle II).
//   - payment_intent.payment_failed -> set Claim.payment_status =
//     payment_failed, release reserved portions to Listing.portions_remaining
//     (FR-008).
//
// The endpoint MUST verify the Stripe-Signature header before processing
// any event payload.
func (h *WebhookHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w)
}

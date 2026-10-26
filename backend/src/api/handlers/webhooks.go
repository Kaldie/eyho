package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/eyho/eyho/src/services"
)

// WebhookHandler serves the payment provider's webhook.
type WebhookHandler struct {
	payments services.PaymentProvider
	claims   *services.ClaimService
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(payments services.PaymentProvider, claims *services.ClaimService) *WebhookHandler {
	return &WebhookHandler{payments: payments, claims: claims}
}

// StripeWebhook handles POST /api/v1/webhooks/stripe. The signature is verified before anything
// else; an unverifiable delivery gets 400 and changes nothing. payment_intent.succeeded and
// payment_intent.payment_failed are applied idempotently (FR-008, FR-009); other events get 200.
// A processing error answers 500 so the provider redelivers.
func (h *WebhookHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "")
		return
	}
	ev, err := h.payments.ParseWebhook(payload, r.Header.Get("Stripe-Signature"))
	if errors.Is(err, services.ErrInvalidWebhookSignature) {
		writeError(w, http.StatusBadRequest, "invalid_signature", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "")
		return
	}
	if err := h.claims.HandlePaymentEvent(r.Context(), ev); err != nil {
		log.Printf("webhook: handling %s for claim %s: %v", ev.Type, ev.ClaimID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	w.WriteHeader(http.StatusOK)
}

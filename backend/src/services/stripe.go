package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/client"
	"github.com/stripe/stripe-go/v87/webhook"
)

// StripeProvider implements PaymentProvider with stripe-go.
//
// LOCAL LIMITATION: docker-compose defaults STRIPE_SECRET_KEY to sk_test_placeholder, which Stripe
// rejects, so POST /claims answers 502 payment_unavailable (and releases the reserved portions)
// until real test keys are supplied. Automated tests use a fake PaymentProvider; a simulated
// provider for demos is feature 004's job.
type StripeProvider struct {
	api           *client.API
	webhookSecret string
}

// NewStripeProvider creates the provider; webhookSecret verifies webhook signatures.
func NewStripeProvider(secretKey, webhookSecret string) *StripeProvider {
	api := &client.API{}
	api.Init(secretKey, nil)
	return &StripeProvider{api: api, webhookSecret: webhookSecret}
}

// CreateIntent creates a PaymentIntent whose metadata carries the claim id.
func (p *StripeProvider) CreateIntent(ctx context.Context, req IntentRequest) (*Intent, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(req.AmountCents)),
		Currency: stripe.String(req.Currency),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	params.Context = ctx
	params.AddMetadata("claim_id", req.ClaimID.String())
	params.SetIdempotencyKey("intent-" + req.ClaimID.String())
	pi, err := p.api.PaymentIntents.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe create payment intent: %w", err)
	}
	return &Intent{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

// Refund refunds the whole payment.
func (p *StripeProvider) Refund(ctx context.Context, intentID, idempotencyKey string) error {
	params := &stripe.RefundParams{PaymentIntent: stripe.String(intentID)}
	params.Context = ctx
	params.SetIdempotencyKey("refund-" + idempotencyKey)
	if _, err := p.api.Refunds.New(params); err != nil {
		return fmt.Errorf("stripe refund: %w", err)
	}
	return nil
}

// CancelIntent cancels an unpaid PaymentIntent.
func (p *StripeProvider) CancelIntent(ctx context.Context, intentID string) error {
	params := &stripe.PaymentIntentCancelParams{}
	params.Context = ctx
	if _, err := p.api.PaymentIntents.Cancel(intentID, params); err != nil {
		return fmt.Errorf("stripe cancel payment intent: %w", err)
	}
	return nil
}

// ParseWebhook verifies Stripe-Signature and maps payment_intent.* events. With an empty webhook
// secret every delivery is rejected: stripe-go would otherwise HMAC with an empty key, which anyone
// can compute, and the endpoint would accept forged payment events.
func (p *StripeProvider) ParseWebhook(payload []byte, signatureHeader string) (*PaymentEvent, error) {
	if p.webhookSecret == "" {
		return nil, ErrInvalidWebhookSignature
	}
	ev, err := webhook.ConstructEventWithOptions(payload, signatureHeader, p.webhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		if errors.Is(err, webhook.ErrInvalidHeader) || errors.Is(err, webhook.ErrNoValidSignature) ||
			errors.Is(err, webhook.ErrNotSigned) || errors.Is(err, webhook.ErrTooOld) {
			return nil, ErrInvalidWebhookSignature
		}
		return nil, err
	}
	out := &PaymentEvent{Type: PaymentIgnored}
	switch ev.Type {
	case "payment_intent.succeeded":
		out.Type = PaymentSucceeded
	case "payment_intent.payment_failed":
		out.Type = PaymentFailed
	default:
		return out, nil
	}
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(ev.Data.Raw, &pi); err != nil {
		return nil, fmt.Errorf("decode payment intent: %w", err)
	}
	out.IntentID = pi.ID
	out.AmountCents = int(pi.Amount)
	out.Currency = string(pi.Currency)
	if pi.AmountReceived > 0 {
		out.AmountCents = int(pi.AmountReceived)
	}
	if id, err := uuid.Parse(pi.Metadata["claim_id"]); err == nil {
		out.ClaimID = id
	}
	return out, nil
}

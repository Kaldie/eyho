package testutil

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/eyho/eyho/src/services"
)

// ValidSignature is the webhook signature header FakePayments accepts.
const ValidSignature = "valid"

// FakePayments is an in-memory services.PaymentProvider for tests. Webhook payloads are JSON:
// {"type":"succeeded|failed","intent_id":"...","claim_id":"...","amount_cents":500}, signed with
// the header ValidSignature.
type FakePayments struct {
	mu         sync.Mutex
	Intents    map[string]services.IntentRequest // intent id -> request
	Refunds    []string                          // intent ids refunded, in order
	Cancelled  []string                          // intent ids cancelled, in order
	FailCreate bool
	FailRefund bool
}

// CreateIntent records the request and returns a deterministic intent.
func (f *FakePayments) CreateIntent(_ context.Context, req services.IntentRequest) (*services.Intent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailCreate {
		return nil, errors.New("provider down")
	}
	if f.Intents == nil {
		f.Intents = map[string]services.IntentRequest{}
	}
	id := "pi_fake_" + req.ClaimID.String()
	f.Intents[id] = req
	return &services.Intent{ID: id, ClientSecret: id + "_secret"}, nil
}

// Refund records the refunded intent.
func (f *FakePayments) Refund(_ context.Context, intentID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailRefund {
		return errors.New("refund failed")
	}
	f.Refunds = append(f.Refunds, intentID)
	return nil
}

// CancelIntent records the cancelled intent.
func (f *FakePayments) CancelIntent(_ context.Context, intentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Cancelled = append(f.Cancelled, intentID)
	return nil
}

// CancelledIntents returns a copy of the cancelled intent ids.
func (f *FakePayments) CancelledIntents() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.Cancelled...)
}

// RefundCount returns how many refunds were issued.
func (f *FakePayments) RefundCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Refunds)
}

// ParseWebhook decodes the JSON test payload when the signature matches.
func (f *FakePayments) ParseWebhook(payload []byte, sig string) (*services.PaymentEvent, error) {
	if sig != ValidSignature {
		return nil, services.ErrInvalidWebhookSignature
	}
	var in struct {
		Type        string `json:"type"`
		IntentID    string `json:"intent_id"`
		ClaimID     string `json:"claim_id"`
		AmountCents int    `json:"amount_cents"`
		Currency    string `json:"currency"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, err
	}
	ev := &services.PaymentEvent{Type: services.PaymentEventType(in.Type), IntentID: in.IntentID, AmountCents: in.AmountCents, Currency: in.Currency}
	if ev.Currency == "" {
		ev.Currency = "eur"
	}
	if id, err := uuid.Parse(in.ClaimID); err == nil {
		ev.ClaimID = id
	}
	return ev, nil
}

// SucceedBody builds the webhook body for a successful payment of the claim.
func SucceedBody(claimID string, amountCents int) string {
	b, _ := json.Marshal(map[string]any{"type": "succeeded", "intent_id": "pi_fake_" + claimID, "claim_id": claimID, "amount_cents": amountCents})
	return string(b)
}

// FailBody builds the webhook body for a failed payment of the claim.
func FailBody(claimID string, amountCents int) string {
	b, _ := json.Marshal(map[string]any{"type": "failed", "intent_id": "pi_fake_" + claimID, "claim_id": claimID, "amount_cents": amountCents})
	return string(b)
}

// SucceedBodyCurrency is SucceedBody with an explicit currency.
func SucceedBodyCurrency(claimID string, amountCents int, cur string) string {
	b, _ := json.Marshal(map[string]any{"type": "succeeded", "intent_id": "pi_fake_" + claimID, "claim_id": claimID, "amount_cents": amountCents, "currency": cur})
	return string(b)
}

package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v76/webhook"
)

const testWebhookSecret = "whsec_unit_test_secret"

func eventPayload(eventType string, claimID uuid.UUID) []byte {
	return []byte(`{"id":"evt_1","object":"event","type":"` + eventType + `","data":{"object":{"id":"pi_123","object":"payment_intent","amount":1000,"amount_received":1000,"currency":"eur","metadata":{"claim_id":"` + claimID.String() + `"}}}}`)
}

func signed(payload []byte, secret string, at time.Time) string {
	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: at}).Header
}

func TestStripeProvider_ParseWebhook_ValidSignature(t *testing.T) {
	p := NewStripeProvider("sk_test_x", testWebhookSecret)
	claim := uuid.New()
	payload := eventPayload("payment_intent.succeeded", claim)

	ev, err := p.ParseWebhook(payload, signed(payload, testWebhookSecret, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, PaymentSucceeded, ev.Type)
	assert.Equal(t, "pi_123", ev.IntentID)
	assert.Equal(t, claim, ev.ClaimID)
	assert.Equal(t, 1000, ev.AmountCents)
	assert.Equal(t, "eur", ev.Currency)

	failed := eventPayload("payment_intent.payment_failed", claim)
	ev, err = p.ParseWebhook(failed, signed(failed, testWebhookSecret, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, PaymentFailed, ev.Type)

	other := eventPayload("charge.refunded", claim)
	ev, err = p.ParseWebhook(other, signed(other, testWebhookSecret, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, PaymentIgnored, ev.Type)
}

func TestStripeProvider_ParseWebhook_TamperedOrWrongSecretOrStale(t *testing.T) {
	p := NewStripeProvider("sk_test_x", testWebhookSecret)
	payload := eventPayload("payment_intent.succeeded", uuid.New())
	header := signed(payload, testWebhookSecret, time.Now())

	tampered := append([]byte(nil), payload...)
	tampered[len(tampered)-5] ^= 1
	_, err := p.ParseWebhook(tampered, header)
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature, "payload changed after signing")

	_, err = p.ParseWebhook(payload, signed(payload, "whsec_other", time.Now()))
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature, "signed with another secret")

	_, err = p.ParseWebhook(payload, "")
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature, "no header")
	_, err = p.ParseWebhook(payload, "garbage")
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature, "malformed header")

	_, err = p.ParseWebhook(payload, signed(payload, testWebhookSecret, time.Now().Add(-time.Hour)))
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature, "replayed outside the tolerance")
}

// B1: with no secret configured stripe-go would HMAC with an empty key, which anyone can compute.
func TestStripeProvider_ParseWebhook_EmptySecretRejectsEverything(t *testing.T) {
	p := NewStripeProvider("sk_test_x", "")
	payload := eventPayload("payment_intent.succeeded", uuid.New())
	forged := signed(payload, "", time.Now()) // a valid signature for the empty key

	_, err := p.ParseWebhook(payload, forged)
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature)
	_, err = p.ParseWebhook(payload, "")
	assert.ErrorIs(t, err, ErrInvalidWebhookSignature)
}

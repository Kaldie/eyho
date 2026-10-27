package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v87/webhook"

	"github.com/eyho/eyho/src/services"
)

// The claim service is nil on purpose: an unverifiable delivery must be refused before any claim
// logic runs.
func post(t *testing.T, secret, payload, header string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewWebhookHandler(services.NewStripeProvider("sk_test_x", secret), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", strings.NewReader(payload))
	if header != "" {
		req.Header.Set("Stripe-Signature", header)
	}
	rec := httptest.NewRecorder()
	h.StripeWebhook(rec, req)
	return rec
}

func TestStripeWebhook_InvalidSignatureIs400(t *testing.T) {
	const payload = `{"id":"evt_1","object":"event","type":"payment_intent.succeeded","data":{"object":{}}}`
	good := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(payload), Secret: "whsec_a", Timestamp: time.Now()}).Header

	for name, c := range map[string]struct{ secret, header string }{
		"missing header": {"whsec_a", ""},
		"wrong header":   {"whsec_a", "t=1,v1=abc"},
		"other secret":   {"whsec_b", good},
		// A signature over the empty key is valid for stripe-go; the provider must still refuse it.
		"empty secret": {"", webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(payload), Secret: "", Timestamp: time.Now()}).Header},
	} {
		rec := post(t, c.secret, payload, c.header)
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Contains(t, rec.Body.String(), `"invalid_signature"`, name)
	}
}

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func demoVerifier(t *testing.T, audience string) (*Verifier, *DemoIssuer) {
	d, err := NewDemoIssuer("eyho-api")
	require.NoError(t, err)
	v := NewVerifier(audience)
	v.AddStaticIssuer(DemoIssuerName, d.KeySet())
	return v, d
}

func TestVerify_DemoTokenClaims(t *testing.T) {
	v, d := demoVerifier(t, "eyho-api")
	tok, exp, err := d.Mint("buyer", "demo-buyer@eyho.example", time.Now())
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(DemoTokenLifetime), exp, 5*time.Second)

	c, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	assert.Equal(t, "eyho-demo", c.Issuer)
	assert.Equal(t, "buyer", c.Subject)
	assert.Equal(t, "demo-buyer@eyho.example", c.Email)
	assert.True(t, c.EmailVerified)
}

func TestVerify_RejectsWrongAudienceExpiredAndUntrusted(t *testing.T) {
	v, d := demoVerifier(t, "another-api")
	tok, _, _ := d.Mint("buyer", "b@x.nl", time.Now())
	_, err := v.Verify(context.Background(), tok)
	assert.ErrorIs(t, err, ErrInvalidToken, "wrong audience")

	v, d = demoVerifier(t, "eyho-api")
	old, _, _ := d.Mint("buyer", "b@x.nl", time.Now().Add(-3*time.Hour))
	_, err = v.Verify(context.Background(), old)
	assert.ErrorIs(t, err, ErrInvalidToken, "expired")

	other, _ := NewDemoIssuer("eyho-api") // same iss, different key
	forged, _, _ := other.Mint("buyer", "b@x.nl", time.Now())
	_, err = v.Verify(context.Background(), forged)
	assert.ErrorIs(t, err, ErrInvalidToken, "signature from another key")

	empty := NewVerifier("eyho-api")
	tok, _, _ = d.Mint("buyer", "b@x.nl", time.Now())
	_, err = empty.Verify(context.Background(), tok)
	assert.ErrorIs(t, err, ErrInvalidToken, "untrusted issuer")

	for _, junk := range []string{"", "abc", "a.b.c", "a.e30.c"} {
		_, err = v.Verify(context.Background(), junk)
		assert.ErrorIs(t, err, ErrInvalidToken, junk)
	}
}

// Docker case: the issuer's host name only resolves for the browser; the backend connects to the
// discovery address but keeps the issuer's Host so the reported `iss` matches.
func TestAddRemoteIssuer_DiscoveryAddressRedirect(t *testing.T) {
	d, err := NewDemoIssuer("eyho-api")
	require.NoError(t, err)
	const issuer = "http://idp.invalid:8081/eyho"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "idp.invalid:8081", r.Host)
		switch r.URL.Path {
		case "/eyho/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "jwks_uri": issuer + "/jwks"})
		case "/eyho/jwks":
			json.NewEncoder(w).Encode(d.JWKS())
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	v := NewVerifier("eyho-api")
	v.AddRemoteIssuer(issuer, srv.URL)

	// Mint a token as that issuer using the demo key material.
	tok := mintAs(t, d, issuer)
	c, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	assert.Equal(t, "someone", c.Subject)
}

func TestVerify_UnreachableIssuerIsNotA401(t *testing.T) {
	v := NewVerifier("eyho-api")
	v.AddRemoteIssuer("http://127.0.0.1:1/eyho", "")
	d, _ := NewDemoIssuer("eyho-api")
	_, err := v.Verify(context.Background(), mintAs(t, d, "http://127.0.0.1:1/eyho"))
	var unavailable *UnavailableError
	assert.ErrorAs(t, err, &unavailable)
	assert.NotErrorIs(t, err, ErrInvalidToken)
}

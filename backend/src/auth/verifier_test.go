package auth

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
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

func claimsJSON(t *testing.T) []byte {
	t.Helper()
	now := time.Now()
	b, err := json.Marshal(map[string]any{
		"iss": DemoIssuerName, "sub": "buyer", "aud": "eyho-api",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		ClaimEmail: "demo-buyer@eyho.example", ClaimEmailVerified: true,
	})
	require.NoError(t, err)
	return b
}

// An unsigned token ("alg":"none") with otherwise valid claims must be rejected.
func TestVerify_RejectsAlgNone(t *testing.T) {
	v, _ := demoVerifier(t, "eyho-api")
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := enc.EncodeToString(claimsJSON(t))
	for _, tok := range []string{header + "." + payload + ".", header + "." + payload + "." + enc.EncodeToString([]byte("sig"))} {
		_, err := v.Verify(context.Background(), tok)
		assert.ErrorIs(t, err, ErrInvalidToken)
	}
}

// Algorithm confusion: an HS256 token whose HMAC secret is the issuer's public key must be
// rejected (only RS256 is accepted).
func TestVerify_RejectsHS256KeyConfusion(t *testing.T) {
	v, d := demoVerifier(t, "eyho-api")
	pub, err := x509.MarshalPKIXPublicKey(&d.key.PublicKey)
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: pub}, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)
	now := time.Now()
	tok, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer: DemoIssuerName, Subject: "buyer", Audience: jwt.Audience{"eyho-api"},
		IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(time.Hour)),
	}).Serialize()
	require.NoError(t, err)
	_, err = v.Verify(context.Background(), tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerify_ExposesIssuedAt(t *testing.T) {
	v, d := demoVerifier(t, "eyho-api")
	at := time.Now().Add(-10 * time.Minute)
	tok, _, err := d.Mint("buyer", "b@x.nl", at)
	require.NoError(t, err)
	c, err := v.Verify(context.Background(), tok)
	require.NoError(t, err)
	assert.WithinDuration(t, at, c.IssuedAt, time.Second)
}

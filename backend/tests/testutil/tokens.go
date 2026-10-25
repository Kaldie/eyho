// Package testutil provides an in-process OIDC test issuer and an integration test environment
// shared by features 001 and 002 (feature 001, T012).
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/eyho/eyho/src/auth"
)

// Audience is the API identifier the test issuer mints tokens for by default.
const Audience = "eyho-api"

// Issuer is an in-process token issuer serving discovery and JWKS over httptest, so the real
// verifier code path (go-oidc, JWKS fetch) is exercised.
type Issuer struct {
	URL      string
	Audience string
	key      *rsa.PrivateKey
	signer   jose.Signer
	srv      *httptest.Server
}

// NewIssuer starts a test issuer; it is closed with the test.
func NewIssuer(t testing.TB) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "test"}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	i := &Issuer{Audience: Audience, key: key, signer: signer}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": i.URL, "jwks_uri": i.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
	})
	i.srv = httptest.NewServer(mux)
	i.URL = i.srv.URL
	t.Cleanup(i.srv.Close)
	return i
}

// Verifier returns a verifier that trusts only this issuer.
func (i *Issuer) Verifier() *auth.Verifier {
	v := auth.NewVerifier(i.Audience)
	v.AddRemoteIssuer(i.URL, "")
	return v
}

// TokenOptions chooses what a minted token says. Zero values mean: audience Audience, valid for
// one hour, no email.
type TokenOptions struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Audience      string        // override (for wrong-audience tests)
	ExpiresIn     time.Duration // negative: already expired
	Issuer        string        // override
}

// Token mints a signed access token.
func (i *Issuer) Token(o TokenOptions) string {
	if o.Audience == "" {
		o.Audience = i.Audience
	}
	if o.ExpiresIn == 0 {
		o.ExpiresIn = time.Hour
	}
	if o.Issuer == "" {
		o.Issuer = i.URL
	}
	now := time.Now()
	std := jwt.Claims{
		Issuer: o.Issuer, Subject: o.Subject, Audience: jwt.Audience{o.Audience},
		IssuedAt: jwt.NewNumericDate(now.Add(-2 * time.Hour)), Expiry: jwt.NewNumericDate(now.Add(o.ExpiresIn)),
	}
	extra := map[string]any{auth.ClaimEmailVerified: o.EmailVerified}
	if o.Email != "" {
		extra[auth.ClaimEmail] = o.Email
	}
	if o.Name != "" {
		extra["name"] = o.Name
	}
	tok, err := jwt.Signed(i.signer).Claims(std).Claims(extra).Serialize()
	if err != nil {
		panic(err)
	}
	return tok
}

package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// DemoIssuerName is the `iss` of demo tokens and the issuer of the seeded demo identities
// (backend/src/db/seed/demo_accounts.sql).
const DemoIssuerName = "eyho-demo"

// DemoTokenLifetime is how long a demo token stays valid.
const DemoTokenLifetime = time.Hour

// DemoIssuer signs short-lived tokens for the seeded demo accounts. The key is generated at
// startup, so tokens do not survive a restart. Only constructed when the safeguard allows it.
type DemoIssuer struct {
	audience string
	key      *rsa.PrivateKey
	kid      string
	signer   jose.Signer
}

// NewDemoIssuer generates a fresh signing key.
func NewDemoIssuer(audience string) (*DemoIssuer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	kid := fmt.Sprintf("demo-%d", time.Now().UnixNano())
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: kid}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return nil, err
	}
	return &DemoIssuer{audience: audience, key: key, kid: kid, signer: signer}, nil
}

// KeySet returns the public keys for in-process verification.
func (d *DemoIssuer) KeySet() oidc.KeySet {
	return &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&d.key.PublicKey}}
}

// JWKS returns the public key set to publish at /demo/jwks.json.
func (d *DemoIssuer) JWKS() jose.JSONWebKeySet {
	return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &d.key.PublicKey, KeyID: d.kid, Algorithm: string(jose.RS256), Use: "sig",
	}}}
}

// Mint signs a token for the demo identity (iss=eyho-demo, sub=key).
func (d *DemoIssuer) Mint(key, email string, now time.Time) (string, time.Time, error) {
	exp := now.Add(DemoTokenLifetime)
	claims := map[string]any{
		ClaimEmail:         email,
		ClaimEmailVerified: true,
	}
	std := jwt.Claims{
		Issuer:   DemoIssuerName,
		Subject:  key,
		Audience: jwt.Audience{d.audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(exp),
	}
	tok, err := jwt.Signed(d.signer).Claims(std).Claims(claims).Serialize()
	return tok, exp, err
}

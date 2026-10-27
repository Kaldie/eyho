package auth

import (
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

// mintAs signs a token with the demo issuer's key but a chosen `iss`.
func mintAs(t *testing.T, d *DemoIssuer, iss string) string {
	t.Helper()
	now := time.Now()
	tok, err := jwt.Signed(d.signer).Claims(jwt.Claims{
		Issuer: iss, Subject: "someone", Audience: jwt.Audience{d.audience},
		IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(time.Hour)),
	}).Claims(map[string]any{ClaimEmail: "s@x.nl", ClaimEmailVerified: true}).Serialize()
	require.NoError(t, err)
	return tok
}

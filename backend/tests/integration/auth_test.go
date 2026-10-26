package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

// SC-003: protected routes answer 401 for missing, wrong-audience, expired and untrusted tokens.
func TestAuth_ProtectedRouteRejectsBadTokens(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, sub := testutil.Unique("auth")

	tokens := map[string]string{
		"none":           "",
		"garbage":        "not-a-jwt",
		"wrong audience": e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true, Audience: "someone-else"}),
		"expired":        e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true, ExpiresIn: -time.Hour}),
		"untrusted iss":  e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true, Issuer: "https://evil.example.com/"}),
	}
	for name, tok := range tokens {
		for _, path := range []string{"/api/v1/claims", "/api/v1/me"} {
			rec := e.Do(http.MethodGet, path, tok, nil)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s", name, path)
			assert.Equal(t, "unauthenticated", testutil.JSON(t, rec)["error"], "%s %s", name, path)
		}
	}
}

func TestAuth_ValidTokenPassesAndProvisionsAccount(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, sub := testutil.Unique("auth")
	tok := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true, Name: "Sanne"})

	// The claims stub answers 501: authentication passed and the request reached the handler.
	rec := e.Do(http.MethodGet, "/api/v1/claims", tok, nil)
	assert.Equal(t, http.StatusNotImplemented, rec.Code)

	rec = e.Do(http.MethodGet, "/api/v1/me", tok, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	me := testutil.JSON(t, rec)
	assert.Equal(t, email, me["email"])
	assert.Equal(t, true, me["email_verified"])
	assert.Equal(t, "Sanne", me["display_name"])
	assert.Equal(t, false, me["is_demo"])
	assert.Nil(t, me["cook_profile"])
}

// FR-003: browsing needs no account, and a stale token must not break it.
func TestAuth_PublicRoutesAreOpen(t *testing.T) {
	e := testutil.NewEnv(t, false)
	for _, tok := range []string{"", "stale.token.value"} {
		rec := e.Do(http.MethodGet, "/api/v1/listings", tok, nil)
		assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
		assert.NotEqual(t, http.StatusForbidden, rec.Code)
	}
	// Creating a listing needs sign-in (FR-004).
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodPost, "/api/v1/listings", "", `{}`).Code)
}

func TestAuth_TokenWithoutEmailIsRejected(t *testing.T) {
	e := testutil.NewEnv(t, false)
	_, sub := testutil.Unique("noemail")
	rec := e.Do(http.MethodGet, "/api/v1/me", e.Issuer.Token(testutil.TokenOptions{Subject: sub}), nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_PatchMeUpdatesDisplayName(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, sub := testutil.Unique("patch")
	tok := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true})
	rec := e.Do(http.MethodPatch, "/api/v1/me", tok, map[string]string{"display_name": "  Nieuwe Naam "})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Nieuwe Naam", testutil.JSON(t, rec)["display_name"])
	rec = e.Do(http.MethodPatch, "/api/v1/me", tok, map[string]string{"display_name": " "})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

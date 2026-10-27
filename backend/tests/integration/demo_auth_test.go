package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

// T035 / Scenario 7 step 3: without demo auth the /demo paths are not routed at all.
func TestDemo_NotRoutedWhenDisabled(t *testing.T) {
	e := testutil.NewEnv(t, false)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/demo/accounts"},
		{http.MethodPost, "/api/v1/demo/sign-in"},
		{http.MethodGet, "/api/v1/demo/jwks.json"},
		{http.MethodGet, "/api/v1/demo/.well-known/openid-configuration"},
	} {
		rec := e.Do(c.method, c.path, "", `{"key":"buyer"}`)
		assert.Equal(t, http.StatusNotFound, rec.Code, c.path)
	}
}

// Scenario 7 step 1: one-tap demo sign-in works against the seeded accounts.
func TestDemo_OneTapSignIn(t *testing.T) {
	e := testutil.NewEnv(t, true)

	rec := e.Do(http.MethodGet, "/api/v1/demo/accounts", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var keys []string
	for _, a := range testutil.JSON(t, rec)["accounts"].([]any) {
		keys = append(keys, a.(map[string]any)["key"].(string))
	}
	assert.ElementsMatch(t, []string{"buyer", "cook_verified", "cook_unverified"}, keys)

	for key, wantCook := range map[string]string{"buyer": "", "cook_verified": "verified", "cook_unverified": "unverified"} {
		rec = e.Do(http.MethodPost, "/api/v1/demo/sign-in", "", map[string]string{"key": key})
		require.Equal(t, http.StatusOK, rec.Code, key)
		body := testutil.JSON(t, rec)
		assert.NotEmpty(t, body["expires_at"])

		rec = e.Do(http.MethodGet, "/api/v1/me", body["access_token"].(string), nil)
		require.Equal(t, http.StatusOK, rec.Code, key)
		me := testutil.JSON(t, rec)
		assert.Equal(t, true, me["is_demo"], key)
		assert.Equal(t, true, me["email_verified"], key)
		if wantCook == "" {
			assert.Nil(t, me["cook_profile"], key)
			continue
		}
		cp := me["cook_profile"].(map[string]any)
		assert.Equal(t, wantCook, cp["verification_status"], key)
		assert.Equal(t, "active", cp["registration_status"], key)
		assert.Equal(t, true, cp["can_publish"], key)
	}

	// Unknown key.
	rec = e.Do(http.MethodPost, "/api/v1/demo/sign-in", "", map[string]string{"key": "admin"})
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Discovery and JWKS are published.
	assert.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/demo/jwks.json", "", nil).Code)
	assert.Equal(t, "eyho-demo", testutil.JSON(t, e.Do(http.MethodGet, "/api/v1/demo/.well-known/openid-configuration", "", nil))["issuer"])
}

// The demo issuer must never create accounts: a token for an unseeded subject is rejected.
func TestDemo_UnseededSubjectCannotSignIn(t *testing.T) {
	e := testutil.NewEnv(t, true)
	tok, _, err := e.Demo.Mint("intruder", "intruder@example.nl", timeNow())
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodGet, "/api/v1/me", tok, nil).Code)
}

// Demo accounts are shared by every visitor and must not be deletable.
func TestDemo_AccountCannotBeDeleted(t *testing.T) {
	e := testutil.NewEnv(t, true)
	rec := e.Do(http.MethodPost, "/api/v1/demo/sign-in", "", map[string]string{"key": "buyer"})
	tok := testutil.JSON(t, rec)["access_token"].(string)
	rec = e.Do(http.MethodDelete, "/api/v1/me", tok, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", tok, nil).Code)
}

func timeNow() time.Time { return time.Now() }

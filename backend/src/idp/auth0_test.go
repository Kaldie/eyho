package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuth0(t *testing.T, h http.HandlerFunc) (*Auth0, *int32) {
	var tokenCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			atomic.AddInt32(&tokenCalls, 1)
			w.Write([]byte(`{"access_token":"mgmt","expires_in":86400}`))
			return
		}
		assert.Equal(t, "Bearer mgmt", r.Header.Get("Authorization"))
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	a := NewAuth0(strings.TrimPrefix(srv.URL, "http://"), "id", "secret", srv.Client())
	a.scheme = "http"
	return a, &tokenCalls
}

func TestAuth0_VerificationEmailAndTokenCache(t *testing.T) {
	var got map[string]string
	a, tokens := newTestAuth0(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/jobs/verification-email", r.URL.Path)
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(201)
	})
	require.NoError(t, a.SendVerificationEmail(context.Background(), "auth0|abc"))
	require.NoError(t, a.SendVerificationEmail(context.Background(), "auth0|abc"))
	assert.Equal(t, "auth0|abc", got["user_id"])
	assert.EqualValues(t, 1, *tokens)
}

func TestAuth0_DeleteUser(t *testing.T) {
	status := 204
	a, _ := newTestAuth0(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v2/users/auth0%7Cabc", r.URL.EscapedPath())
		w.WriteHeader(status)
	})
	require.NoError(t, a.DeleteUser(context.Background(), "auth0|abc"))
	status = 404
	require.NoError(t, a.DeleteUser(context.Background(), "auth0|abc"))
	status = 500
	require.Error(t, a.DeleteUser(context.Background(), "auth0|abc"))
}

func TestAuth0_Manages(t *testing.T) {
	a := NewAuth0("eyho.eu.auth0.com", "i", "s", nil)
	assert.True(t, a.Manages("https://eyho.eu.auth0.com/"))
	assert.False(t, a.Manages("eyho-demo"))
	assert.False(t, a.Manages("http://localhost:8081/eyho"))
}

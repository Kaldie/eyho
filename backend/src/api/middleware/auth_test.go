package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/auth"
	"github.com/eyho/eyho/src/services"
)

type stubVerifier struct{ err error }

func (s stubVerifier) Verify(context.Context, string) (*auth.Claims, error) {
	return nil, s.err
}

type neverResolver struct{ t *testing.T }

func (r neverResolver) ResolveAccount(context.Context, auth.Claims) (*services.Account, error) {
	r.t.Fatal("must not resolve an account without verified claims")
	return nil, nil
}

// S3: an unreachable identity provider must not take down public routes; routes that need an
// account answer 503 identity_provider_unavailable (not 401, which would sign users out).
func TestAuthenticate_IdPUnavailable(t *testing.T) {
	m := NewAuth(stubVerifier{err: &auth.UnavailableError{Err: assert.AnError}}, neverResolver{t})
	public := m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, signedIn := AccountFromContext(r.Context())
		assert.False(t, signedIn)
		w.WriteHeader(http.StatusOK)
	}))
	protected := m.Authenticate(m.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer some.jwt.token")

	rec := httptest.NewRecorder()
	public.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	for name, mw := range map[string]func(http.Handler) http.Handler{
		"RequireAccount": m.RequireAccount, "RequireVerifiedEmail": m.RequireVerifiedEmail, "RequireCook": m.RequireCook,
	} {
		h := m.Authenticate(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })))
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, name)
		assert.JSONEq(t, `{"error":"identity_provider_unavailable"}`, rec.Body.String(), name)
	}
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// An invalid token (not an outage) still means 401 on protected routes and anonymous on public ones.
func TestAuthenticate_InvalidTokenIsUnauthenticated(t *testing.T) {
	m := NewAuth(stubVerifier{err: auth.ErrInvalidToken}, neverResolver{t})
	h := m.Authenticate(m.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("no") })))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

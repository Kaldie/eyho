// Package middleware holds the authentication middleware shared by all features (feature 001,
// T010). Handlers of other features depend only on these functions and AccountFromContext.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/eyho/eyho/src/auth"
	"github.com/eyho/eyho/src/services"
)

// TokenVerifier verifies a bearer token (implemented by *auth.Verifier).
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (*auth.Claims, error)
}

// AccountResolver maps verified claims to an account (implemented by *services.AccountService).
type AccountResolver interface {
	ResolveAccount(ctx context.Context, c auth.Claims) (*services.Account, error)
}

// Auth bundles the middleware functions.
type Auth struct {
	verifier TokenVerifier
	resolver AccountResolver
}

// NewAuth creates the middleware set.
func NewAuth(v TokenVerifier, r AccountResolver) *Auth { return &Auth{verifier: v, resolver: r} }

type ctxKey int

const (
	accountKey ctxKey = iota
	deletedKey
	idpUnavailableKey
)

// AccountFromContext returns the signed-in account set by Authenticate. The account's buyer and
// cook profile IDs come from the verified token, never from client input (FR-012).
func AccountFromContext(ctx context.Context) (*services.Account, bool) {
	a, ok := ctx.Value(accountKey).(*services.Account)
	return a, ok && a != nil
}

// WithAccount returns a context carrying the account (tests and internal callers).
func WithAccount(ctx context.Context, a *services.Account) context.Context {
	return context.WithValue(ctx, accountKey, a)
}

// Authenticate is optional authentication: with a valid bearer token it resolves the account
// (provisioning or linking on first sight) and stores it in the request context. Requests without
// a token, or with an invalid, expired or wrong-audience token, continue anonymously so public
// browsing keeps working (FR-003); the Require* middleware answer 401 where sign-in is needed.
// A token of a deleted account is remembered so RequireAccount can answer 403 account_deleted;
// likewise an unreachable identity provider (503 identity_provider_unavailable).
func (m *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := m.verifier.Verify(r.Context(), raw)
		if err != nil {
			var unavailable *auth.UnavailableError
			if errors.As(err, &unavailable) {
				// Public routes keep working while the provider is down (FR-003); the Require*
				// middleware answer 503 for routes that need a signed-in account.
				log.Printf("auth: %v", err)
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), idpUnavailableKey, true)))
				return
			}
			next.ServeHTTP(w, r) // anonymous
			return
		}
		acct, err := m.resolver.ResolveAccount(r.Context(), *claims)
		switch {
		case err == nil:
			next.ServeHTTP(w, r.WithContext(WithAccount(r.Context(), acct)))
		case errors.Is(err, services.ErrAccountDeleted):
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deletedKey, true)))
		case errors.Is(err, services.ErrUnknownIdentity), errors.Is(err, services.ErrEmailMissing):
			log.Printf("auth: rejecting token: %v", err)
			next.ServeHTTP(w, r)
		default:
			log.Printf("auth: resolving account failed: %v", err)
			WriteError(w, http.StatusInternalServerError, "internal_error")
		}
	})
}

// RequireAccount answers 401 unauthenticated without a valid signed-in account.
func (m *Auth) RequireAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := AccountFromContext(r.Context()); !ok {
			if down, _ := r.Context().Value(idpUnavailableKey).(bool); down {
				WriteError(w, http.StatusServiceUnavailable, "identity_provider_unavailable")
				return
			}
			if deleted, _ := r.Context().Value(deletedKey).(bool); deleted {
				WriteError(w, http.StatusForbidden, "account_deleted")
				return
			}
			WriteError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireVerifiedEmail answers 403 email_not_verified (after the 401 check) (FR-006).
func (m *Auth) RequireVerifiedEmail(next http.Handler) http.Handler {
	return m.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, _ := AccountFromContext(r.Context())
		if !a.EmailVerified {
			WriteError(w, http.StatusForbidden, "email_not_verified")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// RequireCook answers 403 cook_profile_required when the account has no cook profile yet.
func (m *Auth) RequireCook(next http.Handler) http.Handler {
	return m.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, _ := AccountFromContext(r.Context())
		if a.CookProfileID == nil {
			WriteError(w, http.StatusForbidden, "cook_profile_required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// WriteError writes {"error": code}.
func WriteError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": code})
}

// WriteCookNotRegistered writes the 403 cook_not_registered body (FR-020). Feature 002's listing
// service calls services.AccountService.CookPublishGate before publishing and uses this when the
// cook may not publish; registrationStatus is "pending" or "inactive".
func WriteCookNotRegistered(w http.ResponseWriter, registrationStatus string) {
	if registrationStatus != "inactive" {
		registrationStatus = "pending"
	}
	writeJSON(w, http.StatusForbidden, map[string]any{"error": "cook_not_registered", "registration_status": registrationStatus})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/eyho/eyho/src/auth"
	db "github.com/eyho/eyho/src/db/generated"
)

// DemoAuthHandler serves /demo/* (contracts/accounts-api.md). It is only constructed and routed
// when the start-up safeguard allows demo authentication (APP_ENV demo or local), so in any other
// environment these paths do not exist (404).
type DemoAuthHandler struct {
	issuer *auth.DemoIssuer
	q      db.Querier
	now    func() time.Time
}

// NewDemoAuthHandler creates the handler.
func NewDemoAuthHandler(issuer *auth.DemoIssuer, q db.Querier) *DemoAuthHandler {
	return &DemoAuthHandler{issuer: issuer, q: q, now: time.Now}
}

// ListAccounts implements GET /demo/accounts.
func (h *DemoAuthHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListDemoAccounts(r.Context())
	if err != nil {
		log.Printf("demo accounts: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	type item struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	out := make([]item, 0, len(rows))
	for _, a := range rows {
		if a.Issuer != auth.DemoIssuerName {
			continue
		}
		label := a.DemoKey
		if a.DisplayName != nil {
			label = *a.DisplayName
		}
		out = append(out, item{Key: a.DemoKey, Label: label})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

// SignIn implements POST /demo/sign-in: a short-lived token for a seeded demo account.
func (h *DemoAuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !decode(w, r, &body) {
		return
	}
	acc, err := h.q.GetAccountByIdentity(r.Context(), db.GetAccountByIdentityParams{Issuer: auth.DemoIssuerName, Subject: body.Key})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !acc.IsDemo) {
		writeError(w, http.StatusNotFound, "unknown_demo_account", "key")
		return
	}
	if err != nil {
		log.Printf("demo sign-in: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	email := ""
	if acc.Email != nil {
		email = *acc.Email
	}
	tok, exp, err := h.issuer.Mint(body.Key, email, h.now())
	if err != nil {
		log.Printf("demo sign-in: mint: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": tok, "expires_at": exp.UTC().Format(time.RFC3339)})
}

// Discovery implements GET /demo/.well-known/openid-configuration.
func (h *DemoAuthHandler) Discovery(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                auth.DemoIssuerName,
		"jwks_uri":                              scheme + "://" + r.Host + "/api/v1/demo/jwks.json",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"subject_types_supported":               []string{"public"},
		"response_types_supported":              []string{"token"},
	})
}

// JWKS implements GET /demo/jwks.json.
func (h *DemoAuthHandler) JWKS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.issuer.JWKS())
}

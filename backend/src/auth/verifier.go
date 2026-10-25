// Package auth verifies access tokens, hosts the demo issuer and enforces start-up safeguards
// (feature 001).
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Namespaced claims added to access tokens by the Auth0 post-login Action (research.md) and
// emitted by the mock server.
const (
	ClaimEmail         = "https://eyho.app/email"
	ClaimEmailVerified = "https://eyho.app/email_verified"
	ClaimName          = "https://eyho.app/name" // optional; the standard "name" claim is used as fallback
)

// ErrInvalidToken is returned for every token that is missing, malformed, expired, signed by an
// unknown key, issued by an untrusted issuer or meant for another audience.
var ErrInvalidToken = errors.New("invalid token")

// Claims is what the rest of the backend learns from a verified token.
type Claims struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	// IssuedAt is the token's iat (zero when absent). Account resolution compares it with account
	// deletion tombstones so a token issued before a deletion cannot re-create the account.
	IssuedAt time.Time
}

// Verifier verifies access tokens for a set of trusted issuers.
type Verifier struct {
	audience string
	mu       sync.RWMutex
	issuers  map[string]func(context.Context) (*oidc.IDTokenVerifier, error)
}

// NewVerifier creates a verifier that requires the given audience (the eyho API identifier).
func NewVerifier(audience string) *Verifier {
	return &Verifier{
		audience: audience,
		issuers:  map[string]func(context.Context) (*oidc.IDTokenVerifier, error){},
	}
}

func (v *Verifier) config() *oidc.Config {
	return &oidc.Config{ClientID: v.audience, SupportedSigningAlgs: []string{oidc.RS256}}
}

// AddRemoteIssuer trusts an issuer whose discovery document and JWKS are fetched over HTTP. The
// provider is created lazily on first use (and retried after failures) so the backend can start
// before the identity provider is reachable. Keys are cached by go-oidc.
//
// discoveryURL (optional) is a network address under which the issuer can be reached when the
// issuer's own host name does not resolve from here (Docker: the browser sees
// http://localhost:8081/eyho, the backend container reaches http://mock-oidc:8081/eyho). Requests
// still use the issuer URL and Host header, so the `iss` the provider reports matches; only the
// TCP connection is redirected to the discovery host.
func (v *Verifier) AddRemoteIssuer(issuer, discoveryURL string) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	if discoveryURL != "" && discoveryURL != issuer {
		if t, err := redirectedTransport(issuer, discoveryURL); err == nil {
			httpClient.Transport = t
		}
	}
	var (
		mu       sync.Mutex
		verifier *oidc.IDTokenVerifier
	)
	get := func(context.Context) (*oidc.IDTokenVerifier, error) {
		mu.Lock()
		defer mu.Unlock()
		if verifier != nil {
			return verifier, nil
		}
		// A long-lived context: go-oidc keeps using it for JWKS refreshes.
		ctx := oidc.ClientContext(context.Background(), httpClient)
		p, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, fmt.Errorf("oidc discovery for %s: %w", issuer, err)
		}
		verifier = p.Verifier(v.config())
		return verifier, nil
	}
	v.mu.Lock()
	v.issuers[issuer] = get
	v.mu.Unlock()
}

// redirectedTransport dials the host:port of discoveryURL whenever a request targets the
// host:port of issuer.
func redirectedTransport(issuer, discoveryURL string) (*http.Transport, error) {
	from, err := hostPort(issuer)
	if err != nil {
		return nil, err
	}
	to, err := hostPort(discoveryURL)
	if err != nil {
		return nil, err
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 5 * time.Second}
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == from {
			addr = to
		}
		return d.DialContext(ctx, network, addr)
	}
	return t, nil
}

func hostPort(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("bad url %q", raw)
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

// AddStaticIssuer trusts an issuer whose keys are known in-process (demo issuer, tests).
func (v *Verifier) AddStaticIssuer(issuer string, keys oidc.KeySet) {
	ver := oidc.NewVerifier(issuer, keys, v.config())
	v.mu.Lock()
	v.issuers[issuer] = func(context.Context) (*oidc.IDTokenVerifier, error) { return ver, nil }
	v.mu.Unlock()
}

// Verify checks the token's issuer, signature, audience and expiry and extracts the claims.
// Every rejection wraps ErrInvalidToken; an unreachable issuer returns a different error so
// callers can answer 5xx instead of 401 if they wish.
func (v *Verifier) Verify(ctx context.Context, raw string) (*Claims, error) {
	iss, err := unverifiedIssuer(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	v.mu.RLock()
	get, ok := v.issuers[iss]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: untrusted issuer", ErrInvalidToken)
	}
	ver, err := get(ctx)
	if err != nil {
		return nil, &UnavailableError{Err: err}
	}
	tok, err := ver.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	var body map[string]any
	if err := tok.Claims(&body); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	c := &Claims{Issuer: tok.Issuer, Subject: tok.Subject, IssuedAt: tok.IssuedAt}
	if s, ok := body[ClaimEmail].(string); ok {
		c.Email = strings.TrimSpace(s)
	}
	if b, ok := body[ClaimEmailVerified].(bool); ok {
		c.EmailVerified = b
	}
	for _, k := range []string{ClaimName, "name"} {
		if s, ok := body[k].(string); ok && strings.TrimSpace(s) != "" {
			c.Name = strings.TrimSpace(s)
			break
		}
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("%w: missing sub", ErrInvalidToken)
	}
	return c, nil
}

// UnavailableError means the issuer's keys could not be fetched (not the caller's fault).
type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return "token issuer unavailable: " + e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }

func unverifiedIssuer(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("bad payload")
	}
	var p struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(b, &p); err != nil || p.Iss == "" {
		return "", errors.New("no issuer")
	}
	return p.Iss, nil
}

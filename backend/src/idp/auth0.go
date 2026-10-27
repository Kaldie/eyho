package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Auth0 is a minimal Management API client (plain HTTP, cached machine-to-machine token).
type Auth0 struct {
	domain       string
	clientID     string
	clientSecret string
	client       *http.Client
	scheme       string // "https"; tests use http

	mu        sync.Mutex
	token     string
	expiresAt time.Time
	now       func() time.Time
}

// NewAuth0 creates a client for the tenant domain (for example "eyho.eu.auth0.com").
func NewAuth0(domain, clientID, clientSecret string, client *http.Client) *Auth0 {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Auth0{domain: domain, clientID: clientID, clientSecret: clientSecret, client: client, scheme: "https", now: time.Now}
}

func (a *Auth0) base() string { return a.scheme + "://" + a.domain }

// Manages is true for the tenant's own issuer.
func (a *Auth0) Manages(issuer string) bool {
	return strings.TrimRight(issuer, "/") == a.base()
}

func (a *Auth0) accessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && a.now().Before(a.expiresAt) {
		return a.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     a.clientID,
		"client_secret": a.clientSecret,
		"audience":      a.base() + "/api/v2/",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("auth0 token: status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("auth0 token: bad response")
	}
	a.token = out.AccessToken
	// Refresh a minute early.
	a.expiresAt = a.now().Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return a.token, nil
}

func (a *Auth0) do(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	tok, err := a.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var rdr io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base()+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return a.client.Do(req)
}

// SendVerificationEmail uses the verification-email job endpoint.
func (a *Auth0) SendVerificationEmail(ctx context.Context, subject string) error {
	resp, err := a.do(ctx, http.MethodPost, "/api/v2/jobs/verification-email", map[string]string{"user_id": subject})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("auth0 verification email: status %d", resp.StatusCode)
	}
	return nil
}

// DeleteUser deletes the user; 404 counts as success.
func (a *Auth0) DeleteUser(ctx context.Context, subject string) error {
	resp, err := a.do(ctx, http.MethodDelete, "/api/v2/users/"+url.PathEscape(subject), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("auth0 delete user: status %d", resp.StatusCode)
}

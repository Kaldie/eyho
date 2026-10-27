package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// MinKeyRefreshInterval is the shortest gap between two JWKS fetches for one issuer. A token
// signed with an unknown key id only triggers a fetch when the last one is at least this old, so
// forged tokens cannot make the backend call the identity provider on every request.
const MinKeyRefreshInterval = 30 * time.Second

// errUnknownKey is returned when no cached key verifies the token and a refresh is not allowed yet.
var errUnknownKey = errors.New("token signed by an unknown key")

// throttledKeySet is an oidc.KeySet backed by a remote JWKS. Unlike go-oidc's RemoteKeySet, which
// refetches the JWKS whenever a signature does not verify, it refetches at most once per
// MinKeyRefreshInterval. Tokens signed by a cached key never touch the network.
type throttledKeySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu          sync.RWMutex // guards keys
	keys        []jose.JSONWebKey
	refreshMu   sync.Mutex // serialises fetches; guards lastRefresh
	lastRefresh time.Time
}

func newThrottledKeySet(jwksURL string, client *http.Client) *throttledKeySet {
	return &throttledKeySet{url: jwksURL, client: client, now: time.Now}
}

// VerifySignature implements oidc.KeySet. go-oidc has already checked the algorithm.
func (k *throttledKeySet) VerifySignature(ctx context.Context, raw string) ([]byte, error) {
	jws, err := jose.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("malformed jwt: %w", err)
	}
	if payload, ok := k.verifyCached(jws); ok {
		return payload, nil
	}
	if err := k.refresh(ctx); err != nil {
		return nil, err
	}
	if payload, ok := k.verifyCached(jws); ok {
		return payload, nil
	}
	return nil, errUnknownKey
}

func (k *throttledKeySet) verifyCached(jws *jose.JSONWebSignature) ([]byte, bool) {
	kid := jws.Signatures[0].Header.KeyID
	k.mu.RLock()
	keys := k.keys
	k.mu.RUnlock()
	for _, key := range keys {
		if kid != "" && key.KeyID != kid {
			continue
		}
		if payload, err := jws.Verify(&key); err == nil {
			return payload, true
		}
	}
	return nil, false
}

// refresh fetches the JWKS unless a fetch happened within MinKeyRefreshInterval. Concurrent
// callers wait for one fetch instead of starting their own.
func (k *throttledKeySet) refresh(ctx context.Context) error {
	k.refreshMu.Lock()
	defer k.refreshMu.Unlock()
	now := k.now()
	if !k.lastRefresh.IsZero() && now.Sub(k.lastRefresh) < MinKeyRefreshInterval {
		return errUnknownKey
	}
	k.lastRefresh = now // a failed fetch also counts, so an outage is not hammered
	keys, err := k.fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetching keys: %w", err)
	}
	k.mu.Lock()
	k.keys = keys
	k.mu.Unlock()
	return nil
}

func (k *throttledKeySet) fetch(ctx context.Context) ([]jose.JSONWebKey, error) {
	// Not the request context: a cancelled request must not abort a fetch other callers wait on.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	return set.Keys, nil
}

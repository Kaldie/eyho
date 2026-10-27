// Package config loads the runtime configuration for the accounts and sign-in feature from the
// environment (feature 001, T002).
package config

import (
	"os"
	"strings"
	"time"
)

// Environment names. Anything else (including empty) is treated as production-like by the
// safeguard, so a missing APP_ENV can never switch on demo or mock behaviour.
const (
	EnvLocal      = "local"
	EnvDemo       = "demo"
	EnvProduction = "production"
)

// KVK modes (research.md, "Checking business registration").
const (
	KVKModeProduction = "kvk"
	KVKModeTest       = "kvk-test"
	KVKModeFake       = "fake"
)

// DefaultTombstoneRetention is longer than any access token the supported providers issue.
const DefaultTombstoneRetention = 24 * time.Hour

// Config holds the settings read from the environment.
type Config struct {
	AppEnv string // APP_ENV: local | demo | production (default production)

	OIDCIssuer       string // OIDC_ISSUER: the token issuer (Auth0 tenant, or the mock server locally)
	OIDCAudience     string // OIDC_AUDIENCE: the eyho API identifier
	OIDCDiscoveryURL string // OIDC_DISCOVERY_URL (optional): where to fetch discovery/JWKS when it differs from the issuer (Docker networking)

	DemoAuthEnabled bool // DEMO_AUTH_ENABLED

	Auth0MgmtDomain       string // AUTH0_MGMT_DOMAIN
	Auth0MgmtClientID     string // AUTH0_MGMT_CLIENT_ID
	Auth0MgmtClientSecret string // AUTH0_MGMT_CLIENT_SECRET

	// TombstoneRetention (TOMBSTONE_RETENTION, a Go duration, default 24h) is how long a deleted
	// identity is remembered. It is also the maximum token age: tokens issued longer ago are
	// refused whatever their expiry, so a deleted account can never be re-created by an old token.
	TombstoneRetention time.Duration

	// StripeWebhookSecret (STRIPE_WEBHOOK_SECRET) verifies payment webhook signatures.
	StripeWebhookSecret string

	KVKMode   string // KVK_MODE: kvk | kvk-test | fake (default kvk)
	KVKAPIKey string // KVK_API_KEY
}

// Load reads the configuration from environment variables.
func Load() Config {
	return Config{
		AppEnv:                strings.ToLower(strings.TrimSpace(get("APP_ENV", EnvProduction))),
		OIDCIssuer:            strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		OIDCAudience:          strings.TrimSpace(os.Getenv("OIDC_AUDIENCE")),
		OIDCDiscoveryURL:      strings.TrimSpace(os.Getenv("OIDC_DISCOVERY_URL")),
		DemoAuthEnabled:       parseBool(os.Getenv("DEMO_AUTH_ENABLED")),
		Auth0MgmtDomain:       strings.TrimSpace(os.Getenv("AUTH0_MGMT_DOMAIN")),
		Auth0MgmtClientID:     strings.TrimSpace(os.Getenv("AUTH0_MGMT_CLIENT_ID")),
		Auth0MgmtClientSecret: os.Getenv("AUTH0_MGMT_CLIENT_SECRET"),
		TombstoneRetention:    getDuration("TOMBSTONE_RETENTION", DefaultTombstoneRetention),
		StripeWebhookSecret:   strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		KVKMode:               strings.ToLower(strings.TrimSpace(get("KVK_MODE", KVKModeProduction))),
		KVKAPIKey:             os.Getenv("KVK_API_KEY"),
	}
}

// PlaceholderStripeWebhookSecret is the public default of docker-compose.yml. Anyone can sign
// webhooks with it, so it is only acceptable in demo and local.
const PlaceholderStripeWebhookSecret = "whsec_placeholder"

// DemoOrLocal reports whether APP_ENV allows demo-only behaviour.
func (c Config) DemoOrLocal() bool { return c.AppEnv == EnvDemo || c.AppEnv == EnvLocal }

func get(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func getDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil && d > 0 {
		return d
	}
	return def
}

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/config"
)

func prod() config.Config {
	return config.Config{
		AppEnv:          "production",
		OIDCIssuer:      "https://eyho.eu.auth0.com/",
		Auth0MgmtDomain: "eyho.eu.auth0.com",
		// Management API credentials: without them deletion would never reach the provider.
		Auth0MgmtClientID:     "mgmt-client",
		Auth0MgmtClientSecret: "mgmt-secret",
		KVKMode:               "kvk",
		StripeWebhookSecret:   "whsec_realsecret",
	}
}

func TestCheckStartup_RefusesMissingAuth0ManagementCredentialsOutsideDemoLocal(t *testing.T) {
	c := prod()
	c.Auth0MgmtClientID = ""
	err := CheckStartup(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH0_MGMT_CLIENT_ID")

	c = prod()
	c.Auth0MgmtClientSecret = ""
	err = CheckStartup(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH0_MGMT_CLIENT_SECRET")

	// Demo and local keep working without them.
	c = config.Config{AppEnv: "demo", KVKMode: "kvk-test"}
	require.NoError(t, CheckStartup(c))
}

func TestCheckStartup_ProductionOK(t *testing.T) {
	require.NoError(t, CheckStartup(prod()))
}

func TestCheckStartup_RefusesDemoAuthOutsideDemoLocal(t *testing.T) {
	c := prod()
	c.DemoAuthEnabled = true
	err := CheckStartup(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEMO_AUTH_ENABLED")
}

func TestCheckStartup_RefusesNonProductionKVK(t *testing.T) {
	for _, m := range []string{"fake", "kvk-test"} {
		c := prod()
		c.KVKMode = m
		err := CheckStartup(c)
		require.Error(t, err, m)
		assert.Contains(t, err.Error(), "KVK_MODE")
	}
}

func TestCheckStartup_RefusesMockIssuerInProduction(t *testing.T) {
	c := prod()
	c.OIDCIssuer = "http://mock-oidc:8080/eyho"
	err := CheckStartup(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OIDC_ISSUER")

	c = prod()
	c.Auth0MgmtDomain = ""
	require.Error(t, CheckStartup(c))

	c = prod()
	c.OIDCIssuer = "https://evil.example.com/"
	require.Error(t, CheckStartup(c))
}

func TestCheckStartup_UnknownOrEmptyEnvIsProductionLike(t *testing.T) {
	for _, env := range []string{"", "staging", "prod"} {
		c := prod()
		c.AppEnv = env
		c.DemoAuthEnabled = true
		require.Error(t, CheckStartup(c), env)
	}
}

func TestCheckStartup_DemoAndLocalAllowDemoThings(t *testing.T) {
	for _, env := range []string{"demo", "local"} {
		c := config.Config{AppEnv: env, OIDCIssuer: "http://localhost:8081/eyho", DemoAuthEnabled: true, KVKMode: "fake"}
		require.NoError(t, CheckStartup(c), env)
	}
}

func TestCheckStartup_RejectsUnknownKVKMode(t *testing.T) {
	c := config.Config{AppEnv: "local", KVKMode: "bogus"}
	require.Error(t, CheckStartup(c))
}

func TestCheckStartup_RefusesMissingOrPlaceholderStripeWebhookSecretOutsideDemoLocal(t *testing.T) {
	for _, secret := range []string{"", config.PlaceholderStripeWebhookSecret} {
		c := prod()
		c.StripeWebhookSecret = secret
		err := CheckStartup(c)
		require.Error(t, err, "secret %q", secret)
		assert.Contains(t, err.Error(), "STRIPE_WEBHOOK_SECRET")
	}
	// Unknown or empty APP_ENV is production-like.
	c := prod()
	c.AppEnv = ""
	c.StripeWebhookSecret = ""
	require.Error(t, CheckStartup(c))

	// Demo and local may run with the docker-compose placeholder or without a secret.
	for _, env := range []string{"demo", "local"} {
		for _, secret := range []string{"", config.PlaceholderStripeWebhookSecret} {
			c := config.Config{AppEnv: env, KVKMode: "fake", StripeWebhookSecret: secret}
			require.NoError(t, CheckStartup(c), env)
		}
	}
}

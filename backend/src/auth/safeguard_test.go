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
		KVKMode:         "kvk",
	}
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

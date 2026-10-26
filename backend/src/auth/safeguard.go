package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/eyho/eyho/src/config"
)

// CheckStartup refuses configurations that would expose demo or mock behaviour outside the
// demo and local environments (FR-017). It returns an error naming the offending setting; the
// caller must exit when the error is non-nil.
func CheckStartup(c config.Config) error {
	switch c.KVKMode {
	case config.KVKModeProduction, config.KVKModeTest, config.KVKModeFake:
	default:
		return fmt.Errorf("KVK_MODE=%q is not one of kvk, kvk-test, fake", c.KVKMode)
	}
	if c.DemoOrLocal() {
		return nil
	}
	if c.DemoAuthEnabled {
		return fmt.Errorf("DEMO_AUTH_ENABLED=true is not allowed when APP_ENV=%q (only demo or local)", c.AppEnv)
	}
	if c.KVKMode != config.KVKModeProduction {
		return fmt.Errorf("KVK_MODE=%q is not allowed when APP_ENV=%q (only demo or local); use kvk", c.KVKMode, c.AppEnv)
	}
	if c.Auth0MgmtClientID == "" {
		return fmt.Errorf("AUTH0_MGMT_CLIENT_ID is required when APP_ENV=%q (account deletion and verification emails need the Auth0 Management API)", c.AppEnv)
	}
	if c.Auth0MgmtClientSecret == "" {
		return fmt.Errorf("AUTH0_MGMT_CLIENT_SECRET is required when APP_ENV=%q (account deletion and verification emails need the Auth0 Management API)", c.AppEnv)
	}
	if !isConfiguredAuth0Issuer(c) {
		return fmt.Errorf("OIDC_ISSUER=%q is not the configured Auth0 tenant (https://<AUTH0_MGMT_DOMAIN>/) while APP_ENV=%q", c.OIDCIssuer, c.AppEnv)
	}
	return nil
}

// isConfiguredAuth0Issuer: the issuer must be https://<AUTH0_MGMT_DOMAIN>[/].
func isConfiguredAuth0Issuer(c config.Config) bool {
	if c.Auth0MgmtDomain == "" || c.OIDCIssuer == "" {
		return false
	}
	u, err := url.Parse(c.OIDCIssuer)
	if err != nil || u.Scheme != "https" {
		return false
	}
	if strings.Trim(u.Path, "/") != "" {
		return false
	}
	return strings.EqualFold(u.Host, c.Auth0MgmtDomain)
}

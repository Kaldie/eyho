package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/eyho/eyho/src/api"
	"github.com/eyho/eyho/src/api/handlers"
	authmw "github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/auth"
	"github.com/eyho/eyho/src/config"
	dbpool "github.com/eyho/eyho/src/db"
	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/idp"
	"github.com/eyho/eyho/src/registry"
	"github.com/eyho/eyho/src/services"
)

func main() {
	// Load environment variables from .env (if present). In production, env
	// vars are set by the platform and .env is absent — godotenv silently
	// continues if the file does not exist.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on environment variables")
	}

	cfg := config.Load()
	// Refuse to start with demo/mock settings outside demo and local (FR-017).
	if err := auth.CheckStartup(cfg); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	if cfg.OIDCAudience == "" {
		log.Fatal("required environment variable \"OIDC_AUDIENCE\" is not set")
	}
	if cfg.OIDCIssuer == "" && !cfg.DemoAuthEnabled {
		log.Fatal("required environment variable \"OIDC_ISSUER\" is not set")
	}

	dbURL := mustEnv("DATABASE_URL")
	stripeKey := mustEnv("STRIPE_SECRET_KEY")
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if stripeWebhookSecret == "" {
		log.Println("STRIPE_WEBHOOK_SECRET is not set: every payment webhook will be rejected")
	}
	payments := services.NewStripeProvider(stripeKey, stripeWebhookSecret)

	pool, err := dbpool.NewPool(context.Background(), dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// --- Feature 001: sign-in, accounts, registration check ---
	verifier := auth.NewVerifier(cfg.OIDCAudience)
	if cfg.OIDCIssuer != "" {
		verifier.AddRemoteIssuer(cfg.OIDCIssuer, cfg.OIDCDiscoveryURL)
	}
	queries := db.New(pool)
	var demoHandler *handlers.DemoAuthHandler
	if cfg.DemoAuthEnabled { // CheckStartup already ensured APP_ENV is demo or local
		demoIssuer, err := auth.NewDemoIssuer(cfg.OIDCAudience)
		if err != nil {
			log.Fatalf("demo issuer: %v", err)
		}
		verifier.AddStaticIssuer(auth.DemoIssuerName, demoIssuer.KeySet())
		demoHandler = handlers.NewDemoAuthHandler(demoIssuer, queries)
		log.Printf("demo sign-in enabled (APP_ENV=%s)", cfg.AppEnv)
	}

	var register registry.BusinessRegister
	switch cfg.KVKMode {
	case config.KVKModeFake:
		register = registry.NewFake()
		log.Printf("using the FAKE business register (test KvK numbers only)")
	case config.KVKModeTest:
		if cfg.KVKAPIKey == "" {
			log.Fatal("KVK_API_KEY is required when KVK_MODE=kvk-test")
		}
		register = registry.NewKVK(registry.KVKTestBaseURL, cfg.KVKAPIKey, nil)
	default:
		if cfg.KVKAPIKey == "" {
			log.Fatal("KVK_API_KEY is required when KVK_MODE=kvk")
		}
		register = registry.NewKVK(registry.KVKProductionBaseURL, cfg.KVKAPIKey, nil)
	}

	var provider idp.Provider = idp.Noop{}
	if cfg.Auth0MgmtDomain != "" && cfg.Auth0MgmtClientID != "" {
		provider = idp.NewAuth0(cfg.Auth0MgmtDomain, cfg.Auth0MgmtClientID, cfg.Auth0MgmtClientSecret, nil)
	}

	accountSvc := services.NewAccountService(pool, register, provider)

	bg, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	go (&registry.RecheckJob{Q: queries, Register: register}).Run(bg)
	go (&services.IdpDeletionWorker{Q: queries, Provider: provider, TombstoneRetention: cfg.TombstoneRetention}).Run(bg)

	// --- Feature 002: listings, claims, payments, background jobs ---
	listingSvc := services.NewListingService(pool, accountSvc, payments)
	claimSvc := services.NewClaimService(pool, accountSvc, payments)

	jobs := services.NewJobRunner()
	jobs.Register("listing-expiry", listingSvc.ExpiryJob)
	jobs.Register("payment-timeout", claimSvc.PaymentTimeoutJob)
	go jobs.Run(bg)

	r := api.NewRouter(listingSvc, claimSvc, payments, api.AuthDeps{
		Middleware: authmw.NewAuth(verifier, accountSvc),
		Me:         handlers.NewMeHandler(accountSvc),
		Demo:       demoHandler,
	})

	port := getEnv("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)
	log.Printf("eyho backend listening on %s", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// mustEnv returns the value of the named environment variable or exits.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

// getEnv returns the value of the named environment variable or a default.
func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

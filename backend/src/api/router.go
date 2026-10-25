package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/eyho/eyho/src/api/handlers"
	authmw "github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/services"
)

// AuthDeps carries the feature 001 dependencies of the router.
type AuthDeps struct {
	// Middleware authenticates /api/v1 requests (required).
	Middleware *authmw.Auth
	// Me serves /me endpoints (required).
	Me *handlers.MeHandler
	// Demo serves /demo/* and is nil unless the start-up safeguard allowed demo auth; when nil the
	// /demo paths are not routed at all (404, FR-017).
	Demo *handlers.DemoAuthHandler
}

// NewRouter wires all routes defined in the API contracts and returns the
// configured chi router. Route paths and HTTP methods match exactly those
// specified in:
//   - specs/002-surplus-food-listing/contracts/listings-api.md
//   - specs/002-surplus-food-listing/contracts/claims-api.md
//
// Handlers are intentionally thin; business logic lives in the service layer.
func NewRouter(listingSvc *services.ListingService, claimSvc *services.ClaimService, payments services.PaymentProvider, deps AuthDeps) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware stack.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	listingH := handlers.NewListingHandler(listingSvc)
	claimH := handlers.NewClaimHandler(claimSvc)
	webhookH := handlers.NewWebhookHandler(payments, claimSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Optional authentication on every /api/v1 route; endpoints that need sign-in add
		// RequireAccount (401), RequireVerifiedEmail (403) or RequireCook (403).
		r.Use(deps.Middleware.Authenticate)

		// --- Accounts (accounts-api.md) ---
		r.Route("/me", func(r chi.Router) {
			r.With(deps.Middleware.RequireAccount).Get("/", deps.Me.GetMe)
			r.With(deps.Middleware.RequireAccount).Patch("/", deps.Me.PatchMe)
			r.With(deps.Middleware.RequireAccount).Delete("/", deps.Me.DeleteMe)
			r.With(deps.Middleware.RequireAccount).Post("/verification-email", deps.Me.PostVerificationEmail)
			r.With(deps.Middleware.RequireVerifiedEmail).Post("/cook-profile", deps.Me.PostCookProfile)
			r.With(deps.Middleware.RequireCook).Patch("/cook-profile", deps.Me.PatchCookProfile)
			// Cook dashboard: the cook's own listings in every status (listings-api.md).
			r.With(deps.Middleware.RequireCook).Get("/listings", listingH.ListMine)
		})

		// --- Demo sign-in: only routed when allowed by the safeguard (FR-016, FR-017) ---
		if deps.Demo != nil {
			r.Route("/demo", func(r chi.Router) {
				r.Get("/accounts", deps.Demo.ListAccounts)
				r.Post("/sign-in", deps.Demo.SignIn)
				r.Get("/.well-known/openid-configuration", deps.Demo.Discovery)
				r.Get("/jwks.json", deps.Demo.JWKS)
			})
		}

		// --- Listings (listings-api.md) ---
		r.Route("/listings", func(r chi.Router) {
			// Create: signed in with a verified email and a cook profile. The CookCanPublish gate
			// (403 cook_not_registered) lives in the service because drafts stay allowed.
			r.With(deps.Middleware.RequireVerifiedEmail, deps.Middleware.RequireCook).Post("/", listingH.CreateListing)
			// Discovery is public.
			r.Get("/", listingH.ListListings)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", listingH.GetListing)
				// Owning cook only (ownership checked in the service).
				r.With(deps.Middleware.RequireVerifiedEmail, deps.Middleware.RequireCook).Patch("/", listingH.UpdateListing)
			})
		})

		// --- Claims (claims-api.md) ---
		r.Route("/claims", func(r chi.Router) {
			r.With(deps.Middleware.RequireVerifiedEmail).Post("/", claimH.CreateClaim)
			r.With(deps.Middleware.RequireAccount).Get("/", claimH.ListClaims)

			r.Route("/{id}", func(r chi.Router) {
				r.Use(deps.Middleware.RequireAccount)
				r.Get("/", claimH.GetClaim)
				r.Post("/confirm-pickup", claimH.ConfirmPickup)
				r.Post("/rating", claimH.RateClaim)
			})
		})

		// --- Stripe webhook (claims-api.md §Stripe webhook) ---
		r.Post("/webhooks/stripe", webhookH.StripeWebhook)
	})

	return r
}

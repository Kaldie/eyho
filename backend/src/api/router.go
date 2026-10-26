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
func NewRouter(listingSvc services.ListingService, claimSvc services.ClaimService, deps AuthDeps) *chi.Mux {
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
	webhookH := handlers.NewWebhookHandler(claimSvc)

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
			// POST /api/v1/listings — create a new listing (draft or published). Requires
			// sign-in (FR-004); feature 002 adds RequireVerifiedEmail, RequireCook and the
			// CookCanPublish gate.
			r.With(deps.Middleware.RequireAccount).Post("/", listingH.CreateListing)
			// GET /api/v1/listings — discover nearby published listings.
			r.Get("/", listingH.ListListings)

			r.Route("/{id}", func(r chi.Router) {
				// GET /api/v1/listings/{id} — full listing detail.
				r.Get("/", listingH.GetListing)
				// PATCH /api/v1/listings/{id} — cook adjusts portions, window, or cancels.
				r.With(deps.Middleware.RequireAccount).Patch("/", listingH.UpdateListing)
			})
		})

		// --- Claims (claims-api.md) ---
		r.Route("/claims", func(r chi.Router) {
			// Claiming, rating and order history require sign-in (FR-004). Feature 002 adds
			// RequireVerifiedEmail for claim creation.
			r.Use(deps.Middleware.RequireAccount)
			// POST /api/v1/claims — atomic claim + Stripe PaymentIntent creation.
			r.Post("/", claimH.CreateClaim)
			// GET /api/v1/claims — buyer's own claims (order history, feature 003).
			r.Get("/", claimH.ListClaims)

			r.Route("/{id}", func(r chi.Router) {
				// GET /api/v1/claims/{id} — claim detail for buyer/cook.
				r.Get("/", claimH.GetClaim)
				// PATCH /api/v1/claims/{id} — general claim update (future use).
				r.Patch("/", claimH.UpdateClaim)
				// POST /api/v1/claims/{id}/confirm-pickup — buyer confirms collection.
				r.Post("/confirm-pickup", claimH.UpdateClaim)
				// POST /api/v1/claims/{id}/rating — buyer rates after collection (FR-013).
				r.Post("/rating", claimH.RateClaim)
			})
		})

		// --- Stripe webhook (claims-api.md §Stripe webhook) ---
		r.Post("/webhooks/stripe", webhookH.StripeWebhook)
	})

	return r
}

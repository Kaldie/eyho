package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/eyho/eyho/src/api/handlers"
	"github.com/eyho/eyho/src/services"
)

// NewRouter wires all routes defined in the API contracts and returns the
// configured chi router. Route paths and HTTP methods match exactly those
// specified in:
//   - specs/001-surplus-food-listing/contracts/listings-api.md
//   - specs/001-surplus-food-listing/contracts/claims-api.md
//
// Handlers are intentionally thin; business logic lives in the service layer.
func NewRouter(listingSvc services.ListingService, claimSvc services.ClaimService) *chi.Mux {
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
		// --- Listings (listings-api.md) ---
		r.Route("/listings", func(r chi.Router) {
			// POST /api/v1/listings — create a new listing (draft or published).
			r.Post("/", listingH.CreateListing)
			// GET /api/v1/listings — discover nearby published listings.
			r.Get("/", listingH.ListListings)

			r.Route("/{id}", func(r chi.Router) {
				// GET /api/v1/listings/{id} — full listing detail.
				r.Get("/", listingH.GetListing)
				// PATCH /api/v1/listings/{id} — cook adjusts portions, window, or cancels.
				r.Patch("/", listingH.UpdateListing)
			})
		})

		// --- Claims (claims-api.md) ---
		r.Route("/claims", func(r chi.Router) {
			// POST /api/v1/claims — atomic claim + Stripe PaymentIntent creation.
			r.Post("/", claimH.CreateClaim)
			// GET /api/v1/claims — buyer's own claims (order history, feature 002).
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

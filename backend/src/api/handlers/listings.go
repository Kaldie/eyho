package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/services"
)

// ListingHandler maps listing requests and responses; rules live in services.ListingService.
type ListingHandler struct {
	svc *services.ListingService
}

// NewListingHandler constructs a ListingHandler.
func NewListingHandler(svc *services.ListingService) *ListingHandler {
	return &ListingHandler{svc: svc}
}

type createListingBody struct {
	FoodDescription           *string            `json:"food_description"`
	AllergenInfo              *string            `json:"allergen_info"`
	PhotoURL                  *string            `json:"photo_url"`
	TotalPortions             *int               `json:"total_portions"`
	ReducedPriceCents         *int               `json:"reduced_price_cents"`
	RetailReferencePriceCents *int               `json:"retail_reference_price_cents"`
	PickupLocation            *services.Location `json:"pickup_location"`
	PickupAddressText         *string            `json:"pickup_address_text"`
	PickupWindowStart         *string            `json:"pickup_window_start"`
	PickupWindowEnd           *string            `json:"pickup_window_end"`
	Publish                   bool               `json:"publish"`
}

// parseTime parses an RFC3339 field; ok=false when it was already answered with 422.
func parseTime(w http.ResponseWriter, v *string, field string) (time.Time, bool) {
	if v == nil {
		writeError(w, http.StatusUnprocessableEntity, "missing_field", field)
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_field", field)
		return time.Time{}, false
	}
	return t.UTC(), true
}

// CreateListing handles POST /api/v1/listings (contracts/listings-api.md). Routed behind
// RequireVerifiedEmail and RequireCook; the cook comes from the token's account. Any registered
// cook may publish, verified or not (Principle I); only an unconfirmed KvK registration yields
// 403 cook_not_registered.
func (h *ListingHandler) CreateListing(w http.ResponseWriter, r *http.Request) {
	acct, _ := middleware.AccountFromContext(r.Context())
	var b createListingBody
	if !decode(w, r, &b) {
		return
	}
	in := services.CreateListingInput{
		AllergenInfo: b.AllergenInfo, PhotoURL: b.PhotoURL, PickupLocation: b.PickupLocation,
		PickupAddressText: b.PickupAddressText, Publish: b.Publish,
	}
	required := []struct {
		v     *int
		dst   *int
		field string
	}{
		{b.TotalPortions, &in.TotalPortions, "total_portions"},
		{b.ReducedPriceCents, &in.ReducedPriceCents, "reduced_price_cents"},
		{b.RetailReferencePriceCents, &in.RetailReferencePriceCents, "retail_reference_price_cents"},
	}
	if b.FoodDescription == nil {
		writeError(w, http.StatusUnprocessableEntity, "missing_field", "food_description")
		return
	}
	in.FoodDescription = *b.FoodDescription
	for _, f := range required {
		if f.v == nil {
			writeError(w, http.StatusUnprocessableEntity, "missing_field", f.field)
			return
		}
		*f.dst = *f.v
	}
	var ok bool
	if in.PickupWindowStart, ok = parseTime(w, b.PickupWindowStart, "pickup_window_start"); !ok {
		return
	}
	if in.PickupWindowEnd, ok = parseTime(w, b.PickupWindowEnd, "pickup_window_end"); !ok {
		return
	}
	d, err := h.svc.Create(r.Context(), acct, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// ListListings handles GET /api/v1/listings: public; lat and lng required; radius_km optional
// (default: the signed-in buyer's radius, else 2 km). status, when given, must be "published":
// other statuses are never public.
func (h *ListingHandler) ListListings(w http.ResponseWriter, r *http.Request) {
	qv := r.URL.Query()
	num := func(name string) (float64, bool) {
		s := qv.Get(name)
		if s == "" {
			writeError(w, http.StatusUnprocessableEntity, "missing_field", name)
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_field", name)
			return 0, false
		}
		return f, true
	}
	lat, ok := num("lat")
	if !ok {
		return
	}
	lng, ok := num("lng")
	if !ok {
		return
	}
	q := services.ListQuery{Lat: lat, Lng: lng}
	if s := qv.Get("radius_km"); s != "" {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_field", "radius_km")
			return
		}
		q.RadiusKm = &f
	}
	if s := qv.Get("status"); s != "" && s != "published" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_field", "status")
		return
	}
	if acct, ok := middleware.AccountFromContext(r.Context()); ok {
		q.Account = acct
	}
	items, err := h.svc.List(r.Context(), q)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": items})
}

// GetListing handles GET /api/v1/listings/{id}: public for published listings, 404 otherwise
// unless the requester is the owning cook.
func (h *ListingHandler) GetListing(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "listing_not_found", "")
		return
	}
	acct, _ := middleware.AccountFromContext(r.Context())
	// lat and lng together add distance_km to the response; one without the other is an error.
	var from *services.Location
	qv := r.URL.Query()
	if qv.Get("lat") != "" || qv.Get("lng") != "" {
		lat, err1 := strconv.ParseFloat(qv.Get("lat"), 64)
		lng, err2 := strconv.ParseFloat(qv.Get("lng"), 64)
		if err1 != nil || err2 != nil {
			field := "lat"
			if err1 == nil {
				field = "lng"
			}
			writeError(w, http.StatusUnprocessableEntity, "invalid_field", field)
			return
		}
		from = &services.Location{Lat: lat, Lng: lng}
	}
	d, err := h.svc.Get(r.Context(), acct, id, from)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// UpdateListing handles PATCH /api/v1/listings/{id}: owning cook only; portions_remaining,
// pickup_window_end (extend) and status "cancelled". Cancelling refunds paid claims (FR-010).
func (h *ListingHandler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "listing_not_found", "")
		return
	}
	acct, _ := middleware.AccountFromContext(r.Context())
	var b struct {
		PortionsRemaining *int    `json:"portions_remaining"`
		PickupWindowEnd   *string `json:"pickup_window_end"`
		Status            *string `json:"status"`
	}
	if !decode(w, r, &b) {
		return
	}
	in := services.UpdateListingInput{PortionsRemaining: b.PortionsRemaining, Status: b.Status}
	if b.PickupWindowEnd != nil {
		t, ok := parseTime(w, b.PickupWindowEnd, "pickup_window_end")
		if !ok {
			return
		}
		in.PickupWindowEnd = &t
	}
	d, err := h.svc.Update(r.Context(), acct, id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ListMine handles GET /api/v1/me/listings (behind RequireCook): the calling cook's listings in
// every status, newest first, with portions_claimed.
func (h *ListingHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	acct, _ := middleware.AccountFromContext(r.Context())
	items, err := h.svc.ListOwn(r.Context(), acct)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"listings": items})
}

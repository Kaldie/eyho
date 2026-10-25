package services

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/eyho/eyho/src/db/generated"
)

// DefaultDiscoveryRadiusKm applies to signed-out visitors (FR-011).
const DefaultDiscoveryRadiusKm = 2.0

// MaxDiscoveryRadiusKm bounds an explicit radius_km.
const MaxDiscoveryRadiusKm = 100.0

// CookGate is the part of feature 001's AccountService the listing and claim services use.
type CookGate interface {
	CookPublishGate(ctx context.Context, cookProfileID uuid.UUID) (bool, string, error)
	AccountOwnsCookProfile(ctx context.Context, accountID, cookProfileID uuid.UUID) (bool, error)
}

// CookBlock is the cook part of every buyer-facing listing response (FR-004, Principle I): the
// current verification status (clients render the "unverified" flag), rating and review count.
type CookBlock struct {
	DisplayName            string  `json:"display_name"`
	OrgType                string  `json:"org_type"`
	VerificationStatus     string  `json:"verification_status"`
	RatingAvg              float64 `json:"rating_avg"`
	RatingCount            int     `json:"rating_count"`
	VerificationCycleCount int     `json:"verification_cycle_count"`
}

// ListingItem is one entry of GET /listings (contract shape).
type ListingItem struct {
	ID                uuid.UUID `json:"id"`
	FoodDescription   string    `json:"food_description"`
	PhotoURL          *string   `json:"photo_url"`
	ReducedPriceCents int       `json:"reduced_price_cents"`
	PortionsRemaining int       `json:"portions_remaining"`
	PickupWindowStart time.Time `json:"pickup_window_start"`
	PickupWindowEnd   time.Time `json:"pickup_window_end"`
	DistanceKm        *float64  `json:"distance_km,omitempty"`
	Cook              CookBlock `json:"cook"`
}

// ListingDetail is GET /listings/{id}, POST /listings and PATCH /listings/{id}: the list item
// plus allergen_info, pickup_address_text and status. total_portions, retail_reference_price_cents
// and pickup_location are additive fields for the cook dashboard and map.
type ListingDetail struct {
	ListingItem
	AllergenInfo              string   `json:"allergen_info"`
	PickupAddressText         string   `json:"pickup_address_text"`
	Status                    string   `json:"status"`
	TotalPortions             int      `json:"total_portions"`
	RetailReferencePriceCents int      `json:"retail_reference_price_cents"`
	PickupLocation            Location `json:"pickup_location"`
}

// CreateListingInput is the parsed body of POST /listings. The cook comes from the account.
type CreateListingInput struct {
	FoodDescription           string
	AllergenInfo              *string // required; "" means none
	PhotoURL                  *string
	TotalPortions             int
	ReducedPriceCents         int
	RetailReferencePriceCents int
	PickupLocation            *Location // default: cook's default pickup location
	PickupAddressText         *string   // default: cook's default pickup address
	PickupWindowStart         time.Time
	PickupWindowEnd           time.Time
	Publish                   bool
}

// UpdateListingInput is the parsed body of PATCH /listings/{id}.
type UpdateListingInput struct {
	PortionsRemaining *int
	PickupWindowEnd   *time.Time
	Status            *string // only "cancelled"
}

// ListQuery is GET /listings. Account (optional) supplies the buyer's default radius.
type ListQuery struct {
	Lat, Lng float64
	RadiusKm *float64
	Account  *Account
}

// ListingService holds the listing rules: creation and publish gate (FR-001/FR-003), discovery
// (FR-011), cook edits and cancel-with-refund (FR-010), expiry (FR-007).
type ListingService struct {
	q        db.Querier
	tx       TxFunc
	gate     CookGate
	payments PaymentProvider
	now      func() time.Time
}

// NewListingService wires the service to a pool.
func NewListingService(pool *pgxpool.Pool, gate CookGate, payments PaymentProvider) *ListingService {
	return newListingService(db.New(pool), PoolTx(pool), gate, payments)
}

func newListingService(q db.Querier, tx TxFunc, gate CookGate, payments PaymentProvider) *ListingService {
	return &ListingService{q: q, tx: tx, gate: gate, payments: payments, now: time.Now}
}

// Create validates and stores a listing for the signed-in cook. Publishing is gated ONLY by
// feature 001's registration check (403 cook_not_registered); the cook's verification status never
// blocks it, it only flags the listing "unverified" in responses (Principle I). Drafts are always
// allowed.
func (s *ListingService) Create(ctx context.Context, acct *Account, in CreateListingInput) (*ListingDetail, error) {
	if acct.CookProfileID == nil {
		return nil, ErrCookProfileMissing
	}
	cookID := *acct.CookProfileID
	if err := s.validateCreate(in); err != nil {
		return nil, err
	}

	loc, addr := in.PickupLocation, ""
	if in.PickupAddressText != nil {
		addr = strings.TrimSpace(*in.PickupAddressText)
		if addr == "" || utf8.RuneCountInString(addr) > 300 {
			return nil, &ValidationError{Code: "invalid_field", Field: "pickup_address_text"}
		}
	}
	if loc == nil || in.PickupAddressText == nil {
		cp, err := s.q.GetCookProfileByAccount(ctx, pgUUID(acct.ID))
		if err != nil {
			return nil, err
		}
		if loc == nil {
			loc = &Location{Lat: cp.DefaultPickupLat, Lng: cp.DefaultPickupLng}
		}
		if in.PickupAddressText == nil {
			if cp.DefaultPickupAddressText == nil {
				return nil, &ValidationError{Code: "missing_field", Field: "pickup_address_text"}
			}
			addr = *cp.DefaultPickupAddressText
		}
	}

	status := db.ListingStatusDraft
	if in.Publish {
		ok, regStatus, err := s.gate.CookPublishGate(ctx, cookID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, &CookNotRegisteredError{RegistrationStatus: regStatus}
		}
		status = db.ListingStatusPublished
	}

	row, err := s.q.CreateListing(ctx, db.CreateListingParams{
		CookID:                    pgUUID(cookID),
		FoodDescription:           strings.TrimSpace(in.FoodDescription),
		AllergenInfo:              strings.TrimSpace(*in.AllergenInfo),
		PhotoUrl:                  in.PhotoURL,
		TotalPortions:             int32(in.TotalPortions),
		ReducedPriceCents:         int32(in.ReducedPriceCents),
		RetailReferencePriceCents: int32(in.RetailReferencePriceCents),
		Lng:                       loc.Lng,
		Lat:                       loc.Lat,
		PickupAddressText:         addr,
		PickupWindowStart:         pgtype.Timestamptz{Time: in.PickupWindowStart, Valid: true},
		PickupWindowEnd:           pgtype.Timestamptz{Time: in.PickupWindowEnd, Valid: true},
		Status:                    status,
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, uuid.UUID(row.ID.Bytes))
}

func (s *ListingService) validateCreate(in CreateListingInput) error {
	bad := func(code, field string) error { return &ValidationError{Code: code, Field: field} }
	desc := strings.TrimSpace(in.FoodDescription)
	switch {
	case desc == "":
		return bad("missing_field", "food_description")
	case utf8.RuneCountInString(desc) > 1000:
		return bad("invalid_field", "food_description")
	}
	if in.AllergenInfo == nil {
		return bad("missing_field", "allergen_info")
	}
	if utf8.RuneCountInString(*in.AllergenInfo) > 500 {
		return bad("invalid_field", "allergen_info")
	}
	if in.PhotoURL != nil {
		u, err := url.Parse(*in.PhotoURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(*in.PhotoURL) > 2048 {
			return bad("invalid_field", "photo_url")
		}
	}
	if in.TotalPortions < 1 || in.TotalPortions > 1000 {
		return bad("invalid_field", "total_portions")
	}
	if in.ReducedPriceCents < 1 {
		return bad("invalid_field", "reduced_price_cents")
	}
	if in.RetailReferencePriceCents < 1 {
		return bad("invalid_field", "retail_reference_price_cents")
	}
	// FR-003 / Principle V: the price must be below the retail reference.
	if in.ReducedPriceCents >= in.RetailReferencePriceCents {
		return bad("price_not_reduced", "reduced_price_cents")
	}
	if in.PickupLocation != nil && !validLocation(*in.PickupLocation) {
		return bad("invalid_field", "pickup_location")
	}
	if in.PickupWindowStart.IsZero() {
		return bad("missing_field", "pickup_window_start")
	}
	if in.PickupWindowEnd.IsZero() {
		return bad("missing_field", "pickup_window_end")
	}
	if !in.PickupWindowEnd.After(in.PickupWindowStart) {
		return bad("window_invalid", "pickup_window_end")
	}
	if !in.PickupWindowEnd.After(s.now()) {
		return bad("window_in_past", "pickup_window_end")
	}
	return nil
}

func validLocation(l Location) bool {
	return l.Lat >= -90 && l.Lat <= 90 && l.Lng >= -180 && l.Lng <= 180
}

// List returns published listings within the radius: the explicit radius_km, else the signed-in
// buyer's default, else 2 km. Non-profits first, then distance (research.md).
func (s *ListingService) List(ctx context.Context, q ListQuery) ([]ListingItem, error) {
	if !validLocation(Location{Lat: q.Lat, Lng: q.Lng}) {
		return nil, &ValidationError{Code: "invalid_field", Field: "lat"}
	}
	radius := DefaultDiscoveryRadiusKm
	switch {
	case q.RadiusKm != nil:
		if *q.RadiusKm <= 0 || *q.RadiusKm > MaxDiscoveryRadiusKm {
			return nil, &ValidationError{Code: "invalid_field", Field: "radius_km"}
		}
		radius = *q.RadiusKm
	case q.Account != nil:
		bp, err := s.q.GetBuyerProfileByAccount(ctx, pgUUID(q.Account.ID))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			if r := numericFloat(bp.DefaultDiscoveryRadiusKm); r > 0 {
				radius = r
			}
		}
	}
	rows, err := s.q.ListNearbyListings(ctx, db.ListNearbyListingsParams{
		Lng: q.Lng, Lat: q.Lat, RadiusM: radius * 1000, RowLimit: 100, RowOffset: 0,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ListingItem, 0, len(rows))
	for _, r := range rows {
		d := r.DistanceKm
		out = append(out, ListingItem{
			ID: uuid.UUID(r.ID.Bytes), FoodDescription: r.FoodDescription, PhotoURL: r.PhotoUrl,
			ReducedPriceCents: int(r.ReducedPriceCents), PortionsRemaining: int(r.PortionsRemaining),
			PickupWindowStart: r.PickupWindowStart.Time.UTC(), PickupWindowEnd: r.PickupWindowEnd.Time.UTC(),
			DistanceKm: &d,
			Cook: CookBlock{
				DisplayName: r.CookDisplayName, OrgType: string(r.CookOrgType),
				VerificationStatus: string(r.CookVerificationStatus), RatingAvg: numericFloat(r.CookRatingAvg),
				RatingCount: int(r.CookRatingCount), VerificationCycleCount: int(r.CookVerificationCycleCount),
			},
		})
	}
	return out, nil
}

// Get returns one listing. Published listings are public; any other status is visible only to
// the owning cook (everyone else gets ErrListingNotFound).
func (s *ListingService) Get(ctx context.Context, viewer *Account, id uuid.UUID) (*ListingDetail, error) {
	d, ownerCook, err := s.detailWithOwner(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Status != string(db.ListingStatusPublished) {
		if viewer == nil || viewer.CookProfileID == nil || *viewer.CookProfileID != ownerCook {
			return nil, ErrListingNotFound
		}
	}
	return d, nil
}

func (s *ListingService) detail(ctx context.Context, id uuid.UUID) (*ListingDetail, error) {
	d, _, err := s.detailWithOwner(ctx, id)
	return d, err
}

func (s *ListingService) detailWithOwner(ctx context.Context, id uuid.UUID) (*ListingDetail, uuid.UUID, error) {
	r, err := s.q.GetListingByID(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.Nil, ErrListingNotFound
	}
	if err != nil {
		return nil, uuid.Nil, err
	}
	return &ListingDetail{
		ListingItem: ListingItem{
			ID: uuid.UUID(r.ID.Bytes), FoodDescription: r.FoodDescription, PhotoURL: r.PhotoUrl,
			ReducedPriceCents: int(r.ReducedPriceCents), PortionsRemaining: int(r.PortionsRemaining),
			PickupWindowStart: r.PickupWindowStart.Time.UTC(), PickupWindowEnd: r.PickupWindowEnd.Time.UTC(),
			Cook: CookBlock{
				DisplayName: r.CookDisplayName, OrgType: string(r.CookOrgType),
				VerificationStatus: string(r.CookVerificationStatus), RatingAvg: numericFloat(r.CookRatingAvg),
				RatingCount: int(r.CookRatingCount), VerificationCycleCount: int(r.CookVerificationCycleCount),
			},
		},
		AllergenInfo: r.AllergenInfo, PickupAddressText: r.PickupAddressText, Status: string(r.Status),
		TotalPortions: int(r.TotalPortions), RetailReferencePriceCents: int(r.RetailReferencePriceCents),
		PickupLocation: Location{Lat: r.PickupLocation.Y(), Lng: r.PickupLocation.X()},
	}, uuid.UUID(r.CookID.Bytes), nil
}

// Update applies the cook's PATCH: adjust portions_remaining, extend the pickup window, or cancel.
// Only the owning cook, only while the listing is published (published -> cancelled is the only
// status transition, data-model.md). The listing row is locked first; claim creation takes the
// same lock through the atomic decrement, so a concurrent claim cannot slip past a bound check.
//
// portions_remaining is bounded by the unclaimed capacity (total_portions minus portions held by
// paid or reserved claims); see the deviation note in the task report.
//
// Cancelling fails unpaid reservations, then refunds paid claims through the PaymentProvider and
// queues one buyer notification per refunded claim (FR-010). A refund that fails stays paid and
// is retried by the expiry job.
func (s *ListingService) Update(ctx context.Context, acct *Account, id uuid.UUID, in UpdateListingInput) (*ListingDetail, error) {
	if in.PortionsRemaining == nil && in.PickupWindowEnd == nil && in.Status == nil {
		return nil, &ValidationError{Code: "missing_field", Field: "portions_remaining"}
	}
	if in.Status != nil && *in.Status != string(db.ListingStatusCancelled) {
		return nil, &ValidationError{Code: "invalid_field", Field: "status"}
	}
	if acct.CookProfileID == nil {
		return nil, ErrCookProfileMissing
	}
	cancelled := false
	err := s.tx(ctx, func(q db.Querier) error {
		l, err := q.GetListingForUpdate(ctx, pgUUID(id))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrListingNotFound
		}
		if err != nil {
			return err
		}
		if uuid.UUID(l.CookID.Bytes) != *acct.CookProfileID {
			return ErrListingForbidden
		}
		if l.Status != db.ListingStatusPublished {
			return ErrListingNotActive
		}
		if in.PortionsRemaining != nil {
			p := *in.PortionsRemaining
			if p < 0 || p > int(l.TotalPortions) {
				return &ValidationError{Code: "invalid_field", Field: "portions_remaining"}
			}
			claimed, err := q.SumLiveClaimedQuantity(ctx, l.ID)
			if err != nil {
				return err
			}
			if p > int(l.TotalPortions)-int(claimed) {
				return ErrPortionsConflict
			}
			if err := q.UpdateListingPortions(ctx, db.UpdateListingPortionsParams{ID: l.ID, PortionsRemaining: int32(p)}); err != nil {
				return err
			}
		}
		if in.PickupWindowEnd != nil {
			if !in.PickupWindowEnd.After(l.PickupWindowEnd.Time) {
				return &ValidationError{Code: "window_not_extended", Field: "pickup_window_end"}
			}
			if err := q.UpdateListingWindowEnd(ctx, db.UpdateListingWindowEndParams{
				ID: l.ID, PickupWindowEnd: pgtype.Timestamptz{Time: *in.PickupWindowEnd, Valid: true},
			}); err != nil {
				return err
			}
		}
		if in.Status != nil {
			if _, err := q.CancelListing(ctx, l.ID); err != nil {
				return err
			}
			// Unpaid reservations can no longer be paid for; their late payments are refunded by
			// the webhook handler.
			if _, err := q.FailPendingClaimsForListing(ctx, l.ID); err != nil {
				return err
			}
			cancelled = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cancelled {
		if err := s.RefundCancelledListingClaims(ctx, &id); err != nil {
			// The listing is cancelled either way; the job retries the remaining refunds.
			log.Printf("listing %s: refunds incomplete, will retry: %v", id, err)
		}
	}
	return s.detail(ctx, id)
}

// RefundCancelledListingClaims refunds paid, uncollected claims of cancelled listings (one
// listing, or all when listingID is nil), marks them refunded and queues a buyer notification.
// Refund calls carry the claim id as idempotency key, so a crash between the provider call and
// the database update is safe to retry. It returns the first error after trying every claim.
func (s *ListingService) RefundCancelledListingClaims(ctx context.Context, listingID *uuid.UUID) error {
	var lid pgtype.UUID
	if listingID != nil {
		lid = pgUUID(*listingID)
	}
	claims, err := s.q.ListClaimsToRefund(ctx, lid)
	if err != nil {
		return err
	}
	var first error
	for _, c := range claims {
		if c.StripePaymentIntentID == nil {
			log.Printf("claim %x is paid without a payment intent; cannot refund", c.ID.Bytes)
			continue
		}
		claimID := uuid.UUID(c.ID.Bytes)
		if err := s.payments.Refund(ctx, *c.StripePaymentIntentID, claimID.String()); err != nil {
			log.Printf("refund of claim %s failed: %v", claimID, err)
			if first == nil {
				first = err
			}
			continue
		}
		err := s.tx(ctx, func(q db.Querier) error {
			if _, err := q.MarkClaimRefunded(ctx, c.ID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil // already refunded by a concurrent run
				}
				return err
			}
			return q.CreateNotification(ctx, db.CreateNotificationParams{
				BuyerProfileID: c.BuyerID, ClaimID: c.ID, Kind: "listing_cancelled_refund",
			})
		})
		if err != nil && first == nil {
			first = err
		}
	}
	return first
}

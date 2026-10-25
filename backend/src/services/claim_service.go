package services

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/eyho/eyho/src/db/generated"
)

const (
	// PaymentReservationTimeout is how long unpaid portions stay reserved (FR-008).
	PaymentReservationTimeout = 10 * time.Minute
	// MaxClaimQuantity bounds one claim.
	MaxClaimQuantity = 100
	// GramsPerPortion is the fixed estimate behind ImpactRecord.estimated_weight_grams. It is an
	// estimate, stored under an "estimated_" name, and must be presented as one (Principle II).
	GramsPerPortion = 400
	// DefaultClaimsLimit and MaxClaimsLimit bound GET /claims pages.
	DefaultClaimsLimit = 20
	MaxClaimsLimit     = 100

	currency = "eur"
)

// CreateClaimResult is the 201 body of POST /claims.
type CreateClaimResult struct {
	ClaimID                     uuid.UUID `json:"claim_id"`
	Quantity                    int       `json:"quantity"`
	AmountDueCents              int       `json:"amount_due_cents"`
	StripeClientSecret          string    `json:"stripe_client_secret"`
	PaymentReservationExpiresAt time.Time `json:"payment_reservation_expires_at"`
}

// ClaimListing is the listing block of a claim.
type ClaimListing struct {
	ID                uuid.UUID `json:"id"`
	FoodDescription   string    `json:"food_description"`
	PhotoURL          *string   `json:"photo_url"`
	PickupAddressText string    `json:"pickup_address_text"`
	PickupWindowStart time.Time `json:"pickup_window_start"`
	PickupWindowEnd   time.Time `json:"pickup_window_end"`
	CookDisplayName   string    `json:"cook_display_name"`
}

// ClaimView is one item of GET /claims and the body of GET /claims/{id}.
type ClaimView struct {
	ID              uuid.UUID    `json:"id"`
	Quantity        int          `json:"quantity"`
	AmountPaidCents int          `json:"amount_paid_cents"`
	PaymentStatus   string       `json:"payment_status"`
	PickupStatus    string       `json:"pickup_status"`
	Rating          *int16       `json:"rating"`
	CreatedAt       time.Time    `json:"created_at"`
	Listing         ClaimListing `json:"listing"`
}

// ClaimPage is GET /claims.
type ClaimPage struct {
	Claims     []ClaimView `json:"claims"`
	NextCursor *string     `json:"next_cursor"`
}

// RatingResult is the 201 body of POST /claims/{id}/rating.
type RatingResult struct {
	ClaimID uuid.UUID `json:"claim_id"`
	Stars   int       `json:"stars"`
	RatedAt time.Time `json:"rated_at"`
}

// ClaimService holds the claim lifecycle: reservation with atomic decrement (FR-006), payment
// outcome with impact record (FR-009), timeout release (FR-008), pickup (FR-012) and ratings that
// drive cook verification (FR-013/FR-014).
type ClaimService struct {
	q        db.Querier
	tx       TxFunc
	gate     CookGate
	payments PaymentProvider
	now      func() time.Time
	// Timeout is the payment reservation window; tests shorten it.
	Timeout time.Duration
}

// NewClaimService wires the service to a pool.
func NewClaimService(pool *pgxpool.Pool, gate CookGate, payments PaymentProvider) *ClaimService {
	return newClaimService(db.New(pool), PoolTx(pool), gate, payments)
}

func newClaimService(q db.Querier, tx TxFunc, gate CookGate, payments PaymentProvider) *ClaimService {
	return &ClaimService{q: q, tx: tx, gate: gate, payments: payments, now: time.Now, Timeout: PaymentReservationTimeout}
}

// Create reserves qty portions for the signed-in buyer and starts the payment.
//
//  1. The listing must be published and its window open (404 otherwise).
//  2. Own-listing guard: a buyer whose account owns the listing's cook profile gets ErrOwnListing
//     (feature 001 FR-011, Principle I: cooks cannot rate their own listings by claiming them).
//  3. One transaction runs the atomic conditional decrement (research.md) and inserts the claim.
//     No application-level locking: two buyers racing for the last portion get one success and one
//     InsufficientPortionsError.
//  4. The payment intent is created after the commit. If the provider fails, the reservation is
//     released again and ErrPaymentUnavailable is returned.
func (s *ClaimService) Create(ctx context.Context, acct *Account, listingID uuid.UUID, qty int) (*CreateClaimResult, error) {
	if qty < 1 || qty > MaxClaimQuantity {
		return nil, &ValidationError{Code: "invalid_field", Field: "quantity"}
	}
	l, err := s.q.GetListingByID(ctx, pgUUID(listingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrListingNotFound
	}
	if err != nil {
		return nil, err
	}
	if l.Status != db.ListingStatusPublished || !l.PickupWindowEnd.Time.After(s.now()) {
		return nil, ErrListingNotFound
	}
	own, err := s.gate.AccountOwnsCookProfile(ctx, acct.ID, uuid.UUID(l.CookID.Bytes))
	if err != nil {
		return nil, err
	}
	if own {
		return nil, ErrOwnListing
	}

	var claim db.Claim
	err = s.tx(ctx, func(q db.Querier) error {
		res, err := q.DecrementPortionsAtomic(ctx, db.DecrementPortionsAtomicParams{ID: l.ID, PortionsRemaining: int32(qty)})
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			cur, err := q.GetListingByID(ctx, l.ID)
			if err != nil {
				return err
			}
			if cur.Status != db.ListingStatusPublished || !cur.PickupWindowEnd.Time.After(s.now()) {
				return ErrListingNotFound
			}
			return &InsufficientPortionsError{Remaining: int(cur.PortionsRemaining)}
		}
		claim, err = q.CreateClaim(ctx, db.CreateClaimParams{ListingID: l.ID, BuyerID: pgUUID(acct.BuyerProfileID), Quantity: int32(qty)})
		return err
	})
	if err != nil {
		return nil, err
	}

	claimID := uuid.UUID(claim.ID.Bytes)
	amount := int(l.ReducedPriceCents) * qty
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	intent, err := s.payments.CreateIntent(pctx, IntentRequest{ClaimID: claimID, AmountCents: amount, Currency: currency})
	if err != nil {
		log.Printf("claim %s: creating payment intent failed, releasing reservation: %v", claimID, err)
		if rerr := s.release(context.WithoutCancel(ctx), claim.ID); rerr != nil {
			log.Printf("claim %s: release after failed intent: %v", claimID, rerr)
		}
		return nil, ErrPaymentUnavailable
	}
	if err := s.q.SetClaimPaymentIntent(ctx, db.SetClaimPaymentIntentParams{ID: claim.ID, StripePaymentIntentID: &intent.ID}); err != nil {
		// The webhook finds the claim through the intent metadata, so payment still completes.
		log.Printf("claim %s: storing payment intent id: %v", claimID, err)
	}
	return &CreateClaimResult{
		ClaimID: claimID, Quantity: qty, AmountDueCents: amount, StripeClientSecret: intent.ClientSecret,
		PaymentReservationExpiresAt: claim.ReservedAt.Time.Add(s.Timeout).UTC(),
	}, nil
}

// release fails an unpaid claim and returns its portions in one transaction. Idempotent.
func (s *ClaimService) release(ctx context.Context, claimID pgtype.UUID) error {
	return s.tx(ctx, func(q db.Querier) error {
		r, err := q.FailPendingClaim(ctx, claimID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return q.ReturnPortions(ctx, db.ReturnPortionsParams{ID: r.ListingID, Qty: r.Quantity})
	})
}

// HandlePaymentEvent applies a verified payment webhook event (FR-008, FR-009). It is idempotent
// so provider redeliveries are safe.
//
//   - succeeded: pending_payment -> paid with confirmed_at, plus the ImpactRecord, in ONE
//     transaction (SC-005). A payment that arrives for a claim that was already released (timeout
//     or cancelled listing) is refunded instead: its portions have been returned.
//   - failed: pending_payment -> payment_failed and the portions go back to the listing.
func (s *ClaimService) HandlePaymentEvent(ctx context.Context, ev *PaymentEvent) error {
	if ev.Type == PaymentIgnored || ev.ClaimID == uuid.Nil {
		return nil
	}
	cid := pgUUID(ev.ClaimID)
	v, err := s.q.GetClaimViewByID(ctx, cid)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("payment event for unknown claim %s ignored", ev.ClaimID)
		return nil
	}
	if err != nil {
		return err
	}
	if v.StripePaymentIntentID != nil && *v.StripePaymentIntentID != ev.IntentID {
		log.Printf("claim %s: event for intent %s does not match stored intent; ignored", ev.ClaimID, ev.IntentID)
		return nil
	}

	switch ev.Type {
	case PaymentFailed:
		return s.release(ctx, cid)
	case PaymentSucceeded:
		if ev.AmountCents != int(v.AmountPaidCents) {
			log.Printf("claim %s: paid amount %d differs from due %d; not confirming", ev.ClaimID, ev.AmountCents, v.AmountPaidCents)
			return nil
		}
		confirmed := false
		err := s.tx(ctx, func(q db.Querier) error {
			c, err := q.MarkClaimPaid(ctx, cid)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			confirmed = true
			return q.CreateImpactRecord(ctx, db.CreateImpactRecordParams{
				ClaimID: c.ID, ListingID: c.ListingID, Portions: c.Quantity,
				EstimatedWeightGrams: c.Quantity * GramsPerPortion,
			})
		})
		if err != nil {
			return err
		}
		if !confirmed && v.PaymentStatus == db.ClaimPaymentStatusPaymentFailed {
			log.Printf("claim %s: payment arrived after release; refunding", ev.ClaimID)
			if err := s.payments.Refund(ctx, ev.IntentID, ev.ClaimID.String()); err != nil {
				return err // non-2xx makes the provider redeliver
			}
		}
	}
	return nil
}

// PaymentTimeoutJob releases reservations that were not paid within Timeout and returns their
// portions (FR-008).
func (s *ClaimService) PaymentTimeoutJob(ctx context.Context) error {
	return s.tx(ctx, func(q db.Querier) error {
		rows, err := q.ReleasePendingClaims(ctx, pgtype.Interval{Microseconds: s.Timeout.Microseconds(), Valid: true})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := q.ReturnPortions(ctx, db.ReturnPortionsParams{ID: r.ListingID, Qty: r.Quantity}); err != nil {
				return err
			}
		}
		if len(rows) > 0 {
			log.Printf("payment timeout job: released %d reservations", len(rows))
		}
		return nil
	})
}

func claimViewFromList(r db.ListClaimsByBuyerRow) ClaimView {
	return ClaimView{
		ID: uuid.UUID(r.ID.Bytes), Quantity: int(r.Quantity), AmountPaidCents: int(r.AmountPaidCents),
		PaymentStatus: string(r.PaymentStatus), PickupStatus: string(r.PickupStatus), Rating: r.Rating,
		CreatedAt: r.CreatedAt.Time.UTC(),
		Listing: ClaimListing{
			ID: uuid.UUID(r.ListingID.Bytes), FoodDescription: r.FoodDescription, PhotoURL: r.PhotoUrl,
			PickupAddressText: r.PickupAddressText, PickupWindowStart: r.PickupWindowStart.Time.UTC(),
			PickupWindowEnd: r.PickupWindowEnd.Time.UTC(), CookDisplayName: r.CookDisplayName,
		},
	}
}

func claimViewFromGet(r db.GetClaimViewByIDRow) ClaimView {
	return ClaimView{
		ID: uuid.UUID(r.ID.Bytes), Quantity: int(r.Quantity), AmountPaidCents: int(r.AmountPaidCents),
		PaymentStatus: string(r.PaymentStatus), PickupStatus: string(r.PickupStatus), Rating: r.Rating,
		CreatedAt: r.CreatedAt.Time.UTC(),
		Listing: ClaimListing{
			ID: uuid.UUID(r.ListingID.Bytes), FoodDescription: r.FoodDescription, PhotoURL: r.PhotoUrl,
			PickupAddressText: r.PickupAddressText, PickupWindowStart: r.PickupWindowStart.Time.UTC(),
			PickupWindowEnd: r.PickupWindowEnd.Time.UTC(), CookDisplayName: r.CookDisplayName,
		},
	}
}

// List returns the buyer's claims, newest first, without failed payments. cursor is the opaque
// value of the previous page's next_cursor ("" for the first page).
func (s *ClaimService) List(ctx context.Context, acct *Account, cursor string, limit int) (*ClaimPage, error) {
	if limit <= 0 {
		limit = DefaultClaimsLimit
	}
	if limit > MaxClaimsLimit {
		limit = MaxClaimsLimit
	}
	ts, id, err := decodeCursor(cursor)
	if err != nil {
		return nil, &ValidationError{Code: "invalid_field", Field: "cursor"}
	}
	rows, err := s.q.ListClaimsByBuyer(ctx, db.ListClaimsByBuyerParams{
		BuyerID: pgUUID(acct.BuyerProfileID), BeforeTs: pgtype.Timestamptz{Time: ts, Valid: true},
		BeforeID: pgUUID(id), RowLimit: int32(limit + 1),
	})
	if err != nil {
		return nil, err
	}
	page := &ClaimPage{Claims: make([]ClaimView, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		c := encodeCursor(last.CreatedAt.Time, uuid.UUID(last.ID.Bytes))
		page.NextCursor = &c
	}
	for _, r := range rows {
		page.Claims = append(page.Claims, claimViewFromList(r))
	}
	return page, nil
}

// Get returns a claim to its buyer or to the cook of its listing; anyone else gets ErrClaimNotFound.
func (s *ClaimService) Get(ctx context.Context, acct *Account, id uuid.UUID) (*ClaimView, error) {
	r, err := s.q.GetClaimViewByID(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClaimNotFound
	}
	if err != nil {
		return nil, err
	}
	if !isClaimParty(acct, uuid.UUID(r.BuyerID.Bytes), uuid.UUID(r.ListingCookID.Bytes)) {
		return nil, ErrClaimNotFound
	}
	v := claimViewFromGet(r)
	return &v, nil
}

func isClaimParty(acct *Account, buyerID, cookID uuid.UUID) bool {
	if acct.BuyerProfileID == buyerID {
		return true
	}
	return acct.CookProfileID != nil && *acct.CookProfileID == cookID
}

// ConfirmPickup marks a paid claim collected. Allowed for the claim's buyer or the listing's cook
// (404 for anyone else). ErrClaimConflict when the claim is not paid and awaiting pickup, e.g.
// already collected, refunded, or marked no_show.
func (s *ClaimService) ConfirmPickup(ctx context.Context, acct *Account, id uuid.UUID) (*ClaimView, error) {
	if _, err := s.Get(ctx, acct, id); err != nil {
		return nil, err
	}
	if _, err := s.q.MarkClaimCollected(ctx, pgUUID(id)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClaimConflict
		}
		return nil, err
	}
	return s.Get(ctx, acct, id)
}

// Rate stores the buyer's rating of a collected claim and re-evaluates the cook's verification
// status (FR-013/FR-014, Principle I; constitution v3.0.0: every buyer rating counts separately).
//
// One transaction: lock the cook profile (serialises all ratings of this cook), write the rating
// (conditional: collected and not yet rated), recompute rating_avg/rating_count from the claims,
// bump the since-transition counters (>= 4 stars good, <= 2 bad, 3 neither), then
//   - unverified with 5 good ratings -> verified;
//   - verified with 3 bad ratings -> unverified.
//
// A transition resets both counters, increments verification_cycle_count and appends a
// VerificationHistory row listing the triggering claim ids. Listings keep their status; they are
// re-flagged simply because responses read the cook's current status.
func (s *ClaimService) Rate(ctx context.Context, acct *Account, id uuid.UUID, stars int) (*RatingResult, error) {
	c, err := s.q.GetClaimByID(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClaimNotFound
	}
	if err != nil {
		return nil, err
	}
	if uuid.UUID(c.BuyerID.Bytes) != acct.BuyerProfileID {
		return nil, ErrNotClaimBuyer
	}
	if stars < 1 || stars > 5 {
		return nil, &ValidationError{Code: "invalid_field", Field: "stars"}
	}
	// Cooks cannot rate their own listings (Principle I). Claiming them is already refused; this
	// keeps the rule even for claims that predate the guard.
	own, err := s.gate.AccountOwnsCookProfile(ctx, acct.ID, uuid.UUID(c.ListingCookID.Bytes))
	if err != nil {
		return nil, err
	}
	if own {
		return nil, ErrOwnListing
	}

	var res RatingResult
	err = s.tx(ctx, func(q db.Querier) error {
		cook, err := q.LockCookProfile(ctx, c.ListingCookID)
		if err != nil {
			return err
		}
		rated, err := q.RateClaim(ctx, db.RateClaimParams{ID: c.ID, Rating: ptr(int16(stars))})
		if errors.Is(err, pgx.ErrNoRows) {
			cur, gerr := q.GetClaimByID(ctx, c.ID)
			if gerr != nil {
				return gerr
			}
			if cur.Rating != nil {
				return ErrAlreadyRated
			}
			return ErrNotCollected
		}
		if err != nil {
			return err
		}
		agg, err := q.AggregateCookRatings(ctx, cook.ID)
		if err != nil {
			return err
		}
		good, bad := cook.GoodRatingsSinceTransition, cook.BadRatingsSinceTransition
		if stars >= 4 {
			good++
		}
		if stars <= 2 {
			bad++
		}
		if err := q.UpdateCookRatings(ctx, db.UpdateCookRatingsParams{
			ID: cook.ID, RatingAvg: agg.RatingAvg, RatingCount: agg.RatingCount,
			GoodRatingsSinceTransition: good, BadRatingsSinceTransition: bad,
		}); err != nil {
			return err
		}

		var to db.CookVerificationStatus
		var triggers []pgtype.UUID
		switch {
		case cook.VerificationStatus == db.CookVerificationStatusUnverified && good >= PromoteAfterGoodRatings:
			to = db.CookVerificationStatusVerified
			triggers, err = q.ListLatestGoodRatedClaimIDs(ctx, db.ListLatestGoodRatedClaimIDsParams{CookID: cook.ID, RowLimit: good})
		case cook.VerificationStatus == db.CookVerificationStatusVerified && bad >= DemoteAfterBadRatings:
			to = db.CookVerificationStatusUnverified
			triggers, err = q.ListLatestBadRatedClaimIDs(ctx, db.ListLatestBadRatedClaimIDsParams{CookID: cook.ID, RowLimit: bad})
		}
		if err != nil {
			return err
		}
		if to != "" {
			if err := q.InsertVerificationHistory(ctx, db.InsertVerificationHistoryParams{
				CookID: cook.ID, FromStatus: cook.VerificationStatus, ToStatus: to, TriggeringClaimIds: triggers,
			}); err != nil {
				return err
			}
			if _, err := q.UpdateVerificationStatus(ctx, db.UpdateVerificationStatusParams{
				ID: cook.ID, VerificationStatus: to, VerificationCycleCount: cook.VerificationCycleCount + 1,
			}); err != nil {
				return err
			}
		}
		res = RatingResult{ClaimID: id, Stars: stars, RatedAt: rated.RatedAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Verification thresholds (constitution v3.1.0, Principle I).
const (
	PromoteAfterGoodRatings int32 = 5 // ratings >= 4 stars
	DemoteAfterBadRatings   int32 = 3 // ratings <= 2 stars
)

func ptr[T any](v T) *T { return &v }

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/ewkb"

	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/db/geo"
)

type fakeGate struct {
	ok     bool
	status string
}

func (g fakeGate) CookPublishGate(context.Context, uuid.UUID) (bool, string, error) {
	return g.ok, g.status, nil
}
func (g fakeGate) AccountOwnsCookProfile(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// listingQ stores what CreateListing received and answers GetListingByID for it.
type listingQ struct {
	db.Querier
	verification db.CookVerificationStatus
	created      []db.CreateListingParams
}

func (q *listingQ) CreateListing(_ context.Context, a db.CreateListingParams) (db.Listing, error) {
	q.created = append(q.created, a)
	return db.Listing{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}}, nil
}

func (q *listingQ) GetListingByID(_ context.Context, id pgtype.UUID) (db.GetListingByIDRow, error) {
	a := q.created[len(q.created)-1]
	return db.GetListingByIDRow{
		ID: id, CookID: a.CookID, FoodDescription: a.FoodDescription, AllergenInfo: a.AllergenInfo,
		TotalPortions: a.TotalPortions, PortionsRemaining: a.TotalPortions, ReducedPriceCents: a.ReducedPriceCents,
		RetailReferencePriceCents: a.RetailReferencePriceCents, PickupAddressText: a.PickupAddressText,
		PickupWindowStart: a.PickupWindowStart, PickupWindowEnd: a.PickupWindowEnd, Status: a.Status,
		PickupLocation: geo.Point{Point: ewkb.Point{Point: geom.NewPointFlat(geom.XY, []float64{a.Lng, a.Lat})}}, CookDisplayName: "Cook", CookOrgType: db.CookOrgTypeProfessional,
		CookVerificationStatus: q.verification,
	}, nil
}

func (q *listingQ) GetCookProfileByAccount(context.Context, pgtype.UUID) (db.GetCookProfileByAccountRow, error) {
	addr := "Default street 1"
	return db.GetCookProfileByAccountRow{DefaultPickupAddressText: &addr, DefaultPickupLat: 52.1, DefaultPickupLng: 4.2}, nil
}

func newListingTestSvc(q *listingQ, gate fakeGate) (*ListingService, *Account) {
	s := newListingService(q, nil, gate, nil)
	cook := uuid.New()
	return s, &Account{ID: uuid.New(), CookProfileID: &cook}
}

func validListingInput(s *ListingService) CreateListingInput {
	none := ""
	return CreateListingInput{
		FoodDescription: "Lasagne", AllergenInfo: &none, TotalPortions: 4, ReducedPriceCents: 500,
		RetailReferencePriceCents: 1500, PickupWindowStart: s.now().Add(time.Hour),
		PickupWindowEnd: s.now().Add(3 * time.Hour), Publish: true,
	}
}

func assertValidation(t *testing.T, err error, code, field string) {
	t.Helper()
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "want ValidationError, got %v", err)
	assert.Equal(t, code, ve.Code)
	assert.Equal(t, field, ve.Field)
}

func TestListingCreate_PriceRule(t *testing.T) {
	for _, reduced := range []int{1500, 1501} { // equal to and above the retail reference (FR-003)
		q := &listingQ{}
		s, acct := newListingTestSvc(q, fakeGate{ok: true})
		in := validListingInput(s)
		in.ReducedPriceCents = reduced
		_, err := s.Create(context.Background(), acct, in)
		assertValidation(t, err, "price_not_reduced", "reduced_price_cents")
		assert.Empty(t, q.created, "nothing is stored")
	}
}

func TestListingCreate_WindowRules(t *testing.T) {
	q := &listingQ{}
	s, acct := newListingTestSvc(q, fakeGate{ok: true})

	in := validListingInput(s)
	in.PickupWindowStart, in.PickupWindowEnd = s.now().Add(-3*time.Hour), s.now().Add(-time.Hour)
	_, err := s.Create(context.Background(), acct, in)
	assertValidation(t, err, "window_in_past", "pickup_window_end")

	in = validListingInput(s)
	in.PickupWindowEnd = in.PickupWindowStart.Add(-time.Minute)
	_, err = s.Create(context.Background(), acct, in)
	assertValidation(t, err, "window_invalid", "pickup_window_end")

	// A window that already started but has not ended is fine.
	in = validListingInput(s)
	in.PickupWindowStart = s.now().Add(-time.Hour)
	_, err = s.Create(context.Background(), acct, in)
	require.NoError(t, err)
}

func TestListingCreate_RequiredFields(t *testing.T) {
	s, acct := newListingTestSvc(&listingQ{}, fakeGate{ok: true})
	in := validListingInput(s)
	in.AllergenInfo = nil
	_, err := s.Create(context.Background(), acct, in)
	assertValidation(t, err, "missing_field", "allergen_info")

	in = validListingInput(s)
	in.FoodDescription = "  "
	_, err = s.Create(context.Background(), acct, in)
	assertValidation(t, err, "missing_field", "food_description")

	_, err = s.Create(context.Background(), &Account{ID: uuid.New()}, validListingInput(s))
	assert.ErrorIs(t, err, ErrCookProfileMissing)
}

// Principle I: an unverified cook publishes normally; the listing is flagged "unverified".
func TestListingCreate_UnverifiedCookPublishesAndIsFlagged(t *testing.T) {
	q := &listingQ{verification: db.CookVerificationStatusUnverified}
	s, acct := newListingTestSvc(q, fakeGate{ok: true})
	d, err := s.Create(context.Background(), acct, validListingInput(s))
	require.NoError(t, err)
	assert.Equal(t, "published", d.Status)
	assert.Equal(t, "unverified", d.Cook.VerificationStatus)
	require.Len(t, q.created, 1)
	assert.Equal(t, db.ListingStatusPublished, q.created[0].Status)
}

func TestListingCreate_DefaultsPickupFromCookProfile(t *testing.T) {
	q := &listingQ{}
	s, acct := newListingTestSvc(q, fakeGate{ok: true})
	d, err := s.Create(context.Background(), acct, validListingInput(s))
	require.NoError(t, err)
	assert.Equal(t, "Default street 1", d.PickupAddressText)
	assert.Equal(t, 52.1, q.created[0].Lat)
	assert.Equal(t, 4.2, q.created[0].Lng)
}

// FR-001 / FR-020: an unregistered cook cannot publish but can save a draft.
func TestListingCreate_UnregisteredCookCannotPublishButCanDraft(t *testing.T) {
	q := &listingQ{}
	s, acct := newListingTestSvc(q, fakeGate{ok: false, status: "inactive"})

	_, err := s.Create(context.Background(), acct, validListingInput(s))
	var nr *CookNotRegisteredError
	require.True(t, errors.As(err, &nr))
	assert.Equal(t, "inactive", nr.RegistrationStatus)
	assert.Empty(t, q.created)

	in := validListingInput(s)
	in.Publish = false
	d, err := s.Create(context.Background(), acct, in)
	require.NoError(t, err)
	assert.Equal(t, "draft", d.Status)
}

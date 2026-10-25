package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/idp"
	"github.com/eyho/eyho/src/registry"
	"github.com/jackc/pgx/v5/pgtype"
)

// fakeQ implements the few queries these unit tests need; calling anything else panics (nil
// embedded interface), which keeps the tests honest about what they exercise.
type fakeQ struct {
	db.Querier
	owns        bool
	canPublish  bool
	regStatus   db.CookRegistrationStatus
	gotOwner    db.AccountOwnsCookProfileParams
	gotCookID   pgtype.UUID
	cookProfile error
}

func (f *fakeQ) AccountOwnsCookProfile(_ context.Context, a db.AccountOwnsCookProfileParams) (bool, error) {
	f.gotOwner = a
	return f.owns, nil
}
func (f *fakeQ) CookCanPublish(_ context.Context, id pgtype.UUID) (bool, error) {
	f.gotCookID = id
	return f.canPublish, nil
}
func (f *fakeQ) GetCookProfileByID(context.Context, pgtype.UUID) (db.CookProfile, error) {
	return db.CookProfile{RegistrationStatus: f.regStatus}, nil
}

func newTestSvc(q db.Querier) *AccountService {
	return newAccountService(q, nil, registry.NewFake(), idp.Noop{})
}

func TestAccountOwnsCookProfile_PassesBothIDs(t *testing.T) {
	acc, cook := uuid.New(), uuid.New()
	for _, owns := range []bool{true, false} {
		q := &fakeQ{owns: owns}
		got, err := newTestSvc(q).AccountOwnsCookProfile(context.Background(), acc, cook)
		require.NoError(t, err)
		assert.Equal(t, owns, got)
		assert.Equal(t, acc, uuid.UUID(q.gotOwner.AccountID.Bytes))
		assert.Equal(t, cook, uuid.UUID(q.gotOwner.ID.Bytes))
	}
}

func TestCookCanPublish_IndependentOfVerification(t *testing.T) {
	id := uuid.New()
	q := &fakeQ{canPublish: true}
	ok, err := newTestSvc(q).CookCanPublish(context.Background(), id)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, id, uuid.UUID(q.gotCookID.Bytes))
}

func TestCookPublishGate_ReportsPendingOrInactive(t *testing.T) {
	cases := map[db.CookRegistrationStatus]string{
		db.CookRegistrationStatusPending:  "pending",
		db.CookRegistrationStatusInactive: "inactive",
	}
	for status, want := range cases {
		ok, got, err := newTestSvc(&fakeQ{regStatus: status}).CookPublishGate(context.Background(), uuid.New())
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, want, got)
	}
	ok, got, err := newTestSvc(&fakeQ{canPublish: true}).CookPublishGate(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, got)
}

func validInput() CreateCookProfileInput {
	return CreateCookProfileInput{
		DisplayName: "Sanne's Kitchen", OrgType: "professional", Phone: "+31612345678",
		AddressText: "Keizersgracht 88, Amsterdam", Location: &Location{Lat: 52.37, Lng: 4.89},
		KvkNumber: "68750110", NVWA: &NVWADeclaration{Accepted: true, Version: "2026-10-25"},
	}
}

func TestValidateCreate(t *testing.T) {
	require.NoError(t, validateCreate(validInput()))

	missing := map[string]func(*CreateCookProfileInput){
		"display_name":                func(i *CreateCookProfileInput) { i.DisplayName = " " },
		"org_type":                    func(i *CreateCookProfileInput) { i.OrgType = "" },
		"phone":                       func(i *CreateCookProfileInput) { i.Phone = "" },
		"default_pickup_address_text": func(i *CreateCookProfileInput) { i.AddressText = "" },
		"default_pickup_location":     func(i *CreateCookProfileInput) { i.Location = nil },
		"kvk_number":                  func(i *CreateCookProfileInput) { i.KvkNumber = "" },
	}
	for field, mutate := range missing {
		in := validInput()
		mutate(&in)
		var ve *ValidationError
		require.ErrorAs(t, validateCreate(in), &ve, field)
		assert.Equal(t, ValidationError{"missing_field", field}, *ve)
	}

	invalid := map[string]func(*CreateCookProfileInput){
		"org_type":                func(i *CreateCookProfileInput) { i.OrgType = "hobby" },
		"phone":                   func(i *CreateCookProfileInput) { i.Phone = "abc" },
		"kvk_number":              func(i *CreateCookProfileInput) { i.KvkNumber = "1234567" },
		"default_pickup_location": func(i *CreateCookProfileInput) { i.Location = &Location{Lat: 91, Lng: 0} },
	}
	for field, mutate := range invalid {
		in := validInput()
		mutate(&in)
		var ve *ValidationError
		require.ErrorAs(t, validateCreate(in), &ve, field)
		assert.Equal(t, ValidationError{"invalid_field", field}, *ve)
	}
}

func TestValidateCreate_DeclarationRequired(t *testing.T) {
	in := validInput()
	in.NVWA = nil
	assert.ErrorIs(t, validateCreate(in), ErrDeclarationNeeded)
	in.NVWA = &NVWADeclaration{Accepted: false, Version: "2026-10-25"}
	assert.ErrorIs(t, validateCreate(in), ErrDeclarationNeeded)
	// A missing field is reported before the declaration.
	in.Phone = ""
	var ve *ValidationError
	assert.ErrorAs(t, validateCreate(in), &ve)
}

func TestValidateUpdate_DeclarationCannotBeWithdrawn(t *testing.T) {
	var ve *ValidationError
	require.ErrorAs(t, validateUpdate(UpdateCookProfileInput{NVWA: &NVWADeclaration{Accepted: false}}), &ve)
	assert.Equal(t, "nvwa_declaration", ve.Field)
	empty := ""
	require.ErrorAs(t, validateUpdate(UpdateCookProfileInput{Phone: &empty}), &ve)
	assert.Equal(t, "phone", ve.Field)
	require.NoError(t, validateUpdate(UpdateCookProfileInput{}))
}

func TestRequestVerificationEmail_LimitAndAlreadyVerified(t *testing.T) {
	s := newTestSvc(nil)
	now := time.Now()
	s.now = func() time.Time { return now }
	a := &Account{ID: uuid.New()}

	require.NoError(t, s.RequestVerificationEmail(context.Background(), a))
	assert.ErrorIs(t, s.RequestVerificationEmail(context.Background(), a), ErrRateLimited)
	// Another account is not limited.
	require.NoError(t, s.RequestVerificationEmail(context.Background(), &Account{ID: uuid.New()}))
	// After a minute it works again.
	now = now.Add(61 * time.Second)
	require.NoError(t, s.RequestVerificationEmail(context.Background(), a))
	// Verified accounts get 409 before any limiter bookkeeping.
	assert.ErrorIs(t, s.RequestVerificationEmail(context.Background(), &Account{ID: uuid.New(), EmailVerified: true}), ErrAlreadyVerified)
}

type failingIdp struct{ idp.Noop }

func (failingIdp) Manages(string) bool                                 { return true }
func (failingIdp) SendVerificationEmail(context.Context, string) error { return errors.New("down") }

func TestRequestVerificationEmail_ProviderFailureDoesNotConsumeLimit(t *testing.T) {
	s := newAccountService(nil, nil, registry.NewFake(), failingIdp{})
	a := &Account{ID: uuid.New()}
	assert.ErrorIs(t, s.RequestVerificationEmail(context.Background(), a), ErrIdpUnavailable)
	assert.ErrorIs(t, s.RequestVerificationEmail(context.Background(), a), ErrIdpUnavailable) // not 429
}

func TestIdpBackoffGrowsAndCaps(t *testing.T) {
	assert.Equal(t, 30*time.Second, idpBackoff(1))
	assert.Equal(t, time.Minute, idpBackoff(2))
	assert.Equal(t, idpBackoffMax, idpBackoff(50))
}

func TestDisplayNameFallsBackToEmailLocalPart(t *testing.T) {
	assert.Equal(t, "Sanne", displayNameFromClaimsForTest("Sanne", "x@y.nl"))
	assert.Equal(t, "x", displayNameFromClaimsForTest("", "x@y.nl"))
}

func displayNameFromClaimsForTest(name, email string) string {
	return displayNameFromClaims(authClaims(name, email))
}

package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eyho/eyho/src/auth"
	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/idp"
	"github.com/eyho/eyho/src/registry"
)

// Errors returned by AccountService. Handlers map them to the codes in
// specs/001-accounts-sign-in/contracts/accounts-api.md.
var (
	ErrAccountDeleted     = errors.New("account deleted")
	ErrUnknownIdentity    = errors.New("unknown demo identity")
	ErrEmailMissing       = errors.New("token has no email claim")
	ErrCookProfileExists  = errors.New("cook profile exists")
	ErrCookProfileMissing = errors.New("cook profile required")
	ErrDeclarationNeeded  = errors.New("declaration required")
	ErrKvkNotFound        = errors.New("kvk not found")
	ErrKvkInactive        = errors.New("kvk inactive")
	ErrAlreadyVerified    = errors.New("already verified")
	ErrRateLimited        = errors.New("rate limited")
	ErrIdpUnavailable     = errors.New("identity provider unavailable")
	ErrDemoAccount        = errors.New("demo accounts cannot be deleted")
	errRetryResolve       = errors.New("retry resolve")
)

// ValidationError maps to 422 {"error": Code, "field": Field}.
type ValidationError struct{ Code, Field string }

func (e *ValidationError) Error() string { return e.Code + ": " + e.Field }

// BlockedError maps to 409 {"error": Code, "count": Count} on account deletion.
type BlockedError struct {
	Code  string // active_claims | active_listings
	Count int
}

func (e *BlockedError) Error() string { return fmt.Sprintf("%s: %d", e.Code, e.Count) }

// VerificationEmailInterval is the minimum gap between resend requests per account.
const VerificationEmailInterval = time.Minute

// Account is the signed-in caller as the rest of the backend sees it (feature 002 reads it via
// middleware.AccountFromContext). IDs are derived from the verified token, never from the client
// (FR-012).
type Account struct {
	ID             uuid.UUID
	Email          string
	EmailVerified  bool
	DisplayName    *string
	IsDemo         bool
	BuyerProfileID uuid.UUID
	CookProfileID  *uuid.UUID
	// Issuer and Subject identify the identity used on this request.
	Issuer  string
	Subject string
}

// Location is a WGS84 point.
type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// CookProfileView is the cook_profile shape of GET /me (contract).
type CookProfileView struct {
	ID                       uuid.UUID  `json:"id"`
	DisplayName              string     `json:"display_name"`
	OrgType                  string     `json:"org_type"`
	Phone                    *string    `json:"phone"`
	DefaultPickupAddressText *string    `json:"default_pickup_address_text"`
	DefaultPickupLocation    *Location  `json:"default_pickup_location"`
	VerificationStatus       string     `json:"verification_status"`
	KvkNumber                *string    `json:"kvk_number"`
	KvkTradeName             *string    `json:"kvk_trade_name"`
	RegistrationStatus       string     `json:"registration_status"`
	RegistrationCheckedAt    *time.Time `json:"registration_checked_at"`
	NvwaDeclaredAt           *time.Time `json:"nvwa_declared_at"`
	CanPublish               bool       `json:"can_publish"`
}

// NVWADeclaration is the cook's declaration (FR-019).
type NVWADeclaration struct {
	Accepted bool
	Version  string
}

// CreateCookProfileInput is the validated-by-the-service input of POST /me/cook-profile.
type CreateCookProfileInput struct {
	DisplayName string
	OrgType     string
	Phone       string
	AddressText string
	Location    *Location
	KvkNumber   string
	NVWA        *NVWADeclaration
}

// UpdateCookProfileInput is PATCH /me/cook-profile; nil fields are left unchanged.
type UpdateCookProfileInput struct {
	DisplayName *string
	OrgType     *string
	Phone       *string
	AddressText *string
	Location    *Location
	KvkNumber   *string
	NVWA        *NVWADeclaration
}

// TxFunc runs fn inside one database transaction.
type TxFunc func(ctx context.Context, fn func(db.Querier) error) error

// AccountService holds the business rules of feature 001: provisioning, linking, cook onboarding,
// registration gate and deletion.
type AccountService struct {
	q   db.Querier
	tx  TxFunc
	reg registry.BusinessRegister
	idp idp.Provider
	now func() time.Time

	mu        sync.Mutex
	lastEmail map[uuid.UUID]time.Time
}

// NewAccountService wires the service to a connection pool.
func NewAccountService(pool *pgxpool.Pool, reg registry.BusinessRegister, p idp.Provider) *AccountService {
	return newAccountService(db.New(pool), PoolTx(pool), reg, p)
}

func newAccountService(q db.Querier, tx TxFunc, reg registry.BusinessRegister, p idp.Provider) *AccountService {
	if p == nil {
		p = idp.Noop{}
	}
	return &AccountService{q: q, tx: tx, reg: reg, idp: p, now: time.Now, lastEmail: map[uuid.UUID]time.Time{}}
}

// PoolTx returns a TxFunc backed by a pgx pool.
func PoolTx(pool *pgxpool.Pool) TxFunc {
	return func(ctx context.Context, fn func(db.Querier) error) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if err := fn(db.New(tx)); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

// ---------------------------------------------------------------------------------------------
// Provisioning and linking (FR-007, T009)
// ---------------------------------------------------------------------------------------------

// ResolveAccount maps verified token claims to an account:
//  1. a known (issuer, subject) identity -> its account (email_verified refreshed);
//  2. otherwise, when the token's email is verified AND an account owns that verified email ->
//     the identity is attached to it (FR-007);
//  3. otherwise a new account, identity and buyer profile are created.
//
// Unverified emails never link. Deleted accounts are rejected. The demo issuer only ever resolves
// to existing seeded identities.
func (s *AccountService) ResolveAccount(ctx context.Context, c auth.Claims) (*Account, error) {
	for attempt := 0; attempt < 3; attempt++ {
		acct, err := s.resolveOnce(ctx, c)
		if errors.Is(err, errRetryResolve) {
			continue
		}
		return acct, err
	}
	return nil, errors.New("resolve account: too many concurrent changes")
}

func (s *AccountService) resolveOnce(ctx context.Context, c auth.Claims) (*Account, error) {
	a, err := s.q.GetAccountByIdentity(ctx, db.GetAccountByIdentityParams{Issuer: c.Issuer, Subject: c.Subject})
	switch {
	case err == nil:
		if a.DeletedAt.Valid {
			return nil, ErrAccountDeleted
		}
		a, err = s.refreshEmailVerified(ctx, a, c)
		if err != nil {
			return nil, err
		}
		return s.load(ctx, a, c)
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}

	if c.Issuer == auth.DemoIssuerName {
		return nil, ErrUnknownIdentity
	}
	// A token issued at or before the identity's deletion must not re-create the account (US4);
	// a token issued after it is a fresh sign-in and gets a new account.
	if deletedAt, err := s.q.GetDeletedIdentity(ctx, db.GetDeletedIdentityParams{Issuer: c.Issuer, Subject: c.Subject}); err == nil {
		if !c.IssuedAt.After(deletedAt.Time) {
			return nil, ErrAccountDeleted
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if c.Email == "" {
		return nil, ErrEmailMissing
	}

	if c.EmailVerified {
		linked, err := s.linkByVerifiedEmail(ctx, c)
		if err != nil {
			return nil, err
		}
		if linked != nil {
			return s.load(ctx, *linked, c)
		}
	}

	var created db.Account
	err = s.tx(ctx, func(q db.Querier) error {
		email := c.Email
		var name *string
		if n := displayNameFromClaims(c); n != "" {
			name = &n
		}
		acc, err := q.CreateAccount(ctx, db.CreateAccountParams{Email: &email, EmailVerified: c.EmailVerified, DisplayName: name})
		if err != nil {
			return err
		}
		if _, err := q.CreateIdentity(ctx, db.CreateIdentityParams{AccountID: acc.ID, Issuer: c.Issuer, Subject: c.Subject}); err != nil {
			return err
		}
		if _, err := q.CreateBuyerProfile(ctx, acc.ID); err != nil {
			return err
		}
		created = acc
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			// Same identity created concurrently, or a verified owner of the email appeared.
			return nil, errRetryResolve
		}
		return nil, err
	}
	return s.load(ctx, created, c)
}

// linkByVerifiedEmail attaches the token's identity to the live account owning the token's
// verified email (FR-007). The owner row is read FOR SHARE in the same transaction as the identity
// insert, so a concurrent deletion of that account cannot slip in between: it either waits for
// the link (and then removes the new identity too) or has committed first (no owner is found).
// Returns nil when no account owns the email.
func (s *AccountService) linkByVerifiedEmail(ctx context.Context, c auth.Claims) (*db.Account, error) {
	var owner *db.Account
	err := s.tx(ctx, func(q db.Querier) error {
		email := c.Email
		existing, err := q.GetAccountByVerifiedEmailForShare(ctx, &email)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := q.CreateIdentity(ctx, db.CreateIdentityParams{AccountID: existing.ID, Issuer: c.Issuer, Subject: c.Subject}); err != nil {
			return err
		}
		owner = &existing
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errRetryResolve
		}
		return nil, err
	}
	return owner, nil
}

func displayNameFromClaims(c auth.Claims) string {
	if c.Name != "" {
		return c.Name
	}
	if i := strings.IndexByte(c.Email, '@'); i > 0 {
		return c.Email[:i]
	}
	return ""
}

// refreshEmailVerified caches the token's email_verified on the account. When a previously
// unverified account becomes verified and another account already owns that verified email, the
// two are the same person (FR-007): the identity moves to the owner if this account is empty.
func (s *AccountService) refreshEmailVerified(ctx context.Context, a db.Account, c auth.Claims) (db.Account, error) {
	if a.EmailVerified == c.EmailVerified {
		return a, nil
	}
	if a.Email != nil && c.Email != "" && !strings.EqualFold(*a.Email, c.Email) {
		return a, nil // token speaks about another email than the account's: leave the flag alone
	}
	err := s.q.UpdateAccountEmailVerified(ctx, db.UpdateAccountEmailVerifiedParams{ID: a.ID, EmailVerified: c.EmailVerified})
	if err == nil {
		a.EmailVerified = c.EmailVerified
		return a, nil
	}
	if !isUniqueViolation(err) || a.Email == nil {
		return a, err
	}
	owner, oerr := s.q.GetAccountByVerifiedEmail(ctx, a.Email)
	if oerr != nil || owner.ID == a.ID {
		return a, oerr
	}
	empty, eerr := s.q.AccountIsEmpty(ctx, a.ID)
	if eerr != nil {
		return a, eerr
	}
	if empty == nil || !*empty {
		log.Printf("accounts: %s verified its email but another account owns it and this one has data; not merging", uuid.UUID(a.ID.Bytes))
		return a, nil
	}
	err = s.tx(ctx, func(q db.Querier) error {
		if _, err := q.MoveIdentities(ctx, db.MoveIdentitiesParams{ToAccount: owner.ID, FromAccount: a.ID}); err != nil {
			return err
		}
		if err := q.DeleteBuyerProfileByAccount(ctx, a.ID); err != nil {
			return err
		}
		return q.DeleteAccountRow(ctx, a.ID)
	})
	if err != nil {
		return a, err
	}
	return owner, nil
}

func (s *AccountService) load(ctx context.Context, a db.Account, c auth.Claims) (*Account, error) {
	out := &Account{
		ID:            uuid.UUID(a.ID.Bytes),
		EmailVerified: a.EmailVerified,
		DisplayName:   a.DisplayName,
		IsDemo:        a.IsDemo,
		Issuer:        c.Issuer,
		Subject:       c.Subject,
	}
	if a.Email != nil {
		out.Email = *a.Email
	}
	bp, err := s.q.GetBuyerProfileByAccount(ctx, a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		bp, err = s.q.CreateBuyerProfile(ctx, a.ID)
		if isUniqueViolation(err) {
			bp, err = s.q.GetBuyerProfileByAccount(ctx, a.ID)
		}
	}
	if err != nil {
		return nil, err
	}
	out.BuyerProfileID = uuid.UUID(bp.ID.Bytes)
	cp, err := s.q.GetCookProfileByAccount(ctx, a.ID)
	switch {
	case err == nil:
		id := uuid.UUID(cp.ID.Bytes)
		out.CookProfileID = &id
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	return out, nil
}

// UpdateDisplayName implements PATCH /me.
func (s *AccountService) UpdateDisplayName(ctx context.Context, accountID uuid.UUID, name string) (*string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &ValidationError{"missing_field", "display_name"}
	}
	if utf8.RuneCountInString(name) > 100 {
		return nil, &ValidationError{"invalid_field", "display_name"}
	}
	acc, err := s.q.UpdateAccountDisplayName(ctx, db.UpdateAccountDisplayNameParams{ID: pgUUID(accountID), DisplayName: &name})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountDeleted
	}
	if err != nil {
		return nil, err
	}
	return acc.DisplayName, nil
}

// ---------------------------------------------------------------------------------------------
// Verification email (FR-006)
// ---------------------------------------------------------------------------------------------

// RequestVerificationEmail asks the provider to resend the verification email, at most once per
// minute per account (single-instance limiter).
func (s *AccountService) RequestVerificationEmail(ctx context.Context, a *Account) error {
	if a.EmailVerified {
		return ErrAlreadyVerified
	}
	now := s.now()
	s.mu.Lock()
	for id, t := range s.lastEmail {
		if now.Sub(t) >= VerificationEmailInterval {
			delete(s.lastEmail, id)
		}
	}
	if _, limited := s.lastEmail[a.ID]; limited {
		s.mu.Unlock()
		return ErrRateLimited
	}
	s.lastEmail[a.ID] = now
	s.mu.Unlock()

	if !s.idp.Manages(a.Issuer) {
		return nil // mock/demo issuers: nothing to send
	}
	if err := s.idp.SendVerificationEmail(ctx, a.Subject); err != nil {
		s.mu.Lock()
		delete(s.lastEmail, a.ID)
		s.mu.Unlock()
		log.Printf("accounts: verification email failed: %v", err)
		return ErrIdpUnavailable
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// Cook onboarding (FR-009, FR-010, FR-018, FR-019)
// ---------------------------------------------------------------------------------------------

var (
	kvkPattern   = regexp.MustCompile(`^[0-9]{8}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()\-]{4,30}$`)
)

const registerTimeout = 8 * time.Second

// GetCookProfile returns the account's cook profile, or nil when it has none.
func (s *AccountService) GetCookProfile(ctx context.Context, accountID uuid.UUID) (*CookProfileView, error) {
	row, err := s.q.GetCookProfileByAccount(ctx, pgUUID(accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cookView(row), nil
}

func cookView(r db.GetCookProfileByAccountRow) *CookProfileView {
	v := &CookProfileView{
		ID:                       uuid.UUID(r.ID.Bytes),
		DisplayName:              r.DisplayName,
		OrgType:                  string(r.OrgType),
		Phone:                    r.Phone,
		DefaultPickupAddressText: r.DefaultPickupAddressText,
		VerificationStatus:       string(r.VerificationStatus),
		KvkNumber:                r.KvkNumber,
		KvkTradeName:             r.KvkTradeName,
		RegistrationStatus:       string(r.RegistrationStatus),
		RegistrationCheckedAt:    tsPtr(r.RegistrationCheckedAt),
		NvwaDeclaredAt:           tsPtr(r.NvwaDeclaredAt),
	}
	// The pickup point and address are set together (DB check); ST_Y/ST_X of NULL scan as 0.
	if r.DefaultPickupAddressText != nil {
		v.DefaultPickupLocation = &Location{Lat: r.DefaultPickupLat, Lng: r.DefaultPickupLng}
	}
	v.CanPublish = r.RegistrationStatus == db.CookRegistrationStatusActive && r.NvwaDeclaredAt.Valid && !r.AnonymisedAt.Valid
	return v
}

// CreateCookProfile implements POST /me/cook-profile. Nothing is saved when it returns an error.
// A register outage saves the profile as 'pending' (callers answer 202).
func (s *AccountService) CreateCookProfile(ctx context.Context, accountID uuid.UUID, in CreateCookProfileInput) (*CookProfileView, error) {
	if existing, err := s.GetCookProfile(ctx, accountID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrCookProfileExists
	}
	if err := validateCreate(in); err != nil {
		return nil, err
	}

	res, regErr := s.checkRegister(ctx, in.KvkNumber)
	if regErr == nil {
		switch res.Status {
		case registry.StatusNotFound:
			return nil, ErrKvkNotFound
		case registry.StatusInactive:
			return nil, ErrKvkInactive
		}
	}

	now := s.now()
	phone, addr, kvk, ver := strings.TrimSpace(in.Phone), strings.TrimSpace(in.AddressText), in.KvkNumber, strings.TrimSpace(in.NVWA.Version)
	p := db.CreateCookProfileParams{
		AccountID:                pgUUID(accountID),
		DisplayName:              strings.TrimSpace(in.DisplayName),
		OrgType:                  db.CookOrgType(in.OrgType),
		Phone:                    &phone,
		DefaultPickupAddressText: &addr,
		PickupLng:                in.Location.Lng,
		PickupLat:                in.Location.Lat,
		KvkNumber:                &kvk,
		RegistrationStatus:       db.CookRegistrationStatusPending,
		NvwaDeclaredAt:           pgtype.Timestamptz{Time: now, Valid: true},
		NvwaDeclarationVersion:   &ver,
	}
	if regErr == nil { // active
		name := res.TradeName
		p.RegistrationStatus = db.CookRegistrationStatusActive
		p.KvkTradeName = &name
		p.RegistrationCheckedAt = pgtype.Timestamptz{Time: now, Valid: true}
	} else {
		log.Printf("accounts: business register unavailable during onboarding: %v", regErr)
	}
	if _, err := s.q.CreateCookProfile(ctx, p); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrCookProfileExists
		}
		return nil, err
	}
	return s.GetCookProfile(ctx, accountID)
}

// UpdateCookProfile implements PATCH /me/cook-profile. A changed KvK number is checked first; if
// the register rejects it nothing is saved and the old number stays in effect.
//
// It also reports whether the KvK number actually changed (handlers answer 202 only for a changed
// number whose check is still pending). Sending the unchanged number of a profile whose
// registration is not 'active' triggers a fresh register check without counting as a change.
func (s *AccountService) UpdateCookProfile(ctx context.Context, accountID uuid.UUID, in UpdateCookProfileInput) (view *CookProfileView, kvkChanged bool, err error) {
	cur, err := s.q.GetCookProfileByAccount(ctx, pgUUID(accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrCookProfileMissing
	}
	if err != nil {
		return nil, false, err
	}
	if err := validateUpdate(in); err != nil {
		return nil, false, err
	}

	upd := db.UpdateCookProfileParams{ID: cur.ID}
	if in.DisplayName != nil {
		v := strings.TrimSpace(*in.DisplayName)
		upd.DisplayName = &v
	}
	if in.OrgType != nil {
		v := db.CookOrgType(*in.OrgType)
		upd.OrgType = &v
	}
	if in.Phone != nil {
		v := strings.TrimSpace(*in.Phone)
		upd.Phone = &v
	}
	if in.AddressText != nil {
		v := strings.TrimSpace(*in.AddressText)
		upd.DefaultPickupAddressText = &v
	}
	if in.Location != nil {
		upd.PickupLat, upd.PickupLng = &in.Location.Lat, &in.Location.Lng
	}

	kvkChanged = in.KvkNumber != nil && (cur.KvkNumber == nil || *cur.KvkNumber != *in.KvkNumber)
	recheck := kvkChanged || (in.KvkNumber != nil && cur.RegistrationStatus != db.CookRegistrationStatusActive)
	var kvkParams db.UpdateCookKvkRegistrationParams
	if recheck {
		res, regErr := s.checkRegister(ctx, *in.KvkNumber)
		if regErr == nil {
			switch res.Status {
			case registry.StatusNotFound:
				return nil, false, ErrKvkNotFound
			case registry.StatusInactive:
				return nil, false, ErrKvkInactive
			}
		}
		kvk := *in.KvkNumber
		kvkParams = db.UpdateCookKvkRegistrationParams{ID: cur.ID, KvkNumber: &kvk, RegistrationStatus: db.CookRegistrationStatusPending}
		if regErr == nil {
			name := res.TradeName
			kvkParams.RegistrationStatus = db.CookRegistrationStatusActive
			kvkParams.KvkTradeName = &name
			kvkParams.RegistrationCheckedAt = pgtype.Timestamptz{Time: s.now(), Valid: true}
		} else {
			log.Printf("accounts: business register unavailable during KvK change: %v", regErr)
			if !kvkChanged {
				recheck = false // unchanged number and no answer: keep the stored status
			}
		}
	}

	err = s.tx(ctx, func(q db.Querier) error {
		if _, err := q.UpdateCookProfile(ctx, upd); err != nil {
			return err
		}
		if recheck {
			if _, err := q.UpdateCookKvkRegistration(ctx, kvkParams); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	view, err = s.GetCookProfile(ctx, accountID)
	return view, kvkChanged, err
}

func (s *AccountService) checkRegister(ctx context.Context, kvk string) (registry.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	return s.reg.Check(ctx, kvk)
}

func validateCreate(in CreateCookProfileInput) error {
	for _, f := range []struct{ name, val string }{
		{"display_name", in.DisplayName}, {"org_type", in.OrgType}, {"phone", in.Phone},
		{"default_pickup_address_text", in.AddressText}, {"kvk_number", in.KvkNumber},
	} {
		if strings.TrimSpace(f.val) == "" {
			return &ValidationError{"missing_field", f.name}
		}
	}
	if in.Location == nil {
		return &ValidationError{"missing_field", "default_pickup_location"}
	}
	if err := validateFormats(&in.DisplayName, &in.OrgType, &in.Phone, &in.AddressText, in.Location, &in.KvkNumber); err != nil {
		return err
	}
	if in.NVWA == nil || !in.NVWA.Accepted {
		return ErrDeclarationNeeded
	}
	if strings.TrimSpace(in.NVWA.Version) == "" || utf8.RuneCountInString(in.NVWA.Version) > 32 {
		return &ValidationError{"invalid_field", "nvwa_declaration.version"}
	}
	return nil
}

func validateUpdate(in UpdateCookProfileInput) error {
	if in.NVWA != nil && !in.NVWA.Accepted {
		return &ValidationError{"invalid_field", "nvwa_declaration"} // cannot be withdrawn here
	}
	for _, f := range []struct {
		name string
		val  *string
	}{
		{"display_name", in.DisplayName}, {"org_type", in.OrgType}, {"phone", in.Phone},
		{"default_pickup_address_text", in.AddressText}, {"kvk_number", in.KvkNumber},
	} {
		if f.val != nil && strings.TrimSpace(*f.val) == "" {
			return &ValidationError{"invalid_field", f.name}
		}
	}
	// The address text and its point describe one pickup place (DB check); change them together.
	if in.AddressText != nil && in.Location == nil {
		return &ValidationError{"missing_field", "default_pickup_location"}
	}
	if in.Location != nil && in.AddressText == nil {
		return &ValidationError{"missing_field", "default_pickup_address_text"}
	}
	return validateFormats(in.DisplayName, in.OrgType, in.Phone, in.AddressText, in.Location, in.KvkNumber)
}

// validateFormats checks the fields that are present (non-nil).
func validateFormats(name, org, phone, addr *string, loc *Location, kvk *string) error {
	if name != nil && utf8.RuneCountInString(strings.TrimSpace(*name)) > 100 {
		return &ValidationError{"invalid_field", "display_name"}
	}
	if org != nil && *org != "nonprofit" && *org != "professional" {
		return &ValidationError{"invalid_field", "org_type"}
	}
	if phone != nil && !phonePattern.MatchString(strings.TrimSpace(*phone)) {
		return &ValidationError{"invalid_field", "phone"}
	}
	if addr != nil && utf8.RuneCountInString(strings.TrimSpace(*addr)) > 300 {
		return &ValidationError{"invalid_field", "default_pickup_address_text"}
	}
	if loc != nil && (loc.Lat < -90 || loc.Lat > 90 || loc.Lng < -180 || loc.Lng > 180) {
		return &ValidationError{"invalid_field", "default_pickup_location"}
	}
	if kvk != nil && !kvkPattern.MatchString(*kvk) {
		return &ValidationError{"invalid_field", "kvk_number"}
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// Guards used by feature 002 (FR-011, FR-020)
// ---------------------------------------------------------------------------------------------

// AccountOwnsCookProfile reports whether the cook profile belongs to the account. Feature 002 uses
// it for the own-listing guard: a claim on a listing whose cook profile the buyer's account owns
// is rejected with 403 own_listing (FR-011).
func (s *AccountService) AccountOwnsCookProfile(ctx context.Context, accountID, cookProfileID uuid.UUID) (bool, error) {
	return s.q.AccountOwnsCookProfile(ctx, db.AccountOwnsCookProfileParams{ID: pgUUID(cookProfileID), AccountID: pgUUID(accountID)})
}

// CookCanPublish reports whether the cook may publish (FR-020): registration active and the NVWA
// declaration on record. Independent of verification_status (Principle I); feature 002's listing
// service calls it before publishing.
func (s *AccountService) CookCanPublish(ctx context.Context, cookProfileID uuid.UUID) (bool, error) {
	return s.q.CookCanPublish(ctx, pgUUID(cookProfileID))
}

// CookPublishGate returns canPublish plus, when false, the registration status to show in the
// cook_not_registered body ("pending" or "inactive").
func (s *AccountService) CookPublishGate(ctx context.Context, cookProfileID uuid.UUID) (bool, string, error) {
	ok, err := s.CookCanPublish(ctx, cookProfileID)
	if err != nil || ok {
		return ok, "", err
	}
	cp, err := s.q.GetCookProfileByID(ctx, pgUUID(cookProfileID))
	if err != nil {
		return false, "", err
	}
	if cp.RegistrationStatus == db.CookRegistrationStatusInactive {
		return false, "inactive", nil
	}
	return false, "pending", nil
}

// ---------------------------------------------------------------------------------------------
// Deletion (FR-013, T028)
// ---------------------------------------------------------------------------------------------

// DeleteAccount erases the account's personal data in one transaction and queues deletion at the
// identity provider. It is blocked while the buyer has live claims or the cook has listings with
// live claims. The cook's verification history, ratings and listings stay linked to the
// anonymised profile (SC-004). Published listings without live claims are cancelled
// (published -> cancelled) so nothing stays claimable under "Former cook".
func (s *AccountService) DeleteAccount(ctx context.Context, a *Account) error {
	if a.IsDemo {
		return ErrDemoAccount
	}
	return s.tx(ctx, func(q db.Querier) error {
		// Serialise with concurrent linking, sign-in refreshes and other deletions of this account.
		acc, err := q.GetAccountForUpdate(ctx, pgUUID(a.ID))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAccountDeleted
		}
		if err != nil {
			return err
		}
		if acc.DeletedAt.Valid {
			return ErrAccountDeleted
		}
		// New claims reference the buyer profile; locking it makes them wait for this deletion.
		if _, err := q.LockBuyerProfileForAccount(ctx, pgUUID(a.ID)); err != nil {
			return err
		}
		n, err := q.CountActiveClaimsForBuyer(ctx, pgUUID(a.BuyerProfileID))
		if err != nil {
			return err
		}
		if n > 0 {
			return &BlockedError{Code: "active_claims", Count: int(n)}
		}
		if a.CookProfileID != nil {
			cook := pgUUID(*a.CookProfileID)
			// Cancel first (this locks the listing rows, so an in-flight claim on one of them
			// commits before we continue), then count: a live claim found here rolls the whole
			// transaction back, including the cancellations.
			if _, err := q.CancelPublishedListingsForCook(ctx, cook); err != nil {
				return err
			}
			n, err := q.CountListingsWithLiveClaimsForCook(ctx, cook)
			if err != nil {
				return err
			}
			if n > 0 {
				return &BlockedError{Code: "active_listings", Count: int(n)}
			}
			// The cook's pickup addresses must not outlive the profile.
			if _, err := q.ScrubListingPickupForCook(ctx, cook); err != nil {
				return err
			}
			if _, err := q.AnonymiseCookProfile(ctx, pgUUID(a.ID)); err != nil {
				return err
			}
		}
		ids, err := q.DeleteIdentitiesByAccount(ctx, pgUUID(a.ID))
		if err != nil {
			return err
		}
		for _, id := range ids {
			// Tombstone first: tokens issued before now must not re-create the account.
			if err := q.InsertDeletedIdentity(ctx, db.InsertDeletedIdentityParams{Issuer: id.Issuer, Subject: id.Subject}); err != nil {
				return err
			}
			if !s.idp.Manages(id.Issuer) {
				continue
			}
			if err := q.EnqueueIdpDeletion(ctx, db.EnqueueIdpDeletionParams{Issuer: id.Issuer, Subject: id.Subject}); err != nil {
				return err
			}
		}
		rows, err := q.AnonymiseAccount(ctx, pgUUID(a.ID))
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrAccountDeleted
		}
		return nil
	})
}

// ---------------------------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------------------------

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

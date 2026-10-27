package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/registry"
	"github.com/eyho/eyho/tests/testutil"
)

const testutil_active = registry.FakeActiveNumber1

func validCook(kvk string) map[string]any {
	return map[string]any{
		"display_name":                "Sanne's Kitchen",
		"org_type":                    "professional",
		"phone":                       "+31612345678",
		"default_pickup_address_text": "Keizersgracht 88, Amsterdam",
		"default_pickup_location":     map[string]float64{"lat": 52.37, "lng": 4.89},
		"kvk_number":                  kvk,
		"nvwa_declaration":            map[string]any{"accepted": true, "version": "2026-10-25"},
	}
}

func without(m map[string]any, key string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func verifiedBuyer(e *testutil.Env, prefix string) (token, email string) {
	email, sub := testutil.Unique(prefix)
	return e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true}), email
}

// Quickstart Scenario 5 steps 1-4 (SC-007): validation, declaration, register verdicts, success.
func TestOnboarding_RegistrationCheck(t *testing.T) {
	e := testutil.NewEnv(t, false)
	tok, _ := verifiedBuyer(e, "cook")

	// Step 1: missing phone.
	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, without(validCook(registry.FakeActiveNumber1), "phone"))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string]any{"error": "missing_field", "field": "phone"}, testutil.JSON(t, rec))

	// Step 2: no declaration.
	rec = e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, without(validCook(registry.FakeActiveNumber1), "nvwa_declaration"))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "declaration_required", testutil.JSON(t, rec)["error"])

	// Step 3: unknown and inactive numbers; nothing is saved.
	rec = e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook("12345678"))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "kvk_not_found", testutil.JSON(t, rec)["error"])
	rec = e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook(registry.FakeInactiveNumber))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "kvk_inactive", testutil.JSON(t, rec)["error"])
	assert.Nil(t, testutil.JSON(t, e.Do(http.MethodGet, "/api/v1/me", tok, nil))["cook_profile"])

	// Step 4: success.
	rec = e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook(registry.FakeActiveNumber1))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	cp := testutil.JSON(t, rec)
	assert.Equal(t, "unverified", cp["verification_status"], "new cooks start unverified (Principle I)")
	assert.Equal(t, "active", cp["registration_status"])
	assert.Equal(t, "Keuken Katrien B.V.", cp["kvk_trade_name"])
	assert.Equal(t, true, cp["can_publish"])
	assert.NotNil(t, cp["nvwa_declared_at"])
	assert.NotNil(t, cp["registration_checked_at"])
	assert.Equal(t, map[string]any{"lat": 52.37, "lng": 4.89}, cp["default_pickup_location"])

	// GET /me shows it; a second onboarding is a conflict.
	me := testutil.JSON(t, e.Do(http.MethodGet, "/api/v1/me", tok, nil))
	assert.Equal(t, cp["id"], me["cook_profile"].(map[string]any)["id"])
	rec = e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook(registry.FakeActiveNumber1))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "cook_profile_exists", testutil.JSON(t, rec)["error"])
}

func TestOnboarding_PatchAndKvkChange(t *testing.T) {
	e := testutil.NewEnv(t, false)
	tok, _ := verifiedBuyer(e, "patchcook")

	// A buyer without a cook profile cannot PATCH it.
	rec := e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"phone": "+31611111111"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "cook_profile_required", testutil.JSON(t, rec)["error"])

	require.Equal(t, http.StatusCreated, e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook(registry.FakeActiveNumber1)).Code)

	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"phone": "+31611111111", "default_pickup_address_text": "Nieuwe Gracht 1", "default_pickup_location": map[string]float64{"lat": 52.1, "lng": 4.3}})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "+31611111111", testutil.JSON(t, rec)["phone"])

	// The pickup address text and point change together (N10).
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"default_pickup_address_text": "Elders 2"})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string]any{"error": "missing_field", "field": "default_pickup_location"}, testutil.JSON(t, rec))
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"default_pickup_location": map[string]float64{"lat": 52.1, "lng": 4.3}})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string]any{"error": "missing_field", "field": "default_pickup_address_text"}, testutil.JSON(t, rec))

	// Sending the unchanged number of an active profile is a plain 200 (N11).
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeActiveNumber1})
	require.Equal(t, http.StatusOK, rec.Code)

	// A rejected new number leaves the old one in effect.
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeInactiveNumber})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "kvk_inactive", testutil.JSON(t, rec)["error"])
	cp := testutil.JSON(t, e.Do(http.MethodGet, "/api/v1/me", tok, nil))["cook_profile"].(map[string]any)
	assert.Equal(t, registry.FakeActiveNumber1, cp["kvk_number"])

	// A new valid number is checked and applied.
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeActiveNumber2})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Test Eetcafe Jordaan", testutil.JSON(t, rec)["kvk_trade_name"])

	// Register down during the change: 202 and publishing is blocked until it is confirmed.
	e.Register.SetDown(true)
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeActiveNumber1})
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "pending", testutil.JSON(t, rec)["registration_status"])
	assert.Equal(t, false, testutil.JSON(t, rec)["can_publish"])

	// The same number again while pending triggers a re-check; still down -> plain 200, not 202,
	// because the number did not change (N11, S6).
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeActiveNumber1})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "pending", testutil.JSON(t, rec)["registration_status"])
	// Register back: the same number now confirms the registration immediately.
	e.Register.SetDown(false)
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"kvk_number": registry.FakeActiveNumber1})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "active", testutil.JSON(t, rec)["registration_status"])
	assert.Equal(t, true, testutil.JSON(t, rec)["can_publish"])
	e.Register.SetDown(true)
	e.Register.Set(registry.FakeActiveNumber1, registry.StatusActive, "Keuken Katrien B.V.")

	// The declaration cannot be withdrawn here.
	rec = e.Do(http.MethodPatch, "/api/v1/me/cook-profile", tok, map[string]any{"nvwa_declaration": map[string]any{"accepted": false}})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// filterQ restricts the re-check job to the profile under test so shared seed data is untouched.
type filterQ struct {
	db.Querier
	only pgtype.UUID
}

func (f filterQ) ListCookProfilesDueForRecheck(ctx context.Context, a db.ListCookProfilesDueForRecheckParams) ([]db.ListCookProfilesDueForRecheckRow, error) {
	rows, err := f.Querier.ListCookProfilesDueForRecheck(ctx, a)
	var out []db.ListCookProfilesDueForRecheckRow
	for _, r := range rows {
		if r.ID == f.only {
			out = append(out, r)
		}
	}
	return out, err
}

// Quickstart Scenario 8: the registration gate over time.
func TestRegistrationGate_OverTime(t *testing.T) {
	e := testutil.NewEnv(t, false)
	ctx := context.Background()
	tok, _ := verifiedBuyer(e, "gate")
	const kvk = "70000001"
	e.Register.Set(kvk, registry.StatusActive, "Gate Kitchen B.V.")

	// 1. Register down: onboarding is accepted but pending, and publishing is refused.
	e.Register.SetDown(true)
	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, validCook(kvk))
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	cp := testutil.JSON(t, rec)
	assert.Equal(t, "pending", cp["registration_status"])
	assert.Equal(t, false, cp["can_publish"])
	cookID := uuid.MustParse(cp["id"].(string))
	ok, status, err := e.Svc.CookPublishGate(ctx, cookID)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "pending", status)

	now := time.Now()
	job := &registry.RecheckJob{
		Q:        filterQ{Querier: e.Q, only: pgtype.UUID{Bytes: cookID, Valid: true}},
		Register: e.Register,
		Now:      func() time.Time { return now },
	}

	// Still down: the job retries, fails, nothing changes.
	n, err := job.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// 2. Restored: after the backoff window the retry confirms the registration.
	e.Register.SetDown(false)
	n, _ = job.RunOnce(ctx)
	assert.Equal(t, 0, n, "inside the backoff window")
	now = now.Add(2 * time.Minute)
	n, err = job.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	ok, _, _ = e.Svc.CookPublishGate(ctx, cookID)
	assert.True(t, ok)
	view, err := e.Svc.GetCookProfile(ctx, accountOf(t, e, tok))
	require.NoError(t, err)
	assert.Equal(t, "active", view.RegistrationStatus)
	assert.Equal(t, "Gate Kitchen B.V.", *view.KvkTradeName)

	// 4. A register outage during a 30-day re-check never downgrades an active cook.
	later := now.Add(31 * 24 * time.Hour)
	job.Now = func() time.Time { return later }
	e.Register.SetDown(true)
	n, _ = job.RunOnce(ctx)
	assert.Equal(t, 0, n)
	ok, _, _ = e.Svc.CookPublishGate(ctx, cookID)
	assert.True(t, ok, "status stays active when the register is unreachable")
	later = later.Add(2 * time.Minute) // failures back off for every status, not only pending
	job.Now = func() time.Time { return later }

	// 3. The number becomes inactive: the next re-check blocks publishing.
	e.Register.SetDown(false)
	e.Register.Set(kvk, registry.StatusInactive, "Gate Kitchen B.V.")
	n, err = job.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	ok, status, _ = e.Svc.CookPublishGate(ctx, cookID)
	assert.False(t, ok)
	assert.Equal(t, "inactive", status)
	rec = e.Do(http.MethodGet, "/api/v1/me", tok, nil)
	got := testutil.JSON(t, rec)["cook_profile"].(map[string]any)
	assert.Equal(t, "inactive", got["registration_status"])
	assert.Equal(t, false, got["can_publish"])
	assert.Equal(t, "unverified", got["verification_status"], "registration never touches verification status")

	// Resolved later: an inactive cook whose number is active again is restored.
	e.Register.Set(kvk, registry.StatusActive, "Gate Kitchen B.V.")
	later = later.Add(31 * 24 * time.Hour)
	_, err = job.RunOnce(ctx)
	require.NoError(t, err)
	ok, _, _ = e.Svc.CookPublishGate(ctx, cookID)
	assert.True(t, ok)
}

func accountOf(t *testing.T, e *testutil.Env, token string) uuid.UUID {
	t.Helper()
	return uuid.MustParse(meID(t, e, token))
}

// FR-011 building block for feature 002: the own-listing guard.
func TestAccountOwnsCookProfile(t *testing.T) {
	e := testutil.NewEnv(t, false)
	ctx := context.Background()
	tokA, _ := verifiedBuyer(e, "ownA")
	tokB, _ := verifiedBuyer(e, "ownB")
	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", tokA, validCook(registry.FakeActiveNumber1))
	require.Equal(t, http.StatusCreated, rec.Code)
	cookA := uuid.MustParse(testutil.JSON(t, rec)["id"].(string))
	accA, accB := accountOf(t, e, tokA), accountOf(t, e, tokB)

	owns, err := e.Svc.AccountOwnsCookProfile(ctx, accA, cookA)
	require.NoError(t, err)
	assert.True(t, owns)
	owns, err = e.Svc.AccountOwnsCookProfile(ctx, accB, cookA)
	require.NoError(t, err)
	assert.False(t, owns)
}

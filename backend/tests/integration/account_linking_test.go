package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

func meID(t *testing.T, e *testutil.Env, token string) string {
	t.Helper()
	rec := e.Do(http.MethodGet, "/api/v1/me", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.JSON(t, rec)["id"].(string)
}

// Quickstart Scenario 4: same verified email through two identities is one account.
func TestLinking_SameVerifiedEmailIsOneAccount(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, subA := testutil.Unique("link")
	_, subB := testutil.Unique("link")
	a := e.Issuer.Token(testutil.TokenOptions{Subject: subA, Email: email, EmailVerified: true})
	b := e.Issuer.Token(testutil.TokenOptions{Subject: subB, Email: email, EmailVerified: true})

	idA := meID(t, e, a)
	assert.Equal(t, idA, meID(t, e, b))
	assert.Equal(t, idA, meID(t, e, a))
}

// Scenario 4 step 4: an unverified identity never links; a separate account is created.
func TestLinking_UnverifiedEmailNeverLinks(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, subA := testutil.Unique("nolink")
	_, subB := testutil.Unique("nolink")
	a := e.Issuer.Token(testutil.TokenOptions{Subject: subA, Email: email, EmailVerified: true})
	b := e.Issuer.Token(testutil.TokenOptions{Subject: subB, Email: email, EmailVerified: false})

	idA := meID(t, e, a)
	idB := meID(t, e, b)
	assert.NotEqual(t, idA, idB)
	assert.Equal(t, idB, meID(t, e, b))
}

// An unverified account holding someone's email must not block or capture the real owner.
func TestLinking_UnverifiedSquatterDoesNotCaptureOwner(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, subSquat := testutil.Unique("squat")
	_, subOwner := testutil.Unique("squat")
	squat := e.Issuer.Token(testutil.TokenOptions{Subject: subSquat, Email: email, EmailVerified: false})
	owner := e.Issuer.Token(testutil.TokenOptions{Subject: subOwner, Email: email, EmailVerified: true})

	idSquat := meID(t, e, squat)
	idOwner := meID(t, e, owner)
	assert.NotEqual(t, idSquat, idOwner)
}

// A password sign-up that verifies its email later joins the existing account (FR-007).
func TestLinking_LateVerificationMergesEmptyAccount(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, subGoogle := testutil.Unique("late")
	_, subPwd := testutil.Unique("late")
	google := e.Issuer.Token(testutil.TokenOptions{Subject: subGoogle, Email: email, EmailVerified: true})
	pwdUnverified := e.Issuer.Token(testutil.TokenOptions{Subject: subPwd, Email: email, EmailVerified: false})
	pwdVerified := e.Issuer.Token(testutil.TokenOptions{Subject: subPwd, Email: email, EmailVerified: true})

	idGoogle := meID(t, e, google)
	idPwd := meID(t, e, pwdUnverified)
	require.NotEqual(t, idGoogle, idPwd)

	assert.Equal(t, idGoogle, meID(t, e, pwdVerified))
	assert.Equal(t, idGoogle, meID(t, e, pwdUnverified), "the identity now belongs to the merged account")
}

// Scenario 3 / FR-006: unverified accounts are gated and can resend, at most once a minute.
func TestEmailVerification_GateAndResend(t *testing.T) {
	e := testutil.NewEnv(t, false)
	email, sub := testutil.Unique("verify")
	unverified := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: false})

	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", unverified, validCook(testutil_active))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "email_not_verified", testutil.JSON(t, rec)["error"])

	rec = e.Do(http.MethodPost, "/api/v1/me/verification-email", unverified, nil)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, []string{sub}, e.IdP.Emails)

	rec = e.Do(http.MethodPost, "/api/v1/me/verification-email", unverified, nil)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Len(t, e.IdP.Emails, 1)

	// The same user signs in again with a verified email: gate opens, resend is now a 409.
	verified := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email, EmailVerified: true})
	rec = e.Do(http.MethodGet, "/api/v1/me", verified, nil)
	assert.Equal(t, true, testutil.JSON(t, rec)["email_verified"])
	rec = e.Do(http.MethodPost, "/api/v1/me/verification-email", verified, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "already_verified", testutil.JSON(t, rec)["error"])
}

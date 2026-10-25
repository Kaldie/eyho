package integration

import (
	"context"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/src/registry"
	"github.com/eyho/eyho/tests/testutil"
)

// testCook is a signed-in cook with an active registration.
type testCook struct {
	Token     string
	CookID    uuid.UUID
	AccountID uuid.UUID
}

func newCook(t *testing.T, e *testutil.Env, prefix, orgType string) testCook {
	t.Helper()
	tok, _ := verifiedBuyer(e, prefix)
	body := validCook(registry.FakeActiveNumber1)
	body["org_type"] = orgType
	rec := e.Do(http.MethodPost, "/api/v1/me/cook-profile", tok, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return testCook{
		Token:     tok,
		CookID:    uuid.MustParse(testutil.JSON(t, rec)["id"].(string)),
		AccountID: accountOf(t, e, tok),
	}
}

// uniquePoint returns a coordinate in a random 0.5 degree cell of the mid-Atlantic so discovery
// assertions are not disturbed by other tests' or the demo seed's listings.
func uniquePoint() (lat, lng float64) {
	return 10 + rand.Float64()*20, -40 + rand.Float64()*20
}

func listingBody(lat, lng float64) map[string]any {
	return map[string]any{
		"food_description":             "Vegetable lasagne",
		"allergen_info":                "gluten, dairy",
		"total_portions":               4,
		"reduced_price_cents":          500,
		"retail_reference_price_cents": 1500,
		"pickup_location":              map[string]float64{"lat": lat, "lng": lng},
		"pickup_address_text":          "Teststraat 1",
		"pickup_window_start":          time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
		"pickup_window_end":            time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		"publish":                      true,
	}
}

// publish creates a published listing for the cook at the point and returns its id.
func publish(t *testing.T, e *testutil.Env, c testCook, lat, lng float64, portions int) string {
	t.Helper()
	b := listingBody(lat, lng)
	b["total_portions"] = portions
	rec := e.Do(http.MethodPost, "/api/v1/listings", c.Token, b)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return testutil.JSON(t, rec)["id"].(string)
}

// claimOK creates a claim that must succeed and returns its id.
func claimOK(t *testing.T, e *testutil.Env, token, listingID string, qty int) string {
	t.Helper()
	rec := e.Do(http.MethodPost, "/api/v1/claims", token, map[string]any{"listing_id": listingID, "quantity": qty})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return testutil.JSON(t, rec)["claim_id"].(string)
}

// payClaim delivers the payment_intent.succeeded webhook for the claim (amount in cents).
func payClaim(t *testing.T, e *testutil.Env, claimID string, cents int) {
	t.Helper()
	rec := e.Webhook(testutil.SucceedBody(claimID, cents), testutil.ValidSignature)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// paidClaim creates a claim of one portion at 500 cents and pays it.
func paidClaim(t *testing.T, e *testutil.Env, buyerToken, listingID string) string {
	t.Helper()
	id := claimOK(t, e, buyerToken, listingID, 1)
	payClaim(t, e, id, 500)
	return id
}

func collect(t *testing.T, e *testutil.Env, token, claimID string) {
	t.Helper()
	rec := e.Do(http.MethodPost, "/api/v1/claims/"+claimID+"/confirm-pickup", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func str(t *testing.T, e *testutil.Env, q string, args ...any) string {
	t.Helper()
	var s string
	require.NoError(t, e.Pool.QueryRow(context.Background(), q, args...).Scan(&s))
	return s
}

func exec(t *testing.T, e *testutil.Env, q string, args ...any) {
	t.Helper()
	_, err := e.Pool.Exec(context.Background(), q, args...)
	require.NoError(t, err)
}

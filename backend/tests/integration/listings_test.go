package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

func listNear(t *testing.T, e *testutil.Env, token string, lat, lng float64, extra string) []map[string]any {
	t.Helper()
	rec := e.Do(http.MethodGet, fmt.Sprintf("/api/v1/listings?lat=%f&lng=%f%s", lat, lng, extra), token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out []map[string]any
	for _, l := range testutil.JSON(t, rec)["listings"].([]any) {
		out = append(out, l.(map[string]any))
	}
	return out
}

func cookBlock(l map[string]any) map[string]any { return l["cook"].(map[string]any) }

// Scenario 1 (FR-001, FR-004): any registered cook publishes; unverified cooks are flagged.
func TestListings_Scenario1_UnverifiedCookPublishesAndIsFlagged(t *testing.T) {
	e := testutil.NewEnv(t, false)
	verified := newCook(t, e, "v1cook", "professional")
	unverified := newCook(t, e, "u1cook", "professional")
	exec(t, e, `UPDATE cook_profiles SET verification_status='verified', verification_cycle_count=1 WHERE id=$1`, verified.CookID)
	lat, lng := uniquePoint()

	for name, c := range map[string]testCook{"verified": verified, "unverified": unverified} {
		rec := e.Do(http.MethodPost, "/api/v1/listings", c.Token, listingBody(lat, lng))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		d := testutil.JSON(t, rec)
		assert.Equal(t, "published", d["status"])
		assert.Equal(t, name, cookBlock(d)["verification_status"])
		assert.EqualValues(t, 4, d["portions_remaining"])
		assert.Equal(t, "Teststraat 1", d["pickup_address_text"])
	}

	buyer, _ := verifiedBuyer(e, "v1buyer")
	items := listNear(t, e, buyer, lat, lng, "")
	require.Len(t, items, 2)
	statuses := map[string]bool{}
	for _, l := range items {
		statuses[cookBlock(l)["verification_status"].(string)] = true
		assert.Contains(t, cookBlock(l), "rating_avg")
		assert.Contains(t, cookBlock(l), "rating_count")
		assert.Contains(t, cookBlock(l), "verification_cycle_count")
		assert.Contains(t, l, "distance_km")
		assert.NotContains(t, l, "allergen_info", "list items follow the contract shape")
	}
	assert.Equal(t, map[string]bool{"verified": true, "unverified": true}, statuses)
}

// Registration gate (feature 001 FR-020, quickstart Scenario 1 step 6): drafts allowed, publishing refused.
func TestListings_UnregisteredCookCannotPublish(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "unregcook", "professional")
	lat, lng := uniquePoint()
	for _, st := range []string{"pending", "inactive"} {
		exec(t, e, `UPDATE cook_profiles SET registration_status=$2::cook_registration_status WHERE id=$1`, cook.CookID, st)
		rec := e.Do(http.MethodPost, "/api/v1/listings", cook.Token, listingBody(lat, lng))
		require.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, map[string]any{"error": "cook_not_registered", "registration_status": st}, testutil.JSON(t, rec))

		b := listingBody(lat, lng)
		b["publish"] = false
		rec = e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
		require.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "draft", testutil.JSON(t, rec)["status"])
	}
	assert.Empty(t, listNear(t, e, "", lat, lng, ""))
}

func TestListings_CreateValidationAndAuth(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "valcook2", "professional")
	buyer, _ := verifiedBuyer(e, "valbuyer2")
	lat, lng := uniquePoint()
	post := func(tok string, mut func(map[string]any)) (int, map[string]any) {
		b := listingBody(lat, lng)
		mut(b)
		rec := e.Do(http.MethodPost, "/api/v1/listings", tok, b)
		return rec.Code, testutil.JSON(t, rec)
	}

	code, body := post("", func(map[string]any) {})
	assert.Equal(t, http.StatusUnauthorized, code, "%v", body)
	code, body = post(buyer, func(map[string]any) {})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "cook_profile_required", body["error"])
	email, sub := testutil.Unique("unvcook")
	unv := e.Issuer.Token(testutil.TokenOptions{Subject: sub, Email: email})
	code, body = post(unv, func(map[string]any) {})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "email_not_verified", body["error"])

	cases := []struct {
		name        string
		mut         func(map[string]any)
		code, field string
	}{
		{"price equal to retail", func(b map[string]any) { b["reduced_price_cents"] = 1500 }, "price_not_reduced", "reduced_price_cents"},
		{"price above retail", func(b map[string]any) { b["reduced_price_cents"] = 2000 }, "price_not_reduced", "reduced_price_cents"},
		{"window in the past", func(b map[string]any) {
			b["pickup_window_start"] = time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339)
			b["pickup_window_end"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		}, "window_in_past", "pickup_window_end"},
		{"window reversed", func(b map[string]any) {
			b["pickup_window_start"] = time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339)
		}, "window_invalid", "pickup_window_end"},
		{"missing allergen_info", func(b map[string]any) { delete(b, "allergen_info") }, "missing_field", "allergen_info"},
		{"missing description", func(b map[string]any) { delete(b, "food_description") }, "missing_field", "food_description"},
		{"zero portions", func(b map[string]any) { b["total_portions"] = 0 }, "invalid_field", "total_portions"},
		{"bad time", func(b map[string]any) { b["pickup_window_end"] = "tomorrow" }, "invalid_field", "pickup_window_end"},
		{"bad photo", func(b map[string]any) { b["photo_url"] = "javascript:alert(1)" }, "invalid_field", "photo_url"},
		{"bad location", func(b map[string]any) { b["pickup_location"] = map[string]float64{"lat": 99, "lng": 0} }, "invalid_field", "pickup_location"},
	}
	for _, c := range cases {
		code, body := post(cook.Token, c.mut)
		assert.Equal(t, http.StatusUnprocessableEntity, code, c.name)
		assert.Equal(t, c.code, body["error"], c.name)
		assert.Equal(t, c.field, body["field"], c.name)
	}
	assert.Zero(t, count(t, e, `SELECT count(*) FROM listings WHERE cook_id=$1`, cook.CookID), "invalid listings are not stored")

	// allergen_info may be an explicit empty string; the cook comes from the token, not the body.
	code, body = post(cook.Token, func(b map[string]any) { b["allergen_info"] = ""; b["cook_id"] = "ignored" })
	assert.Equal(t, http.StatusCreated, code, "%v", body)
	assert.Equal(t, 1, count(t, e, `SELECT count(*) FROM listings WHERE cook_id=$1`, cook.CookID))
}

func TestListings_DefaultPickupAddressFromCookProfile(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "defaddr", "professional")
	b := listingBody(0, 0)
	delete(b, "pickup_location")
	delete(b, "pickup_address_text")
	b["publish"] = false // the default location is Amsterdam: keep it out of local discovery
	rec := e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	d := testutil.JSON(t, rec)
	assert.Equal(t, "Keizersgracht 88, Amsterdam", d["pickup_address_text"])
	loc := d["pickup_location"].(map[string]any)
	assert.InDelta(t, 52.37, loc["lat"], 1e-6)
	assert.InDelta(t, 4.89, loc["lng"], 1e-6)
}

// Principle III / FR-011: non-profits first, then distance; the radius limits the default view.
func TestListings_DiscoveryOrderAndRadius(t *testing.T) {
	e := testutil.NewEnv(t, false)
	pro := newCook(t, e, "orderpro", "professional")
	np := newCook(t, e, "ordernp", "nonprofit")
	lat, lng := uniquePoint()
	proL := publish(t, e, pro, lat, lng, 2)       // 0 km
	npL := publish(t, e, np, lat+0.0045, lng, 2)  // ~0.5 km
	farL := publish(t, e, pro, lat+0.027, lng, 2) // ~3 km

	items := listNear(t, e, "", lat, lng, "")
	require.Len(t, items, 2, "signed out: default 2 km excludes the 3 km listing")
	assert.Equal(t, npL, items[0]["id"], "non-profit first although farther")
	assert.Equal(t, proL, items[1]["id"])
	assert.Less(t, items[0]["distance_km"].(float64), 1.0)

	items = listNear(t, e, "", lat, lng, "&radius_km=5")
	assert.Len(t, items, 3)
	assert.Equal(t, farL, items[2]["id"])
	assert.Len(t, listNear(t, e, "", lat, lng, "&radius_km=0.2"), 1)

	// The signed-in buyer's default radius applies unless radius_km is given.
	buyer, _ := verifiedBuyer(e, "radiusbuyer")
	require.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/me", buyer, nil).Code)
	acct := accountOf(t, e, buyer)
	exec(t, e, `UPDATE buyer_profiles SET default_discovery_radius_km = 4 WHERE account_id=$1`, acct)
	assert.Len(t, listNear(t, e, buyer, lat, lng, ""), 3)
	assert.Len(t, listNear(t, e, buyer, lat, lng, "&radius_km=1"), 2)

	// Validation.
	for _, q := range []string{"", "?lat=52", "?lng=4", "?lat=x&lng=4", "?lat=52&lng=4&radius_km=0", "?lat=52&lng=4&radius_km=-3", "?lat=52&lng=4&radius_km=1000", "?lat=91&lng=4", "?lat=52&lng=4&status=draft"} {
		rec := e.Do(http.MethodGet, "/api/v1/listings"+q, "", nil)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, q)
	}
}

func TestListings_GetVisibilityAndDistance(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "getcook", "professional")
	other := newCook(t, e, "getother", "professional")
	lat, lng := uniquePoint()
	pub := publish(t, e, cook, lat, lng, 2)
	b := listingBody(lat, lng)
	b["publish"] = false
	rec := e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
	draft := testutil.JSON(t, rec)["id"].(string)

	rec = e.Do(http.MethodGet, "/api/v1/listings/"+pub, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	d := testutil.JSON(t, rec)
	assert.Equal(t, "gluten, dairy", d["allergen_info"])
	assert.Equal(t, "published", d["status"])
	assert.NotContains(t, d, "distance_km", "absent without lat/lng")

	// distance_km appears when the visitor position is given.
	rec = e.Do(http.MethodGet, fmt.Sprintf("/api/v1/listings/%s?lat=%f&lng=%f", pub, lat+0.0045, lng), "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.InDelta(t, 0.5, testutil.JSON(t, rec)["distance_km"], 0.05)
	assert.Equal(t, http.StatusUnprocessableEntity, e.Do(http.MethodGet, "/api/v1/listings/"+pub+"?lat=52", "", nil).Code)

	// Drafts are visible to the owner only.
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodGet, "/api/v1/listings/"+draft, "", nil).Code)
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodGet, "/api/v1/listings/"+draft, other.Token, nil).Code)
	assert.Equal(t, http.StatusOK, e.Do(http.MethodGet, "/api/v1/listings/"+draft, cook.Token, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.Do(http.MethodGet, "/api/v1/listings/not-a-uuid", "", nil).Code)
	assert.Len(t, listNear(t, e, "", lat, lng, ""), 1)
}

// PATCH status "published" publishes a draft: draft only, owner only, future window, registration gate.
func TestListings_PublishDraft(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "pubdraft", "professional")
	other := newCook(t, e, "pubother", "professional")
	lat, lng := uniquePoint()
	b := listingBody(lat, lng)
	b["publish"] = false
	rec := e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
	draft := testutil.JSON(t, rec)["id"].(string)

	code, _ := patch(e, other.Token, draft, map[string]any{"status": "published"})
	assert.Equal(t, http.StatusForbidden, code)

	// Registration gate.
	exec(t, e, `UPDATE cook_profiles SET registration_status='inactive' WHERE id=$1`, cook.CookID)
	code, body := patch(e, cook.Token, draft, map[string]any{"status": "published"})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, map[string]any{"error": "cook_not_registered", "registration_status": "inactive"}, body)
	exec(t, e, `UPDATE cook_profiles SET registration_status='active' WHERE id=$1`, cook.CookID)

	// Window already over.
	exec(t, e, `UPDATE listings SET pickup_window_start=NOW()-INTERVAL '3 hours', pickup_window_end=NOW()-INTERVAL '1 hour' WHERE id=$1`, draft)
	code, body = patch(e, cook.Token, draft, map[string]any{"status": "published"})
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.Equal(t, "window_in_past", body["error"])
	assert.Equal(t, "draft", str(t, e, `SELECT status::text FROM listings WHERE id=$1`, draft))

	// Extending the window in the same request makes it publishable.
	end := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	code, body = patch(e, cook.Token, draft, map[string]any{"status": "published", "pickup_window_end": end})
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, "published", body["status"])
	assert.Equal(t, "unverified", cookBlock(body)["verification_status"], "a new cook publishes, flagged")
	assert.Len(t, listNear(t, e, "", lat, lng, ""), 1)

	// Only drafts can be published; cancelled listings stay cancelled.
	code, body = patch(e, cook.Token, draft, map[string]any{"status": "published"})
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "listing_not_draft", body["error"])
	patch(e, cook.Token, draft, map[string]any{"status": "cancelled"})
	code, _ = patch(e, cook.Token, draft, map[string]any{"status": "published"})
	assert.Equal(t, http.StatusConflict, code)
}

// GET /me/listings: the cook's own listings, every status, newest first, with portions_claimed.
func TestListings_MineEveryStatus(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "minecook", "professional")
	other := newCook(t, e, "mineother", "professional")
	buyer, _ := verifiedBuyer(e, "minebuyer")
	lat, lng := uniquePoint()

	b := listingBody(lat, lng)
	b["publish"] = false
	draftRec := e.Do(http.MethodPost, "/api/v1/listings", cook.Token, b)
	draft := testutil.JSON(t, draftRec)["id"].(string)
	expired := publish(t, e, cook, lat, lng, 2)
	exec(t, e, `UPDATE listings SET pickup_window_start=NOW()-INTERVAL '3 hours', pickup_window_end=NOW()-INTERVAL '1 hour', status='expired', created_at=NOW()-INTERVAL '2 minutes' WHERE id=$1`, expired)
	cancelled := publish(t, e, cook, lat, lng, 2)
	patch(e, cook.Token, cancelled, map[string]any{"status": "cancelled"})
	exec(t, e, `UPDATE listings SET created_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, cancelled)
	published := publish(t, e, cook, lat, lng, 5)
	claimOK(t, e, buyer, published, 2)
	paidClaim(t, e, buyer, published)
	publish(t, e, other, lat, lng, 1) // someone else's listing never shows up
	exec(t, e, `UPDATE listings SET created_at=NOW()-INTERVAL '10 minutes' WHERE id=$1`, draft)

	rec := e.Do(http.MethodGet, "/api/v1/me/listings", cook.Token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var ids, statuses []string
	var claimed []float64
	for _, l := range testutil.JSON(t, rec)["listings"].([]any) {
		m := l.(map[string]any)
		ids = append(ids, m["id"].(string))
		statuses = append(statuses, m["status"].(string))
		claimed = append(claimed, m["portions_claimed"].(float64))
		assert.Contains(t, m, "allergen_info")
		assert.Contains(t, m, "total_portions")
		assert.Equal(t, "unverified", cookBlock(m)["verification_status"])
	}
	assert.Equal(t, []string{published, cancelled, expired, draft}, ids, "newest first")
	assert.Equal(t, []string{"published", "cancelled", "expired", "draft"}, statuses)
	assert.Equal(t, []float64{3, 0, 0, 0}, claimed)

	assert.Equal(t, http.StatusForbidden, e.Do(http.MethodGet, "/api/v1/me/listings", buyer, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, e.Do(http.MethodGet, "/api/v1/me/listings", "", nil).Code)
}

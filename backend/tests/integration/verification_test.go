package integration

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eyho/eyho/tests/testutil"
)

type ratedClaim struct {
	ID      string
	Buyer   string
	Listing string
}

// collectedClaim makes a paid, collected claim of a fresh buyer on the listing.
func collectedClaim(t *testing.T, e *testutil.Env, cook testCook, listing, prefix string) ratedClaim {
	t.Helper()
	buyer, _ := verifiedBuyer(e, prefix)
	id := paidClaim(t, e, buyer, listing)
	collect(t, e, cook.Token, id)
	return ratedClaim{ID: id, Buyer: buyer, Listing: listing}
}

func rate(e *testutil.Env, c ratedClaim, stars any) (int, map[string]any) {
	rec := e.Do(http.MethodPost, "/api/v1/claims/"+c.ID+"/rating", c.Buyer, map[string]any{"stars": stars})
	var m map[string]any
	if rec.Body.Len() > 0 {
		m = testutil.JSON(e.T, rec)
	}
	return rec.Code, m
}

type cookState struct {
	Status      string
	Good, Bad   int
	Cycle       int
	RatingCount int
	RatingAvg   float64
}

func cookRow(t *testing.T, e *testutil.Env, cook testCook) cookState {
	t.Helper()
	var s cookState
	require.NoError(t, e.Pool.QueryRow(context.Background(), `
		SELECT verification_status::text, good_ratings_since_transition, bad_ratings_since_transition,
		       verification_cycle_count, rating_count, rating_avg::float8
		FROM cook_profiles WHERE id=$1`, cook.CookID).
		Scan(&s.Status, &s.Good, &s.Bad, &s.Cycle, &s.RatingCount, &s.RatingAvg))
	return s
}

func historyRows(t *testing.T, e *testutil.Env, cook testCook) (from, to []string, triggers [][]string) {
	t.Helper()
	rows, err := e.Pool.Query(context.Background(), `SELECT from_status::text, to_status::text, triggering_claim_ids::text[] FROM verification_history WHERE cook_id=$1 ORDER BY created_at, id`, cook.CookID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var f, tt string
		var ids []string
		require.NoError(t, rows.Scan(&f, &tt, &ids))
		from, to, triggers = append(from, f), append(to, tt), append(triggers, ids)
	}
	require.NoError(t, rows.Err())
	return
}

func listedCookStatus(t *testing.T, e *testutil.Env, listing string) string {
	t.Helper()
	rec := e.Do(http.MethodGet, "/api/v1/listings/"+listing, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	d := testutil.JSON(t, rec)
	require.Equal(t, "published", d["status"], "demotion never cancels listings")
	return d["cook"].(map[string]any)["verification_status"].(string)
}

// Quickstart Scenario 7 (FR-013, FR-014): 5 good ratings promote, 3 bad ratings demote; every
// transition is recorded with its triggering claims; listings stay published but flagged.
func TestVerification_Scenario7(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "vercook", "professional")
	lat, lng := uniquePoint()

	// Step 1-2: five listings, one collected claim each, five 5-star ratings.
	var claims []ratedClaim
	for i := 0; i < 5; i++ {
		l := publish(t, e, cook, lat, lng, 2)
		claims = append(claims, collectedClaim(t, e, cook, l, fmt.Sprintf("verbuyer%d", i)))
	}
	assert.Equal(t, "unverified", listedCookStatus(t, e, claims[0].Listing))

	for i, c := range claims[:4] {
		code, body := rate(e, c, 5)
		require.Equal(t, http.StatusCreated, code, "%v", body)
		assert.Equal(t, c.ID, body["claim_id"])
		assert.EqualValues(t, 5, body["stars"])
		assert.NotEmpty(t, body["rated_at"])
		s := cookRow(t, e, cook)
		assert.Equal(t, "unverified", s.Status, "after %d ratings", i+1)
		assert.Equal(t, i+1, s.Good)
	}
	code, _ := rate(e, claims[4], 5)
	require.Equal(t, http.StatusCreated, code)

	// Step 3: promoted, counters reset, cycle 1, one history row naming all five claims.
	s := cookRow(t, e, cook)
	assert.Equal(t, cookState{Status: "verified", Cycle: 1, RatingCount: 5, RatingAvg: 5}, s)
	from, to, triggers := historyRows(t, e, cook)
	assert.Equal(t, []string{"unverified"}, from)
	assert.Equal(t, []string{"verified"}, to)
	want := []string{}
	for _, c := range claims {
		want = append(want, c.ID)
	}
	sort.Strings(want)
	sort.Strings(triggers[0])
	assert.Equal(t, want, triggers[0])
	assert.Equal(t, "verified", listedCookStatus(t, e, claims[0].Listing))

	// Step 4: rating twice -> 409 already_rated; nothing changes.
	code, body := rate(e, claims[0], 1)
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "already_rated", body["error"])
	assert.Equal(t, s, cookRow(t, e, cook))

	// Step 5: a claim that is not collected -> 409 not_collected.
	buyer, _ := verifiedBuyer(e, "notcollected")
	pending := ratedClaim{ID: paidClaim(t, e, buyer, claims[0].Listing), Buyer: buyer}
	code, body = rate(e, pending, 5)
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "not_collected", body["error"])

	// Only the claim's buyer may rate; stars must be an integer from 1 to 5.
	stranger := ratedClaim{ID: claims[1].ID, Buyer: claims[0].Buyer}
	code, _ = rate(e, stranger, 5)
	assert.Equal(t, http.StatusForbidden, code)
	fresh := collectedClaim(t, e, cook, claims[0].Listing, "validation")
	for _, bad := range []any{0, 6, -1, 4.5, "5", nil} {
		code, _ = rate(e, fresh, bad)
		assert.Equal(t, http.StatusUnprocessableEntity, code, "stars=%v", bad)
	}
	assert.Zero(t, count(t, e, `SELECT count(*) FROM claims WHERE id=$1 AND rating IS NOT NULL`, fresh.ID))

	// A 3-star rating counts toward neither threshold, but toward the average.
	code, _ = rate(e, fresh, 3)
	require.Equal(t, http.StatusCreated, code)
	s = cookRow(t, e, cook)
	assert.Equal(t, 0, s.Good)
	assert.Equal(t, 0, s.Bad)
	assert.Equal(t, 6, s.RatingCount)
	assert.InDelta(t, 28.0/6.0, s.RatingAvg, 0.01)

	// Step 6: three 1-star ratings demote the verified cook.
	var bad []ratedClaim
	for i := 0; i < 3; i++ {
		bad = append(bad, collectedClaim(t, e, cook, claims[0].Listing, fmt.Sprintf("badbuyer%d", i)))
	}
	for i, c := range bad {
		code, _ = rate(e, c, 1)
		require.Equal(t, http.StatusCreated, code)
		if i < 2 {
			assert.Equal(t, "verified", cookRow(t, e, cook).Status)
			assert.Equal(t, i+1, cookRow(t, e, cook).Bad)
		}
	}
	s = cookRow(t, e, cook)
	assert.Equal(t, "unverified", s.Status)
	assert.Equal(t, 2, s.Cycle)
	assert.Equal(t, 0, s.Good)
	assert.Equal(t, 0, s.Bad)
	from, to, triggers = historyRows(t, e, cook)
	assert.Equal(t, []string{"unverified", "verified"}, from)
	assert.Equal(t, []string{"verified", "unverified"}, to)
	wantBad := []string{bad[0].ID, bad[1].ID, bad[2].ID}
	sort.Strings(wantBad)
	sort.Strings(triggers[1])
	assert.Equal(t, wantBad, triggers[1])

	// Published listings are still published, now flagged unverified (spec Edge Case), and the
	// cycle count is exposed to buyers.
	for _, c := range claims {
		assert.Equal(t, "unverified", listedCookStatus(t, e, c.Listing))
	}
	rec := e.Do(http.MethodGet, "/api/v1/listings/"+claims[0].Listing, "", nil)
	assert.EqualValues(t, 2, testutil.JSON(t, rec)["cook"].(map[string]any)["verification_cycle_count"])
}

// Constitution v3.0.0: each buyer's rating counts separately, so five buyers of ONE listing promote.
func TestVerification_PerBuyerRatingsOnOneListing(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "perbuyercook", "nonprofit")
	lat, lng := uniquePoint()
	l := publish(t, e, cook, lat, lng, 5)
	for i := 0; i < 5; i++ {
		c := collectedClaim(t, e, cook, l, fmt.Sprintf("perbuyer%d", i))
		code, _ := rate(e, c, 4) // 4 stars is "good"
		require.Equal(t, http.StatusCreated, code)
	}
	s := cookRow(t, e, cook)
	assert.Equal(t, "verified", s.Status)
	assert.Equal(t, 1, s.Cycle)
}

// Ratings that point the other way do not transition: bad ratings on an unverified cook and good
// ratings on a verified cook only move their counters.
func TestVerification_OppositeRatingsDoNotTransition(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "oppositecook", "professional")
	lat, lng := uniquePoint()
	l := publish(t, e, cook, lat, lng, 10)
	for i := 0; i < 4; i++ {
		c := collectedClaim(t, e, cook, l, fmt.Sprintf("opp%d", i))
		code, _ := rate(e, c, 2) // 2 stars is "bad"
		require.Equal(t, http.StatusCreated, code)
	}
	s := cookRow(t, e, cook)
	assert.Equal(t, "unverified", s.Status, "cannot be demoted while unverified")
	assert.Equal(t, 4, s.Bad)
	assert.Zero(t, s.Cycle)
	from, _, _ := historyRows(t, e, cook)
	assert.Empty(t, from)
}

// The rating transaction is atomic: concurrent ratings of one cook never lose a count.
func TestVerification_ConcurrentRatingsAreSerialised(t *testing.T) {
	e := testutil.NewEnv(t, false)
	cook := newCook(t, e, "concratecook", "professional")
	lat, lng := uniquePoint()
	l := publish(t, e, cook, lat, lng, 10)
	var cs []ratedClaim
	for i := 0; i < 5; i++ {
		cs = append(cs, collectedClaim(t, e, cook, l, fmt.Sprintf("concrate%d", i)))
	}
	done := make(chan int, len(cs))
	for _, c := range cs {
		go func() { code, _ := rate(e, c, 5); done <- code }()
	}
	for range cs {
		assert.Equal(t, http.StatusCreated, <-done)
	}
	s := cookRow(t, e, cook)
	assert.Equal(t, "verified", s.Status)
	assert.Equal(t, 5, s.RatingCount)
	from, _, triggers := historyRows(t, e, cook)
	require.Len(t, from, 1)
	assert.Len(t, triggers[0], 5)
}

package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const activeBody = `{"pagina":1,"totaal":1,"resultaten":[{"kvkNummer":"68750110","naam":"Test NV Katrien"}]}`
const emptyBody = `{"pagina":1,"totaal":0,"resultaten":[]}`

func server(t *testing.T, handler func(inactive bool, w http.ResponseWriter)) (*KVK, *int32) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "/zoeken", r.URL.Path)
		assert.Equal(t, "key123", r.Header.Get("apikey"))
		assert.Equal(t, "68750110", r.URL.Query().Get("kvkNummer"))
		handler(r.URL.Query().Get("inclusiefInactieveRegistraties") == "true", w)
	}))
	t.Cleanup(srv.Close)
	return NewKVK(srv.URL, "key123", srv.Client()), &calls
}

func TestKVK_ActiveHitStopsAfterFirstQuery(t *testing.T) {
	k, calls := server(t, func(inactive bool, w http.ResponseWriter) { w.Write([]byte(activeBody)) })
	res, err := k.Check(context.Background(), "68750110")
	require.NoError(t, err)
	assert.Equal(t, Result{StatusActive, "Test NV Katrien"}, res)
	assert.EqualValues(t, 1, *calls)
}

func TestKVK_InactiveNeedsSecondQuery(t *testing.T) {
	k, calls := server(t, func(inactive bool, w http.ResponseWriter) {
		if inactive {
			w.Write([]byte(activeBody))
			return
		}
		w.Write([]byte(emptyBody))
	})
	res, err := k.Check(context.Background(), "68750110")
	require.NoError(t, err)
	assert.Equal(t, StatusInactive, res.Status)
	assert.EqualValues(t, 2, *calls)
}

func TestKVK_NotFound(t *testing.T) {
	k, _ := server(t, func(bool, http.ResponseWriter) {})
	res, err := k.Check(context.Background(), "68750110")
	require.Error(t, err) // empty 200 body is unparseable: no answer
	_ = res

	k, _ = server(t, func(_ bool, w http.ResponseWriter) { w.Write([]byte(emptyBody)) })
	res, err = k.Check(context.Background(), "68750110")
	require.NoError(t, err)
	assert.Equal(t, StatusNotFound, res.Status)
}

func TestKVK_404WithErrorListIsNoHit_BareIs404Error(t *testing.T) {
	k, _ := server(t, func(_ bool, w http.ResponseWriter) {
		w.WriteHeader(404)
		w.Write([]byte(`{"fout":[{"code":"IPD5200","omschrijving":"Geen resultaten"}]}`))
	})
	res, err := k.Check(context.Background(), "68750110")
	require.NoError(t, err)
	assert.Equal(t, StatusNotFound, res.Status)

	k, _ = server(t, func(_ bool, w http.ResponseWriter) { w.WriteHeader(404) })
	_, err = k.Check(context.Background(), "68750110")
	require.Error(t, err)
}

func TestKVK_TransportAndServerErrorsAreNeverDefinite(t *testing.T) {
	for _, code := range []int{401, 403, 429, 500, 503} {
		code := code
		k, _ := server(t, func(_ bool, w http.ResponseWriter) { w.WriteHeader(code) })
		_, err := k.Check(context.Background(), "68750110")
		require.Error(t, err, code)
	}
	// Connection refused.
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := NewKVK(url, "k", nil).Check(context.Background(), "68750110")
	require.Error(t, err)
}

func TestKVK_IgnoresResultsForOtherNumbers(t *testing.T) {
	k, _ := server(t, func(_ bool, w http.ResponseWriter) {
		w.Write([]byte(`{"totaal":1,"resultaten":[{"kvkNummer":"11111111","naam":"Other"}]}`))
	})
	res, err := k.Check(context.Background(), "68750110")
	require.NoError(t, err)
	assert.Equal(t, StatusNotFound, res.Status)
}

func TestFake(t *testing.T) {
	f := NewFake()
	ctx := context.Background()
	r, _ := f.Check(ctx, FakeActiveNumber1)
	assert.Equal(t, StatusActive, r.Status)
	assert.Equal(t, "Keuken Katrien B.V.", r.TradeName)
	r, _ = f.Check(ctx, FakeInactiveNumber)
	assert.Equal(t, StatusInactive, r.Status)
	r, _ = f.Check(ctx, "12345678")
	assert.Equal(t, StatusNotFound, r.Status)
	f.SetDown(true)
	_, err := f.Check(ctx, FakeActiveNumber1)
	assert.ErrorIs(t, err, ErrFakeUnavailable)
}

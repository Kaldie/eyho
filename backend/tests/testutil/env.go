package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eyho/eyho/src/api"
	"github.com/eyho/eyho/src/api/handlers"
	"github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/auth"
	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/registry"
	"github.com/eyho/eyho/src/services"
)

// FakeIdP records calls and manages the test issuer's identities.
type FakeIdP struct {
	Issuer string
	mu     sync.Mutex
	Emails []string
	// Deleted lists subjects DeleteUser succeeded for. FailDelete makes DeleteUser fail.
	Deleted    []string
	FailDelete bool
}

func (f *FakeIdP) Manages(issuer string) bool { return issuer == f.Issuer }
func (f *FakeIdP) SendVerificationEmail(_ context.Context, sub string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Emails = append(f.Emails, sub)
	return nil
}
func (f *FakeIdP) DeleteUser(_ context.Context, sub string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailDelete {
		return fmt.Errorf("idp down")
	}
	f.Deleted = append(f.Deleted, sub)
	return nil
}

// Env is a full backend (router + services) over the real database.
type Env struct {
	T        *testing.T
	Pool     *pgxpool.Pool
	Q        *db.Queries
	Issuer   *Issuer
	Register *registry.Fake
	IdP      *FakeIdP
	Svc      *services.AccountService
	Router   http.Handler
	Demo     *auth.DemoIssuer
}

// NewEnv connects to DATABASE_URL (the test is skipped when unset) and builds the router with the
// demo issuer enabled when withDemo is true.
func NewEnv(t *testing.T, withDemo bool) *Env {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	e := &Env{T: t, Pool: pool, Q: db.New(pool), Issuer: NewIssuer(t), Register: registry.NewFake()}
	e.IdP = &FakeIdP{Issuer: e.Issuer.URL}
	// The issuer URL is unique per test run; don't leave its deletion requests behind.
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM idp_deletion_requests WHERE issuer=$1`, e.Issuer.URL)
	})
	e.Svc = services.NewAccountService(pool, e.Register, e.IdP)

	verifier := e.Issuer.Verifier()
	var demoH *handlers.DemoAuthHandler
	if withDemo {
		e.Demo, err = auth.NewDemoIssuer(Audience)
		if err != nil {
			t.Fatal(err)
		}
		verifier.AddStaticIssuer(auth.DemoIssuerName, e.Demo.KeySet())
		demoH = handlers.NewDemoAuthHandler(e.Demo, e.Q)
	}
	e.Router = api.NewRouter(services.NewStubListingService(), services.NewStubClaimService(), api.AuthDeps{
		Middleware: middleware.NewAuth(verifier, e.Svc),
		Me:         handlers.NewMeHandler(e.Svc),
		Demo:       demoH,
	})
	return e
}

// Do performs a request against the router. body may be nil, a string or a value to marshal.
func (e *Env) Do(method, path, token string, body any) *httptest.ResponseRecorder {
	e.T.Helper()
	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rdr = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rdr = strings.NewReader(string(raw))
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	return rec
}

// JSON decodes a recorder body into a map.
func JSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not JSON (%d): %s", rec.Code, rec.Body.String())
	}
	return m
}

// Unique returns a fresh email and subject so tests never collide on the shared database.
func Unique(prefix string) (email, sub string) {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	return fmt.Sprintf("%s-%s@example.nl", prefix, id), fmt.Sprintf("%s|%s", prefix, id)
}

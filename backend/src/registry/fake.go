package registry

import (
	"context"
	"errors"
	"sync"
)

// Fixed fake-register fixtures. They match backend/src/db/seed/demo_accounts.sql. They are
// fake-register fixtures only: they were not confirmed against KVK's own test set.
const (
	FakeActiveNumber1  = "68750110" // "Keuken Katrien B.V."
	FakeActiveNumber2  = "69599084" // "Test Eetcafe Jordaan"
	FakeInactiveNumber = "90000002"
)

// ErrFakeUnavailable is returned while the fake register is switched to fail.
var ErrFakeUnavailable = errors.New("fake register unavailable")

// Fake is an in-memory register for local development and tests (KVK_MODE=fake). It must never
// run outside demo/local (see auth.CheckStartup).
type Fake struct {
	mu   sync.Mutex
	data map[string]Result
	down bool
}

// NewFake returns a register holding the fixed test numbers.
func NewFake() *Fake {
	return &Fake{data: map[string]Result{
		FakeActiveNumber1:  {StatusActive, "Keuken Katrien B.V."},
		FakeActiveNumber2:  {StatusActive, "Test Eetcafe Jordaan"},
		FakeInactiveNumber: {StatusInactive, "Gestopte Keuken B.V."},
	}}
}

// Check implements BusinessRegister. Unknown numbers are not_found.
func (f *Fake) Check(_ context.Context, kvk string) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return Result{}, ErrFakeUnavailable
	}
	if r, ok := f.data[kvk]; ok {
		return r, nil
	}
	return Result{Status: StatusNotFound}, nil
}

// SetDown makes every Check fail with a transport-style error (tests, quickstart Scenario 8).
func (f *Fake) SetDown(down bool) { f.mu.Lock(); f.down = down; f.mu.Unlock() }

// Set adds or changes a number (tests).
func (f *Fake) Set(kvk string, status Status, tradeName string) {
	f.mu.Lock()
	f.data[kvk] = Result{status, tradeName}
	f.mu.Unlock()
}

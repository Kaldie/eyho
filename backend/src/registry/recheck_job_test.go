package registry

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/eyho/eyho/src/db/generated"
)

type recheckQ struct {
	db.Querier
	rows    []db.ListCookProfilesDueForRecheckRow
	updates []db.UpdateCookRegistrationResultParams
}

func (q *recheckQ) ListCookProfilesDueForRecheck(context.Context, db.ListCookProfilesDueForRecheckParams) ([]db.ListCookProfilesDueForRecheckRow, error) {
	return q.rows, nil
}
func (q *recheckQ) UpdateCookRegistrationResult(_ context.Context, p db.UpdateCookRegistrationResultParams) (int64, error) {
	q.updates = append(q.updates, p)
	return 1, nil
}

func row(status db.CookRegistrationStatus, kvk string) db.ListCookProfilesDueForRecheckRow {
	return db.ListCookProfilesDueForRecheckRow{
		ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, KvkNumber: &kvk, RegistrationStatus: status,
	}
}

func TestRecheck_TransportErrorNeverChangesActiveProfile(t *testing.T) {
	q := &recheckQ{rows: []db.ListCookProfilesDueForRecheckRow{row(db.CookRegistrationStatusActive, FakeActiveNumber1)}}
	reg := NewFake()
	reg.SetDown(true)
	n, err := (&RecheckJob{Q: q, Register: reg}).RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, q.updates)
}

func TestRecheck_DefiniteAnswersUpdate(t *testing.T) {
	cases := map[string]db.CookRegistrationStatus{
		FakeActiveNumber1:  db.CookRegistrationStatusActive,
		FakeInactiveNumber: db.CookRegistrationStatusInactive,
		"12345678":         db.CookRegistrationStatusInactive, // not found
	}
	for kvk, want := range cases {
		q := &recheckQ{rows: []db.ListCookProfilesDueForRecheckRow{row(db.CookRegistrationStatusActive, kvk)}}
		_, err := (&RecheckJob{Q: q, Register: NewFake()}).RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, q.updates, 1, kvk)
		assert.Equal(t, want, q.updates[0].RegistrationStatus, kvk)
		assert.Equal(t, kvk, *q.updates[0].KvkNumber, "update is guarded by the number that was checked")
		assert.True(t, q.updates[0].RegistrationCheckedAt.Valid)
	}
}

func TestRecheck_PendingBacksOffAfterFailure(t *testing.T) {
	q := &recheckQ{rows: []db.ListCookProfilesDueForRecheckRow{row(db.CookRegistrationStatusPending, FakeActiveNumber1)}}
	reg := NewFake()
	reg.SetDown(true)
	now := time.Now()
	job := &RecheckJob{Q: q, Register: reg, Now: func() time.Time { return now }}

	_, err := job.RunOnce(context.Background())
	require.NoError(t, err)

	reg.SetDown(false)
	n, _ := job.RunOnce(context.Background()) // still inside the backoff window
	assert.Equal(t, 0, n)

	now = now.Add(2 * time.Minute)
	n, _ = job.RunOnce(context.Background())
	assert.Equal(t, 1, n)
	assert.Equal(t, db.CookRegistrationStatusActive, q.updates[0].RegistrationStatus)
}

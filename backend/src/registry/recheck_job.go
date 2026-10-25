package registry

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/eyho/eyho/src/db/generated"
)

// StaleAfter is how old a successful check may get before the job repeats it (FR-021).
const StaleAfter = 30 * 24 * time.Hour

// RecheckJob retries 'pending' registrations with backoff and re-checks profiles not checked for
// 30 days (FR-021). Only a definite answer (active, inactive, not found) changes a profile; a
// transport error never touches the database, so an outage cannot downgrade an active cook.
// Backoff state for pending profiles lives in memory (a restart simply retries sooner).
type RecheckJob struct {
	Q        db.Querier
	Register BusinessRegister
	Interval time.Duration // poll interval (default 1 minute)
	MaxRows  int32         // profiles per poll (default 500)
	Now      func() time.Time

	mu      sync.Mutex
	backoff map[pgtype.UUID]backoffState
}

type backoffState struct {
	attempts int
	next     time.Time
}

const (
	pendingBackoffBase = time.Minute
	pendingBackoffMax  = time.Hour
)

func (j *RecheckJob) now() time.Time {
	if j.Now != nil {
		return j.Now()
	}
	return time.Now()
}

// RunOnce visits the profiles that are due and returns how many got a definite answer stored.
func (j *RecheckJob) RunOnce(ctx context.Context) (int, error) {
	max := j.MaxRows
	if max <= 0 {
		max = 500
	}
	now := j.now()
	rows, err := j.Q.ListCookProfilesDueForRecheck(ctx, db.ListCookProfilesDueForRecheckParams{
		StaleBefore: pgtype.Timestamptz{Time: now.Add(-StaleAfter), Valid: true},
		MaxRows:     max,
	})
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, r := range rows {
		if r.KvkNumber == nil {
			continue
		}
		pending := r.RegistrationStatus == db.CookRegistrationStatusPending
		if pending && !j.due(r.ID, now) {
			continue
		}
		res, err := j.Register.Check(ctx, *r.KvkNumber)
		if err != nil {
			// No answer: leave the profile untouched; only pending profiles back off.
			log.Printf("registration re-check: register unavailable: %v", err)
			if pending {
				j.fail(r.ID, now)
			}
			continue
		}
		p := db.UpdateCookRegistrationResultParams{
			ID:                    r.ID,
			KvkNumber:             r.KvkNumber,
			RegistrationCheckedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}
		switch res.Status {
		case StatusActive:
			p.RegistrationStatus = db.CookRegistrationStatusActive
		case StatusInactive, StatusNotFound:
			p.RegistrationStatus = db.CookRegistrationStatusInactive
		default:
			continue
		}
		if res.TradeName != "" {
			name := res.TradeName
			p.KvkTradeName = &name
		}
		if _, err := j.Q.UpdateCookRegistrationResult(ctx, p); err != nil {
			return updated, err
		}
		j.clear(r.ID)
		updated++
	}
	return updated, nil
}

func (j *RecheckJob) due(id pgtype.UUID, now time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	s, ok := j.backoff[id]
	return !ok || !now.Before(s.next)
}

func (j *RecheckJob) fail(id pgtype.UUID, now time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.backoff == nil {
		j.backoff = map[pgtype.UUID]backoffState{}
	}
	s := j.backoff[id]
	d := pendingBackoffBase
	for i := 0; i < s.attempts && d < pendingBackoffMax; i++ {
		d *= 2
	}
	if d > pendingBackoffMax {
		d = pendingBackoffMax
	}
	j.backoff[id] = backoffState{attempts: s.attempts + 1, next: now.Add(d)}
}

func (j *RecheckJob) clear(id pgtype.UUID) {
	j.mu.Lock()
	delete(j.backoff, id)
	j.mu.Unlock()
}

// Run polls until ctx is cancelled.
func (j *RecheckJob) Run(ctx context.Context) {
	iv := j.Interval
	if iv <= 0 {
		iv = time.Minute
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		if _, err := j.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("registration re-check job: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

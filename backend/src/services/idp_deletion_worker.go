package services

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/eyho/eyho/src/db/generated"
	"github.com/eyho/eyho/src/idp"
)

// IdpDeletionWorker drains idp_deletion_requests: it deletes users at the identity provider after
// local account deletion, retrying with exponential backoff (research.md, "Account deletion").
type IdpDeletionWorker struct {
	Q        db.Querier
	Provider idp.Provider
	Interval time.Duration // poll interval (default 30 s)
	Batch    int32         // rows per poll (default 20)
	// TombstoneRetention is how long deleted-identity tombstones are kept (default 24h); the
	// worker purges older ones on every poll.
	TombstoneRetention time.Duration
	Now                func() time.Time
}

const (
	idpLeaseSeconds = 120
	idpBackoffBase  = 30 * time.Second
	idpBackoffMax   = 6 * time.Hour
)

// backoff returns the delay after the given (1-based) failed attempt.
func idpBackoff(attempts int32) time.Duration {
	d := idpBackoffBase
	for i := int32(1); i < attempts && d < idpBackoffMax; i++ {
		d *= 2
	}
	if d > idpBackoffMax {
		d = idpBackoffMax
	}
	return d
}

// RunOnce processes the requests that are due and returns how many were completed.
func (w *IdpDeletionWorker) RunOnce(ctx context.Context) (int, error) {
	if err := w.purgeTombstones(ctx); err != nil {
		log.Printf("idp deletion worker: purging tombstones: %v", err)
	}
	batch := w.Batch
	if batch <= 0 {
		batch = 20
	}
	rows, err := w.Q.ClaimDueIdpDeletions(ctx, db.ClaimDueIdpDeletionsParams{LeaseSeconds: idpLeaseSeconds, MaxRows: batch})
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range rows {
		if err := w.Provider.DeleteUser(ctx, r.Subject); err != nil {
			log.Printf("idp deletion: attempt %d failed: %v", r.Attempts, err)
			if rerr := w.Q.RescheduleIdpDeletion(ctx, db.RescheduleIdpDeletionParams{ID: r.ID, BackoffSeconds: idpBackoff(r.Attempts).Seconds()}); rerr != nil {
				log.Printf("idp deletion: reschedule failed: %v", rerr)
			}
			continue
		}
		if err := w.Q.CompleteIdpDeletion(ctx, r.ID); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

func (w *IdpDeletionWorker) purgeTombstones(ctx context.Context) error {
	keep := w.TombstoneRetention
	if keep <= 0 {
		keep = 24 * time.Hour
	}
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	_, err := w.Q.PurgeDeletedIdentities(ctx, pgtype.Timestamptz{Time: now().Add(-keep), Valid: true})
	return err
}

// Run polls until ctx is cancelled.
func (w *IdpDeletionWorker) Run(ctx context.Context) {
	iv := w.Interval
	if iv <= 0 {
		iv = 30 * time.Second
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		if _, err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("idp deletion worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

package services

import (
	"context"
	"log"
)

// ExpiryJob is the once-a-minute job of FR-007 / FR-012 (research.md):
//  1. published listings whose pickup window ended become expired;
//  2. paid, uncollected claims whose window ended become no_show (not refunded);
//  3. refunds that failed when a listing was cancelled are retried.
func (s *ListingService) ExpiryJob(ctx context.Context) error {
	expired, err := s.q.ExpireListings(ctx)
	if err != nil {
		return err
	}
	noShows, err := s.q.MarkNoShows(ctx)
	if err != nil {
		return err
	}
	if len(expired) > 0 || noShows > 0 {
		log.Printf("expiry job: %d listings expired, %d claims marked no_show", len(expired), noShows)
	}
	return s.RefundCancelledListingClaims(ctx, nil)
}

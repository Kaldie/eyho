package models

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus tracks the Stripe payment lifecycle of a claim.
type PaymentStatus string

const (
	PaymentStatusPendingPayment PaymentStatus = "pending_payment"
	PaymentStatusPaid           PaymentStatus = "paid"
	PaymentStatusPaymentFailed  PaymentStatus = "payment_failed"
	PaymentStatusRefunded       PaymentStatus = "refunded"
)

// PickupStatus tracks whether the buyer physically collected the food.
type PickupStatus string

const (
	PickupStatusPending   PickupStatus = "pending"
	PickupStatusCollected PickupStatus = "collected"
	// PickupStatusNoShow is set when the pickup window ends without collection
	// on a paid claim. Non-refundable per FR-012.
	PickupStatusNoShow PickupStatus = "no_show"
)

// Claim represents a buyer's reservation of one or more portions from a
// listing, plus the associated payment and pickup lifecycle.
//
// Creating a Claim and decrementing Listing.PortionsRemaining MUST happen
// atomically via a conditional-UPDATE (FR-006, SC-003).
type Claim struct {
	ID        uuid.UUID `json:"id"`
	ListingID uuid.UUID `json:"listing_id"`
	BuyerID   uuid.UUID `json:"buyer_id"`
	// Quantity must be > 0 and <= Listing.PortionsRemaining at claim time.
	Quantity      int           `json:"quantity"`
	PaymentStatus PaymentStatus `json:"payment_status"`
	PickupStatus  PickupStatus  `json:"pickup_status"`
	// StripePaymentIntentID is nil until a payment attempt is made.
	StripePaymentIntentID *string `json:"stripe_payment_intent_id,omitempty"`
	// ReservedAt marks the start of the payment-timeout window (FR-008).
	// A background job releases portions when now() > reserved_at + timeout.
	ReservedAt  time.Time  `json:"reserved_at"`
	// ConfirmedAt is set when Stripe reports payment_intent.succeeded.
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

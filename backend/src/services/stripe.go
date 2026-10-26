package services

import "github.com/stripe/stripe-go/v87"

// InitStripe configures the stripe-go SDK with the given secret key.
// Call this once at startup (before any ClaimService.Create calls).
// The key is read from the STRIPE_SECRET_KEY environment variable in main.go.
func InitStripe(secretKey string) {
	stripe.Key = secretKey
}

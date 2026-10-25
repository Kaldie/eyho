// Package idp talks to the identity provider's management API (feature 001): resend
// verification emails and delete users after local account deletion.
package idp

import "context"

// Provider performs management actions at the identity provider for the identities it manages.
type Provider interface {
	// Manages reports whether identities of this token issuer live at this provider. The mock and
	// demo issuers are not managed: nothing is sent or queued for them.
	Manages(issuer string) bool
	// SendVerificationEmail asks the provider to resend the verification email.
	SendVerificationEmail(ctx context.Context, subject string) error
	// DeleteUser deletes the user at the provider. A user that is already gone is success.
	DeleteUser(ctx context.Context, subject string) error
}

// Noop is used when no provider management is configured (local, mock issuer).
type Noop struct{}

func (Noop) Manages(string) bool                                 { return false }
func (Noop) SendVerificationEmail(context.Context, string) error { return nil }
func (Noop) DeleteUser(context.Context, string) error            { return nil }

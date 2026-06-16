// Package billing abstracts payment providers. It is dormant until
// monetization is enabled from the admin dashboard. Telegram Stars will be the
// first concrete provider; a ZainCash/FastPay provider is added once a licensed
// company exists.
package billing

import (
	"context"
	"errors"
)

// ErrMonetizationDisabled is returned while paid features are not yet active.
var ErrMonetizationDisabled = errors.New("monetization is disabled")

// Provider issues payments for a subscription tier.
type Provider interface {
	Name() string
	// CreateInvoice returns a payable reference (a Telegram invoice payload or a
	// payment URL) for the given tier and user.
	CreateInvoice(ctx context.Context, userID int64, tier string) (string, error)
}

// Disabled is the default no-op provider used while monetization is OFF.
type Disabled struct{}

// Name implements Provider.
func (Disabled) Name() string { return "disabled" }

// CreateInvoice implements Provider.
func (Disabled) CreateInvoice(context.Context, int64, string) (string, error) {
	return "", ErrMonetizationDisabled
}

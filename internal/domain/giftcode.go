package domain

import "time"

// GiftCode is a redeemable code that grants a premium Tier when a user enters it
// in the bot. Days is the validity length (0 = permanent). RedeemedBy is 0 until
// a user claims it.
type GiftCode struct {
	Code       string
	Tier       Tier
	Days       int
	RedeemedBy int64
	CreatedAt  time.Time
	RedeemedAt *time.Time
}

// Used reports whether the code has already been redeemed.
func (g GiftCode) Used() bool { return g.RedeemedBy != 0 }

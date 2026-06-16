// Package domain holds the core business types shared across adapters.
package domain

import "time"

// Tier is the subscription level of a user.
type Tier string

const (
	TierFree       Tier = "free"
	TierStudent    Tier = "student"
	TierResearcher Tier = "researcher"
)

// User is a Manhal user, identified by their Telegram ID.
type User struct {
	TelegramID   int64
	Name         string
	Field        string     // academic discipline, used for content filtering
	Tier         Tier       // subscription level
	PremiumUntil *time.Time // nil => free, or permanent grant when Tier != free
	CreatedAt    time.Time
}

// IsPremium reports whether the user currently has paid-tier access.
func (u *User) IsPremium(now time.Time) bool {
	if u.Tier == TierFree {
		return false
	}
	if u.PremiumUntil == nil {
		return true // permanent grant
	}
	return now.Before(*u.PremiumUntil)
}

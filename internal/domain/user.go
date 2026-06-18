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

// IsExpired reports whether the user holds a non-free tier whose time-limited
// grant has lapsed (so they should be downgraded back to free).
func (u *User) IsExpired(now time.Time) bool {
	return u.Tier != TierFree && u.PremiumUntil != nil && !now.Before(*u.PremiumUntil)
}

// PremiumDaysLeft returns the whole days remaining on a time-limited grant and
// ok=true; ok=false for free or permanent grants (no countdown applies).
func (u *User) PremiumDaysLeft(now time.Time) (int, bool) {
	if u.Tier == TierFree || u.PremiumUntil == nil {
		return 0, false
	}
	d := u.PremiumUntil.Sub(now)
	days := int(d.Hours() / 24)
	if d > 0 && days < 1 {
		days = 1 // less than a day left still shows as 1
	}
	return days, true
}

// GrantPremium activates (or extends) a paid tier for the given number of
// months. months <= 0 grants a permanent tier (no expiry). Extending an
// already-active grant adds to the remaining time rather than truncating it.
func (u *User) GrantPremium(tier Tier, months int, now time.Time) {
	u.Tier = tier
	if months <= 0 {
		u.PremiumUntil = nil
		return
	}
	base := now
	if u.PremiumUntil != nil && u.PremiumUntil.After(now) {
		base = *u.PremiumUntil // renew from current expiry
	}
	until := base.AddDate(0, months, 0)
	u.PremiumUntil = &until
}

// RevokePremium returns the user to the free tier.
func (u *User) RevokePremium() {
	u.Tier = TierFree
	u.PremiumUntil = nil
}

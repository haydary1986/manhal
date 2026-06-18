package domain

import (
	"testing"
	"time"
)

func TestUser_GrantPremium_Months(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u := &User{TelegramID: 1}
	u.GrantPremium(TierResearcher, 1, now)
	if !u.IsPremium(now) {
		t.Fatal("should be premium after grant")
	}
	if u.PremiumUntil == nil || !u.PremiumUntil.Equal(now.AddDate(0, 1, 0)) {
		t.Errorf("PremiumUntil = %v, want %v", u.PremiumUntil, now.AddDate(0, 1, 0))
	}
	// Just after expiry → expired, not premium.
	after := now.AddDate(0, 1, 1)
	if u.IsPremium(after) || !u.IsExpired(after) {
		t.Errorf("after expiry: IsPremium=%v IsExpired=%v", u.IsPremium(after), u.IsExpired(after))
	}
}

func TestUser_GrantPremium_RenewExtends(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u := &User{TelegramID: 1}
	u.GrantPremium(TierResearcher, 1, now) // until Feb 1
	u.GrantPremium(TierResearcher, 1, now) // renew while active → until Mar 1
	want := now.AddDate(0, 2, 0)
	if u.PremiumUntil == nil || !u.PremiumUntil.Equal(want) {
		t.Errorf("renew extends: got %v, want %v", u.PremiumUntil, want)
	}
}

func TestUser_GrantPremium_Permanent(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u := &User{TelegramID: 1}
	u.GrantPremium(TierStudent, 0, now)
	if u.PremiumUntil != nil {
		t.Errorf("permanent grant should have nil expiry, got %v", u.PremiumUntil)
	}
	if !u.IsPremium(now.AddDate(5, 0, 0)) {
		t.Error("permanent grant should never expire")
	}
	if u.IsExpired(now.AddDate(5, 0, 0)) {
		t.Error("permanent grant is never expired")
	}
}

func TestUser_PremiumDaysLeft(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u := &User{TelegramID: 1}
	u.GrantPremium(TierResearcher, 0, now)
	if _, ok := u.PremiumDaysLeft(now); ok {
		t.Error("permanent grant has no countdown")
	}
	until := now.AddDate(0, 0, 10)
	u.Tier, u.PremiumUntil = TierResearcher, &until
	if d, ok := u.PremiumDaysLeft(now); !ok || d != 10 {
		t.Errorf("days left = %d ok=%v, want 10", d, ok)
	}
}

func TestUser_RevokePremium(t *testing.T) {
	now := time.Now()
	until := now.AddDate(0, 1, 0)
	u := &User{TelegramID: 1, Tier: TierResearcher, PremiumUntil: &until}
	u.RevokePremium()
	if u.Tier != TierFree || u.PremiumUntil != nil || u.IsPremium(now) {
		t.Errorf("revoke failed: %+v", u)
	}
}

package bot

import (
	"context"
	"testing"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/store"
)

// constLimit builds a fixed per-user limit resolver for tests.
func constLimit(n int) func(int64) int { return func(int64) int { return n } }

func TestUsageLimiter_EnforcesDailyLimit(t *testing.T) {
	u := newUsageLimiter(constLimit(3))
	const uid int64 = 1

	if u.remaining(uid) != 3 {
		t.Errorf("fresh remaining = %d, want 3", u.remaining(uid))
	}
	for i := 0; i < 3; i++ {
		if !u.allow(uid) {
			t.Fatalf("call %d should be allowed", i+1)
		}
		u.record(uid)
	}
	if u.allow(uid) {
		t.Error("4th call should be denied")
	}
	if u.remaining(uid) != 0 {
		t.Errorf("remaining = %d, want 0", u.remaining(uid))
	}
}

func TestUsageLimiter_Unlimited(t *testing.T) {
	u := newUsageLimiter(constLimit(0))
	const uid int64 = 2
	for i := 0; i < 100; i++ {
		if !u.allow(uid) {
			t.Fatal("unlimited limiter should always allow")
		}
		u.record(uid)
	}
	if u.remaining(uid) != -1 {
		t.Errorf("unlimited remaining = %d, want -1", u.remaining(uid))
	}
}

func TestUsageLimiter_PerUser(t *testing.T) {
	u := newUsageLimiter(constLimit(1))
	u.record(1)
	if u.allow(1) {
		t.Error("user 1 is at limit")
	}
	if !u.allow(2) {
		t.Error("user 2 should be independent")
	}
}

func TestUsageLimiter_TierAware(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 1, Tier: domain.TierFree})
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 2, Tier: domain.TierResearcher}) // premium

	sm := config.NewSettingsManager(t.TempDir(), &config.BotSettings{FreeAILimit: 2, PremiumAILimit: 0})
	a := &App{cfg: &config.Config{AIDailyLimit: 5}, settings: sm, store: st}
	a.usage = newUsageLimiter(a.aiLimit)

	// Free user: capped at 2/day.
	if !a.usage.allow(1) {
		t.Fatal("free user's first request should be allowed")
	}
	a.usage.record(1)
	a.usage.record(1)
	if a.usage.allow(1) {
		t.Error("free user should be blocked after the limit of 2")
	}

	// Premium user: premium limit 0 => unlimited.
	for i := 0; i < 20; i++ {
		a.usage.record(2)
	}
	if !a.usage.allow(2) {
		t.Error("premium user should be unlimited when premium limit is 0")
	}
	if r := a.usage.remaining(2); r != -1 {
		t.Errorf("premium remaining = %d, want -1 (unlimited)", r)
	}
}

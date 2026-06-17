package bot

import (
	"context"
	"sync"
	"time"
)

// recordUsage logs that a user invoked a feature (menu action) for the admin
// analytics dashboard. Best-effort and fire-and-forget: a nil store or DB error
// never blocks a feature. Uses a fresh context so the write isn't cancelled
// when the request handler returns.
func (a *App) recordUsage(_ context.Context, userID int64, action string) {
	if a.store == nil || action == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.store.RecordUsage(ctx, userID, action)
	}()
}

// usageLimiter enforces a per-user daily quota for AI requests, to bound cost.
// It is in-memory and resets on restart. The applicable limit is resolved per
// user via limitFor (so it can vary by subscription tier); a limit <= 0 means
// unlimited.
type usageLimiter struct {
	mu       sync.Mutex
	limitFor func(userID int64) int
	used     map[int64]dayCount
}

type dayCount struct {
	day string
	n   int
}

func newUsageLimiter(limitFor func(userID int64) int) *usageLimiter {
	return &usageLimiter{limitFor: limitFor, used: make(map[int64]dayCount)}
}

// limit returns the user's applicable daily limit (0/negative => unlimited).
func (u *usageLimiter) limit(userID int64) int {
	if u.limitFor == nil {
		return 0
	}
	return u.limitFor(userID)
}

// dayKey is the UTC calendar day used to bucket usage.
func dayKey() string { return time.Now().UTC().Format("2006-01-02") }

// countToday returns today's count for a user, resetting stale days. Caller
// holds the lock.
func (u *usageLimiter) countToday(userID int64) int {
	c := u.used[userID]
	if c.day != dayKey() {
		return 0
	}
	return c.n
}

// allow reports whether the user is under their daily limit (no increment).
func (u *usageLimiter) allow(userID int64) bool {
	limit := u.limit(userID)
	if limit <= 0 {
		return true
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.countToday(userID) < limit
}

// record increments today's count for the user.
func (u *usageLimiter) record(userID int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	today := dayKey()
	c := u.used[userID]
	if c.day != today {
		c = dayCount{day: today}
	}
	c.n++
	u.used[userID] = c
}

// remaining returns the user's remaining quota today; -1 means unlimited.
func (u *usageLimiter) remaining(userID int64) int {
	limit := u.limit(userID)
	if limit <= 0 {
		return -1
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if r := limit - u.countToday(userID); r > 0 {
		return r
	}
	return 0
}

// aiLimit resolves a user's daily AI quota from the admin-configured tier limits
// (free vs premium), falling back to the env default. A premium limit of 0 means
// unlimited for subscribers.
func (a *App) aiLimit(userID int64) int {
	bs := a.settings.Get()
	free := bs.FreeAILimit
	if free == 0 {
		free = a.cfg.AIDailyLimit
	}
	if u, err := a.store.GetUser(context.Background(), userID); err == nil && u.IsPremium(time.Now()) {
		return bs.PremiumAILimit // 0 => unlimited
	}
	return free
}

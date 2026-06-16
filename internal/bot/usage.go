package bot

import (
	"sync"
	"time"
)

// usageLimiter enforces a per-user daily quota for AI requests, to bound cost.
// It is in-memory and resets on restart (a durable store comes with Postgres).
// A limit <= 0 means unlimited.
type usageLimiter struct {
	mu    sync.Mutex
	limit int
	used  map[int64]dayCount
}

type dayCount struct {
	day string
	n   int
}

func newUsageLimiter(limit int) *usageLimiter {
	return &usageLimiter{limit: limit, used: make(map[int64]dayCount)}
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

// allow reports whether the user is under the daily limit (no increment).
func (u *usageLimiter) allow(userID int64) bool {
	if u.limit <= 0 {
		return true
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.countToday(userID) < u.limit
}

// record increments today's count for the user.
func (u *usageLimiter) record(userID int64) {
	if u.limit <= 0 {
		return
	}
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
	if u.limit <= 0 {
		return -1
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if r := u.limit - u.countToday(userID); r > 0 {
		return r
	}
	return 0
}

package bot

import "testing"

func TestUsageLimiter_EnforcesDailyLimit(t *testing.T) {
	u := newUsageLimiter(3)
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
	u := newUsageLimiter(0)
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
	u := newUsageLimiter(1)
	u.record(1)
	if u.allow(1) {
		t.Error("user 1 is at limit")
	}
	if !u.allow(2) {
		t.Error("user 2 should be independent")
	}
}

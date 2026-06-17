package store

import (
	"context"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
)

func TestMemoryUsageAnalytics(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.SaveUser(ctx, &domain.User{TelegramID: 1, Name: "علي"})
	_ = m.SaveUser(ctx, &domain.User{TelegramID: 2, Name: "سارة"})

	// user 1: search x3, cite x1 ; user 2: search x1
	for i := 0; i < 3; i++ {
		_ = m.RecordUsage(ctx, 1, "search")
	}
	_ = m.RecordUsage(ctx, 1, "cite")
	_ = m.RecordUsage(ctx, 2, "search")

	// FeatureUsage: search(4) before cite(1).
	feats, err := m.FeatureUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feats) != 2 || feats[0].Action != "search" || feats[0].Count != 4 {
		t.Fatalf("FeatureUsage = %+v, want search=4 first", feats)
	}
	if feats[1].Action != "cite" || feats[1].Count != 1 {
		t.Errorf("second feature = %+v, want cite=1", feats[1])
	}

	// TopUsers: user 1 (4) before user 2 (1), with names.
	top, err := m.TopUsers(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 || top[0].UserID != 1 || top[0].Count != 4 || top[0].Name != "علي" {
		t.Fatalf("TopUsers = %+v, want user1=4 first", top)
	}

	// Limit is honored.
	if one, _ := m.TopUsers(ctx, 1); len(one) != 1 {
		t.Errorf("TopUsers limit=1 returned %d", len(one))
	}

	// Totals: 5 actions, 2 active users.
	actions, active, err := m.UsageTotals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if actions != 5 || active != 2 {
		t.Errorf("UsageTotals = (%d,%d), want (5,2)", actions, active)
	}

	// Weekday & hour buckets sum to the total action count.
	wd, _ := m.UsageByWeekday(ctx)
	sum := 0
	for _, n := range wd {
		sum += n
	}
	if sum != 5 {
		t.Errorf("weekday sum = %d, want 5", sum)
	}
	hr, _ := m.UsageByHour(ctx)
	sum = 0
	for _, n := range hr {
		sum += n
	}
	if sum != 5 {
		t.Errorf("hour sum = %d, want 5", sum)
	}

	// UserEvents returns only that user's actions.
	ev, _ := m.UserEvents(ctx, 1, 50)
	if len(ev) != 4 { // user 1 did search x3 + cite x1
		t.Errorf("user 1 events = %d, want 4", len(ev))
	}
	if ev2, _ := m.UserEvents(ctx, 2, 50); len(ev2) != 1 {
		t.Errorf("user 2 events = %d, want 1", len(ev2))
	}
	if ev3, _ := m.UserEvents(ctx, 999, 50); len(ev3) != 0 {
		t.Errorf("unknown user events = %d, want 0", len(ev3))
	}
}

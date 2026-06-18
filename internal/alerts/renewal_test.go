package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/domain"
)

func TestRenewalReminder_NudgesOncePerExpiry(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	soon := now.AddDate(0, 0, 2)  // within window
	later := now.AddDate(0, 1, 0) // far away

	users := fakeUsers{users: []domain.User{
		{TelegramID: 1, Tier: domain.TierResearcher, PremiumUntil: &soon},  // remind
		{TelegramID: 2, Tier: domain.TierResearcher, PremiumUntil: &later}, // too far
		{TelegramID: 3, Tier: domain.TierStudent, PremiumUntil: nil},       // permanent
		{TelegramID: 4, Tier: domain.TierFree},                             // free
	}}
	logm := &memLog{}
	notify := &fakeNotify{}
	r := NewRenewalReminder(users, logm, notify, 3, time.Hour)

	r.RunOnce(context.Background(), now)
	if notify.calls != 1 || notify.lastTo != 1 {
		t.Fatalf("first pass: calls=%d to=%d, want 1 to user 1", notify.calls, notify.lastTo)
	}

	// Second pass must not re-notify (deduped by expiry date).
	r.RunOnce(context.Background(), now)
	if notify.calls != 1 {
		t.Errorf("second pass re-notified: calls=%d", notify.calls)
	}
}

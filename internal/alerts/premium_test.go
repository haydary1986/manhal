package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/domain"
)

type fakeSaver struct{ saved []domain.User }

func (f *fakeSaver) SaveUser(_ context.Context, u *domain.User) error {
	f.saved = append(f.saved, *u)
	return nil
}

func TestPremiumExpiry_DowngradesOnlyExpired(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -1)
	future := now.AddDate(0, 1, 0)

	users := fakeUsers{users: []domain.User{
		{TelegramID: 1, Tier: domain.TierResearcher, PremiumUntil: &past},   // expired → downgrade
		{TelegramID: 2, Tier: domain.TierResearcher, PremiumUntil: &future}, // active → keep
		{TelegramID: 3, Tier: domain.TierStudent, PremiumUntil: nil},        // permanent → keep
		{TelegramID: 4, Tier: domain.TierFree},                              // free → ignore
	}}
	saver := &fakeSaver{}
	notify := &fakeNotify{}

	NewPremiumExpiry(users, saver, notify, time.Hour).RunOnce(context.Background(), now)

	if len(saver.saved) != 1 {
		t.Fatalf("saved %d users, want 1", len(saver.saved))
	}
	got := saver.saved[0]
	if got.TelegramID != 1 || got.Tier != domain.TierFree || got.PremiumUntil != nil {
		t.Errorf("downgrade wrong: %+v", got)
	}
	if notify.calls != 1 || notify.lastTo != 1 {
		t.Errorf("notify calls=%d to=%d, want 1 to user 1", notify.calls, notify.lastTo)
	}
}

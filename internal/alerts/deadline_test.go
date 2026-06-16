package alerts

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/domain"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func ptr(t time.Time) *time.Time { return &t }

type fakeAnnounce struct{ items []announce.Announcement }

func (f fakeAnnounce) List(now time.Time, fil announce.Filter) []announce.Announcement {
	var out []announce.Announcement
	for _, a := range f.items {
		if a.Expired(now) || !a.MatchesDiscipline(fil.Discipline) {
			continue
		}
		out = append(out, a)
	}
	return out
}

type fakeUsers struct{ users []domain.User }

func (f fakeUsers) ListUsers(context.Context) ([]domain.User, error) { return f.users, nil }

type memLog struct{ sent map[string]bool }

func (l *memLog) WasReminded(_ context.Context, uid int64, key string) (bool, error) {
	return l.sent[keyOf(uid, key)], nil
}
func (l *memLog) MarkReminded(_ context.Context, uid int64, key string) error {
	if l.sent == nil {
		l.sent = map[string]bool{}
	}
	l.sent[keyOf(uid, key)] = true
	return nil
}
func keyOf(uid int64, key string) string { return string(rune(uid)) + key }

func TestDeadlineReminder_NotifiesMatchingUsersOnce(t *testing.T) {
	now := date(2026, time.June, 16)
	items := []announce.Announcement{
		{ID: "conf", Title: "مؤتمر الحوسبة", Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.June, 20))}, // 4 days
		{ID: "far", Title: "بعيد", Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.August, 1))},          // >window
		{ID: "gen", Title: "عام", Deadline: ptr(date(2026, time.June, 18))},                                         // general, 2 days
	}
	users := []domain.User{
		{TelegramID: 1, Field: "cs"},
		{TelegramID: 2, Field: "med"},
	}
	notify := &fakeNotify{}
	log := &memLog{}
	dr := NewDeadlineReminder(fakeAnnounce{items}, fakeUsers{users}, log, notify, 7, 0)

	dr.RunOnce(context.Background(), now)

	// User 1 (cs): "conf" (cs match) + "gen" (general) = 2.
	// User 2 (med): only "gen" (general) = 1. "far" is beyond the window for both.
	if notify.calls != 3 {
		t.Errorf("notifications = %d, want 3", notify.calls)
	}
	if !strings.Contains(notify.lastMsg, "تذكير") {
		t.Errorf("reminder text wrong: %q", notify.lastMsg)
	}

	// A second pass must not re-notify (dedup).
	notify.calls = 0
	dr.RunOnce(context.Background(), now)
	if notify.calls != 0 {
		t.Errorf("second pass re-notified %d times, want 0", notify.calls)
	}
}

func TestDeadlineReminder_SkipsExpiredAndNoDeadline(t *testing.T) {
	now := date(2026, time.June, 16)
	items := []announce.Announcement{
		{ID: "expired", Title: "منتهٍ", Deadline: ptr(date(2026, time.May, 1))},
		{ID: "nodate", Title: "بلا موعد"},
	}
	notify := &fakeNotify{}
	dr := NewDeadlineReminder(fakeAnnounce{items}, fakeUsers{[]domain.User{{TelegramID: 1}}}, &memLog{}, notify, 7, 0)
	dr.RunOnce(context.Background(), now)
	if notify.calls != 0 {
		t.Errorf("expired/no-deadline items should not notify, got %d", notify.calls)
	}
}

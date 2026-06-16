package alerts

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/scholar"
)

type digestStore struct {
	users    []domain.User
	subs     map[int64][]domain.Subscription
	watches  map[int64][]domain.CitationWatch
	reminded map[string]bool
}

func (d *digestStore) ListUsers(context.Context) ([]domain.User, error) { return d.users, nil }
func (d *digestStore) ListSubscriptions(_ context.Context, uid int64) ([]domain.Subscription, error) {
	return d.subs[uid], nil
}
func (d *digestStore) ListCitationWatches(_ context.Context, uid int64) ([]domain.CitationWatch, error) {
	return d.watches[uid], nil
}
func (d *digestStore) WasReminded(_ context.Context, uid int64, key string) (bool, error) {
	return d.reminded[keyOf(uid, key)], nil
}
func (d *digestStore) MarkReminded(_ context.Context, uid int64, key string) error {
	if d.reminded == nil {
		d.reminded = map[string]bool{}
	}
	d.reminded[keyOf(uid, key)] = true
	return nil
}

var (
	sunday = date(2026, time.June, 14) // a Sunday
	monday = date(2026, time.June, 15)
)

func digestAnnounce() fakeAnnounce {
	return fakeAnnounce{items: []announce.Announcement{
		{ID: "conf", Title: "مؤتمر قريب", Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.June, 20))},
		{ID: "far", Title: "بعيد", Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.August, 1))},
	}}
}

func TestWeeklyDigest_ComposesAndDedups(t *testing.T) {
	store := &digestStore{
		users:   []domain.User{{TelegramID: 1, Name: "أحمد", Field: "cs"}},
		subs:    map[int64][]domain.Subscription{1: {{ID: "s1", Topic: "transformers"}}},
		watches: map[int64][]domain.CitationWatch{1: {{ID: "w1", AuthorName: "Yann LeCun", LastCitedBy: 300000}}},
	}
	notify := &fakeNotify{}
	search := fakeSearch{out: []scholar.SearchResult{res("10.1/p", "ورقة بارزة")}}
	d := NewWeeklyDigest(store, search, digestAnnounce(), notify, time.Sunday, 0)

	// Wrong weekday: nothing happens.
	d.RunOnce(context.Background(), monday)
	if notify.calls != 0 {
		t.Fatal("digest must not send on the wrong weekday")
	}

	// On the digest weekday: one push with all sections.
	d.RunOnce(context.Background(), sunday)
	if notify.calls != 1 {
		t.Fatalf("expected one digest, got %d", notify.calls)
	}
	for _, want := range []string{"موجزك الأسبوعي", "أحمد", "transformers", "ورقة بارزة", "مؤتمر قريب", "Yann LeCun"} {
		if !strings.Contains(notify.lastMsg, want) {
			t.Errorf("digest missing %q:\n%s", want, notify.lastMsg)
		}
	}
	if strings.Contains(notify.lastMsg, "بعيد") {
		t.Error("deadlines beyond the 14-day window should be excluded")
	}

	// Same ISO week again: deduped.
	notify.calls = 0
	d.RunOnce(context.Background(), sunday)
	if notify.calls != 0 {
		t.Error("digest must be sent once per ISO week")
	}
}

func TestWeeklyDigest_EmptyUserNoSend(t *testing.T) {
	store := &digestStore{users: []domain.User{{TelegramID: 2, Field: "med"}}} // med matches no cs deadlines
	notify := &fakeNotify{}
	d := NewWeeklyDigest(store, fakeSearch{}, digestAnnounce(), notify, time.Sunday, 0)
	d.RunOnce(context.Background(), sunday)
	if notify.calls != 0 {
		t.Error("a user with nothing relevant should not be messaged")
	}
}

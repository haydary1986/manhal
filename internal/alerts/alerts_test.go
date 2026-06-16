package alerts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/scholar"
)

func res(doi, title string) scholar.SearchResult {
	return scholar.SearchResult{DOI: doi, Title: title, Year: 2024}
}

func TestFresh(t *testing.T) {
	results := []scholar.SearchResult{res("10.1/a", "A"), res("10.1/b", "B"), res("", "no-doi"), res("10.1/c", "C")}
	fresh, seen := Fresh(results, []string{"10.1/a"})

	if len(fresh) != 2 || fresh[0].DOI != "10.1/b" || fresh[1].DOI != "10.1/c" {
		t.Errorf("fresh = %+v, want b and c", fresh)
	}
	// no-doi result is ignored; seen now has a, b, c.
	if len(seen) != 3 {
		t.Errorf("seen = %v, want 3", seen)
	}
}

func TestFresh_NothingNew(t *testing.T) {
	results := []scholar.SearchResult{res("10.1/a", "A")}
	fresh, _ := Fresh(results, []string{"10.1/a"})
	if len(fresh) != 0 {
		t.Errorf("expected nothing fresh, got %d", len(fresh))
	}
}

// --- scheduler test doubles ---

type fakeSearch struct{ out []scholar.SearchResult }

func (f fakeSearch) Search(_ context.Context, _ string, _ int) ([]scholar.SearchResult, error) {
	return f.out, nil
}

type fakeStore struct {
	subs    []domain.Subscription
	updated map[string][]string
}

func (f *fakeStore) ListAllSubscriptions(context.Context) ([]domain.Subscription, error) {
	return f.subs, nil
}
func (f *fakeStore) UpdateSubscriptionSeen(_ context.Context, id string, seen []string) error {
	if f.updated == nil {
		f.updated = map[string][]string{}
	}
	f.updated[id] = seen
	return nil
}

type fakeNotify struct {
	calls   int
	lastTo  int64
	lastMsg string
}

func (f *fakeNotify) Notify(userID int64, text string) error {
	f.calls++
	f.lastTo, f.lastMsg = userID, text
	return nil
}

func TestScheduler_RunOnce_PushesAndUpdatesSeen(t *testing.T) {
	store := &fakeStore{subs: []domain.Subscription{
		{ID: "s1", UserID: 42, Topic: "transformers", SeenDOIs: []string{"10.1/old"}},
	}}
	search := fakeSearch{out: []scholar.SearchResult{res("10.1/old", "Old"), res("10.1/new", "New")}}
	notify := &fakeNotify{}

	NewScheduler(store, search, notify, 0).RunOnce(context.Background())

	if notify.calls != 1 || notify.lastTo != 42 {
		t.Fatalf("expected one push to user 42, got calls=%d to=%d", notify.calls, notify.lastTo)
	}
	if !strings.Contains(notify.lastMsg, "New") || strings.Contains(notify.lastMsg, "Old") {
		t.Errorf("digest should contain only the new paper:\n%s", notify.lastMsg)
	}
	if got := store.updated["s1"]; len(got) != 2 {
		t.Errorf("seen should be updated to 2 dois, got %v", got)
	}
}

func TestScheduler_RunOnce_NoNewNoPush(t *testing.T) {
	store := &fakeStore{subs: []domain.Subscription{
		{ID: "s1", UserID: 1, Topic: "x", SeenDOIs: []string{"10.1/a"}},
	}}
	search := fakeSearch{out: []scholar.SearchResult{res("10.1/a", "A")}}
	notify := &fakeNotify{}

	NewScheduler(store, search, notify, 0).RunOnce(context.Background())
	if notify.calls != 0 {
		t.Error("no push expected when nothing is new")
	}
	if _, ok := store.updated["s1"]; ok {
		t.Error("seen should not be updated when nothing is new")
	}
}

// notifyFails verifies seen is NOT advanced when delivery fails (retry next run).
type notifyFails struct{}

func (notifyFails) Notify(int64, string) error { return errors.New("down") }

func TestScheduler_RunOnce_KeepsSeenOnNotifyFailure(t *testing.T) {
	store := &fakeStore{subs: []domain.Subscription{
		{ID: "s1", UserID: 1, Topic: "x", SeenDOIs: nil},
	}}
	search := fakeSearch{out: []scholar.SearchResult{res("10.1/new", "New")}}

	NewScheduler(store, search, notifyFails{}, 0).RunOnce(context.Background())
	if _, ok := store.updated["s1"]; ok {
		t.Error("seen must stay unchanged when notify fails, so we retry")
	}
}

package alerts

import (
	"context"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/scholar"
)

type fakeAuthors struct{ citedBy int }

func (f fakeAuthors) SearchAuthors(context.Context, string, int) ([]scholar.Author, error) {
	return []scholar.Author{{Name: "X", CitedBy: f.citedBy}}, nil
}

type watchStore struct {
	watches []domain.CitationWatch
	counts  map[string]int
}

func (w *watchStore) ListAllCitationWatches(context.Context) ([]domain.CitationWatch, error) {
	return w.watches, nil
}
func (w *watchStore) UpdateCitationCount(_ context.Context, id string, count int) error {
	if w.counts == nil {
		w.counts = map[string]int{}
	}
	w.counts[id] = count
	return nil
}

func TestCitationWatcher_NotifiesOnIncrease(t *testing.T) {
	store := &watchStore{watches: []domain.CitationWatch{{ID: "w1", UserID: 7, AuthorName: "Yann LeCun", LastCitedBy: 100}}}
	notify := &fakeNotify{}

	NewCitationWatcher(store, fakeAuthors{citedBy: 130}, notify, 0).RunOnce(context.Background())

	if notify.calls != 1 || notify.lastTo != 7 {
		t.Fatalf("expected one push to user 7, got calls=%d to=%d", notify.calls, notify.lastTo)
	}
	if !strings.Contains(notify.lastMsg, "+30") || !strings.Contains(notify.lastMsg, "130") {
		t.Errorf("alert should show delta and total: %q", notify.lastMsg)
	}
	if store.counts["w1"] != 130 {
		t.Errorf("count should advance to 130, got %d", store.counts["w1"])
	}
}

func TestCitationWatcher_NoChangeNoPush(t *testing.T) {
	store := &watchStore{watches: []domain.CitationWatch{{ID: "w1", UserID: 1, AuthorName: "A", LastCitedBy: 50}}}
	notify := &fakeNotify{}
	NewCitationWatcher(store, fakeAuthors{citedBy: 50}, notify, 0).RunOnce(context.Background())
	if notify.calls != 0 {
		t.Error("no push when count is unchanged")
	}
	if _, ok := store.counts["w1"]; ok {
		t.Error("count should not be updated when unchanged")
	}
}

func TestCitationWatcher_DownwardCorrectionUpdatesSilently(t *testing.T) {
	store := &watchStore{watches: []domain.CitationWatch{{ID: "w1", UserID: 1, AuthorName: "A", LastCitedBy: 50}}}
	notify := &fakeNotify{}
	NewCitationWatcher(store, fakeAuthors{citedBy: 40}, notify, 0).RunOnce(context.Background())
	if notify.calls != 0 {
		t.Error("a decrease should not notify")
	}
	if store.counts["w1"] != 40 {
		t.Errorf("count should be corrected to 40, got %d", store.counts["w1"])
	}
}

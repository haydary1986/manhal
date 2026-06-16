package store

import (
	"context"
	"testing"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/domain"
)

func TestMemory_Users(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if _, err := m.GetUser(ctx, 1); err != ErrNotFound {
		t.Errorf("missing user err = %v, want ErrNotFound", err)
	}
	if err := m.SaveUser(ctx, &domain.User{TelegramID: 1, Name: "A"}); err != nil {
		t.Fatal(err)
	}
	u, err := m.GetUser(ctx, 1)
	if err != nil || u.Name != "A" {
		t.Errorf("GetUser = (%+v, %v)", u, err)
	}
	if u.CreatedAt.IsZero() {
		t.Error("SaveUser should stamp CreatedAt")
	}
}

func TestMemory_Library(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	const uid int64 = 5

	if items, _ := m.ListLibrary(ctx, uid); len(items) != 0 {
		t.Errorf("fresh library should be empty, got %d", len(items))
	}

	a := domain.LibraryItem{ID: "a", Work: cite.Work{Title: "First"}}
	b := domain.LibraryItem{ID: "b", Work: cite.Work{Title: "Second"}}
	_ = m.AddLibraryItem(ctx, uid, a)
	_ = m.AddLibraryItem(ctx, uid, b)
	_ = m.AddLibraryItem(ctx, uid, a) // duplicate id ignored

	items, _ := m.ListLibrary(ctx, uid)
	if len(items) != 2 {
		t.Fatalf("library size = %d, want 2 (dedup)", len(items))
	}
	if items[0].ID != "b" {
		t.Errorf("newest first expected; got %q", items[0].ID)
	}
	if items[0].SavedAt.IsZero() {
		t.Error("AddLibraryItem should stamp SavedAt")
	}

	if err := m.RemoveLibraryItem(ctx, uid, "a"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if items, _ := m.ListLibrary(ctx, uid); len(items) != 1 || items[0].ID != "b" {
		t.Errorf("after remove = %+v", items)
	}
	if err := m.RemoveLibraryItem(ctx, uid, "missing"); err != ErrNotFound {
		t.Errorf("remove missing err = %v, want ErrNotFound", err)
	}
}

func TestMemory_Subscriptions(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	_ = m.AddSubscription(ctx, domain.Subscription{ID: "s1", UserID: 1, Topic: "ai"})
	_ = m.AddSubscription(ctx, domain.Subscription{ID: "s1", UserID: 1, Topic: "ai"}) // dup id ignored
	_ = m.AddSubscription(ctx, domain.Subscription{ID: "s2", UserID: 2, Topic: "nlp"})

	if subs, _ := m.ListSubscriptions(ctx, 1); len(subs) != 1 {
		t.Errorf("user 1 subs = %d, want 1", len(subs))
	}
	if all, _ := m.ListAllSubscriptions(ctx); len(all) != 2 {
		t.Errorf("all subs = %d, want 2", len(all))
	}

	if err := m.UpdateSubscriptionSeen(ctx, "s1", []string{"10.1/a"}); err != nil {
		t.Fatal(err)
	}
	subs, _ := m.ListSubscriptions(ctx, 1)
	if len(subs[0].SeenDOIs) != 1 {
		t.Errorf("seen not updated: %+v", subs[0])
	}

	if err := m.RemoveSubscription(ctx, 1, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveSubscription(ctx, 1, "s1"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

func TestMemory_CitationWatches(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()

	_ = m.AddCitationWatch(ctx, domain.CitationWatch{ID: "w1", UserID: 1, AuthorName: "A", LastCitedBy: 10})
	_ = m.AddCitationWatch(ctx, domain.CitationWatch{ID: "w1", UserID: 1, AuthorName: "A"}) // dup ignored
	_ = m.AddCitationWatch(ctx, domain.CitationWatch{ID: "w2", UserID: 2, AuthorName: "B"})

	if ws, _ := m.ListCitationWatches(ctx, 1); len(ws) != 1 || ws[0].LastCitedBy != 10 {
		t.Errorf("user 1 watches wrong: %+v", ws)
	}
	if all, _ := m.ListAllCitationWatches(ctx); len(all) != 2 {
		t.Errorf("all watches = %d, want 2", len(all))
	}

	if err := m.UpdateCitationCount(ctx, "w1", 25); err != nil {
		t.Fatal(err)
	}
	ws, _ := m.ListCitationWatches(ctx, 1)
	if ws[0].LastCitedBy != 25 {
		t.Errorf("count not updated: %+v", ws[0])
	}

	if err := m.RemoveCitationWatch(ctx, 1, "w1"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveCitationWatch(ctx, 1, "w1"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

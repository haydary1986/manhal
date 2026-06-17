package announce

import (
	"testing"
	"time"
)

func TestRepoAddRemoveAndSchedule(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	r := NewRepo(nil)

	future := now.Add(48 * time.Hour)
	_ = r.Add(Announcement{ID: "a1", Kind: KindGrant, Title: "منحة", PostedAt: now})
	_ = r.Add(Announcement{ID: "a2", Kind: KindJob, Title: "وظيفة مجدوَلة", PostedAt: now, PublishAt: &future})

	if r.Len() != 2 {
		t.Fatalf("Len = %d, want 2", r.Len())
	}
	if len(r.All()) != 2 {
		t.Errorf("All should show both (admin view), got %d", len(r.All()))
	}

	// The scheduled item is hidden from the public feed until its time.
	vis := r.List(now, Filter{})
	if len(vis) != 1 || vis[0].ID != "a1" {
		t.Fatalf("List now = %+v, want only a1", vis)
	}
	// After the publish time, both appear.
	if got := r.List(future.Add(time.Minute), Filter{}); len(got) != 2 {
		t.Errorf("List after schedule = %d, want 2", len(got))
	}

	// Duplicate id rejected.
	if err := r.Add(Announcement{ID: "a1", Kind: KindGrant, Title: "x"}); err != ErrDuplicateID {
		t.Errorf("duplicate add err = %v, want ErrDuplicateID", err)
	}

	// Remove.
	if err := r.Remove("a1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := r.Remove("missing"); err != ErrNotFound {
		t.Errorf("remove missing err = %v, want ErrNotFound", err)
	}
	if r.Len() != 1 {
		t.Errorf("Len after remove = %d, want 1", r.Len())
	}
}

package plans

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
)

func TestManager_UpsertAddUpdateRemove(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, DefaultPlans())
	if len(m.List()) != 3 {
		t.Fatalf("seed = %d, want 3", len(m.List()))
	}

	// Add a new plan.
	if err := m.Upsert(Plan{ID: "weekly", Name: "أسبوعي", Months: 0, Price: 1500, Tier: domain.TierStudent}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, ok := m.Get("weekly"); !ok {
		t.Fatal("added plan not found")
	}

	// Update existing (same id) changes in place, no duplicate.
	if err := m.Upsert(Plan{ID: "monthly", Name: "شهري مميّز", Months: 1, Price: 6000, Tier: domain.TierResearcher}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := m.Get("monthly")
	if got.Price != 6000 || got.Name != "شهري مميّز" {
		t.Errorf("update not applied: %+v", got)
	}
	if len(m.List()) != 4 {
		t.Errorf("update should not add a row: %d", len(m.List()))
	}

	// Persisted to disk.
	saved, err := os.ReadFile(filepath.Join(dir, "plans.yaml"))
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if !strings.Contains(string(saved), "شهري مميّز") {
		t.Errorf("saved file missing edit: %s", saved)
	}

	// Remove.
	if err := m.Remove("weekly"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := m.Get("weekly"); ok {
		t.Error("removed plan still present")
	}
	if err := m.Remove("nope"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

func TestManager_UpsertValidation(t *testing.T) {
	m := NewManager("", nil)
	if err := m.Upsert(Plan{ID: "", Name: "x"}); err != ErrInvalid {
		t.Errorf("empty id should be invalid, got %v", err)
	}
	// Negative months/price are clamped; unknown tier defaults to researcher.
	if err := m.Upsert(Plan{ID: "p", Name: "ع", Months: -5, Price: -9, Tier: "bogus"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	p, _ := m.Get("p")
	if p.Months != 0 || p.Price != 0 || p.Tier != domain.TierResearcher {
		t.Errorf("clamp/default failed: %+v", p)
	}
}

func TestLoad_DefaultsWhenAbsent(t *testing.T) {
	got, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("defaults = %d, want 3", len(got))
	}
}

func TestPlan_DurationLabel(t *testing.T) {
	cases := map[int]string{0: "دائم", 1: "شهر", 6: "6 أشهر", 12: "سنة"}
	for months, want := range cases {
		if got := (Plan{Months: months}).DurationLabel(); got != want {
			t.Errorf("DurationLabel(%d) = %q, want %q", months, got, want)
		}
	}
}

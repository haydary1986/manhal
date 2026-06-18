package promotion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_YAMLRoundTrip(t *testing.T) {
	m := NewManager("", DefaultRules())
	text, err := m.YAML()
	if err != nil {
		t.Fatalf("YAML: %v", err)
	}
	if !strings.Contains(text, "activities:") || !strings.Contains(text, "ranks:") {
		t.Fatalf("YAML missing sections: %q", text[:min(80, len(text))])
	}
	// Re-applying the serialized rules must succeed and keep the rank count.
	if err := m.SetYAML(text); err != nil {
		t.Fatalf("SetYAML round-trip: %v", err)
	}
	if got := len(m.Ranks()); got != 3 {
		t.Errorf("ranks = %d, want 3", got)
	}
}

func TestManager_SetYAML_LiveAndPersisted(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, DefaultRules())
	doc := `ranks:
  - key: lecturer
    label: مدرس
    next_label: أستاذ مساعد
    required_total: 100
    required_table1: 60
    required_table2: 30
    min_service_years: 4
activities:
  - key: if_first
    label: بحث IF — الأول
    table: 1
    points: {lecturer: 25}
`
	if err := m.SetYAML(doc); err != nil {
		t.Fatalf("SetYAML: %v", err)
	}
	// Live read reflects the edit.
	rk, ok := m.FindRank("lecturer")
	if !ok || rk.RequiredTotal != 100 {
		t.Fatalf("live rank not updated: %+v ok=%v", rk, ok)
	}
	if got := m.ActivitiesByTable(1); len(got) != 1 || got[0].PointsFor("lecturer") != 25 {
		t.Fatalf("activities not updated: %+v", got)
	}
	// Persisted to data/promotion.yaml.
	saved, err := os.ReadFile(filepath.Join(dir, "promotion.yaml"))
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if !strings.Contains(string(saved), "required_total: 100") {
		t.Errorf("saved file missing edit: %s", saved)
	}
}

func TestManager_SetYAML_InvalidLeavesLiveIntact(t *testing.T) {
	m := NewManager("", DefaultRules())
	before := len(m.Ranks())
	cases := []string{
		"::: not yaml :::",            // malformed
		"ranks: []\nactivities: []\n", // empty
		"ranks:\n  - {key: x, label: y}\nactivities:\n  - {key: a, label: b, table: 9}\n", // bad table
	}
	for _, c := range cases {
		if err := m.SetYAML(c); err == nil {
			t.Errorf("SetYAML(%q) accepted invalid input", c)
		}
	}
	if got := len(m.Ranks()); got != before {
		t.Errorf("live rules mutated after invalid edits: %d != %d", got, before)
	}
}

func TestManager_ResetDefault(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, &Rules{
		Ranks:      []Rank{{Key: "x", Label: "y"}},
		Activities: []Activity{{Key: "a", Label: "b", Table: 1, Points: map[string]float64{"*": 1}}},
	})
	if err := m.ResetDefault(); err != nil {
		t.Fatalf("ResetDefault: %v", err)
	}
	if got := len(m.Ranks()); got != 3 {
		t.Errorf("ranks after reset = %d, want 3", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "promotion.yaml")); err != nil {
		t.Errorf("reset did not persist: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

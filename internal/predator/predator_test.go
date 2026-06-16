package predator

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleList() *List {
	return NewList([]Flag{
		{Pattern: "Example Predatory Press", Reason: "ناشر في قائمة تحذير"},
		{Pattern: "fake metrics", Reason: "مقاييس وهمية"},
	})
}

func TestCheck_MatchesCaseInsensitiveSubstring(t *testing.T) {
	l := sampleList()

	got := l.Check("Journal by EXAMPLE PREDATORY PRESS Ltd")
	if len(got) != 1 || got[0].Reason != "ناشر في قائمة تحذير" {
		t.Errorf("expected one publisher match, got %+v", got)
	}

	if got := l.Check("Reputable Journal", "Springer"); len(got) != 0 {
		t.Errorf("clean journal should not match, got %+v", got)
	}
}

func TestCheck_MultipleFields(t *testing.T) {
	l := sampleList()
	got := l.Check("Some Title", "Fake Metrics Publishing")
	if len(got) != 1 || got[0].Pattern != "fake metrics" {
		t.Errorf("expected publisher-field match, got %+v", got)
	}
}

func TestAssess_RiskLevels(t *testing.T) {
	flags := []Flag{{Pattern: "x", Reason: "r"}}

	if a := Assess(false, flags); a.Risk != RiskHigh {
		t.Errorf("flagged => %v, want high", a.Risk)
	}
	if a := Assess(true, flags); a.Risk != RiskHigh {
		t.Errorf("flagged overrides indexing => %v, want high", a.Risk)
	}
	if a := Assess(true, nil); a.Risk != RiskLow {
		t.Errorf("indexed, unflagged => %v, want low", a.Risk)
	}
	if a := Assess(false, nil); a.Risk != RiskMedium {
		t.Errorf("unknown => %v, want medium", a.Risk)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	yaml := "flags:\n  - pattern: \"Bad Press\"\n    reason: \"سبب\"\n"
	if err := os.WriteFile(filepath.Join(dir, "predatory.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if l.Len() != 1 {
		t.Fatalf("Len = %d, want 1", l.Len())
	}
	if got := l.Check("published by bad press"); len(got) != 1 {
		t.Errorf("loaded flag should match, got %+v", got)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	l, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if l.Len() != 0 {
		t.Errorf("Len = %d, want 0", l.Len())
	}
}

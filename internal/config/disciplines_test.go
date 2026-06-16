package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDisciplines_DefaultWhenMissing(t *testing.T) {
	got, err := LoadDisciplines(t.TempDir())
	if err != nil {
		t.Fatalf("LoadDisciplines: %v", err)
	}
	if len(got) != len(DefaultDisciplines()) {
		t.Errorf("missing file should yield defaults, got %d", len(got))
	}
}

func TestLoadDisciplines_FromFile(t *testing.T) {
	dir := t.TempDir()
	yaml := "disciplines:\n  - id: phys\n    label: الفيزياء\n  - id: chem\n    label: الكيمياء\n"
	if err := os.WriteFile(filepath.Join(dir, "disciplines.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDisciplines(dir)
	if err != nil {
		t.Fatalf("LoadDisciplines: %v", err)
	}
	if len(got) != 2 || got[0].ID != "phys" || got[1].Label != "الكيمياء" {
		t.Errorf("parsed disciplines = %+v", got)
	}
}

func TestDisciplineLabel(t *testing.T) {
	list := []Discipline{{ID: "cs", Label: "علوم الحاسوب"}}
	if got := DisciplineLabel(list, "cs"); got != "علوم الحاسوب" {
		t.Errorf("label = %q", got)
	}
	if got := DisciplineLabel(list, "unknown"); got != "unknown" {
		t.Errorf("unknown id should echo back, got %q", got)
	}
}

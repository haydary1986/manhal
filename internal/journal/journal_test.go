package journal

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleIndex() *Index {
	return NewIndex([]Journal{
		{Title: "Nature", ISSNs: []string{"0028-0836", "1476-4687"}, SJR: "18.5", Quartile: "Q1",
			Publisher: "Nature Research", Country: "United Kingdom", Categories: []string{"Multidisciplinary"}},
		{Title: "Heliyon", ISSNs: []string{"2405-8440"}, Quartile: "Q2", Publisher: "Elsevier"},
	})
}

func TestLookup_ByTitle(t *testing.T) {
	ix := sampleIndex()
	j, ok := ix.Lookup("  nature ")
	if !ok || j.Quartile != "Q1" {
		t.Fatalf("lookup by title = (%+v, %v)", j, ok)
	}
}

func TestLookup_ByISSN(t *testing.T) {
	ix := sampleIndex()
	// Hyphenated and bare forms must both resolve.
	for _, q := range []string{"0028-0836", "00280836", "1476-4687"} {
		j, ok := ix.Lookup(q)
		if !ok || j.Title != "Nature" {
			t.Errorf("lookup %q = (%+v, %v)", q, j, ok)
		}
	}
}

func TestLookup_NotFound(t *testing.T) {
	ix := sampleIndex()
	if _, ok := ix.Lookup("Unknown Journal"); ok {
		t.Error("expected miss for unknown title")
	}
	if _, ok := ix.Lookup("9999-9999"); ok {
		t.Error("expected miss for unknown ISSN")
	}
}

func TestSearch_Contains(t *testing.T) {
	ix := sampleIndex()
	got := ix.Search("heli", 5)
	if len(got) != 1 || got[0].Title != "Heliyon" {
		t.Errorf("search = %+v", got)
	}
}

func TestQuartileLabel(t *testing.T) {
	if got := QuartileLabel("q1"); got == "" || got[:2] != "Q1" {
		t.Errorf("Q1 label = %q", got)
	}
	if got := QuartileLabel(""); got != "غير مصنّفة" {
		t.Errorf("empty quartile label = %q", got)
	}
}

func TestLoadCSV(t *testing.T) {
	dir := t.TempDir()
	csv := "title,issn,sjr,quartile,h_index,publisher,country,categories\n" +
		"Science,\"00368075,10959203\",13.328,Q1,1296,AAAS,United States,Multidisciplinary\n" +
		"Heliyon,24058440,0.563,Q2,98,Elsevier,United Kingdom,Multidisciplinary\n"
	if err := os.WriteFile(filepath.Join(dir, "journals.csv"), []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}

	ix, err := LoadCSV(dir)
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}
	if ix.Len() != 2 {
		t.Fatalf("Len = %d, want 2", ix.Len())
	}
	j, ok := ix.Lookup("0036-8075")
	if !ok || j.Title != "Science" || j.Quartile != "Q1" {
		t.Errorf("Science lookup = (%+v, %v)", j, ok)
	}
	if len(j.ISSNs) != 2 {
		t.Errorf("Science ISSNs = %v, want 2", j.ISSNs)
	}
}

func TestLoadCSV_MissingFile(t *testing.T) {
	ix, err := LoadCSV(t.TempDir())
	if err != nil {
		t.Fatalf("LoadCSV missing: %v", err)
	}
	if ix.Len() != 0 {
		t.Errorf("Len = %d, want 0", ix.Len())
	}
}

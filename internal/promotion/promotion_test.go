package promotion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompute_AllGatesMet(t *testing.T) {
	r := DefaultRules()
	// Assistant lecturer -> مدرس: total 70, t1 46, t2 24, 3 years.
	// Two first-author IF papers (30 each) = 60 in Table 1.
	in := Input{
		RankKey: "assistant_lecturer",
		Counts: map[string]float64{
			"if_first":         2, // 60 (table 1)
			"book_local_solo":  2, // 16 (table 2)
			"conference_paper": 2, // 10 (table 2) => table2 = 26
		},
		ServiceYears: 4,
	}
	res, ok := r.Compute(in)
	if !ok {
		t.Fatal("compute should succeed")
	}
	if res.Table1 != 60 {
		t.Errorf("table1 = %v, want 60", res.Table1)
	}
	if res.Table2 != 26 {
		t.Errorf("table2 = %v, want 26", res.Table2)
	}
	if res.Total != 86 {
		t.Errorf("total = %v, want 86", res.Total)
	}
	if !res.Table1Met || !res.Table2Met || !res.TotalMet || !res.ServiceMet || !res.Eligible {
		t.Errorf("all gates should pass: %+v", res)
	}
}

func TestCompute_PerRankPoints(t *testing.T) {
	r := DefaultRules()
	// A first-author IF paper is worth 30 to a مدرس مساعد but 20 to a مدرس.
	al, _ := r.Compute(Input{RankKey: "assistant_lecturer", Counts: map[string]float64{"if_first": 1}})
	l, _ := r.Compute(Input{RankKey: "lecturer", Counts: map[string]float64{"if_first": 1}})
	if al.Table1 != 30 {
		t.Errorf("assistant_lecturer if_first = %v, want 30", al.Table1)
	}
	if l.Table1 != 20 {
		t.Errorf("lecturer if_first = %v, want 20", l.Table1)
	}
}

func TestCompute_Table1ShortfallBlocksEligibility(t *testing.T) {
	r := DefaultRules()
	// Lots of Table-2 points but almost no research => Table1 gate fails.
	res, _ := r.Compute(Input{
		RankKey:      "assistant_lecturer",
		Counts:       map[string]float64{"if_first": 1, "patent_intl": 3}, // t1=30 (<46), t2=25
		ServiceYears: 5,
	})
	if res.Table1Met {
		t.Error("table1 (30) should not meet the 46 requirement")
	}
	if res.Eligible {
		t.Error("must not be eligible when the Table-1 gate fails")
	}
}

func TestCompute_AppliesCap(t *testing.T) {
	r := DefaultRules()
	// book_chapter is 5 each, capped at 6.
	res, _ := r.Compute(Input{RankKey: "lecturer", Counts: map[string]float64{"book_chapter": 5}})
	if res.Table2 != 6 {
		t.Errorf("capped book_chapter = %v, want 6", res.Table2)
	}
	if len(res.Breakdown2) != 1 || !res.Breakdown2[0].Capped {
		t.Errorf("breakdown should flag the cap: %+v", res.Breakdown2)
	}
}

func TestCompute_UnknownRank(t *testing.T) {
	if _, ok := DefaultRules().Compute(Input{RankKey: "nope"}); ok {
		t.Error("unknown rank should return ok=false")
	}
}

func TestActivitiesByTable(t *testing.T) {
	r := DefaultRules()
	if len(r.ActivitiesByTable(1)) != 12 {
		t.Errorf("table 1 activities = %d, want 12", len(r.ActivitiesByTable(1)))
	}
	if len(r.ActivitiesByTable(2)) == 0 {
		t.Error("table 2 should have activities")
	}
}

func TestParseActivities(t *testing.T) {
	counts, years := ParseActivities("if_first: 3\nbook_chapter: ٢\nyears: 4\ngarbage\nbad: x")
	if counts["if_first"] != 3 || counts["book_chapter"] != 2 {
		t.Errorf("counts = %v", counts)
	}
	if years != 4 {
		t.Errorf("years = %d, want 4", years)
	}
	if _, exists := counts["bad"]; exists {
		t.Error("non-numeric value should be skipped")
	}
}

func TestLoad_FromFileMatchesDefaults(t *testing.T) {
	// A round-trip through YAML must preserve the per-rank point maps.
	dir := t.TempDir()
	yaml := `activities:
  - key: if_first
    label: بحث
    table: 1
    points:
      assistant_lecturer: 30
      lecturer: 20
ranks:
  - key: assistant_lecturer
    label: مدرس مساعد
    next_label: مدرس
    required_total: 70
    required_table1: 46
    required_table2: 24
    min_service_years: 3
`
	if err := os.WriteFile(filepath.Join(dir, "promotion.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	res, ok := r.Compute(Input{RankKey: "assistant_lecturer", Counts: map[string]float64{"if_first": 1}})
	if !ok || res.Table1 != 30 {
		t.Errorf("loaded per-rank points wrong: table1=%v ok=%v", res.Table1, ok)
	}
}

func TestLoad_DefaultWhenMissing(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(r.Ranks) != 3 {
		t.Errorf("default ranks = %d, want 3", len(r.Ranks))
	}
}

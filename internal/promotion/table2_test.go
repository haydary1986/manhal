package promotion

import "testing"

// The calculator must now encode the full official Table-2 annex, including the
// items that were previously reference-only.
func TestDefaultRules_Table2CoversNewItems(t *testing.T) {
	r := DefaultRules()
	byKey := map[string]Activity{}
	for _, a := range r.ActivitiesByTable(2) {
		byKey[a.Key] = a
	}
	wantPoints := map[string]float64{
		"review_eval_in":      1,
		"review_eval_out":     2,
		"seminar_committee":   1,
		"new_dept_committee":  1,
		"training_lecturer":   1,
		"edu_guidance":        1,
		"extracurricular":     1,
		"dorm_supervision":    2,
		"intl_cooperation":    2,
		"teaching_hospital":   15,
		"sports_activity":     2,
		"graduate_employment": 1,
		"inventory_committee": 1,
		"perf_eval_70":        4,
		"perf_eval_80":        6,
		"perf_eval_90":        8,
		"creative_work":       1,
	}
	for k, pts := range wantPoints {
		a, ok := byKey[k]
		if !ok {
			t.Errorf("missing Table-2 item %q", k)
			continue
		}
		if a.PointsFor("*") != pts {
			t.Errorf("%s points = %v, want %v", k, a.PointsFor("*"), pts)
		}
	}
}

func TestCompute_NewItemsRespectCaps(t *testing.T) {
	r := DefaultRules()
	// teaching_hospital is worth 15 but capped at 15 → two still yields 15.
	res, ok := r.Compute(Input{RankKey: "lecturer", Counts: map[string]float64{"teaching_hospital": 2}})
	if !ok {
		t.Fatal("compute failed")
	}
	if res.Table2 != 15 {
		t.Errorf("teaching_hospital capped value = %v, want 15", res.Table2)
	}
	// inventory_committee: 1 each, cap 3 → five committees → 3.
	res2, _ := r.Compute(Input{RankKey: "lecturer", Counts: map[string]float64{"inventory_committee": 5}})
	if res2.Table2 != 3 {
		t.Errorf("inventory cap = %v, want 3", res2.Table2)
	}
}

package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func TestRadarScreen_DashboardAndWorks(t *testing.T) {
	au := scholar.Author{
		ID: "A1", Name: "Yann LeCun", Institution: "NYU",
		WorksCount: 400, CitedBy: 300000, HIndex: 150, I10Index: 350,
		Concepts: []string{"Deep Learning"}, ORCID: "0000-0002-1825-0097",
	}
	works := []scholar.SearchResult{
		{Title: "Deep Learning", Year: 2015, CitedBy: 90000, DOI: "10.1/dl"},
	}
	scr := radarScreen(au, works)
	for _, want := range []string{"رادار البحث", "Yann LeCun", "NYU", "h-index: 150", "i10: 350",
		"الاستشهادات: 300000", "أبرز أعماله", "Deep Learning", "90000", "ORCID"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("radar missing %q in:\n%s", want, scr.Text)
		}
	}
	// Must offer a follow-citations action.
	var hasFollow bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "cwatch:add" {
				hasFollow = true
			}
		}
	}
	if !hasFollow {
		t.Error("radar should offer a follow-citations button")
	}
}

func TestRadarScreen_NoWorks(t *testing.T) {
	scr := radarScreen(scholar.Author{Name: "X", HIndex: 3}, nil)
	if strings.Contains(scr.Text, "أبرز أعماله") {
		t.Error("works section should be omitted when there are no works")
	}
}

func TestSessions_RadarState(t *testing.T) {
	s := newSessions()
	s.set(6, stateAwaitRadar)
	if s.get(6) != stateAwaitRadar {
		t.Error("radar state should round-trip")
	}
}

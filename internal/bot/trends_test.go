package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func TestTrendsResultScreen(t *testing.T) {
	results := []scholar.SearchResult{
		{Title: "Attention Is All You Need", Authors: []string{"A. Vaswani"}, Year: 2017, CitedBy: 100000, DOI: "10.5/x"},
	}
	scr := trendsResultScreen("transformers", results)
	for _, want := range []string{"الأكثر استشهاداً", "transformers", "Attention Is All You Need", "100000", "OpenAlex"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("trends screen missing %q in:\n%s", want, scr.Text)
		}
	}
}

func TestTrendsResultScreen_Empty(t *testing.T) {
	if !strings.Contains(trendsResultScreen("nothing", nil).Text, "لا توجد نتائج") {
		t.Error("empty trends should say no results")
	}
}

func TestSessions_Trend(t *testing.T) {
	s := newSessions()
	s.set(4, stateAwaitTrend)
	if s.get(4) != stateAwaitTrend {
		t.Error("trend state should round-trip")
	}
}

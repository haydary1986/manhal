package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func gapPapers() []scholar.SearchResult {
	return []scholar.SearchResult{
		{Title: "Survey of X", Authors: []string{"A. B"}, Year: 2022, DOI: "10.1/x"},
		{Title: "Method Y", Authors: []string{"C. D"}, Year: 2023},
	}
}

func TestGapUserPrompt_GroundsInPapers(t *testing.T) {
	p := gapUserPrompt("federated learning", gapPapers())
	for _, want := range []string{"federated learning", "استند إليها حصراً", "Survey of X", "[2]"} {
		if !strings.Contains(p, want) {
			t.Errorf("gap prompt missing %q", want)
		}
	}
}

func TestGapResultScreen_AnalysisAndPapers(t *testing.T) {
	scr := gapResultScreen("الفجوة: لا توجد دراسات عربية.", gapPapers())
	for _, want := range []string{"تحليل الفجوة", "لا توجد دراسات عربية", "الأوراق المُحلَّلة", "Survey of X", "استرشادي"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("gap result missing %q:\n%s", want, scr.Text)
		}
	}
}

func TestSessions_Gap(t *testing.T) {
	s := newSessions()
	s.set(8, stateAwaitGap)
	if s.get(8) != stateAwaitGap {
		t.Error("gap state should round-trip")
	}
}

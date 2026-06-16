package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func TestSimilarityResultScreen_AdvisoryAndSources(t *testing.T) {
	related := []scholar.SearchResult{
		{Title: "A Related Study", Authors: []string{"A. B"}, Year: 2021, DOI: "10.1/r"},
	}
	scr := similarityResultScreen("هذه الجملة تحتاج استشهاداً.", related)
	for _, want := range []string{"مراجعة التشابه", "تحتاج استشهاداً", "مصادر مفتوحة ذات صلة", "A Related Study", "ليس بديلاً عن Turnitin"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("similarity result missing %q:\n%s", want, scr.Text)
		}
	}
}

func TestSimilarityPromptScreen_Disclaimer(t *testing.T) {
	if !strings.Contains(similarityPromptScreen().Text, "Turnitin") {
		t.Error("prompt should carry the not-a-Turnitin disclaimer")
	}
}

func TestSimilarityResultScreen_NoSources(t *testing.T) {
	scr := similarityResultScreen("ملاحظات.", nil)
	if strings.Contains(scr.Text, "مصادر مفتوحة ذات صلة") {
		t.Error("sources section should be omitted when none are found")
	}
	if !strings.Contains(scr.Text, "Turnitin") {
		t.Error("disclaimer must always be present")
	}
}

func TestSessions_SimilarityState(t *testing.T) {
	s := newSessions()
	s.set(2, stateAwaitSimilarity)
	if s.get(2) != stateAwaitSimilarity {
		t.Error("similarity state should round-trip")
	}
}

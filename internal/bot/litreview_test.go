package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func litPapers() []scholar.SearchResult {
	return []scholar.SearchResult{
		{Title: "Deep Learning", Authors: []string{"Y. LeCun", "Y. Bengio"}, Year: 2015, DOI: "10.1038/nature14539"},
		{Title: "Attention Is All You Need", Authors: []string{"A. Vaswani"}, Year: 2017},
	}
}

func TestLitReviewUserPrompt_EmbedsRealPapers(t *testing.T) {
	prompt := litReviewUserPrompt("machine learning", litPapers())
	for _, want := range []string{"machine learning", "استند إليها حصراً", "Deep Learning", "[2]", "Vaswani"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("grounding prompt missing %q", want)
		}
	}
}

func TestLitReviewResultScreen_AppendsReferences(t *testing.T) {
	scr := litReviewResultScreen("نص المراجعة...", litPapers())
	for _, want := range []string{"مسودة مراجعة", "المراجع (أوراق حقيقية)", "Deep Learning", "10.1038/nature14539", "استرشادية"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("result missing %q:\n%s", want, scr.Text)
		}
	}
}

func TestReferenceLine(t *testing.T) {
	withDOI := referenceLine(scholar.SearchResult{Title: "T", Authors: []string{"A. B"}, Year: 2020, DOI: "10.1/x"})
	if !strings.Contains(withDOI, "A. B · 2020. T") || !strings.Contains(withDOI, "doi.org/10.1/x") {
		t.Errorf("reference line = %q", withDOI)
	}
	bare := referenceLine(scholar.SearchResult{Title: "Solo"})
	if bare != "Solo" {
		t.Errorf("bare reference = %q, want 'Solo'", bare)
	}
}

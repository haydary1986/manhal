package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func sampleResults() []scholar.SearchResult {
	return []scholar.SearchResult{
		{Title: "Attention Is All You Need", Authors: []string{"A. Vaswani", "N. Shazeer"},
			Year: 2017, Venue: "NeurIPS", CitedBy: 100000, DOI: "10.5555/abc"},
		{Title: "Paper Without DOI", Authors: []string{"J. Roe"}, Year: 2020, CitedBy: 3},
	}
}

func TestSearchResultsScreen_CiteButtonsOnlyForDOI(t *testing.T) {
	scr := searchResultsScreen("transformers", sampleResults())

	if !strings.Contains(scr.Text, "Attention Is All You Need") {
		t.Error("missing first result title")
	}
	if !strings.Contains(scr.Text, "بدون DOI") {
		t.Error("expected no-DOI warning on second result")
	}

	var citeButtons int
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "search:cite:") {
				citeButtons++
			}
		}
	}
	if citeButtons != 1 {
		t.Errorf("cite buttons = %d, want 1 (only the DOI result)", citeButtons)
	}
}

func TestSearchResultsScreen_Empty(t *testing.T) {
	scr := searchResultsScreen("nothing", nil)
	if !strings.Contains(scr.Text, "لا توجد نتائج") {
		t.Errorf("empty results screen = %q", scr.Text)
	}
}

func TestRenderSearchResult(t *testing.T) {
	r := sampleResults()[0]
	got := renderSearchResult(r)
	for _, want := range []string{"Attention Is All You Need", "A. Vaswani", "2017", "NeurIPS", "100000"} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "بدون DOI") {
		t.Error("result with DOI should not show the warning")
	}
}

func TestAuthorsLine(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"A"}, "A"},
		{[]string{"A", "B", "C"}, "A، B، C"},
		{[]string{"A", "B", "C", "D"}, "A، B، C وآخرون"},
	}
	for _, tt := range tests {
		if got := authorsLine(tt.in); got != tt.want {
			t.Errorf("authorsLine(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSessionsResults_RoundTrip(t *testing.T) {
	s := newSessions()
	const uid int64 = 9

	if _, ok := s.resultAt(uid, 0); ok {
		t.Error("empty session should have no result at 0")
	}

	s.set(uid, stateAwaitQuery)
	s.setResults(uid, sampleResults())

	if s.get(uid) != stateNone {
		t.Error("setResults should reset state to none")
	}
	r, ok := s.resultAt(uid, 1)
	if !ok || r.Title != "Paper Without DOI" {
		t.Errorf("resultAt(1) = (%+v, %v)", r, ok)
	}
	if _, ok := s.resultAt(uid, 5); ok {
		t.Error("out-of-range index should return ok=false")
	}

	s.clear(uid)
	if _, ok := s.resultAt(uid, 0); ok {
		t.Error("clear should drop stored results")
	}
}

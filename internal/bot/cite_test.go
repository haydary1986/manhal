package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/scholar"
)

func TestSessions(t *testing.T) {
	s := newSessions()
	const uid int64 = 42

	if got := s.get(uid); got != stateNone {
		t.Errorf("fresh session = %q, want none", got)
	}
	s.set(uid, stateAwaitDOI)
	if got := s.get(uid); got != stateAwaitDOI {
		t.Errorf("after set = %q, want await_doi", got)
	}
	s.clear(uid)
	if got := s.get(uid); got != stateNone {
		t.Errorf("after clear = %q, want none", got)
	}
}

func TestCiteResultScreen(t *testing.T) {
	w := &cite.Work{
		Type:           "journal-article",
		Title:          "On computable numbers",
		Authors:        []cite.Author{{Family: "Turing", Given: "Alan M"}},
		ContainerTitle: "Proc. London Math. Soc.",
		Volume:         "42",
		Year:           1936,
		DOI:            "10.1112/plms/s2-42.1.230",
	}
	scr := citeResultScreen(w)

	for _, want := range []string{"On computable numbers", "APA", "MLA", "Chicago", "Harvard", "IEEE", "Vancouver", "BibTeX", "@article"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("result screen missing %q", want)
		}
	}
	if scr.Keyboard == nil || len(scr.Keyboard.Rows) == 0 {
		t.Error("result screen should have navigation buttons")
	}
}

func TestCiteResultScreen_NoTitleUsesDOI(t *testing.T) {
	w := &cite.Work{DOI: "10.1/abc"}
	if !strings.Contains(citeResultScreen(w).Text, "10.1/abc") {
		t.Error("expected DOI fallback when title is empty")
	}
}

func TestCiteErrorScreen(t *testing.T) {
	tests := []struct {
		err      error
		fragment string
	}{
		{scholar.ErrInvalidDOI, "DOI صحيح"},
		{scholar.ErrNotFound, "ما لقيت"},
		{scholar.ErrNotFound, "ما لقيت"},
	}
	for _, tt := range tests {
		if got := citeErrorScreen(tt.err).Text; !strings.Contains(got, tt.fragment) {
			t.Errorf("error screen for %v = %q, missing %q", tt.err, got, tt.fragment)
		}
	}
}

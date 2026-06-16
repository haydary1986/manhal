package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func TestAuthorProfileScreen(t *testing.T) {
	au := scholar.Author{
		Name: "Yann LeCun", Institution: "New York University",
		WorksCount: 400, CitedBy: 300000, HIndex: 150, I10Index: 350,
		ORCID: "0000-0002-1825-0097", Concepts: []string{"Computer science", "AI"},
	}
	scr := authorProfileScreen(au)

	for _, want := range []string{"Yann LeCun", "New York University", "h-index: 150", "i10-index: 350", "400", "300000", "Computer science", "0000-0002-1825-0097", "OpenAlex"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("profile missing %q in:\n%s", want, scr.Text)
		}
	}
}

func TestAuthorProfileScreen_OmitsZeroI10(t *testing.T) {
	scr := authorProfileScreen(scholar.Author{Name: "X", HIndex: 2})
	if strings.Contains(scr.Text, "i10-index") {
		t.Error("zero i10-index should be omitted")
	}
}

func TestAuthorChoicesScreen_OneButtonPerAuthor(t *testing.T) {
	authors := []scholar.Author{
		{Name: "John Smith", Institution: "MIT", WorksCount: 50, HIndex: 20},
		{Name: "John Smith", Institution: "Oxford", WorksCount: 10, HIndex: 5},
	}
	scr := authorChoicesScreen("John Smith", authors)

	if !strings.Contains(scr.Text, "MIT") || !strings.Contains(scr.Text, "Oxford") {
		t.Errorf("choices missing institutions:\n%s", scr.Text)
	}
	var profileButtons int
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "author:profile:") {
				profileButtons++
			}
		}
	}
	if profileButtons != 2 {
		t.Errorf("profile buttons = %d, want 2", profileButtons)
	}
}

func TestSessionsAuthors_RoundTrip(t *testing.T) {
	s := newSessions()
	const uid int64 = 3

	if _, ok := s.authorAt(uid, 0); ok {
		t.Error("empty session should have no author")
	}
	s.set(uid, stateAwaitAuthor)
	s.setAuthors(uid, []scholar.Author{{Name: "A"}, {Name: "B"}})

	if s.get(uid) != stateNone {
		t.Error("setAuthors should reset state to none")
	}
	a, ok := s.authorAt(uid, 1)
	if !ok || a.Name != "B" {
		t.Errorf("authorAt(1) = (%+v, %v)", a, ok)
	}
	if _, ok := s.authorAt(uid, 9); ok {
		t.Error("out-of-range index should be false")
	}
}

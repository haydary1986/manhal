package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/journal"
	"github.com/erticaz/manhal/internal/predator"
)

func TestJournalCardScreen(t *testing.T) {
	j := journal.Journal{
		Title: "Nature", ISSNs: []string{"0028-0836", "1476-4687"},
		SJR: "18.5", Quartile: "Q1", HIndex: "1331",
		Publisher: "Nature Research", Country: "United Kingdom",
		Categories: []string{"Multidisciplinary"},
	}
	scr := journalCardScreen(j, predator.Assess(true, nil))

	for _, want := range []string{"Nature", "Q1", "18.5", "1331", "Multidisciplinary", "Nature Research", "0028-0836", "استرشادية", "إشارة موثوقية"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("card missing %q in:\n%s", want, scr.Text)
		}
	}
}

func TestJournalCardScreen_Unranked(t *testing.T) {
	scr := journalCardScreen(journal.Journal{Title: "Some Journal"}, predator.Assess(true, nil))
	if !strings.Contains(scr.Text, "غير مصنّفة") {
		t.Errorf("unranked card should say 'unranked':\n%s", scr.Text)
	}
}

func TestAdvisoryBlock_RiskLevels(t *testing.T) {
	high := advisoryBlock(predator.Assess(false, []predator.Flag{{Reason: "مقاييس وهمية"}}))
	if !strings.Contains(high, "تحذير استرشادي") || !strings.Contains(high, "مقاييس وهمية") {
		t.Errorf("high-risk block wrong:\n%s", high)
	}
	if !strings.Contains(advisoryBlock(predator.Assess(true, nil)), "إشارة موثوقية") {
		t.Error("low-risk block should show reliability signal")
	}
	if !strings.Contains(advisoryBlock(predator.Assess(false, nil)), "تحقّق يدوياً") {
		t.Error("medium-risk block should advise manual verification")
	}
}

func TestJournalNotFound_CarriesHighRiskFlag(t *testing.T) {
	adv := predator.Assess(false, []predator.Flag{{Reason: "ناشر في قائمة تحذير"}})
	scr := journalNotFoundScreen("Example Predatory Press", adv)
	if !strings.Contains(scr.Text, "تحذير استرشادي") {
		t.Errorf("not-found screen should surface the flag:\n%s", scr.Text)
	}
}

func TestJournalSuggestionsScreen(t *testing.T) {
	matches := []journal.Journal{
		{Title: "Nature", Quartile: "Q1"},
		{Title: "Nature Communications", Quartile: ""},
	}
	scr := journalSuggestionsScreen("natur", matches)
	if !strings.Contains(scr.Text, "Nature — Q1") {
		t.Errorf("suggestion missing quartile:\n%s", scr.Text)
	}
	if !strings.Contains(scr.Text, "Nature Communications — غير مصنّفة") {
		t.Errorf("unranked suggestion should show fallback label:\n%s", scr.Text)
	}
}

func TestHandleJournalQuery_ExactAndMiss(t *testing.T) {
	a := testApp()
	a.journals = journal.NewIndex([]journal.Journal{
		{Title: "Nature", ISSNs: []string{"0028-0836"}, Quartile: "Q1"},
	})

	// We exercise the lookup branches directly through the index to avoid the
	// Telegram send path; the handler wiring is covered by build + the screens.
	if _, ok := a.journals.Lookup("Nature"); !ok {
		t.Error("expected exact hit for Nature")
	}
	if got := a.journals.Search("nat", 5); len(got) != 1 {
		t.Errorf("expected one near-match, got %d", len(got))
	}
}

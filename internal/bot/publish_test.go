package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/predator"
)

func TestPublishResultScreen_GuidanceAndCrossLink(t *testing.T) {
	scr := publishResultScreen("١) المجال: علوم الحاسوب\n٢) المجلات: ...", nil)
	for _, want := range []string{"مجلات مقترحة", "علوم الحاسوب", "فحص مجلة", "APC", "استرشادية"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("publish result missing %q", want)
		}
	}
	// Cross-link button to the journal checker.
	var hasJournalBtn bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "menu:journal" {
				hasJournalBtn = true
			}
		}
	}
	if !hasJournalBtn {
		t.Error("result should cross-link to the journal checker")
	}
}

func TestPublishResultScreen_PredatoryWarning(t *testing.T) {
	flags := []predator.Flag{{Reason: "ناشر في قائمة تحذير"}}
	scr := publishResultScreen("اقتراح يشمل Example Predatory Press", flags)
	if !strings.Contains(scr.Text, "تنبيه استرشادي") || !strings.Contains(scr.Text, "ناشر في قائمة تحذير") {
		t.Errorf("predatory flag should surface a warning:\n%s", scr.Text)
	}
}

func TestPublishResultScreen_NoWarningWhenClean(t *testing.T) {
	scr := publishResultScreen("اقتراحات نظيفة", nil)
	if strings.Contains(scr.Text, "تنبيه استرشادي") {
		t.Error("clean recommendations should not show a predatory warning")
	}
}

func TestPublish_PredatorIntegration(t *testing.T) {
	// The watch list scans the AI reply text for flagged names.
	list := predator.NewList([]predator.Flag{{Pattern: "Fake Metrics Journal", Reason: "مقاييس وهمية"}})
	got := list.Check("أقترح النشر في Fake Metrics Journal وهي سريعة")
	if len(got) != 1 {
		t.Fatalf("expected the reply scan to flag the journal, got %d", len(got))
	}
	scr := publishResultScreen("...", got)
	if !strings.Contains(scr.Text, "مقاييس وهمية") {
		t.Errorf("flagged reason should appear:\n%s", scr.Text)
	}
}

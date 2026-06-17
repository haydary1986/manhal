package bot

import (
	"strconv"
	"strings"
	"testing"
)

func TestHelpScreen_NumberedAndComplete(t *testing.T) {
	scr := helpScreen()

	for _, want := range []string{"1. ", "🎓 حاسبة الترقيات", "🤖 المساعد الذكي", "محادثة PDF", "حدّ يومي"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("help missing %q", want)
		}
	}

	// Numbering reaches the total number of catalogued tools.
	total := 0
	for _, g := range helpGroups {
		total += len(g.Tools)
	}
	if !strings.Contains(scr.Text, strconv.Itoa(total)+". ") {
		t.Errorf("help should number up to %d", total)
	}

	// Must fit in a single Telegram message (4096-char limit).
	if n := len([]rune(scr.Text)); n > 4096 {
		t.Errorf("help text too long for one message: %d chars", n)
	}
}

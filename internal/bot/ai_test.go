package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/assist"
	"github.com/erticaz/manhal/internal/config"
)

func aiApp(deepSeekKey string, limit int) *App {
	return &App{
		cfg:      &config.Config{DeepSeekKey: deepSeekKey},
		usage:    newUsageLimiter(constLimit(limit)),
		sessions: newSessions(),
	}
}

func TestAIMenuScreen_DisabledWithoutKey(t *testing.T) {
	scr := aiApp("", 5).aiMenuScreen(1)
	if !strings.Contains(scr.Text, "غير مفعّلة") {
		t.Errorf("no-key menu should say disabled:\n%s", scr.Text)
	}
}

func TestAIMenuScreen_ListsToolsAndQuota(t *testing.T) {
	scr := aiApp("key", 5).aiMenuScreen(1)
	if !strings.Contains(scr.Text, "متبقّي") || !strings.Contains(scr.Text, "5") {
		t.Errorf("menu should show remaining quota:\n%s", scr.Text)
	}
	var toolButtons int
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "ai:tool:") {
				toolButtons++
			}
		}
	}
	if toolButtons != len(assist.Tools()) {
		t.Errorf("tool buttons = %d, want %d", toolButtons, len(assist.Tools()))
	}
}

func TestAIMenuScreen_UnlimitedHidesQuota(t *testing.T) {
	scr := aiApp("key", 0).aiMenuScreen(1)
	if strings.Contains(scr.Text, "متبقّي") {
		t.Error("unlimited quota should not show a remaining line")
	}
}

func TestAIResultScreen_TruncatedNote(t *testing.T) {
	tool, _ := assist.Find("summarize")
	if !strings.Contains(aiResultScreen(tool, "ملخّص", true).Text, "اقتصار") {
		t.Error("truncated result should carry the trim note")
	}
	if strings.Contains(aiResultScreen(tool, "ملخّص", false).Text, "اقتصار") {
		t.Error("untruncated result should not carry the trim note")
	}
}

func TestTruncateRunes(t *testing.T) {
	s, cut := truncateRunes("أربعة", 10)
	if cut || s != "أربعة" {
		t.Errorf("short string should pass through: %q cut=%v", s, cut)
	}
	long := strings.Repeat("ن", 20)
	s2, cut2 := truncateRunes(long, 5)
	if !cut2 || len([]rune(s2)) != 5 {
		t.Errorf("long string should be cut to 5 runes: len=%d cut=%v", len([]rune(s2)), cut2)
	}
}

func TestSessions_AITool(t *testing.T) {
	s := newSessions()
	s.startAITool(7, "translate")
	if s.get(7) != stateAwaitAIInput {
		t.Error("startAITool should set the await-input state")
	}
	if s.aiTool(7) != "translate" {
		t.Errorf("aiTool = %q, want translate", s.aiTool(7))
	}
}

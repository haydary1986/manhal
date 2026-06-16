package bot

import (
	"strings"
	"testing"
)

func TestCheckedName(t *testing.T) {
	if got := checkedName("paper.tex", ".tex"); got != "paper-checked.tex" {
		t.Errorf("checkedName = %q", got)
	}
	if got := checkedName("thesis.final.tex", ".tex"); got != "thesis.final-checked.tex" {
		t.Errorf("checkedName multi-dot = %q", got)
	}
	if got := checkedName("noext", ".tex"); got != "noext-checked" {
		t.Errorf("checkedName no ext = %q", got)
	}
}

func TestLatexPromptScreen(t *testing.T) {
	scr := latexPromptScreen()
	if !strings.Contains(scr.Text, ".tex") || !strings.Contains(scr.Text, ".docx") {
		t.Error("prompt should mention both formats")
	}
	if !strings.Contains(scr.Text, "المعادلات") {
		t.Error("prompt should reassure about equations safety")
	}
}

func TestSessions_LatexState(t *testing.T) {
	s := newSessions()
	s.set(7, stateAwaitLatexFile)
	if s.get(7) != stateAwaitLatexFile {
		t.Error("latex-file state should round-trip")
	}
}

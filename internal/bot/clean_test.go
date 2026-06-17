package bot

import (
	"strings"
	"testing"
)

func TestCleanMarkdown(t *testing.T) {
	in := "### العنوان\nنص فيه **عريض** ومهم\n#1 هذا ليس عنواناً\nسطر عادي"
	out := cleanMarkdown(in)

	if strings.Contains(out, "**") {
		t.Errorf("bold markers not stripped: %q", out)
	}
	if strings.Contains(out, "### ") {
		t.Errorf("heading markers not stripped: %q", out)
	}
	for _, want := range []string{"العنوان", "عريض", "#1 هذا ليس عنواناً", "سطر عادي"} {
		if !strings.Contains(out, want) {
			t.Errorf("content lost (%q): %q", want, out)
		}
	}
	if cleanMarkdown("") != "" {
		t.Error("empty stays empty")
	}
}

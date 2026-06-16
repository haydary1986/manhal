package assist

import (
	"strings"
	"testing"
)

func TestTools_AllWellFormed(t *testing.T) {
	tools := Tools()
	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if tool.Key == "" || tool.Label == "" || tool.Prompt == "" || tool.System == "" {
			t.Errorf("tool %+v has an empty required field", tool)
		}
		if seen[tool.Key] {
			t.Errorf("duplicate tool key %q", tool.Key)
		}
		seen[tool.Key] = true
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("translate"); !ok {
		t.Error("translate tool should exist")
	}
	if _, ok := Find("nonexistent"); ok {
		t.Error("unknown key should not be found")
	}
}

func TestParaphrase_ForbidsDetectorEvasion(t *testing.T) {
	// The spec bans AI-detector-evasion framing; the paraphrase prompt must make
	// the honest-editing constraint explicit.
	tool, ok := Find("paraphrase")
	if !ok {
		t.Fatal("paraphrase tool missing")
	}
	if !strings.Contains(tool.System, "كاشفات") {
		t.Errorf("paraphrase system prompt should forbid detector evasion:\n%s", tool.System)
	}
}

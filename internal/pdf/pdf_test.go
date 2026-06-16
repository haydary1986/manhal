package pdf

import "testing"

func TestExtractText_InvalidInput(t *testing.T) {
	// Non-PDF bytes must return an error (or empty), never panic.
	if _, err := ExtractText([]byte("this is not a pdf")); err == nil {
		t.Error("expected an error for non-PDF input")
	}
}

func TestExtractText_Empty(t *testing.T) {
	if _, err := ExtractText(nil); err == nil {
		t.Error("expected an error for empty input")
	}
}

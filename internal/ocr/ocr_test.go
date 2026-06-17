package ocr

import (
	"context"
	"testing"
)

func TestExtractPDF_UnavailableFallback(t *testing.T) {
	if Available() {
		t.Skip("OCR tools present on this host; skipping the unavailable-path test")
	}
	if _, err := ExtractPDF(context.Background(), []byte("not a pdf")); err != ErrUnavailable {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

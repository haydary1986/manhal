// Package pdf extracts plain text from a PDF file in pure Go (no CGo), used by
// the PDF chat / RAG feature (#24). It is read-only and never modifies input.
package pdf

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"
)

// maxText bounds the extracted text (defensive against huge documents).
const maxText = 8 << 20 // 8 MiB

// ExtractText returns the plain text of a PDF given its raw bytes. It recovers
// from panics in the underlying parser so malformed files yield an error rather
// than crashing the bot.
func ExtractText(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text, err = "", fmt.Errorf("pdf: parser panic: %v", r)
		}
	}()

	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	rc, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf text: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, maxText)); err != nil {
		return "", fmt.Errorf("pdf read: %w", err)
	}
	return buf.String(), nil
}

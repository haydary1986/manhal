// Package ocr extracts text from scanned PDFs by rendering pages to images
// (poppler's pdftoppm) and running Tesseract OCR (Arabic + English). The tools
// are external binaries; when absent, callers get ErrUnavailable and can fall
// back gracefully.
package ocr

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrUnavailable means the OCR toolchain (pdftoppm/tesseract) is not installed.
var ErrUnavailable = errors.New("ocr: pdftoppm/tesseract not installed")

// maxPages bounds how many pages we OCR, to cap runtime.
const maxPages = 40

// Available reports whether both external tools are on PATH.
func Available() bool {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return false
	}
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// ExtractPDF renders a PDF to PNGs and OCRs them, returning the concatenated
// text. Languages default to Arabic + English.
func ExtractPDF(ctx context.Context, pdf []byte) (string, error) {
	if !Available() {
		return "", ErrUnavailable
	}
	dir, err := os.MkdirTemp("", "manhal-ocr-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		return "", err
	}
	if err := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", "200", in, filepath.Join(dir, "p")).Run(); err != nil {
		return "", err
	}

	pages, _ := filepath.Glob(filepath.Join(dir, "p*.png"))
	sort.Strings(pages)
	if len(pages) > maxPages {
		pages = pages[:maxPages]
	}

	var out strings.Builder
	for _, png := range pages {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, "tesseract", png, "stdout", "-l", "ara+eng")
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			continue // skip a page that fails rather than aborting the whole file
		}
		out.Write(bytes.TrimSpace(buf.Bytes()))
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String()), nil
}

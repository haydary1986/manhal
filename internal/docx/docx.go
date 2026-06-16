// Package docx extracts plain text from a .docx (Office Open XML) file. It reads
// word/document.xml from the zip and concatenates the run texts, inserting line
// breaks at paragraph boundaries. It is read-only: it never modifies the file.
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ErrNotDocx is returned when the archive has no word/document.xml.
var ErrNotDocx = fmt.Errorf("docx: word/document.xml not found")

// maxXMLBytes bounds the document body we will read (defensive).
const maxXMLBytes = 16 << 20

// ExtractText returns the prose of a .docx file given its raw bytes.
func ExtractText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx: open zip: %w", err)
	}

	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", ErrNotDocx
	}

	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("docx: open document.xml: %w", err)
	}
	defer rc.Close()

	return parseBody(io.LimitReader(rc, maxXMLBytes))
}

// parseBody streams the WordprocessingML, collecting <w:t> text and breaking
// lines at </w:p> (paragraph) boundaries.
func parseBody(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var b strings.Builder
	inText := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("docx: parse xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
			if t.Name.Local == "tab" {
				b.WriteByte('\t')
			}
			if t.Name.Local == "br" {
				b.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteByte('\n')
			}
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

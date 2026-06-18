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

	// Resolve the main document part via the package relationships rather than
	// assuming "word/document.xml" (some producers use a different name).
	part := mainDocumentPart(zr)
	doc := findFile(zr, part)
	if doc == nil {
		doc = findFile(zr, "word/document.xml") // fallback
	}
	if doc == nil {
		// Last resort: any word/document*.xml.
		for _, f := range zr.File {
			n := strings.ToLower(f.Name)
			if strings.HasPrefix(n, "word/document") && strings.HasSuffix(n, ".xml") {
				doc = f
				break
			}
		}
	}
	if doc == nil {
		return "", ErrNotDocx
	}

	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("docx: open %s: %w", doc.Name, err)
	}
	defer rc.Close()

	return parseBody(io.LimitReader(rc, maxXMLBytes))
}

// mainDocumentPart reads _rels/.rels and returns the officeDocument target part
// name (without a leading slash), defaulting to word/document.xml.
func mainDocumentPart(zr *zip.Reader) string {
	rels := findFile(zr, "_rels/.rels")
	if rels == nil {
		return "word/document.xml"
	}
	rc, err := rels.Open()
	if err != nil {
		return "word/document.xml"
	}
	defer rc.Close()
	var doc struct {
		Rel []struct {
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&doc); err != nil {
		return "word/document.xml"
	}
	for _, r := range doc.Rel {
		if strings.HasSuffix(r.Type, "/officeDocument") {
			return strings.TrimPrefix(r.Target, "/")
		}
	}
	return "word/document.xml"
}

func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// parseBody streams the WordprocessingML, collecting <w:t> text and breaking
// lines at </w:p> (paragraph) boundaries.
func parseBody(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false // tolerate quirks some producers emit
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
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

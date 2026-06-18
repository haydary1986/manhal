package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// Some producers name the main document part differently from word/document.xml;
// ExtractText must resolve it through the package relationships.
func TestExtractText_NonStandardPart(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, data string }{
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/mydoc.xml"/></Relationships>`},
		{"word/mydoc.xml", `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:body><w:p><w:r><w:t>مرحبا OOXML</w:t></w:r></w:p><w:p><w:r><w:t>السطر الثاني</w:t></w:r></w:p></w:body></w:document>`},
	}
	for _, p := range parts {
		w, _ := zw.Create(p.name)
		_, _ = w.Write([]byte(p.data))
	}
	_ = zw.Close()

	out, err := ExtractText(buf.Bytes())
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(out, "مرحبا OOXML") || !strings.Contains(out, "السطر الثاني") {
		t.Errorf("extracted %q, want both lines", out)
	}
}

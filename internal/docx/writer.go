package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

// Build creates a minimal but valid .docx from plain text, one paragraph per
// line. Each paragraph's direction is detected from its first strong-directional
// character: Arabic/Hebrew → RTL + right alignment, Latin → LTR + left.
func Build(text string) ([]byte, error) {
	var body strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if isRTL(line) {
			body.WriteString(`<w:p><w:pPr><w:bidi/><w:jc w:val="right"/></w:pPr>`)
		} else {
			body.WriteString(`<w:p><w:pPr><w:jc w:val="left"/></w:pPr>`)
		}
		if t := strings.TrimSpace(line); t != "" {
			body.WriteString(`<w:r><w:t xml:space="preserve">` + escapeXML(line) + `</w:t></w:r>`)
		}
		body.WriteString(`</w:p>`)
	}

	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + body.String() + `<w:sectPr/></w:body>
</w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, data string }{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", relsXML},
		{"word/document.xml", doc},
	}
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, fmt.Errorf("docx zip %s: %w", p.name, err)
		}
		if _, err := w.Write([]byte(p.data)); err != nil {
			return nil, fmt.Errorf("docx write %s: %w", p.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("docx close: %w", err)
	}
	return buf.Bytes(), nil
}

// isRTL reports whether a line's paragraph direction is right-to-left, using the
// Unicode "first strong character" rule: the first Arabic/Hebrew letter means
// RTL, the first Latin letter means LTR. Defaults to LTR (English) when neither.
func isRTL(line string) bool {
	for _, r := range line {
		switch {
		case (r >= 0x0590 && r <= 0x05FF), // Hebrew
			(r >= 0x0600 && r <= 0x06FF), // Arabic
			(r >= 0x0750 && r <= 0x077F), // Arabic Supplement
			(r >= 0x08A0 && r <= 0x08FF), // Arabic Extended-A
			(r >= 0xFB50 && r <= 0xFDFF), // Arabic Presentation Forms-A
			(r >= 0xFE70 && r <= 0xFEFF): // Arabic Presentation Forms-B
			return true
		case (r >= 'A' && r <= 'Z'), (r >= 'a' && r <= 'z'):
			return false
		}
	}
	return false
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

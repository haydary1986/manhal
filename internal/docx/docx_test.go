package docx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// makeDocx builds a minimal .docx in memory with the given document.xml body.
func makeDocx(t *testing.T, documentXML string, includeDoc bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if includeDoc {
		w, err := zw.Create("word/document.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(documentXML)); err != nil {
			t.Fatal(err)
		}
	}
	// An unrelated file, to mimic a real archive.
	other, _ := zw.Create("[Content_Types].xml")
	_, _ = other.Write([]byte("<Types/>"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const sampleDocXML = `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>أهلاً</w:t></w:r><w:r><w:t xml:space="preserve"> بالعالم</w:t></w:r></w:p>
<w:p><w:r><w:t>Second</w:t><w:tab/><w:t>line</w:t></w:r></w:p>
</w:body>
</w:document>`

func TestExtractText(t *testing.T) {
	got, err := ExtractText(makeDocx(t, sampleDocXML, true))
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	want := "أهلاً بالعالم\nSecond\tline"
	if got != want {
		t.Errorf("ExtractText = %q, want %q", got, want)
	}
}

func TestExtractText_NotDocx(t *testing.T) {
	if _, err := ExtractText(makeDocx(t, "", false)); err != ErrNotDocx {
		t.Errorf("err = %v, want ErrNotDocx", err)
	}
}

func TestExtractText_BadZip(t *testing.T) {
	if _, err := ExtractText([]byte("not a zip")); err == nil {
		t.Error("expected an error for non-zip input")
	}
}

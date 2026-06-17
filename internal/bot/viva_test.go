package bot

import (
	"strings"
	"testing"
)

func TestSupportedDocExt(t *testing.T) {
	for _, name := range []string{"thesis.docx", "thesis.TEX", "notes.txt", "thesis.PDF"} {
		if _, err := supportedDocExt(name); err != nil {
			t.Errorf("%q should be supported: %v", name, err)
		}
	}
	if _, err := supportedDocExt("thesis.rtf"); err == nil {
		t.Error("rtf should be unsupported")
	}
}

func TestExtractDocText_Tex(t *testing.T) {
	got, err := extractDocText("paper.tex", []byte("\\section{Intro} body"))
	if err != nil || !strings.Contains(got, "Intro") {
		t.Errorf("tex extraction = (%q, %v)", got, err)
	}
	// .pdf is now routed to the PDF extractor (not rejected as unsupported);
	// invalid bytes fail in the extractor, not with errUnsupportedDoc.
	if _, err := extractDocText("x.pdf", []byte("not a real pdf")); err == errUnsupportedDoc {
		t.Error("pdf should be handled by the extractor, not rejected as unsupported")
	}
	if _, err := extractDocText("x.rtf", []byte("x")); err != errUnsupportedDoc {
		t.Errorf("rtf err = %v, want errUnsupportedDoc", err)
	}
}

func TestVivaResultScreen(t *testing.T) {
	scr := vivaResultScreen("الشريحة 1: المقدمة\nسؤال متوقع: ...")
	for _, want := range []string{"تحضير المناقشة", "المقدمة", "استرشادية"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("viva result missing %q", want)
		}
	}
}

func TestSessions_VivaState(t *testing.T) {
	s := newSessions()
	s.set(3, stateAwaitVivaFile)
	if s.get(3) != stateAwaitVivaFile {
		t.Error("viva-file state should round-trip")
	}
}

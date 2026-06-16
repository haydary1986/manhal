package bot

import (
	"strings"
	"testing"
)

func TestSupportedDocExt(t *testing.T) {
	for _, name := range []string{"thesis.docx", "thesis.TEX", "notes.txt"} {
		if _, err := supportedDocExt(name); err != nil {
			t.Errorf("%q should be supported: %v", name, err)
		}
	}
	if _, err := supportedDocExt("thesis.pdf"); err == nil {
		t.Error("pdf should be unsupported")
	}
}

func TestExtractDocText_Tex(t *testing.T) {
	got, err := extractDocText("paper.tex", []byte("\\section{Intro} body"))
	if err != nil || !strings.Contains(got, "Intro") {
		t.Errorf("tex extraction = (%q, %v)", got, err)
	}
	if _, err := extractDocText("x.pdf", []byte("x")); err != errUnsupportedDoc {
		t.Errorf("pdf err = %v, want errUnsupportedDoc", err)
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

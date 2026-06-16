package cite

import (
	"strings"
	"testing"
)

func TestRIS(t *testing.T) {
	w := Work{
		Type:           "journal-article",
		Title:          "Attention Is All You Need",
		Authors:        []Author{{Family: "Vaswani", Given: "Ashish"}, {Family: "Shazeer", Given: "Noam"}},
		ContainerTitle: "NeurIPS",
		Volume:         "30",
		Issue:          "1",
		Pages:          "5998-6008",
		Year:           2017,
		DOI:            "10.5555/abc",
	}
	got := RIS(w)
	for _, want := range []string{
		"TY  - JOUR",
		"AU  - Vaswani, Ashish",
		"AU  - Shazeer, Noam",
		"TI  - Attention Is All You Need",
		"JO  - NeurIPS",
		"VL  - 30",
		"SP  - 5998",
		"EP  - 6008",
		"PY  - 2017",
		"DO  - 10.5555/abc",
		"ER  - ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RIS missing %q in:\n%s", want, got)
		}
	}
}

func TestRIS_TypeAndSinglePage(t *testing.T) {
	if !strings.Contains(RIS(Work{Type: "book", Title: "X"}), "TY  - BOOK") {
		t.Error("book type should map to BOOK")
	}
	sp, ep := splitPages("42")
	if sp != "42" || ep != "" {
		t.Errorf("splitPages single = (%q,%q)", sp, ep)
	}
	sp, ep = splitPages("10–20") // en-dash
	if sp != "10" || ep != "20" {
		t.Errorf("splitPages en-dash = (%q,%q)", sp, ep)
	}
}

func TestKeywords(t *testing.T) {
	kw := Keywords(Work{Title: "Deep Learning for the Analysis of Medical Images"})
	// "for", "the", "analysis" are dropped; short words excluded.
	joined := strings.ToLower(strings.Join(kw, " "))
	if !strings.Contains(joined, "deep") || !strings.Contains(joined, "learning") || !strings.Contains(joined, "medical") {
		t.Errorf("keywords = %v", kw)
	}
	for _, bad := range []string{"for", "the", "analysis"} {
		if strings.Contains(joined, bad) {
			t.Errorf("stopword %q should be excluded: %v", bad, kw)
		}
	}
	if len(kw) > maxKeywords {
		t.Errorf("keywords exceed cap: %d", len(kw))
	}
}

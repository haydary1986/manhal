package cite

import (
	"strings"
	"unicode/utf8"
)

// RIS renders an RIS-format entry for the Work (importable into Zotero,
// Mendeley, EndNote, etc.).
func RIS(w Work) string {
	lines := []string{"TY  - " + risType(w.Type)}
	for _, a := range w.Authors {
		lines = appendRIS(lines, "AU", joinName(a.Family, strings.TrimSpace(a.Given), ", "))
	}
	lines = appendRIS(lines, "TI", w.Title)
	lines = appendRIS(lines, "JO", w.ContainerTitle)
	lines = appendRIS(lines, "VL", w.Volume)
	lines = appendRIS(lines, "IS", w.Issue)

	sp, ep := splitPages(w.Pages)
	lines = appendRIS(lines, "SP", sp)
	lines = appendRIS(lines, "EP", ep)

	if w.HasYear() {
		lines = appendRIS(lines, "PY", itoa(w.Year))
	}
	lines = appendRIS(lines, "DO", strings.TrimSpace(w.DOI))
	lines = appendRIS(lines, "UR", w.doiURL())
	lines = append(lines, "ER  - ")
	return strings.Join(lines, "\n")
}

// risType maps a Crossref type to an RIS reference type tag.
func risType(t string) string {
	switch t {
	case "journal-article", "journal-issue":
		return "JOUR"
	case "book", "monograph", "reference-book":
		return "BOOK"
	case "book-chapter":
		return "CHAP"
	case "proceedings-article":
		return "CPAPER"
	case "dissertation":
		return "THES"
	case "dataset":
		return "DATA"
	default:
		return "GEN"
	}
}

func appendRIS(lines []string, tag, val string) []string {
	if val = strings.TrimSpace(val); val != "" {
		return append(lines, tag+"  - "+val)
	}
	return lines
}

// splitPages splits "45-67" into ("45", "67"); a single page yields ("45", "").
func splitPages(p string) (start, end string) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", ""
	}
	if i := strings.IndexAny(p, "-–"); i >= 0 {
		r, _ := utf8.DecodeRuneInString(p[i:])
		return strings.TrimSpace(p[:i]), strings.TrimSpace(p[i+utf8.RuneLen(r):])
	}
	return p, ""
}

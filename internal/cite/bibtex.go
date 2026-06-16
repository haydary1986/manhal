package cite

import "strings"

// BibTeX renders a BibTeX entry for the Work.
func BibTeX(w Work) string {
	var b strings.Builder
	b.WriteString("@" + bibType(w.Type) + "{" + bibKey(w) + ",\n")

	fields := []struct{ key, val string }{
		{"author", bibAuthors(w.Authors)},
		{"title", w.Title},
		{"journal", w.ContainerTitle},
		{"publisher", w.Publisher},
		{"year", yearOrEmpty(w)},
		{"volume", w.Volume},
		{"number", w.Issue},
		{"pages", bibPages(w.Pages)},
		{"doi", strings.TrimSpace(w.DOI)},
		{"url", bibURL(w)},
	}

	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		if f.val != "" {
			lines = append(lines, "  "+f.key+" = {"+f.val+"}")
		}
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n}")
	return b.String()
}

// bibType maps a Crossref type to a BibTeX entry type.
func bibType(t string) string {
	switch t {
	case "journal-article", "journal-issue":
		return "article"
	case "book", "monograph", "reference-book":
		return "book"
	case "book-chapter":
		return "incollection"
	case "proceedings-article":
		return "inproceedings"
	case "dataset":
		return "misc"
	case "dissertation":
		return "phdthesis"
	default:
		return "misc"
	}
}

// bibKey builds a citation key: first-author-family + year (ASCII letters only).
func bibKey(w Work) string {
	stem := "ref"
	if len(w.Authors) > 0 && w.Authors[0].Family != "" {
		stem = asciiLetters(w.Authors[0].Family)
	} else if w.Title != "" {
		if fields := strings.Fields(w.Title); len(fields) > 0 {
			stem = asciiLetters(fields[0])
		}
	}
	if stem == "" {
		stem = "ref"
	}
	if w.HasYear() {
		return strings.ToLower(stem) + itoa(w.Year)
	}
	return strings.ToLower(stem)
}

// bibAuthors joins authors with BibTeX's " and " separator, inverted.
func bibAuthors(authors []Author) string {
	items := make([]string, 0, len(authors))
	for _, a := range authors {
		items = append(items, joinName(a.Family, strings.TrimSpace(a.Given), ", "))
	}
	return strings.Join(items, " and ")
}

// bibPages normalizes a page range to BibTeX's en-dash convention (45--67).
func bibPages(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.Contains(p, "--") {
		return p
	}
	return strings.Replace(p, "-", "--", 1)
}

func bibURL(w Work) string {
	if w.DOI != "" {
		return "" // doi field already carries the link
	}
	return strings.TrimSpace(w.URL)
}

func yearOrEmpty(w Work) string {
	if w.HasYear() {
		return itoa(w.Year)
	}
	return ""
}

// asciiLetters keeps only ASCII letters (drops accents, spaces, punctuation).
func asciiLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

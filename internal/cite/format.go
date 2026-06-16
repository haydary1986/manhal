package cite

import "strings"

// Style is a named, rendered citation.
type Style struct {
	Name string
	Text string
}

// All renders the Work in every supported inline style, in display order.
// BibTeX is produced separately by BibTeX because it is a multi-line block.
func All(w Work) []Style {
	return []Style{
		{"APA", APA(w)},
		{"MLA", MLA(w)},
		{"Chicago", Chicago(w)},
		{"Harvard", Harvard(w)},
		{"IEEE", IEEE(w)},
		{"Vancouver", Vancouver(w)},
	}
}

// APA renders APA 7th style.
func APA(w Work) string {
	var b strings.Builder
	if a := authorsAPA(w.Authors); a != "" {
		b.WriteString(a)
		b.WriteString(" ")
	}
	b.WriteString("(" + w.yearText() + "). ")
	if w.Title != "" {
		b.WriteString(endSentence(w.Title) + " ")
	}
	if w.ContainerTitle != "" {
		b.WriteString(w.ContainerTitle)
		b.WriteString(volIssuePages(w.Volume, w.Issue, w.Pages, "apa"))
		b.WriteString(". ")
	}
	b.WriteString(w.doiURL())
	return tidy(b.String())
}

// MLA renders MLA 9th style.
func MLA(w Work) string {
	parts := []string{}
	if a := authorsMLA(w.Authors); a != "" {
		parts = append(parts, a+".")
	}
	if w.Title != "" {
		parts = append(parts, `"`+strings.TrimRight(w.Title, ".")+`."`)
	}
	segs := []string{}
	if w.ContainerTitle != "" {
		segs = append(segs, w.ContainerTitle)
	}
	if w.Volume != "" {
		segs = append(segs, "vol. "+w.Volume)
	}
	if w.Issue != "" {
		segs = append(segs, "no. "+w.Issue)
	}
	if w.HasYear() {
		segs = append(segs, w.yearText())
	}
	if w.Pages != "" {
		segs = append(segs, "pp. "+w.Pages)
	}
	if len(segs) > 0 {
		parts = append(parts, strings.Join(segs, ", ")+".")
	}
	if link := w.doiURL(); link != "" {
		parts = append(parts, link+".")
	}
	return tidy(strings.Join(parts, " "))
}

// Chicago renders Chicago 17th author-date (reference-list) style.
func Chicago(w Work) string {
	parts := []string{}
	if a := authorsChicago(w.Authors); a != "" {
		parts = append(parts, a+".")
	}
	parts = append(parts, w.yearText()+".")
	if w.Title != "" {
		parts = append(parts, `"`+strings.TrimRight(w.Title, ".")+`."`)
	}
	if w.ContainerTitle != "" {
		seg := w.ContainerTitle
		if w.Volume != "" {
			seg += " " + w.Volume
		}
		if w.Issue != "" {
			seg += " (" + w.Issue + ")"
		}
		if w.Pages != "" {
			if w.Volume != "" || w.Issue != "" {
				seg += ": " + w.Pages
			} else {
				seg += ", " + w.Pages
			}
		}
		parts = append(parts, seg+".")
	}
	if link := w.doiURL(); link != "" {
		parts = append(parts, link+".")
	}
	return tidy(strings.Join(parts, " "))
}

// Harvard renders Harvard (Cite Them Right) style.
func Harvard(w Work) string {
	var b strings.Builder
	if a := authorsHarvard(w.Authors); a != "" {
		b.WriteString(a + " ")
	}
	b.WriteString("(" + w.yearText() + ") ")
	if w.Title != "" {
		b.WriteString("'" + strings.TrimRight(w.Title, ".") + "', ")
	}
	if w.ContainerTitle != "" {
		b.WriteString(w.ContainerTitle)
		b.WriteString(volIssuePages(w.Volume, w.Issue, w.Pages, "harvard"))
		b.WriteString(". ")
	}
	if w.DOI != "" {
		b.WriteString("doi:" + strings.TrimSpace(w.DOI) + ".")
	} else if w.URL != "" {
		b.WriteString("Available at: " + strings.TrimSpace(w.URL) + ".")
	}
	return tidy(b.String())
}

// IEEE renders IEEE style.
func IEEE(w Work) string {
	prefix := "[1]"
	if a := authorsIEEE(w.Authors); a != "" {
		prefix += " " + a + ","
	}
	if w.Title != "" {
		prefix += ` "` + strings.TrimRight(w.Title, ".") + `,"`
	}
	segs := []string{}
	if w.ContainerTitle != "" {
		segs = append(segs, w.ContainerTitle)
	}
	if w.Volume != "" {
		segs = append(segs, "vol. "+w.Volume)
	}
	if w.Issue != "" {
		segs = append(segs, "no. "+w.Issue)
	}
	if w.Pages != "" {
		segs = append(segs, "pp. "+w.Pages)
	}
	if w.HasYear() {
		segs = append(segs, w.yearText())
	}
	if w.DOI != "" {
		segs = append(segs, "doi: "+strings.TrimSpace(w.DOI))
	}
	out := prefix
	if len(segs) > 0 {
		out += " " + strings.Join(segs, ", ")
	}
	return tidy(out + ".")
}

// Vancouver renders Vancouver (ICMJE) style.
func Vancouver(w Work) string {
	parts := []string{}
	if a := authorsVancouver(w.Authors); a != "" {
		parts = append(parts, a+".")
	}
	if w.Title != "" {
		parts = append(parts, endSentence(w.Title))
	}
	if w.ContainerTitle != "" {
		parts = append(parts, endSentence(w.ContainerTitle))
	}
	loc := w.yearText()
	if w.Volume != "" || w.Issue != "" || w.Pages != "" {
		loc += ";" + w.Volume
		if w.Issue != "" {
			loc += "(" + w.Issue + ")"
		}
		if w.Pages != "" {
			loc += ":" + w.Pages
		}
	}
	parts = append(parts, loc+".")
	return tidy(strings.Join(parts, " "))
}

// volIssuePages renders the ", 12(3), 45-67" tail shared by APA/Harvard.
func volIssuePages(volume, issue, pages, style string) string {
	var b strings.Builder
	if volume != "" {
		b.WriteString(", " + volume)
		if issue != "" {
			b.WriteString("(" + issue + ")")
		}
	} else if issue != "" {
		b.WriteString(", (" + issue + ")")
	}
	if pages != "" {
		switch style {
		case "harvard":
			b.WriteString(", pp. " + pages)
		default: // apa
			b.WriteString(", " + pages)
		}
	}
	return b.String()
}

// endSentence ensures the text ends with sentence punctuation.
func endSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return s
	default:
		return s + "."
	}
}

// tidy collapses runs of spaces and trims, cleaning up gaps left by missing
// fields without disturbing intended punctuation.
func tidy(s string) string {
	s = strings.ReplaceAll(s, " ,", ",")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

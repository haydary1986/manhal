// Package cite turns bibliographic metadata into formatted citations in the
// common academic styles (APA, MLA, Chicago, Harvard, IEEE, Vancouver) plus
// BibTeX. It holds no I/O: a Work is built by a fetcher (see internal/scholar)
// and rendered here with pure functions.
package cite

import "strings"

// Author is one contributor. Family is the surname; Given is everything else
// (one or more given names, already space-separated).
type Author struct {
	Family string
	Given  string
}

// Work is the normalized bibliographic record a citation is built from.
// Fields may be empty; formatters degrade gracefully when data is missing.
type Work struct {
	Type           string // Crossref type, e.g. "journal-article", "book"
	Title          string
	Authors        []Author
	ContainerTitle string // journal or book series title
	Publisher      string
	Volume         string
	Issue          string
	Pages          string
	Year           int
	DOI            string
	URL            string
}

// HasYear reports whether a usable publication year is present.
func (w Work) HasYear() bool { return w.Year > 0 }

// yearText renders the year, or "n.d." (no date) when unknown.
func (w Work) yearText() string {
	if w.HasYear() {
		return itoa(w.Year)
	}
	return "n.d."
}

// doiURL returns the canonical https://doi.org/... link, or the raw URL, or "".
func (w Work) doiURL() string {
	if w.DOI != "" {
		return "https://doi.org/" + strings.TrimSpace(w.DOI)
	}
	return strings.TrimSpace(w.URL)
}

// itoa avoids importing strconv across the package for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

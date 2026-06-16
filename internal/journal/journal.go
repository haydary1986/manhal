// Package journal provides journal-ranking lookups (Scimago quartile / SJR) used
// by the "check a journal" feature. Data is loaded from an editable CSV derived
// from the Scimago Journal Rank export; this package holds no network concerns.
package journal

import "strings"

// Journal is one ranked journal record.
type Journal struct {
	Title      string
	ISSNs      []string
	SJR        string
	Quartile   string // Q1..Q4, or "" when unranked
	HIndex     string
	Publisher  string
	Country    string
	Categories []string
}

// normalizeISSN reduces an ISSN to its 8 significant characters (digits plus a
// possible trailing X), uppercased, so "0036-8075" and "00368075" match.
func normalizeISSN(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		if (r >= '0' && r <= '9') || r == 'X' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeTitle lowercases, trims, and collapses internal whitespace so minor
// spacing differences still match.
func normalizeTitle(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// looksLikeISSN reports whether a query is an ISSN rather than a title.
func looksLikeISSN(s string) bool {
	n := normalizeISSN(s)
	if len(n) != 8 {
		return false
	}
	// Reject titles that happen to be 8 digits only if they contain letters
	// other than the trailing X — normalizeISSN already dropped letters, so a
	// title like "Science" yields "" and won't reach here.
	return true
}

// QuartileLabel returns a short Arabic descriptor for a quartile code.
func QuartileLabel(q string) string {
	switch strings.ToUpper(strings.TrimSpace(q)) {
	case "Q1":
		return "Q1 — الربع الأول (الأعلى)"
	case "Q2":
		return "Q2 — الربع الثاني"
	case "Q3":
		return "Q3 — الربع الثالث"
	case "Q4":
		return "Q4 — الربع الرابع"
	default:
		return "غير مصنّفة"
	}
}

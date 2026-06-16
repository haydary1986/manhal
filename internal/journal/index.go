package journal

import "strings"

// Index is an in-memory lookup over journals, keyed by ISSN and normalized title.
type Index struct {
	all     []Journal
	byISSN  map[string]int // normalized ISSN -> index into all
	byTitle map[string]int // normalized title -> index into all
}

// NewIndex builds an Index from journal records.
func NewIndex(journals []Journal) *Index {
	ix := &Index{
		all:     journals,
		byISSN:  make(map[string]int),
		byTitle: make(map[string]int),
	}
	for i, j := range journals {
		if t := normalizeTitle(j.Title); t != "" {
			if _, exists := ix.byTitle[t]; !exists {
				ix.byTitle[t] = i
			}
		}
		for _, issn := range j.ISSNs {
			if n := normalizeISSN(issn); n != "" {
				if _, exists := ix.byISSN[n]; !exists {
					ix.byISSN[n] = i
				}
			}
		}
	}
	return ix
}

// Len reports how many journals are indexed.
func (ix *Index) Len() int { return len(ix.all) }

// Lookup returns an exact match by ISSN or normalized title.
func (ix *Index) Lookup(query string) (Journal, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Journal{}, false
	}
	if looksLikeISSN(query) {
		if i, ok := ix.byISSN[normalizeISSN(query)]; ok {
			return ix.all[i], true
		}
		return Journal{}, false
	}
	if i, ok := ix.byTitle[normalizeTitle(query)]; ok {
		return ix.all[i], true
	}
	return Journal{}, false
}

// Search returns up to `limit` journals whose title contains the query
// (case-insensitive). Used to suggest matches when Lookup finds nothing exact.
func (ix *Index) Search(query string, limit int) []Journal {
	q := normalizeTitle(query)
	if q == "" {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	out := make([]Journal, 0, limit)
	for _, j := range ix.all {
		if strings.Contains(normalizeTitle(j.Title), q) {
			out = append(out, j)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

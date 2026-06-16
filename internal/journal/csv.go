package journal

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// columnAliases maps a logical field to the header names that may carry it,
// easing mapping from a raw Scimago export.
var columnAliases = map[string][]string{
	"title":      {"title"},
	"issn":       {"issn", "issns"},
	"sjr":        {"sjr"},
	"quartile":   {"quartile", "sjr best quartile"},
	"h_index":    {"h_index", "h index", "hindex"},
	"publisher":  {"publisher"},
	"country":    {"country"},
	"categories": {"categories", "category"},
}

// LoadCSV reads data/journals.csv (a Scimago-derived, comma-delimited file with
// a header row) into an Index. A missing file yields an empty Index so the bot
// still starts on a fresh checkout.
func LoadCSV(dataDir string) (*Index, error) {
	path := filepath.Join(dataDir, "journals.csv")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewIndex(nil), nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(rows) < 2 {
		return NewIndex(nil), nil
	}

	cols := headerIndex(rows[0])
	journals := make([]Journal, 0, len(rows)-1)
	for _, row := range rows[1:] {
		j := rowToJournal(row, cols)
		if j.Title == "" {
			continue
		}
		journals = append(journals, j)
	}
	return NewIndex(journals), nil
}

// headerIndex maps each logical field to its column position in the header.
func headerIndex(header []string) map[string]int {
	norm := make(map[string]int, len(header))
	for i, h := range header {
		norm[strings.ToLower(strings.TrimSpace(h))] = i
	}
	cols := make(map[string]int)
	for field, aliases := range columnAliases {
		for _, a := range aliases {
			if i, ok := norm[a]; ok {
				cols[field] = i
				break
			}
		}
	}
	return cols
}

func rowToJournal(row []string, cols map[string]int) Journal {
	get := func(field string) string {
		if i, ok := cols[field]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	return Journal{
		Title:      get("title"),
		ISSNs:      splitList(get("issn")),
		SJR:        get("sjr"),
		Quartile:   strings.ToUpper(get("quartile")),
		HIndex:     get("h_index"),
		Publisher:  get("publisher"),
		Country:    get("country"),
		Categories: splitList(get("categories")),
	}
}

// splitList splits a comma- or semicolon-separated cell into trimmed values.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

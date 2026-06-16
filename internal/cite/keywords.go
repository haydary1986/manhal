package cite

import (
	"strings"
	"unicode"
)

// maxKeywords caps how many auto-keywords a work yields.
const maxKeywords = 6

// stopwords are common English words excluded from auto-keywords.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"that": true, "this": true, "into": true, "using": true, "based": true,
	"study": true, "analysis": true, "approach": true, "method": true,
	"toward": true, "towards": true, "between": true, "their": true, "about": true,
}

// Keywords derives simple auto-keywords from the title (and container), used by
// the smart reference manager (#21). It is a light heuristic, not AI: it keeps
// distinct significant words (length >= 4, not stopwords).
func Keywords(w Work) []string {
	seen := map[string]bool{}
	var out []string

	add := func(text string) {
		for _, word := range strings.FieldsFunc(text, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		}) {
			key := strings.ToLower(word)
			if len([]rune(key)) < 4 || stopwords[key] || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, word)
			if len(out) == maxKeywords {
				return
			}
		}
	}

	add(w.Title)
	return out
}

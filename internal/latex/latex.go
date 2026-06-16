// Package latex masks the parts of a LaTeX document that must never be altered
// by a language checker — math, tables, code — replacing them with opaque
// placeholders, then restores them verbatim afterwards. This lets the safe Word/
// LaTeX checker (P11) proofread prose without corrupting equations or listings.
package latex

import (
	"regexp"
	"strconv"
	"strings"
)

// protectedEnvs are LaTeX environments whose bodies must be preserved verbatim.
var protectedEnvs = []string{
	"verbatim", "lstlisting", "minted",
	"equation", "equation*", "align", "align*", "gather", "gather*",
	"multline", "multline*", "math", "displaymath", "eqnarray", "eqnarray*",
	"tabular", "array", "table", "figure", "tikzpicture",
}

var protectPatterns = buildPatterns()

func buildPatterns() []*regexp.Regexp {
	pats := make([]*regexp.Regexp, 0, len(protectedEnvs)+5)
	// Environments first, so $ inside code/tables is never seen as math.
	for _, env := range protectedEnvs {
		e := regexp.QuoteMeta(env)
		pats = append(pats, regexp.MustCompile(`(?s)\\begin\{`+e+`\}.*?\\end\{`+e+`\}`))
	}
	// Display then inline math, then inline verbatim.
	pats = append(pats,
		regexp.MustCompile(`(?s)\$\$.*?\$\$`),
		regexp.MustCompile(`(?s)\\\[.*?\\\]`),
		regexp.MustCompile(`(?s)\\\(.*?\\\)`),
		regexp.MustCompile(`\$[^$]*\$`),
		regexp.MustCompile(`\\verb\|[^|]*\|`),
	)
	return pats
}

func placeholder(i int) string { return "⟦⟦" + strconv.Itoa(i) + "⟧⟧" }

// Protect replaces every protected span with a placeholder and returns the
// masked source plus the ordered list of original spans.
func Protect(src string) (masked string, tokens []string) {
	for _, re := range protectPatterns {
		src = re.ReplaceAllStringFunc(src, func(m string) string {
			ph := placeholder(len(tokens))
			tokens = append(tokens, m)
			return ph
		})
	}
	return src, tokens
}

// Restore puts the protected spans back in place of their placeholders.
func Restore(masked string, tokens []string) string {
	for i, tok := range tokens {
		masked = strings.ReplaceAll(masked, placeholder(i), tok)
	}
	return masked
}

// PlaceholdersIntact reports whether every placeholder still exists in the text
// (used to detect if a language model mangled the protected markers).
func PlaceholdersIntact(masked string, tokens []string) bool {
	for i := range tokens {
		if !strings.Contains(masked, placeholder(i)) {
			return false
		}
	}
	return true
}

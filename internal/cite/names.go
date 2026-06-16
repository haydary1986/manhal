package cite

import "strings"

// givenParts splits a given-name string into its components. Spaces, periods,
// and hyphens all separate, so "John A" and "Mary-Jane" both yield two parts.
func givenParts(given string) []string {
	fields := strings.FieldsFunc(given, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '.' || r == '-'
	})
	return fields
}

// firstLetter returns the first rune of s as a string (UTF-8 safe), or "".
func firstLetter(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// initialsSpaced renders given names as "J. A." (period + space).
func initialsSpaced(given string) string {
	parts := givenParts(given)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if l := firstLetter(p); l != "" {
			out = append(out, l+".")
		}
	}
	return strings.Join(out, " ")
}

// initialsTight renders given names as "J.A." (period, no space) for Harvard.
func initialsTight(given string) string {
	parts := givenParts(given)
	var b strings.Builder
	for _, p := range parts {
		if l := firstLetter(p); l != "" {
			b.WriteString(l)
			b.WriteString(".")
		}
	}
	return b.String()
}

// initialsBare renders given names as "JA" (no periods) for Vancouver.
func initialsBare(given string) string {
	parts := givenParts(given)
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(firstLetter(p))
	}
	return b.String()
}

// joinList combines items with " conj " before the last item and no serial
// comma for the two-item case (Harvard, IEEE): "A and B", "A, B and C".
func joinList(items []string, conj string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + conj + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", " + conj + " " + items[len(items)-1]
	}
}

// joinSerial always places a comma before the final conjunction, including the
// two-item case (APA, MLA, Chicago): "A, & B", "A, B, & C". This is required
// when the first author's name is inverted and already contains a comma.
func joinSerial(items []string, conj string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", " + conj + " " + items[len(items)-1]
	}
}

// --- per-style author renderers ---

// authorsAPA: "Smith, J. A., & Doe, J. B." (inverted, spaced initials, &).
func authorsAPA(authors []Author) string {
	items := make([]string, 0, len(authors))
	for _, a := range authors {
		items = append(items, joinName(a.Family, initialsSpaced(a.Given), ", "))
	}
	return joinSerial(items, "&")
}

// authorsHarvard: "Smith, J.A. and Doe, J.B." (inverted, tight initials, and).
func authorsHarvard(authors []Author) string {
	items := make([]string, 0, len(authors))
	for _, a := range authors {
		items = append(items, joinName(a.Family, initialsTight(a.Given), ", "))
	}
	return joinList(items, "and")
}

// authorsMLA: first author inverted full name, rest "Given Family"; 3+ -> et al.
func authorsMLA(authors []Author) string {
	if len(authors) == 0 {
		return ""
	}
	first := joinName(authors[0].Family, authors[0].Given, ", ")
	if len(authors) >= 3 {
		return first + ", et al"
	}
	items := []string{first}
	for _, a := range authors[1:] {
		items = append(items, strings.TrimSpace(a.Given+" "+a.Family))
	}
	return joinSerial(items, "and")
}

// authorsChicago: first author inverted, rest "Given Family"; 4+ -> et al.
func authorsChicago(authors []Author) string {
	if len(authors) == 0 {
		return ""
	}
	first := joinName(authors[0].Family, authors[0].Given, ", ")
	if len(authors) >= 4 {
		return first + ", et al"
	}
	items := []string{first}
	for _, a := range authors[1:] {
		items = append(items, strings.TrimSpace(a.Given+" "+a.Family))
	}
	return joinSerial(items, "and")
}

// authorsIEEE: "J. A. Smith and J. B. Doe" (initials first); 7+ -> first + et al.
func authorsIEEE(authors []Author) string {
	if len(authors) == 0 {
		return ""
	}
	render := func(a Author) string {
		return strings.TrimSpace(initialsSpaced(a.Given) + " " + a.Family)
	}
	if len(authors) > 6 {
		return render(authors[0]) + " et al."
	}
	items := make([]string, 0, len(authors))
	for _, a := range authors {
		items = append(items, render(a))
	}
	return joinList(items, "and")
}

// authorsVancouver: "Smith JA, Doe JB" (bare initials after family); 7+ -> et al.
func authorsVancouver(authors []Author) string {
	render := func(a Author) string {
		return strings.TrimSpace(a.Family + " " + initialsBare(a.Given))
	}
	if len(authors) > 6 {
		authors = authors[:6]
		items := make([]string, 0, 6)
		for _, a := range authors {
			items = append(items, render(a))
		}
		return strings.Join(items, ", ") + ", et al"
	}
	items := make([]string, 0, len(authors))
	for _, a := range authors {
		items = append(items, render(a))
	}
	return strings.Join(items, ", ")
}

// joinName combines a family name and a rendered given part with sep, dropping
// empty pieces so a missing given name doesn't leave a dangling comma.
func joinName(family, given, sep string) string {
	family = strings.TrimSpace(family)
	given = strings.TrimSpace(given)
	switch {
	case family == "" && given == "":
		return ""
	case given == "":
		return family
	case family == "":
		return given
	default:
		return family + sep + given
	}
}

package cite

import "testing"

// sample is a fully-populated two-author journal article used across styles.
func sample() Work {
	return Work{
		Type:           "journal-article",
		Title:          "The structure of scientific revolutions",
		Authors:        []Author{{Family: "Smith", Given: "John A"}, {Family: "Doe", Given: "Jane B"}},
		ContainerTitle: "Journal of Testing",
		Volume:         "12",
		Issue:          "3",
		Pages:          "45-67",
		Year:           2020,
		DOI:            "10.1000/xyz123",
	}
}

func TestStyles_FullArticle(t *testing.T) {
	w := sample()
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			"APA",
			APA(w),
			"Smith, J. A., & Doe, J. B. (2020). The structure of scientific revolutions. Journal of Testing, 12(3), 45-67. https://doi.org/10.1000/xyz123",
		},
		{
			"MLA",
			MLA(w),
			`Smith, John A, and Jane B Doe. "The structure of scientific revolutions." Journal of Testing, vol. 12, no. 3, 2020, pp. 45-67. https://doi.org/10.1000/xyz123.`,
		},
		{
			"Chicago",
			Chicago(w),
			`Smith, John A, and Jane B Doe. 2020. "The structure of scientific revolutions." Journal of Testing 12 (3): 45-67. https://doi.org/10.1000/xyz123.`,
		},
		{
			"Harvard",
			Harvard(w),
			"Smith, J.A. and Doe, J.B. (2020) 'The structure of scientific revolutions', Journal of Testing, 12(3), pp. 45-67. doi:10.1000/xyz123.",
		},
		{
			"IEEE",
			IEEE(w),
			`[1] J. A. Smith and J. B. Doe, "The structure of scientific revolutions," Journal of Testing, vol. 12, no. 3, pp. 45-67, 2020, doi: 10.1000/xyz123.`,
		},
		{
			"Vancouver",
			Vancouver(w),
			"Smith JA, Doe JB. The structure of scientific revolutions. Journal of Testing. 2020;12(3):45-67.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s mismatch:\n got: %q\nwant: %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestBibTeX_FullArticle(t *testing.T) {
	want := "@article{smith2020,\n" +
		"  author = {Smith, John A and Doe, Jane B},\n" +
		"  title = {The structure of scientific revolutions},\n" +
		"  journal = {Journal of Testing},\n" +
		"  year = {2020},\n" +
		"  volume = {12},\n" +
		"  number = {3},\n" +
		"  pages = {45--67},\n" +
		"  doi = {10.1000/xyz123}\n" +
		"}"
	if got := BibTeX(sample()); got != want {
		t.Errorf("BibTeX mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStyles_SingleAuthorNoIssue(t *testing.T) {
	w := Work{
		Type:           "journal-article",
		Title:          "On computable numbers",
		Authors:        []Author{{Family: "Turing", Given: "Alan M"}},
		ContainerTitle: "Proc. London Math. Soc.",
		Volume:         "42",
		Pages:          "230-265",
		Year:           1936,
		DOI:            "10.1112/plms/s2-42.1.230",
	}
	cases := map[string]string{
		"APA":       "Turing, A. M. (1936). On computable numbers. Proc. London Math. Soc., 42, 230-265. https://doi.org/10.1112/plms/s2-42.1.230",
		"Vancouver": "Turing AM. On computable numbers. Proc. London Math. Soc. 1936;42:230-265.",
	}
	if got := APA(w); got != cases["APA"] {
		t.Errorf("APA:\n got: %q\nwant: %q", got, cases["APA"])
	}
	if got := Vancouver(w); got != cases["Vancouver"] {
		t.Errorf("Vancouver:\n got: %q\nwant: %q", got, cases["Vancouver"])
	}
}

func TestStyles_ThreeAuthorsEtAl(t *testing.T) {
	authors := []Author{
		{Family: "Vaswani", Given: "Ashish"},
		{Family: "Shazeer", Given: "Noam"},
		{Family: "Parmar", Given: "Niki"},
	}
	w := Work{Type: "journal-article", Title: "Attention is all you need", Authors: authors, Year: 2017}
	// MLA and Chicago collapse 3+ authors with "et al".
	if got := authorsMLA(authors); got != "Vaswani, Ashish, et al" {
		t.Errorf("authorsMLA = %q", got)
	}
	if got := authorsChicago(authors); got != "Vaswani, Ashish, Noam Shazeer, and Niki Parmar" {
		t.Errorf("authorsChicago = %q", got)
	}
	if APA(w) == "" {
		t.Error("APA returned empty for valid work")
	}
}

func TestStyles_MissingYear(t *testing.T) {
	w := Work{Title: "Untitled", Authors: []Author{{Family: "Anon", Given: "A"}}}
	if got := APA(w); got != "Anon, A. (n.d.). Untitled." {
		t.Errorf("APA n.d.:\n got: %q", got)
	}
	if got := bibKey(w); got != "anon" {
		t.Errorf("bibKey without year = %q, want %q", got, "anon")
	}
}

func TestInitials(t *testing.T) {
	tests := []struct {
		given               string
		spaced, tight, bare string
	}{
		{"John A", "J. A.", "J.A.", "JA"},
		{"Jane", "J.", "J.", "J"},
		{"Mary-Jane", "M. J.", "M.J.", "MJ"},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		if got := initialsSpaced(tt.given); got != tt.spaced {
			t.Errorf("initialsSpaced(%q) = %q, want %q", tt.given, got, tt.spaced)
		}
		if got := initialsTight(tt.given); got != tt.tight {
			t.Errorf("initialsTight(%q) = %q, want %q", tt.given, got, tt.tight)
		}
		if got := initialsBare(tt.given); got != tt.bare {
			t.Errorf("initialsBare(%q) = %q, want %q", tt.given, got, tt.bare)
		}
	}
}

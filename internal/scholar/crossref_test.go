package scholar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeDOI(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"10.1000/xyz123", "10.1000/xyz123", true},
		{"  10.1000/xyz123  ", "10.1000/xyz123", true},
		{"https://doi.org/10.1000/xyz123", "10.1000/xyz123", true},
		{"http://dx.doi.org/10.1000/xyz123", "10.1000/xyz123", true},
		{"doi:10.1000/xyz123", "10.1000/xyz123", true},
		{"not-a-doi", "", false},
		{"10.1000", "", false}, // no slash
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeDOI(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeDOI(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

const sampleBody = `{
  "status": "ok",
  "message": {
    "DOI": "10.1000/xyz123",
    "type": "journal-article",
    "title": ["The structure of scientific revolutions"],
    "author": [
      {"given": "John A", "family": "Smith"},
      {"given": "Jane B", "family": "Doe"}
    ],
    "container-title": ["Journal of Testing"],
    "volume": "12",
    "issue": "3",
    "page": "45-67",
    "publisher": "Test Press",
    "URL": "https://doi.org/10.1000/xyz123",
    "issued": {"date-parts": [[2020, 5, 1]]}
  }
}`

func TestParseCrossref(t *testing.T) {
	w, err := parseCrossref([]byte(sampleBody))
	if err != nil {
		t.Fatalf("parseCrossref: %v", err)
	}
	if w.Title != "The structure of scientific revolutions" {
		t.Errorf("Title = %q", w.Title)
	}
	if len(w.Authors) != 2 || w.Authors[0].Family != "Smith" || w.Authors[1].Given != "Jane B" {
		t.Errorf("Authors = %+v", w.Authors)
	}
	if w.ContainerTitle != "Journal of Testing" {
		t.Errorf("ContainerTitle = %q", w.ContainerTitle)
	}
	if w.Volume != "12" || w.Issue != "3" || w.Pages != "45-67" {
		t.Errorf("vol/issue/pages = %q/%q/%q", w.Volume, w.Issue, w.Pages)
	}
	if w.Year != 2020 {
		t.Errorf("Year = %d, want 2020", w.Year)
	}
}

func TestParseCrossref_OrgAuthorAndFallbackYear(t *testing.T) {
	body := `{"message":{
		"DOI":"10.1/x","type":"report",
		"title":["A report"],
		"author":[{"name":"World Health Organization"}],
		"published-print":{"date-parts":[[2019]]}
	}}`
	w, err := parseCrossref([]byte(body))
	if err != nil {
		t.Fatalf("parseCrossref: %v", err)
	}
	if len(w.Authors) != 1 || w.Authors[0].Family != "World Health Organization" {
		t.Errorf("org author = %+v", w.Authors)
	}
	if w.Year != 2019 {
		t.Errorf("fallback year = %d, want 2019", w.Year)
	}
}

func TestFetchByDOI(t *testing.T) {
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUA = r.Header.Get("User-Agent")
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	c := NewCrossref("dev@example.com")
	c.baseURL = srv.URL + "/works/"

	w, err := c.FetchByDOI(context.Background(), "https://doi.org/10.1000/xyz123")
	if err != nil {
		t.Fatalf("FetchByDOI: %v", err)
	}
	if w.DOI != "10.1000/xyz123" {
		t.Errorf("DOI = %q", w.DOI)
	}
	if gotPath != "/works/10.1000/xyz123" {
		t.Errorf("request path = %q", gotPath)
	}
	if !strings.Contains(gotUA, "mailto:dev@example.com") {
		t.Errorf("User-Agent missing mailto: %q", gotUA)
	}
}

func TestFetchByDOI_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewCrossref("")
	c.baseURL = srv.URL + "/works/"

	if _, err := c.FetchByDOI(context.Background(), "10.1000/missing"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestFetchByDOI_InvalidDOI(t *testing.T) {
	c := NewCrossref("")
	if _, err := c.FetchByDOI(context.Background(), "garbage"); err != ErrInvalidDOI {
		t.Errorf("err = %v, want ErrInvalidDOI", err)
	}
}

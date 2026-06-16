package scholar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const openAlexBody = `{
  "results": [
    {
      "title": "Attention Is All You Need",
      "publication_year": 2017,
      "cited_by_count": 100000,
      "doi": "https://doi.org/10.5555/3295222.3295349",
      "authorships": [
        {"author": {"display_name": "Ashish Vaswani"}},
        {"author": {"display_name": "Noam Shazeer"}}
      ],
      "primary_location": {"source": {"display_name": "NeurIPS"}}
    },
    {
      "display_name": "A paper without DOI",
      "publication_year": 2020,
      "cited_by_count": 3,
      "doi": "",
      "authorships": [{"author": {"display_name": "Jane Roe"}}],
      "primary_location": {"source": {"display_name": "Unknown Journal"}}
    }
  ]
}`

func TestParseOpenAlex(t *testing.T) {
	got, err := parseOpenAlex([]byte(openAlexBody))
	if err != nil {
		t.Fatalf("parseOpenAlex: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	first := got[0]
	if first.Title != "Attention Is All You Need" {
		t.Errorf("title = %q", first.Title)
	}
	if first.Year != 2017 || first.CitedBy != 100000 || first.Venue != "NeurIPS" {
		t.Errorf("meta = %d/%d/%q", first.Year, first.CitedBy, first.Venue)
	}
	if first.DOI != "10.5555/3295222.3295349" {
		t.Errorf("DOI not normalized: %q", first.DOI)
	}
	if len(first.Authors) != 2 || first.Authors[0] != "Ashish Vaswani" {
		t.Errorf("authors = %v", first.Authors)
	}

	if got[1].Title != "A paper without DOI" {
		t.Errorf("display_name fallback failed: %q", got[1].Title)
	}
	if got[1].DOI != "" {
		t.Errorf("expected empty DOI, got %q", got[1].DOI)
	}
}

func TestSearch_BlankQueryNoCall(t *testing.T) {
	o := NewOpenAlex("")
	o.baseURL = "http://invalid.invalid/should-not-be-called"
	got, err := o.Search(context.Background(), "", 5)
	if err != nil || got != nil {
		t.Errorf("blank query = (%v, %v), want (nil, nil)", got, err)
	}
}

const openAlexAuthorsBody = `{
  "results": [
    {
      "display_name": "Yann LeCun",
      "works_count": 400,
      "cited_by_count": 300000,
      "orcid": "https://orcid.org/0000-0002-1825-0097",
      "summary_stats": {"h_index": 150, "i10_index": 350},
      "last_known_institutions": [{"display_name": "New York University"}],
      "x_concepts": [
        {"display_name": "Computer science"},
        {"display_name": "Artificial intelligence"},
        {"display_name": "Machine learning"},
        {"display_name": "Mathematics"}
      ]
    },
    {
      "display_name": "Jane Doe",
      "works_count": 12,
      "cited_by_count": 40,
      "summary_stats": {"h_index": 4},
      "last_known_institution": {"display_name": "Baghdad University"}
    }
  ]
}`

func TestParseOpenAlexAuthors(t *testing.T) {
	got, err := parseOpenAlexAuthors([]byte(openAlexAuthorsBody))
	if err != nil {
		t.Fatalf("parseOpenAlexAuthors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	a := got[0]
	if a.Name != "Yann LeCun" || a.WorksCount != 400 || a.CitedBy != 300000 {
		t.Errorf("author meta = %+v", a)
	}
	if a.HIndex != 150 || a.I10Index != 350 {
		t.Errorf("indices = %d/%d", a.HIndex, a.I10Index)
	}
	if a.Institution != "New York University" {
		t.Errorf("institution = %q", a.Institution)
	}
	if a.ORCID != "0000-0002-1825-0097" {
		t.Errorf("orcid not normalized: %q", a.ORCID)
	}
	if len(a.Concepts) != 3 { // capped at 3
		t.Errorf("concepts = %v, want 3", a.Concepts)
	}

	// Second author uses the singular institution field.
	if got[1].Institution != "Baghdad University" {
		t.Errorf("singular institution fallback failed: %q", got[1].Institution)
	}
}

func TestSearchAuthors_HTTP(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = rw.Write([]byte(openAlexAuthorsBody))
	}))
	defer srv.Close()

	o := NewOpenAlex("dev@example.com")
	o.authorsURL = srv.URL

	got, err := o.SearchAuthors(context.Background(), "lecun", 5)
	if err != nil {
		t.Fatalf("SearchAuthors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if gotQuery.Get("search") != "lecun" {
		t.Errorf("search param = %q", gotQuery.Get("search"))
	}
}

func TestTrending_HTTPParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = rw.Write([]byte(openAlexBody))
	}))
	defer srv.Close()

	o := NewOpenAlex("")
	o.baseURL = srv.URL

	got, err := o.Trending(context.Background(), "transformers", 5)
	if err != nil {
		t.Fatalf("Trending: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if gotQuery.Get("sort") != "cited_by_count:desc" {
		t.Errorf("sort = %q, want cited_by_count:desc", gotQuery.Get("sort"))
	}
	if !strings.HasPrefix(gotQuery.Get("filter"), "from_publication_date:") {
		t.Errorf("filter = %q, want a from_publication_date filter", gotQuery.Get("filter"))
	}
}

func TestTrending_BlankNoCall(t *testing.T) {
	o := NewOpenAlex("")
	o.baseURL = "http://invalid.invalid/should-not-be-called"
	got, err := o.Trending(context.Background(), "", 5)
	if err != nil || got != nil {
		t.Errorf("blank trending = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestParseOpenAlexAuthors_CapturesID(t *testing.T) {
	got, err := parseOpenAlexAuthors([]byte(openAlexAuthorsBody))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "" && got[0].ID == "https://openalex.org/" {
		t.Errorf("id should be stripped, got %q", got[0].ID)
	}
}

func TestAuthorWorks_HTTPParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = rw.Write([]byte(openAlexBody))
	}))
	defer srv.Close()

	o := NewOpenAlex("")
	o.baseURL = srv.URL

	got, err := o.AuthorWorks(context.Background(), "A123", 5)
	if err != nil {
		t.Fatalf("AuthorWorks: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if gotQuery.Get("filter") != "authorships.author.id:A123" {
		t.Errorf("filter = %q", gotQuery.Get("filter"))
	}
	if gotQuery.Get("sort") != "cited_by_count:desc" {
		t.Errorf("sort = %q", gotQuery.Get("sort"))
	}
}

func TestAuthorWorks_BlankNoCall(t *testing.T) {
	o := NewOpenAlex("")
	o.baseURL = "http://invalid.invalid/should-not-be-called"
	if got, err := o.AuthorWorks(context.Background(), "", 5); err != nil || got != nil {
		t.Errorf("blank author id = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestSearchAuthors_BlankNoCall(t *testing.T) {
	o := NewOpenAlex("")
	o.authorsURL = "http://invalid.invalid/should-not-be-called"
	got, err := o.SearchAuthors(context.Background(), "", 5)
	if err != nil || got != nil {
		t.Errorf("blank author query = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestSearch_HTTP(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = rw.Write([]byte(openAlexBody))
	}))
	defer srv.Close()

	o := NewOpenAlex("dev@example.com")
	o.baseURL = srv.URL

	got, err := o.Search(context.Background(), "transformers", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if gotQuery.Get("search") != "transformers" {
		t.Errorf("search param = %q", gotQuery.Get("search"))
	}
	if gotQuery.Get("mailto") != "dev@example.com" {
		t.Errorf("mailto param = %q", gotQuery.Get("mailto"))
	}
	if gotQuery.Get("per-page") != "5" {
		t.Errorf("per-page param = %q", gotQuery.Get("per-page"))
	}
}

package scholar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSemanticScholar_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "graphene" {
			t.Errorf("query = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"title":"Graphene basics","year":2019,"venue":"Nature","citationCount":42,
			 "authors":[{"name":"A. One"},{"name":"B. Two"}],
			 "externalIds":{"DOI":"10.1/abc"},"tldr":{"text":"It is about graphene."}},
			{"title":"","year":2020}
		]}`))
	}))
	defer srv.Close()

	c := &SemanticScholar{base: srv.URL, http: srv.Client()}
	res, err := c.Search(context.Background(), "graphene", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 1 { // blank-title row dropped
		t.Fatalf("got %d results, want 1", len(res))
	}
	r := res[0]
	if r.Title != "Graphene basics" || r.CitedBy != 42 || r.DOI != "10.1/abc" ||
		r.Summary != "It is about graphene." || len(r.Authors) != 2 {
		t.Errorf("unexpected result: %+v", r)
	}
}

func TestArxiv_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("search_query"); got != "all:transformers" {
			t.Errorf("search_query = %q", got)
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Attention Is All You Need</title>
    <summary>We propose the Transformer. It is great.</summary>
    <published>2017-06-12T00:00:00Z</published>
    <id>http://arxiv.org/abs/1706.03762v5</id>
    <author><name>Ashish Vaswani</name></author>
    <author><name>Noam Shazeer</name></author>
  </entry>
</feed>`))
	}))
	defer srv.Close()

	c := &Arxiv{base: srv.URL, http: srv.Client()}
	res, err := c.Search(context.Background(), "transformers", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	r := res[0]
	if r.Title != "Attention Is All You Need" || r.Year != 2017 || r.Venue != "arXiv" ||
		r.URL != "http://arxiv.org/abs/1706.03762v5" || len(r.Authors) != 2 {
		t.Errorf("unexpected result: %+v", r)
	}
	if r.Summary != "We propose the Transformer." {
		t.Errorf("summary = %q", r.Summary)
	}
}

func TestPubMed_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/esearch.fcgi":
			_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["111","222"]}}`))
		case r.URL.Path == "/esummary.fcgi":
			_, _ = w.Write([]byte(`{"result":{"uids":["111","222"],
				"111":{"title":"CRISPR review.","pubdate":"2021 Mar","source":"Cell",
					"authors":[{"name":"Doe J"}],
					"articleids":[{"idtype":"pubmed","value":"111"},{"idtype":"doi","value":"10.9/xyz"}]},
				"222":{"title":"Second paper","pubdate":"2018","source":"BMJ","authors":[]}
			}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &PubMed{base: srv.URL, http: srv.Client()}
	res, err := c.Search(context.Background(), "crispr", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}
	if res[0].Title != "CRISPR review" || res[0].Year != 2021 || res[0].DOI != "10.9/xyz" ||
		res[0].Venue != "Cell" || res[0].URL != "https://pubmed.ncbi.nlm.nih.gov/111/" {
		t.Errorf("result 0 unexpected: %+v", res[0])
	}
	if res[1].Year != 2018 || res[1].DOI != "" {
		t.Errorf("result 1 unexpected: %+v", res[1])
	}
}

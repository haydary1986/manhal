package scholar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const unpaywallOABody = `{
  "doi": "10.1000/xyz123",
  "title": "An Open Paper",
  "is_oa": true,
  "best_oa_location": {
    "url": "https://repo.example/landing",
    "url_for_pdf": "https://repo.example/paper.pdf",
    "host_type": "repository",
    "version": "publishedVersion"
  },
  "oa_locations": [
    {"url": "https://repo.example/landing", "url_for_pdf": "https://repo.example/paper.pdf", "host_type": "repository", "version": "publishedVersion"},
    {"url": "https://publisher.example/article", "url_for_pdf": "", "host_type": "publisher", "version": "acceptedVersion"}
  ]
}`

func TestParseUnpaywall_OA(t *testing.T) {
	r, err := parseUnpaywall([]byte(unpaywallOABody))
	if err != nil {
		t.Fatalf("parseUnpaywall: %v", err)
	}
	if !r.IsOA || r.Title != "An Open Paper" {
		t.Errorf("result = %+v", r)
	}
	if r.Best == nil || !r.Best.IsPDF || r.Best.URL != "https://repo.example/paper.pdf" {
		t.Errorf("best location = %+v", r.Best)
	}
	if len(r.Locations) != 2 {
		t.Fatalf("locations = %d, want 2", len(r.Locations))
	}
	// Second location has no PDF, so it falls back to the landing URL.
	if r.Locations[1].IsPDF || r.Locations[1].URL != "https://publisher.example/article" {
		t.Errorf("landing fallback failed: %+v", r.Locations[1])
	}
}

func TestParseUnpaywall_Closed(t *testing.T) {
	r, err := parseUnpaywall([]byte(`{"doi":"10.1/x","is_oa":false,"best_oa_location":null,"oa_locations":[]}`))
	if err != nil {
		t.Fatalf("parseUnpaywall: %v", err)
	}
	if r.IsOA || r.Best != nil || len(r.Locations) != 0 {
		t.Errorf("closed result = %+v", r)
	}
}

func TestFindOA_HTTP(t *testing.T) {
	var gotEmail string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotEmail = r.URL.Query().Get("email")
		_, _ = rw.Write([]byte(unpaywallOABody))
	}))
	defer srv.Close()

	u := NewUnpaywall("dev@example.com")
	u.baseURL = srv.URL + "/v2/"

	r, err := u.FindOA(context.Background(), "https://doi.org/10.1000/xyz123")
	if err != nil {
		t.Fatalf("FindOA: %v", err)
	}
	if !r.IsOA {
		t.Error("expected OA")
	}
	if gotEmail != "dev@example.com" {
		t.Errorf("email param = %q", gotEmail)
	}
}

func TestFindOA_Errors(t *testing.T) {
	if _, err := NewUnpaywall("").FindOA(context.Background(), "10.1/x"); err != ErrNotConfigured {
		t.Errorf("no email => %v, want ErrNotConfigured", err)
	}
	if _, err := NewUnpaywall("e@x.com").FindOA(context.Background(), "garbage"); err != ErrInvalidDOI {
		t.Errorf("bad doi => %v, want ErrInvalidDOI", err)
	}
}

func TestFindOA_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	u := NewUnpaywall("e@x.com")
	u.baseURL = srv.URL + "/v2/"
	if _, err := u.FindOA(context.Background(), "10.1000/missing"); err != ErrNotFound {
		t.Errorf("404 => %v, want ErrNotFound", err)
	}
}

func TestFindOA_PathContainsDOI(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = rw.Write([]byte(unpaywallOABody))
	}))
	defer srv.Close()
	u := NewUnpaywall("e@x.com")
	u.baseURL = srv.URL + "/v2/"
	if _, err := u.FindOA(context.Background(), "10.1000/xyz123"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotPath, "/v2/10.1000/xyz123") {
		t.Errorf("path = %q", gotPath)
	}
}

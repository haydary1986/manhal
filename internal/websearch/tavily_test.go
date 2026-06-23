package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTavily_Disabled(t *testing.T) {
	c := NewTavily("")
	if c.Enabled() {
		t.Error("empty key should be disabled")
	}
	if _, err := c.Search(context.Background(), "x", 5); err == nil {
		t.Error("disabled client should error")
	}
}

func TestTavily_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tvly-test" {
			t.Errorf("auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"Photosynthesis converts light to energy.",
			"results":[
				{"title":"Photosynthesis - Wikipedia","url":"https://en.wikipedia.org/wiki/Photosynthesis"},
				{"title":"","url":"https://example.org/p"},
				{"title":"skip","url":""}
			]}`))
	}))
	defer srv.Close()

	c := NewTavily("tvly-test")
	c.endpoint = srv.URL
	c.http = srv.Client()

	ans, err := c.Search(context.Background(), "what is photosynthesis", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if ans.Text != "Photosynthesis converts light to energy." {
		t.Errorf("answer = %q", ans.Text)
	}
	if len(ans.Results) != 2 { // empty-URL row dropped
		t.Fatalf("got %d results, want 2", len(ans.Results))
	}
	if ans.Results[1].Title != "https://example.org/p" { // blank title falls back to URL
		t.Errorf("title fallback failed: %+v", ans.Results[1])
	}
}

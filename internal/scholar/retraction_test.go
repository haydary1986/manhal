package scholar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// crossrefRetractedBody mirrors the real Crossref "updates" response shape for a
// retracted DOI (Retraction Watch source).
const crossrefRetractedBody = `{"message":{"items":[
  {"DOI":"10.9999/notice","update-to":[
    {"DOI":"10.1/x","type":"retraction","updated":{"date-parts":[[2010,2,6]]}}
  ]}
]}}`

func TestCheckRetraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(crossrefRetractedBody))
	}))
	defer srv.Close()
	c := NewCrossref("dev@example.com")
	c.http = srv.Client()
	// CheckRetraction builds an absolute api.crossref.org URL, so point the
	// client's transport at the test server.
	c.http.Transport = rewriteHost(srv.URL)

	r, err := c.CheckRetraction(context.Background(), "10.1/x")
	if err != nil {
		t.Fatalf("CheckRetraction: %v", err)
	}
	if !r.Retracted || r.NoticeDOI != "10.9999/notice" || r.Date != "2010-02-06" {
		t.Errorf("got %+v, want retracted notice 10.9999/notice on 2010-02-06", r)
	}

	if _, err := c.CheckRetraction(context.Background(), "garbage"); err != ErrInvalidDOI {
		t.Errorf("bad doi => %v, want ErrInvalidDOI", err)
	}
}

// rewriteHost redirects any request to the test server.
type hostRewriter struct{ target string }

func (h hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	u, _ := req.URL.Parse(h.target)
	req.URL.Scheme, req.URL.Host = u.Scheme, u.Host
	return http.DefaultTransport.RoundTrip(req)
}

func rewriteHost(target string) http.RoundTripper { return hostRewriter{target: target} }

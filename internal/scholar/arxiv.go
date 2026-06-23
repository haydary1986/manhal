package scholar

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// arxivBase is the public arXiv Atom query API (no key required).
const arxivBase = "http://export.arxiv.org/api/query"

// Arxiv searches arXiv preprints (CS, physics, math, engineering, etc.).
type Arxiv struct {
	base string
	http *http.Client
}

// NewArxiv builds a client with sane timeouts.
func NewArxiv() *Arxiv {
	return &Arxiv{base: arxivBase, http: &http.Client{Timeout: 20 * time.Second}}
}

// arxivFeed mirrors the parts of the Atom response we use.
type arxivFeed struct {
	Entries []struct {
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		Published string `xml:"published"`
		ID        string `xml:"id"`
		DOI       string `xml:"doi"` // arxiv:doi when present
		Authors   []struct {
			Name string `xml:"name"`
		} `xml:"author"`
	} `xml:"entry"`
}

// Search returns up to limit arXiv preprints matching the query.
func (a *Arxiv) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	q := url.Values{}
	q.Set("search_query", "all:"+query)
	q.Set("start", "0")
	q.Set("max_results", fmt.Sprintf("%d", limit))
	q.Set("sortBy", "relevance")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("arxiv: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic search bot)")
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arxiv: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: status %d", resp.StatusCode)
	}

	var feed arxivFeed
	dec := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if err := dec.Decode(&feed); err != nil {
		return nil, fmt.Errorf("arxiv: decode: %w", err)
	}

	out := make([]SearchResult, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		title := normalizeSpace(e.Title)
		if title == "" {
			continue
		}
		authors := make([]string, 0, len(e.Authors))
		for _, au := range e.Authors {
			if n := strings.TrimSpace(au.Name); n != "" {
				authors = append(authors, n)
			}
		}
		out = append(out, SearchResult{
			Title:   title,
			Authors: authors,
			Year:    parseYear(e.Published),
			Venue:   "arXiv",
			DOI:     strings.TrimSpace(e.DOI),
			Summary: firstSentence(normalizeSpace(e.Summary)),
			URL:     strings.TrimSpace(e.ID),
		})
	}
	return out, nil
}

// parseYear extracts the leading 4-digit year from an ISO timestamp.
func parseYear(ts string) int {
	ts = strings.TrimSpace(ts)
	if len(ts) < 4 {
		return 0
	}
	y := 0
	for _, r := range ts[:4] {
		if r < '0' || r > '9' {
			return 0
		}
		y = y*10 + int(r-'0')
	}
	return y
}

// normalizeSpace collapses internal whitespace/newlines to single spaces.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// firstSentence returns the first sentence (bounded) for a compact gist.
func firstSentence(s string) string {
	const maxLen = 220
	if i := strings.IndexAny(s, ".؟!"); i > 0 && i < maxLen {
		return s[:i+1]
	}
	if len(s) > maxLen {
		return strings.TrimSpace(s[:maxLen]) + "…"
	}
	return s
}

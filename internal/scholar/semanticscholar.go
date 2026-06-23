package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// semanticScholarBase is the public Graph API search endpoint (no key required;
// rate-limited). A key can raise limits but is not needed for light use.
const semanticScholarBase = "https://api.semanticscholar.org/graph/v1/paper/search"

// SemanticScholar searches Semantic Scholar, which adds an AI-generated one-line
// gist (TLDR) and influential citation counts on top of the usual metadata.
type SemanticScholar struct {
	base string
	http *http.Client
}

// NewSemanticScholar builds a client with sane timeouts.
func NewSemanticScholar() *SemanticScholar {
	return &SemanticScholar{base: semanticScholarBase, http: &http.Client{Timeout: 20 * time.Second}}
}

// Search returns up to limit papers matching the free-text query.
func (s *SemanticScholar) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("fields", "title,authors,year,venue,citationCount,externalIds,tldr")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("semanticscholar: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic search bot)")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("semanticscholar: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semanticscholar: status %d", resp.StatusCode)
	}

	var doc struct {
		Data []struct {
			Title         string `json:"title"`
			Year          int    `json:"year"`
			Venue         string `json:"venue"`
			CitationCount int    `json:"citationCount"`
			Authors       []struct {
				Name string `json:"name"`
			} `json:"authors"`
			ExternalIDs struct {
				DOI string `json:"DOI"`
			} `json:"externalIds"`
			TLDR struct {
				Text string `json:"text"`
			} `json:"tldr"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("semanticscholar: decode: %w", err)
	}

	out := make([]SearchResult, 0, len(doc.Data))
	for _, p := range doc.Data {
		if strings.TrimSpace(p.Title) == "" {
			continue
		}
		authors := make([]string, 0, len(p.Authors))
		for _, a := range p.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}
		out = append(out, SearchResult{
			Title:   p.Title,
			Authors: authors,
			Year:    p.Year,
			Venue:   p.Venue,
			CitedBy: p.CitationCount,
			DOI:     strings.TrimSpace(p.ExternalIDs.DOI),
			Summary: strings.TrimSpace(p.TLDR.Text),
		})
	}
	return out, nil
}

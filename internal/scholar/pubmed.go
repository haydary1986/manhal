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

// pubmedBase is the NCBI E-utilities base (no key required; a key raises limits).
const pubmedBase = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

// PubMed searches the PubMed biomedical literature database via E-utilities.
type PubMed struct {
	base string
	http *http.Client
}

// NewPubMed builds a client with sane timeouts.
func NewPubMed() *PubMed {
	return &PubMed{base: pubmedBase, http: &http.Client{Timeout: 20 * time.Second}}
}

// Search runs esearch then esummary and maps the results.
func (p *PubMed) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	ids, err := p.esearch(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return p.esummary(ctx, ids)
}

func (p *PubMed) esearch(ctx context.Context, query string, limit int) ([]string, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("term", query)
	q.Set("retmax", fmt.Sprintf("%d", limit))
	q.Set("retmode", "json")
	q.Set("sort", "relevance")

	var doc struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := p.getJSON(ctx, p.base+"/esearch.fcgi?"+q.Encode(), &doc); err != nil {
		return nil, fmt.Errorf("pubmed esearch: %w", err)
	}
	return doc.ESearchResult.IDList, nil
}

func (p *PubMed) esummary(ctx context.Context, ids []string) ([]SearchResult, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("id", strings.Join(ids, ","))
	q.Set("retmode", "json")

	var doc struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := p.getJSON(ctx, p.base+"/esummary.fcgi?"+q.Encode(), &doc); err != nil {
		return nil, fmt.Errorf("pubmed esummary: %w", err)
	}

	out := make([]SearchResult, 0, len(ids))
	for _, id := range ids {
		raw, ok := doc.Result[id]
		if !ok {
			continue
		}
		var item struct {
			Title   string `json:"title"`
			PubDate string `json:"pubdate"`
			Source  string `json:"source"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			ArticleIDs []struct {
				IDType string `json:"idtype"`
				Value  string `json:"value"`
			} `json:"articleids"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		authors := make([]string, 0, len(item.Authors))
		for _, a := range item.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}
		doi := ""
		for _, a := range item.ArticleIDs {
			if strings.EqualFold(a.IDType, "doi") {
				doi = strings.TrimSpace(a.Value)
				break
			}
		}
		out = append(out, SearchResult{
			Title:   strings.TrimRight(normalizeSpace(item.Title), "."),
			Authors: authors,
			Year:    parseYear(item.PubDate),
			Venue:   item.Source,
			DOI:     doi,
			URL:     "https://pubmed.ncbi.nlm.nih.gov/" + id + "/",
		})
	}
	return out, nil
}

func (p *PubMed) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic search bot)")
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v)
}

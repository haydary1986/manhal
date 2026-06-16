package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	openAlexBase        = "https://api.openalex.org/works"
	openAlexAuthorsBase = "https://api.openalex.org/authors"
)

// SearchResult is a lightweight paper summary returned by a search.
type SearchResult struct {
	Title   string
	Authors []string // author display names
	Year    int
	Venue   string // journal/conference name
	CitedBy int
	DOI     string // bare DOI, or "" when none is recorded
}

// Author is a researcher profile summary.
type Author struct {
	ID          string // bare OpenAlex id (e.g. "A5023888391"), for fetching works
	Name        string
	Institution string
	WorksCount  int
	CitedBy     int
	HIndex      int
	I10Index    int
	ORCID       string   // bare ORCID id, or "" when none
	Concepts    []string // top research areas
}

// OpenAlex searches the free OpenAlex works and authors APIs.
type OpenAlex struct {
	http       *http.Client
	baseURL    string // works endpoint
	authorsURL string // authors endpoint
	mailto     string // joins the polite pool when set
}

// NewOpenAlex builds an OpenAlex client. mailto is optional but recommended.
func NewOpenAlex(mailto string) *OpenAlex {
	return &OpenAlex{
		http:       &http.Client{Timeout: 15 * time.Second},
		baseURL:    openAlexBase,
		authorsURL: openAlexAuthorsBase,
		mailto:     mailto,
	}
}

// Search returns up to `limit` works matching the free-text query, ordered by
// OpenAlex relevance. A blank query yields no results without a network call.
func (o *OpenAlex) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 25 {
		limit = 5
	}

	u, err := url.Parse(o.baseURL)
	if err != nil {
		return nil, fmt.Errorf("openalex base url: %w", err)
	}
	q := u.Query()
	q.Set("search", query)
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("select", "title,display_name,publication_year,cited_by_count,doi,authorships,primary_location")
	if o.mailto != "" {
		q.Set("mailto", o.mailto)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic search bot)")
	req.Header.Set("Accept", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("openalex read: %w", err)
	}
	return parseOpenAlex(body)
}

// --- OpenAlex JSON shape (only the fields we use) ---

type openAlexResponse struct {
	Results []openAlexWork `json:"results"`
}

type openAlexWork struct {
	Title           string `json:"title"`
	DisplayName     string `json:"display_name"`
	PublicationYear int    `json:"publication_year"`
	CitedByCount    int    `json:"cited_by_count"`
	DOI             string `json:"doi"`
	Authorships     []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation struct {
		Source struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
	} `json:"primary_location"`
}

func parseOpenAlex(body []byte) ([]SearchResult, error) {
	var r openAlexResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openalex decode: %w", err)
	}
	out := make([]SearchResult, 0, len(r.Results))
	for _, w := range r.Results {
		out = append(out, w.toResult())
	}
	return out, nil
}

func (w openAlexWork) toResult() SearchResult {
	title := w.Title
	if title == "" {
		title = w.DisplayName
	}
	res := SearchResult{
		Title:   title,
		Year:    w.PublicationYear,
		Venue:   w.PrimaryLocation.Source.DisplayName,
		CitedBy: w.CitedByCount,
	}
	if doi, ok := NormalizeDOI(w.DOI); ok {
		res.DOI = doi
	}
	for _, a := range w.Authorships {
		if name := a.Author.DisplayName; name != "" {
			res.Authors = append(res.Authors, name)
		}
	}
	return res
}

// Trending returns the most-cited works for a topic published within the last
// three years — a proxy for "what's hot" in a field (#6). A blank topic yields
// no results without a network call.
func (o *OpenAlex) Trending(ctx context.Context, topic string, limit int) ([]SearchResult, error) {
	if topic == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 25 {
		limit = 5
	}

	u, err := url.Parse(o.baseURL)
	if err != nil {
		return nil, fmt.Errorf("openalex base url: %w", err)
	}
	since := time.Now().AddDate(-3, 0, 0).Format("2006-01-02")
	q := u.Query()
	q.Set("search", topic)
	q.Set("sort", "cited_by_count:desc")
	q.Set("filter", "from_publication_date:"+since)
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("select", "title,display_name,publication_year,cited_by_count,doi,authorships,primary_location")
	if o.mailto != "" {
		q.Set("mailto", o.mailto)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic trends)")
	req.Header.Set("Accept", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex trending request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex trending: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("openalex trending read: %w", err)
	}
	return parseOpenAlex(body)
}

// SearchAuthors returns up to `limit` researcher profiles matching the name.
// A blank query yields no results without a network call.
func (o *OpenAlex) SearchAuthors(ctx context.Context, name string, limit int) ([]Author, error) {
	if name == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 25 {
		limit = 5
	}

	u, err := url.Parse(o.authorsURL)
	if err != nil {
		return nil, fmt.Errorf("openalex authors url: %w", err)
	}
	q := u.Query()
	q.Set("search", name)
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("select", "id,display_name,works_count,cited_by_count,summary_stats,last_known_institutions,orcid,x_concepts")
	if o.mailto != "" {
		q.Set("mailto", o.mailto)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (academic search bot)")
	req.Header.Set("Accept", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex authors request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex authors: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("openalex authors read: %w", err)
	}
	return parseOpenAlexAuthors(body)
}

// AuthorWorks returns an author's most-cited works by OpenAlex author id (#8).
func (o *OpenAlex) AuthorWorks(ctx context.Context, authorID string, limit int) ([]SearchResult, error) {
	if authorID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 25 {
		limit = 5
	}

	u, err := url.Parse(o.baseURL)
	if err != nil {
		return nil, fmt.Errorf("openalex base url: %w", err)
	}
	q := u.Query()
	q.Set("filter", "authorships.author.id:"+authorID)
	q.Set("sort", "cited_by_count:desc")
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("select", "title,display_name,publication_year,cited_by_count,doi,authorships,primary_location")
	if o.mailto != "" {
		q.Set("mailto", o.mailto)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Manhal/0.1 (research radar)")
	req.Header.Set("Accept", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openalex author works request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openalex author works: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("openalex author works read: %w", err)
	}
	return parseOpenAlex(body)
}

type openAlexAuthorsResponse struct {
	Results []openAlexAuthor `json:"results"`
}

type openAlexAuthor struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	WorksCount   int    `json:"works_count"`
	CitedByCount int    `json:"cited_by_count"`
	ORCID        string `json:"orcid"`
	SummaryStats struct {
		HIndex   int `json:"h_index"`
		I10Index int `json:"i10_index"`
	} `json:"summary_stats"`
	LastKnownInstitutions []struct {
		DisplayName string `json:"display_name"`
	} `json:"last_known_institutions"`
	LastKnownInstitution struct {
		DisplayName string `json:"display_name"`
	} `json:"last_known_institution"`
	XConcepts []struct {
		DisplayName string `json:"display_name"`
	} `json:"x_concepts"`
}

func parseOpenAlexAuthors(body []byte) ([]Author, error) {
	var r openAlexAuthorsResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("openalex authors decode: %w", err)
	}
	out := make([]Author, 0, len(r.Results))
	for _, a := range r.Results {
		out = append(out, a.toAuthor())
	}
	return out, nil
}

func (a openAlexAuthor) toAuthor() Author {
	res := Author{
		ID:          strings.TrimPrefix(a.ID, "https://openalex.org/"),
		Name:        a.DisplayName,
		WorksCount:  a.WorksCount,
		CitedBy:     a.CitedByCount,
		HIndex:      a.SummaryStats.HIndex,
		I10Index:    a.SummaryStats.I10Index,
		Institution: a.institution(),
	}
	if id := strings.TrimPrefix(a.ORCID, "https://orcid.org/"); id != a.ORCID {
		res.ORCID = id
	}
	for i, c := range a.XConcepts {
		if i == 3 {
			break
		}
		if c.DisplayName != "" {
			res.Concepts = append(res.Concepts, c.DisplayName)
		}
	}
	return res
}

// institution prefers the newer array field, falling back to the singular one.
func (a openAlexAuthor) institution() string {
	if len(a.LastKnownInstitutions) > 0 {
		return a.LastKnownInstitutions[0].DisplayName
	}
	return a.LastKnownInstitution.DisplayName
}

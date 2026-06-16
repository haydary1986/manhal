// Package scholar fetches bibliographic metadata from scholarly APIs and
// normalizes it into the cite.Work model. Crossref (DOI -> metadata) is the
// first source; Semantic Scholar / OpenAlex are added later.
package scholar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/cite"
)

// ErrNotFound is returned when the DOI does not resolve to a record.
var ErrNotFound = errors.New("scholar: DOI not found")

// ErrInvalidDOI is returned when the input is not a plausible DOI.
var ErrInvalidDOI = errors.New("scholar: invalid DOI")

const crossrefBase = "https://api.crossref.org/works/"

// Crossref fetches works from the Crossref REST API.
type Crossref struct {
	http    *http.Client
	baseURL string
	mailto  string // included in the User-Agent for the polite pool
}

// NewCrossref builds a Crossref client. mailto is optional but recommended;
// when set, requests join Crossref's faster "polite" pool.
func NewCrossref(mailto string) *Crossref {
	return &Crossref{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: crossrefBase,
		mailto:  mailto,
	}
}

// FetchByDOI resolves a DOI to a normalized cite.Work.
func (c *Crossref) FetchByDOI(ctx context.Context, doi string) (*cite.Work, error) {
	clean, ok := NormalizeDOI(doi)
	if !ok {
		return nil, ErrInvalidDOI
	}

	// Keep the DOI's slashes literal (Crossref matches on the raw DOI) while
	// escaping any other unsafe characters via url.URL's canonical encoding.
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("crossref base url: %w", err)
	}
	base.Path += clean
	endpoint := base.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crossref request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("crossref: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("crossref read: %w", err)
	}
	return parseCrossref(body)
}

func (c *Crossref) userAgent() string {
	ua := "Manhal/0.1 (https://t.me; academic citation bot)"
	if c.mailto != "" {
		ua += " mailto:" + c.mailto
	}
	return ua
}

// NormalizeDOI strips common prefixes and validates the basic DOI shape
// (must start with "10." and contain a "/"). Returns the bare DOI.
func NormalizeDOI(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://doi.org/")
	s = strings.TrimPrefix(s, "http://doi.org/")
	s = strings.TrimPrefix(s, "https://dx.doi.org/")
	s = strings.TrimPrefix(s, "http://dx.doi.org/")
	s = strings.TrimPrefix(s, "doi:")
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "10.") || !strings.Contains(s, "/") {
		return "", false
	}
	return s, true
}

// --- Crossref JSON shape (only the fields we use) ---

type crossrefResponse struct {
	Message crossrefMessage `json:"message"`
}

type crossrefMessage struct {
	DOI                 string           `json:"DOI"`
	Type                string           `json:"type"`
	Title               []string         `json:"title"`
	Author              []crossrefAuthor `json:"author"`
	ContainerTitle      []string         `json:"container-title"`
	ShortContainerTitle []string         `json:"short-container-title"`
	Publisher           string           `json:"publisher"`
	Volume              string           `json:"volume"`
	Issue               string           `json:"issue"`
	Page                string           `json:"page"`
	URL                 string           `json:"URL"`
	Issued              crossrefDate     `json:"issued"`
	PublishedPrint      crossrefDate     `json:"published-print"`
	PublishedOnline     crossrefDate     `json:"published-online"`
	Published           crossrefDate     `json:"published"`
}

type crossrefAuthor struct {
	Given  string `json:"given"`
	Family string `json:"family"`
	Name   string `json:"name"` // organizations use a single "name" field
}

type crossrefDate struct {
	DateParts [][]int `json:"date-parts"`
}

func (d crossrefDate) year() int {
	if len(d.DateParts) > 0 && len(d.DateParts[0]) > 0 {
		return d.DateParts[0][0]
	}
	return 0
}

// parseCrossref converts a Crossref /works/{doi} response body to a cite.Work.
func parseCrossref(body []byte) (*cite.Work, error) {
	var r crossrefResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("crossref decode: %w", err)
	}
	return r.Message.toWork(), nil
}

func (m crossrefMessage) toWork() *cite.Work {
	w := &cite.Work{
		Type:           m.Type,
		Title:          first(m.Title),
		ContainerTitle: firstNonEmpty(first(m.ContainerTitle), first(m.ShortContainerTitle)),
		Publisher:      m.Publisher,
		Volume:         m.Volume,
		Issue:          m.Issue,
		Pages:          m.Page,
		DOI:            m.DOI,
		URL:            m.URL,
		Year:           pickYear(m),
	}
	for _, a := range m.Author {
		family := a.Family
		given := a.Given
		if family == "" && given == "" && a.Name != "" {
			family = a.Name // organization author
		}
		w.Authors = append(w.Authors, cite.Author{Family: family, Given: given})
	}
	return w
}

// pickYear returns the most authoritative publication year available.
func pickYear(m crossrefMessage) int {
	for _, d := range []crossrefDate{m.Issued, m.PublishedPrint, m.PublishedOnline, m.Published} {
		if y := d.year(); y > 0 {
			return y
		}
	}
	return 0
}

func first(s []string) string {
	if len(s) > 0 {
		return strings.TrimSpace(s[0])
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

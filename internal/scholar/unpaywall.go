package scholar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrNotConfigured is returned when a required contact email is missing.
var ErrNotConfigured = errors.New("scholar: feature not configured")

const unpaywallBase = "https://api.unpaywall.org/v2/"

// OALocation is one legal open-access copy of a work.
type OALocation struct {
	URL      string // direct PDF when available, else the landing page
	IsPDF    bool
	HostType string // "repository" | "publisher"
	Version  string // publishedVersion | acceptedVersion | submittedVersion
}

// OAResult is the open-access status of a work.
type OAResult struct {
	DOI       string
	Title     string
	IsOA      bool
	Best      *OALocation
	Locations []OALocation
}

// Unpaywall finds legal open-access copies via the Unpaywall API.
type Unpaywall struct {
	http    *http.Client
	baseURL string
	email   string // required by Unpaywall
}

// defaultUnpaywallEmail is a polite-pool contact used when the admin hasn't set
// one, so the open-access feature works out of the box. Unpaywall only needs a
// contact address, not a deliverable inbox.
const defaultUnpaywallEmail = "manhal-bot@users.noreply.github.com"

// NewUnpaywall builds an Unpaywall client. When email is empty a default contact
// is used so the feature stays enabled.
func NewUnpaywall(email string) *Unpaywall {
	if email == "" {
		email = defaultUnpaywallEmail
	}
	return &Unpaywall{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: unpaywallBase,
		email:   email,
	}
}

// FindOA resolves a DOI to its open-access status and legal copies.
func (u *Unpaywall) FindOA(ctx context.Context, doi string) (*OAResult, error) {
	if u.email == "" {
		return nil, ErrNotConfigured
	}
	clean, ok := NormalizeDOI(doi)
	if !ok {
		return nil, ErrInvalidDOI
	}

	base, err := url.Parse(u.baseURL)
	if err != nil {
		return nil, fmt.Errorf("unpaywall base url: %w", err)
	}
	base.Path += clean
	base.RawQuery = url.Values{"email": {u.email}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := u.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unpaywall request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnprocessableEntity:
		return nil, ErrInvalidDOI
	default:
		return nil, fmt.Errorf("unpaywall: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("unpaywall read: %w", err)
	}
	return parseUnpaywall(body)
}

type unpaywallResponse struct {
	DOI            string              `json:"doi"`
	Title          string              `json:"title"`
	IsOA           bool                `json:"is_oa"`
	BestOALocation *unpaywallLocation  `json:"best_oa_location"`
	OALocations    []unpaywallLocation `json:"oa_locations"`
}

type unpaywallLocation struct {
	URL       string `json:"url"`
	URLForPDF string `json:"url_for_pdf"`
	HostType  string `json:"host_type"`
	Version   string `json:"version"`
}

func (l unpaywallLocation) toLocation() OALocation {
	loc := OALocation{HostType: l.HostType, Version: l.Version}
	if l.URLForPDF != "" {
		loc.URL = l.URLForPDF
		loc.IsPDF = true
	} else {
		loc.URL = l.URL
	}
	return loc
}

func parseUnpaywall(body []byte) (*OAResult, error) {
	var r unpaywallResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("unpaywall decode: %w", err)
	}
	res := &OAResult{DOI: r.DOI, Title: r.Title, IsOA: r.IsOA}
	if r.BestOALocation != nil {
		best := r.BestOALocation.toLocation()
		res.Best = &best
	}
	for _, l := range r.OALocations {
		res.Locations = append(res.Locations, l.toLocation())
	}
	return res, nil
}

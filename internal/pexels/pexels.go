// Package pexels fetches a relevant stock photo for a query via the Pexels API.
// The API key is supplied from configuration (never hardcoded). When no key is
// set the client is disabled and callers skip images gracefully.
package pexels

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
)

// ErrNoImage is returned when a search yields no photo.
var ErrNoImage = errors.New("pexels: no image found")

// Client calls the Pexels REST API.
type Client struct {
	key  string
	http *http.Client
}

// New builds a client; an empty key disables it.
func New(key string) *Client {
	return &Client{key: strings.TrimSpace(key), http: &http.Client{Timeout: 20 * time.Second}}
}

// Enabled reports whether a key is configured.
func (c *Client) Enabled() bool { return c != nil && c.key != "" }

// Photo searches for a landscape photo and downloads its JPEG bytes.
func (c *Client) Photo(ctx context.Context, query string) ([]byte, error) {
	if !c.Enabled() {
		return nil, errors.New("pexels: not configured")
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, ErrNoImage
	}
	endpoint := "https://api.pexels.com/v1/search?per_page=1&orientation=landscape&query=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pexels request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pexels: status %d", resp.StatusCode)
	}
	var sr struct {
		Photos []struct {
			Src struct {
				Large  string `json:"large"`
				Medium string `json:"medium"`
			} `json:"src"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("pexels parse: %w", err)
	}
	if len(sr.Photos) == 0 {
		return nil, ErrNoImage
	}
	imgURL := sr.Photos[0].Src.Large
	if imgURL == "" {
		imgURL = sr.Photos[0].Src.Medium
	}
	if imgURL == "" {
		return nil, ErrNoImage
	}
	return c.download(ctx, imgURL)
}

func (c *Client) download(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pexels image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pexels image: status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 6<<20))
}

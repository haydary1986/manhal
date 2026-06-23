// Package websearch provides AI web search for the bot via Tavily, a search API
// built for LLMs that returns a synthesized answer plus source links. The key is
// optional: when unset the feature reports itself disabled rather than failing.
package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const tavilyEndpoint = "https://api.tavily.com/search"

// Result is one web source backing an answer.
type Result struct {
	Title string
	URL   string
}

// Answer is a synthesized response with its supporting sources.
type Answer struct {
	Text    string
	Results []Result
}

// Tavily calls the Tavily search API.
type Tavily struct {
	key      string
	endpoint string
	http     *http.Client
}

// NewTavily builds a client; an empty key leaves it disabled.
func NewTavily(key string) *Tavily {
	return &Tavily{
		key:      strings.TrimSpace(key),
		endpoint: tavilyEndpoint,
		http:     &http.Client{Timeout: 25 * time.Second},
	}
}

// Enabled reports whether a key is configured.
func (t *Tavily) Enabled() bool { return t != nil && t.key != "" }

// Search returns a synthesized answer and up to maxResults sources.
func (t *Tavily) Search(ctx context.Context, query string, maxResults int) (Answer, error) {
	if !t.Enabled() {
		return Answer{}, fmt.Errorf("websearch: not configured")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	body, err := json.Marshal(map[string]any{
		"query":          query,
		"include_answer": true,
		"search_depth":   "basic",
		"max_results":    maxResults,
	})
	if err != nil {
		return Answer{}, fmt.Errorf("websearch: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(body))
	if err != nil {
		return Answer{}, fmt.Errorf("websearch: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.key)

	resp, err := t.http.Do(req)
	if err != nil {
		return Answer{}, fmt.Errorf("websearch: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Answer{}, fmt.Errorf("websearch: status %d", resp.StatusCode)
	}

	var doc struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return Answer{}, fmt.Errorf("websearch: decode: %w", err)
	}

	out := Answer{Text: strings.TrimSpace(doc.Answer)}
	for _, r := range doc.Results {
		if r.URL == "" {
			continue
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = r.URL
		}
		out.Results = append(out.Results, Result{Title: title, URL: r.URL})
	}
	return out, nil
}

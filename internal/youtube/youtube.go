// Package youtube fetches a video's caption transcript so the bot can summarize
// lectures and talks. It scrapes the public watch page for the caption track URL
// (no API key); when a video has no captions it returns ErrNoTranscript.
package youtube

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ErrNoTranscript is returned when a video exposes no caption track.
var ErrNoTranscript = errors.New("youtube: no transcript available")

// Client fetches transcripts.
type Client struct {
	http *http.Client
}

// New builds a client with sane timeouts.
func New() *Client {
	return &Client{http: &http.Client{Timeout: 25 * time.Second}}
}

var (
	idFromV       = regexp.MustCompile(`[?&]v=([A-Za-z0-9_-]{11})`)
	idFromShort   = regexp.MustCompile(`(?:youtu\.be/|/shorts/|/embed/|/live/)([A-Za-z0-9_-]{11})`)
	baseURLRegexp = regexp.MustCompile(`"baseUrl":"(https://www\.youtube\.com/api/timedtext[^"]+)"`)
	textTagRegexp = regexp.MustCompile(`(?s)<text[^>]*>(.*?)</text>`)
)

// VideoID extracts the 11-char video id from common YouTube URL shapes.
func VideoID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if m := idFromV.FindStringSubmatch(raw); m != nil {
		return m[1], true
	}
	if m := idFromShort.FindStringSubmatch(raw); m != nil {
		return m[1], true
	}
	// Bare id.
	if len(raw) == 11 && !strings.ContainsAny(raw, "/ .:") {
		return raw, true
	}
	return "", false
}

// Transcript returns the plain-text transcript for a video id.
func (c *Client) Transcript(ctx context.Context, videoID string) (string, error) {
	page, err := c.get(ctx, "https://www.youtube.com/watch?v="+videoID+"&hl=en")
	if err != nil {
		return "", err
	}
	m := baseURLRegexp.FindStringSubmatch(page)
	if m == nil {
		return "", ErrNoTranscript
	}
	// The captured URL is JSON-escaped inside the page (&, \/, &amp;).
	trackURL := m[1]
	for _, rep := range [][2]string{{"\\u0026", "&"}, {"\\/", "/"}, {"&amp;", "&"}} {
		trackURL = strings.ReplaceAll(trackURL, rep[0], rep[1])
	}
	xml, err := c.get(ctx, trackURL)
	if err != nil {
		return "", err
	}
	text := parseTimedText(xml)
	if strings.TrimSpace(text) == "" {
		return "", ErrNoTranscript
	}
	return text, nil
}

func (c *Client) get(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("youtube: build request: %w", err)
	}
	// A browser UA avoids the consent interstitial for most regions.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ar;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("youtube: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube: status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("youtube: read: %w", err)
	}
	return string(b), nil
}

// parseTimedText turns the timedtext XML into plain prose.
func parseTimedText(xml string) string {
	var b strings.Builder
	for _, m := range textTagRegexp.FindAllStringSubmatch(xml, -1) {
		seg := html.UnescapeString(m[1]) // entities are double-escaped in timedtext
		seg = html.UnescapeString(seg)
		seg = strings.ReplaceAll(seg, "\n", " ")
		seg = strings.TrimSpace(seg)
		if seg != "" {
			b.WriteString(seg)
			b.WriteByte(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

// Package embed abstracts a text-embedding provider so it can be swapped (a
// local Ollama model now, others later) without touching feature code. It powers
// semantic library search (#22) and PDF chat / RAG (#24).
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Provider turns text into vectors.
type Provider interface {
	// Embed returns one vector per input text (same order).
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Name identifies the provider (for logging/diagnostics).
	Name() string
}

// Ollama is a Provider backed by a local Ollama server (/api/embed).
type Ollama struct {
	http  *http.Client
	url   string // base URL, e.g. http://localhost:11434
	model string // e.g. bge-m3, multilingual-e5-base
}

// NewOllama builds an Ollama embedding client.
func NewOllama(url, model string) *Ollama {
	return &Ollama{
		http:  &http.Client{Timeout: 60 * time.Second},
		url:   url,
		model: model,
	}
}

// Name implements Provider.
func (o *Ollama) Name() string { return "ollama:" + o.model }

// Embed implements Provider via Ollama's batch /api/embed endpoint.
func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: o.model, Input: texts})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d: %s", resp.StatusCode, string(data))
	}

	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed: got %d vectors for %d inputs", len(out.Embeddings), len(texts))
	}
	return out.Embeddings, nil
}

// EmbedOne is a convenience for embedding a single text.
func EmbedOne(ctx context.Context, p Provider, text string) ([]float32, error) {
	vecs, err := p.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embed: empty result")
	}
	return vecs[0], nil
}

// Cosine returns the cosine similarity of two equal-length vectors in [-1, 1].
// Mismatched or zero vectors yield 0.
func Cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

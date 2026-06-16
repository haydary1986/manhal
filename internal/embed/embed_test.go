package embed

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCosine(t *testing.T) {
	if got := Cosine([]float32{1, 0}, []float32{1, 0}); math.Abs(float64(got)-1) > 1e-6 {
		t.Errorf("identical = %v, want 1", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(float64(got)) > 1e-6 {
		t.Errorf("orthogonal = %v, want 0", got)
	}
	if got := Cosine([]float32{1, 1}, []float32{2, 2}); math.Abs(float64(got)-1) > 1e-6 {
		t.Errorf("parallel = %v, want 1", got)
	}
	// Mismatched / zero vectors are safe.
	if Cosine([]float32{1}, []float32{1, 2}) != 0 {
		t.Error("mismatched length should be 0")
	}
	if Cosine([]float32{0, 0}, []float32{1, 1}) != 0 {
		t.Error("zero vector should be 0")
	}
}

func TestOllama_Embed(t *testing.T) {
	var gotBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = rw.Write([]byte(`{"embeddings":[[0.1,0.2,0.3],[0.4,0.5,0.6]]}`))
	}))
	defer srv.Close()

	o := NewOllama(srv.URL, "bge-m3")
	vecs, err := o.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if gotBody.Model != "bge-m3" || len(gotBody.Input) != 2 {
		t.Errorf("request body = %+v", gotBody)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 || vecs[1][2] != 0.6 {
		t.Errorf("vectors = %+v", vecs)
	}
}

func TestOllama_EmbedOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte(`{"embeddings":[[1,2,3]]}`))
	}))
	defer srv.Close()

	v, err := EmbedOne(context.Background(), NewOllama(srv.URL, "m"), "hello")
	if err != nil || len(v) != 3 {
		t.Fatalf("EmbedOne = (%v, %v)", v, err)
	}
}

func TestOllama_EmbedCountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte(`{"embeddings":[[1,2,3]]}`)) // one vector for two inputs
	}))
	defer srv.Close()
	if _, err := NewOllama(srv.URL, "m").Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Error("expected an error on count mismatch")
	}
}

func TestOllama_EmptyNoCall(t *testing.T) {
	o := NewOllama("http://invalid.invalid", "m")
	if got, err := o.Embed(context.Background(), nil); err != nil || got != nil {
		t.Errorf("empty input = (%v, %v), want (nil, nil)", got, err)
	}
}

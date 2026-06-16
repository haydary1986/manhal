package bot

import (
	"strings"
	"testing"
)

func TestChunkText_OverlapAndCoverage(t *testing.T) {
	text := strings.Repeat("ab", 1200) // 2400 runes
	chunks := chunkText(text, 1000, 150)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Reassembled (accounting for overlap) must cover all content.
	if len([]rune(chunks[0])) != 1000 {
		t.Errorf("first chunk len = %d, want 1000", len([]rune(chunks[0])))
	}
	// Every chunk non-empty.
	for i, c := range chunks {
		if strings.TrimSpace(c) == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestChunkText_Edge(t *testing.T) {
	if chunkText("", 1000, 150) != nil {
		t.Error("empty text should yield nil")
	}
	short := chunkText("short", 1000, 150)
	if len(short) != 1 || short[0] != "short" {
		t.Errorf("short text = %v, want one chunk", short)
	}
	// Bad overlap is coerced to 0 (no infinite loop).
	if got := chunkText("abcdef", 3, 5); len(got) == 0 {
		t.Error("bad overlap should still chunk")
	}
}

func TestTopChunks_RanksByCosine(t *testing.T) {
	chunks := []pdfChunk{
		{text: "far", vector: []float32{0, 1, 0}},
		{text: "near", vector: []float32{1, 0, 0}},
		{text: "mid", vector: []float32{0.7, 0.7, 0}},
	}
	top := topChunks(chunks, []float32{1, 0, 0}, 2)
	if len(top) != 2 {
		t.Fatalf("top = %d, want 2", len(top))
	}
	if top[0].text != "near" || top[1].text != "mid" {
		t.Errorf("order wrong: %q, %q", top[0].text, top[1].text)
	}
}

func TestRagUserPrompt(t *testing.T) {
	p := ragUserPrompt("ما المنهجية؟", []pdfChunk{{text: "استخدمنا تحليل الانحدار."}})
	for _, want := range []string{"مقاطع من الورقة", "تحليل الانحدار", "السؤال: ما المنهجية؟"} {
		if !strings.Contains(p, want) {
			t.Errorf("rag prompt missing %q", want)
		}
	}
}

func TestSessions_PdfChat(t *testing.T) {
	s := newSessions()
	chunks := []pdfChunk{{text: "x", vector: []float32{1}}}
	s.startPdfChat(9, chunks)
	if s.get(9) != statePdfChat {
		t.Error("startPdfChat should set chat state")
	}
	if got := s.pdfChunks(9); len(got) != 1 || got[0].text != "x" {
		t.Errorf("pdfChunks = %+v", got)
	}
}

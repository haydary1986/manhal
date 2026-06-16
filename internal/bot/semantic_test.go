package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/domain"
)

func TestLibraryEmbedText(t *testing.T) {
	it := domain.LibraryItem{
		Work: cite.Work{Title: "Deep Learning", ContainerTitle: "Nature"},
		Tags: []string{"ai", "nlp"},
	}
	got := libraryEmbedText(it)
	for _, want := range []string{"Deep Learning", "ai nlp", "Nature"} {
		if !strings.Contains(got, want) {
			t.Errorf("embed text missing %q: %q", want, got)
		}
	}
}

func TestRankBySimilarity(t *testing.T) {
	items := []domain.LibraryItem{
		{ID: "a", Work: cite.Work{Title: "A"}, Vector: []float32{1, 0, 0}},
		{ID: "b", Work: cite.Work{Title: "B"}, Vector: []float32{0, 1, 0}},
		{ID: "c", Work: cite.Work{Title: "C"}, Vector: []float32{0.9, 0.1, 0}},
		{ID: "novec", Work: cite.Work{Title: "No Vector"}}, // skipped
	}
	ranked := rankBySimilarity(items, []float32{1, 0, 0}, 5)
	if len(ranked) != 3 {
		t.Fatalf("ranked = %d, want 3 (no-vector skipped)", len(ranked))
	}
	if ranked[0].item.ID != "a" || ranked[1].item.ID != "c" {
		t.Errorf("order wrong: %s, %s", ranked[0].item.ID, ranked[1].item.ID)
	}
	if ranked[0].score <= ranked[1].score {
		t.Error("scores should be descending")
	}
}

func TestRankBySimilarity_RespectsLimit(t *testing.T) {
	items := []domain.LibraryItem{
		{ID: "a", Vector: []float32{1, 0}},
		{ID: "b", Vector: []float32{0, 1}},
		{ID: "c", Vector: []float32{1, 1}},
	}
	if got := rankBySimilarity(items, []float32{1, 0}, 2); len(got) != 2 {
		t.Errorf("limit not respected: %d", len(got))
	}
}

func TestSemanticResultScreen(t *testing.T) {
	ranked := []scoredItem{
		{item: domain.LibraryItem{Work: cite.Work{Title: "Medical AI", Year: 2021, DOI: "10.1/m"}}, score: 0.87},
	}
	scr := semanticResultScreen("ai in medicine", ranked)
	for _, want := range []string{"أقرب المراجع", "Medical AI", "87%", "10.1/m"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("semantic result missing %q:\n%s", want, scr.Text)
		}
	}
}

func TestHasVectors(t *testing.T) {
	if hasVectors([]domain.LibraryItem{{ID: "a"}}) {
		t.Error("no vectors should report false")
	}
	if !hasVectors([]domain.LibraryItem{{ID: "a", Vector: []float32{1}}}) {
		t.Error("a vector should report true")
	}
}

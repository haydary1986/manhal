package domain

import (
	"time"

	"github.com/erticaz/manhal/internal/cite"
)

// LibraryItem is one saved reference in a user's personal library (#21–23).
type LibraryItem struct {
	ID      string    // stable id (derived from the DOI/title)
	Work    cite.Work // bibliographic data, ready for BibTeX/RIS export
	Tags    []string  // auto-derived keywords
	Vector  []float32 // embedding for semantic search (#22); empty when disabled
	SavedAt time.Time
}

package domain

import "time"

// CitationWatch tracks a researcher's total citation count so the alerts engine
// can notify the user when it increases (#5). The user typically watches their
// own author profile.
type CitationWatch struct {
	ID          string
	UserID      int64
	AuthorName  string
	LastCitedBy int
	CreatedAt   time.Time
}

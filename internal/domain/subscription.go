package domain

import "time"

// Subscription is a user's standing interest in a research topic. The alerts
// scheduler polls each topic and pushes newly published papers; SeenDOIs is the
// dedup memory so a paper is announced only once (#4 / #7).
type Subscription struct {
	ID        string
	UserID    int64
	Topic     string
	SeenDOIs  []string
	CreatedAt time.Time
}

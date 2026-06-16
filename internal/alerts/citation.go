package alerts

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/scholar"
)

// AuthorSearcher looks up a researcher's current metrics (implemented by
// scholar.OpenAlex).
type AuthorSearcher interface {
	SearchAuthors(ctx context.Context, name string, limit int) ([]scholar.Author, error)
}

// WatchStore is the persistence the citation watcher needs.
type WatchStore interface {
	ListAllCitationWatches(ctx context.Context) ([]domain.CitationWatch, error)
	UpdateCitationCount(ctx context.Context, id string, count int) error
}

// CitationWatcher notifies users when a watched researcher's total citation
// count increases (#5).
type CitationWatcher struct {
	store    WatchStore
	authors  AuthorSearcher
	notify   Notifier
	interval time.Duration
}

// NewCitationWatcher builds a CitationWatcher. A non-positive interval disables it.
func NewCitationWatcher(store WatchStore, authors AuthorSearcher, notify Notifier, interval time.Duration) *CitationWatcher {
	return &CitationWatcher{store: store, authors: authors, notify: notify, interval: interval}
}

// Run polls on the configured interval until ctx is cancelled.
func (c *CitationWatcher) Run(ctx context.Context) {
	if c.interval <= 0 || c.store == nil || c.authors == nil || c.notify == nil {
		log.Println("citation watcher disabled")
		return
	}
	log.Printf("citation watcher running every %s", c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.RunOnce(ctx)
		}
	}
}

// RunOnce checks every watch once.
func (c *CitationWatcher) RunOnce(ctx context.Context) {
	watches, err := c.store.ListAllCitationWatches(ctx)
	if err != nil {
		log.Printf("citation watcher: list: %v", err)
		return
	}
	for _, w := range watches {
		authors, err := c.authors.SearchAuthors(ctx, w.AuthorName, 1)
		if err != nil || len(authors) == 0 {
			continue
		}
		current := authors[0].CitedBy
		if current == w.LastCitedBy {
			continue
		}
		if current > w.LastCitedBy {
			delta := current - w.LastCitedBy
			if err := c.notify.Notify(w.UserID, citationText(w.AuthorName, delta, current)); err != nil {
				log.Printf("citation watcher: notify %d: %v", w.UserID, err)
				continue // keep old count so we retry next pass
			}
		}
		// Update on any change (increase delivered, or a downward data correction).
		if err := c.store.UpdateCitationCount(ctx, w.ID, current); err != nil {
			log.Printf("citation watcher: update %s: %v", w.ID, err)
		}
	}
}

// citationText renders a new-citation alert.
func citationText(name string, delta, total int) string {
	return "🔔 استشهادات جديدة!\nحصل «" + name + "» على +" + strconv.Itoa(delta) +
		" استشهاد (الإجمالي " + strconv.Itoa(total) + ")."
}

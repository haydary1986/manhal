// Package alerts is the proactive notifications engine (Phase 1 #4/#7). A
// background scheduler periodically polls each topic subscription, finds papers
// the user hasn't been told about yet, and pushes a short digest. The "what's
// new" decision is a pure function (Fresh) so it is fully testable.
package alerts

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/scholar"
)

// maxSeen bounds a subscription's dedup memory so it cannot grow unbounded.
const maxSeen = 300

// resultsPerTopic is how many papers each poll fetches per topic.
const resultsPerTopic = 10

// Fresh returns the results whose DOI is not already in seen, plus the updated
// (capped) seen set. Results without a DOI are ignored (no stable identity).
func Fresh(results []scholar.SearchResult, seen []string) (fresh []scholar.SearchResult, updatedSeen []string) {
	seenSet := make(map[string]bool, len(seen))
	for _, d := range seen {
		seenSet[d] = true
	}
	updatedSeen = append([]string(nil), seen...)
	for _, r := range results {
		if r.DOI == "" || seenSet[r.DOI] {
			continue
		}
		seenSet[r.DOI] = true
		fresh = append(fresh, r)
		updatedSeen = append(updatedSeen, r.DOI)
	}
	if len(updatedSeen) > maxSeen {
		updatedSeen = updatedSeen[len(updatedSeen)-maxSeen:]
	}
	return fresh, updatedSeen
}

// Searcher fetches papers for a topic (implemented by scholar.OpenAlex).
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]scholar.SearchResult, error)
}

// Notifier pushes a message to a user (implemented by the bot).
type Notifier interface {
	Notify(userID int64, text string) error
}

// SubStore is the persistence the scheduler needs.
type SubStore interface {
	ListAllSubscriptions(ctx context.Context) ([]domain.Subscription, error)
	UpdateSubscriptionSeen(ctx context.Context, id string, seen []string) error
}

// Scheduler periodically delivers new-paper alerts.
type Scheduler struct {
	store    SubStore
	search   Searcher
	notify   Notifier
	interval time.Duration
}

// NewScheduler builds a Scheduler. A non-positive interval disables it.
func NewScheduler(store SubStore, search Searcher, notify Notifier, interval time.Duration) *Scheduler {
	return &Scheduler{store: store, search: search, notify: notify, interval: interval}
}

// Run polls on the configured interval until the context is cancelled. It does
// nothing when the interval is non-positive or a dependency is missing.
func (s *Scheduler) Run(ctx context.Context) {
	if s.interval <= 0 || s.store == nil || s.search == nil || s.notify == nil {
		log.Println("alerts scheduler disabled")
		return
	}
	log.Printf("alerts scheduler running every %s", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce performs a single polling pass over all subscriptions.
func (s *Scheduler) RunOnce(ctx context.Context) {
	subs, err := s.store.ListAllSubscriptions(ctx)
	if err != nil {
		log.Printf("alerts: list subscriptions: %v", err)
		return
	}
	for _, sub := range subs {
		results, err := s.search.Search(ctx, sub.Topic, resultsPerTopic)
		if err != nil {
			log.Printf("alerts: search %q: %v", sub.Topic, err)
			continue
		}
		fresh, newSeen := Fresh(results, sub.SeenDOIs)
		if len(fresh) == 0 {
			continue
		}
		if err := s.notify.Notify(sub.UserID, Digest(sub.Topic, fresh)); err != nil {
			log.Printf("alerts: notify %d: %v", sub.UserID, err)
			continue // keep seen unchanged so we retry next time
		}
		if err := s.store.UpdateSubscriptionSeen(ctx, sub.ID, newSeen); err != nil {
			log.Printf("alerts: update seen %s: %v", sub.ID, err)
		}
	}
}

// Digest renders a short Arabic alert for newly found papers.
func Digest(topic string, fresh []scholar.SearchResult) string {
	var b strings.Builder
	b.WriteString("🔔 أوراق جديدة في «" + topic + "»:\n")
	for i, r := range fresh {
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + r.Title)
		if r.Year > 0 {
			b.WriteString(" (" + strconv.Itoa(r.Year) + ")")
		}
		if r.DOI != "" {
			b.WriteString("\n   https://doi.org/" + r.DOI)
		}
	}
	return b.String()
}

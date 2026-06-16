package alerts

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/domain"
)

// digestWindowDays is how far ahead the weekly digest looks for deadlines.
const digestWindowDays = 14

// digestTopicLimit / digestDeadlineLimit cap each section's length.
const (
	digestTopicLimit    = 3
	digestDeadlineLimit = 3
)

// DigestStore is the persistence the weekly digest needs.
type DigestStore interface {
	ListUsers(ctx context.Context) ([]domain.User, error)
	ListSubscriptions(ctx context.Context, userID int64) ([]domain.Subscription, error)
	ListCitationWatches(ctx context.Context, userID int64) ([]domain.CitationWatch, error)
	WasReminded(ctx context.Context, userID int64, key string) (bool, error)
	MarkReminded(ctx context.Context, userID int64, key string) error
}

// WeeklyDigest composes and pushes a once-a-week personalized roundup that
// blends a user's followed topics, upcoming deadlines in their discipline, and
// their tracked citation counts (P5 — the recurring-engagement driver).
type WeeklyDigest struct {
	store    DigestStore
	search   Searcher
	announce AnnounceLister
	notify   Notifier
	weekday  time.Weekday
	interval time.Duration
}

// NewWeeklyDigest builds a WeeklyDigest that fires on the given weekday. A
// non-positive interval disables it.
func NewWeeklyDigest(store DigestStore, search Searcher, ann AnnounceLister, notify Notifier, weekday time.Weekday, interval time.Duration) *WeeklyDigest {
	return &WeeklyDigest{store: store, search: search, announce: ann, notify: notify, weekday: weekday, interval: interval}
}

// Run checks on the configured interval until ctx is cancelled.
func (d *WeeklyDigest) Run(ctx context.Context) {
	if d.interval <= 0 || d.store == nil || d.search == nil || d.announce == nil || d.notify == nil {
		log.Println("weekly digest disabled")
		return
	}
	log.Printf("weekly digest running (sends on weekday %d)", int(d.weekday))
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.RunOnce(ctx, time.Now())
		}
	}
}

// RunOnce sends the digest to users who haven't received this ISO-week's edition,
// but only on the configured weekday.
func (d *WeeklyDigest) RunOnce(ctx context.Context, now time.Time) {
	if now.Weekday() != d.weekday {
		return
	}
	key := digestKey(now)
	users, err := d.store.ListUsers(ctx)
	if err != nil {
		log.Printf("weekly digest: list users: %v", err)
		return
	}
	for _, u := range users {
		sent, err := d.store.WasReminded(ctx, u.TelegramID, key)
		if err != nil || sent {
			continue
		}
		body := d.compose(ctx, u, now)
		if body == "" {
			continue // nothing relevant this week; recompute is cheap, so don't mark
		}
		if err := d.notify.Notify(u.TelegramID, body); err != nil {
			log.Printf("weekly digest: notify %d: %v", u.TelegramID, err)
			continue
		}
		if err := d.store.MarkReminded(ctx, u.TelegramID, key); err != nil {
			log.Printf("weekly digest: mark %d: %v", u.TelegramID, err)
		}
	}
}

// compose builds the digest body for one user (empty when nothing is relevant).
func (d *WeeklyDigest) compose(ctx context.Context, u domain.User, now time.Time) string {
	topics := d.topicLines(ctx, u)
	deadlines := d.deadlineLines(now, u)
	citations := d.citationLines(ctx, u)

	if len(topics)+len(deadlines)+len(citations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("📬 موجزك الأسبوعي")
	if name := strings.TrimSpace(u.Name); name != "" {
		b.WriteString("، " + name)
	}
	b.WriteString("\n")
	writeSection(&b, "🔔 متابعاتك", topics)
	writeSection(&b, "🗓️ مواعيد قريبة", deadlines)
	writeSection(&b, "📈 استشهاداتك", citations)
	b.WriteString("\nℹ️ يصلك أسبوعياً. عدّل متابعاتك من «🔔 متابعاتي».")
	return b.String()
}

func (d *WeeklyDigest) topicLines(ctx context.Context, u domain.User) []string {
	subs, _ := d.store.ListSubscriptions(ctx, u.TelegramID)
	var lines []string
	for _, s := range subs {
		if len(lines) == digestTopicLimit {
			break
		}
		line := "• " + s.Topic
		if results, err := d.search.Search(ctx, s.Topic, 1); err == nil && len(results) > 0 {
			line += " — " + results[0].Title
		}
		lines = append(lines, line)
	}
	return lines
}

func (d *WeeklyDigest) deadlineLines(now time.Time, u domain.User) []string {
	items := d.announce.List(now, announce.Filter{Discipline: u.Field})
	var lines []string
	for _, it := range items {
		days, ok := it.DaysLeft(now)
		if !ok || days > digestWindowDays {
			continue
		}
		lines = append(lines, "• "+it.Title+" — متبقّي "+strconv.Itoa(days)+" يوم")
		if len(lines) == digestDeadlineLimit {
			break
		}
	}
	return lines
}

func (d *WeeklyDigest) citationLines(ctx context.Context, u domain.User) []string {
	watches, _ := d.store.ListCitationWatches(ctx, u.TelegramID)
	var lines []string
	for _, w := range watches {
		lines = append(lines, "• "+w.AuthorName+": "+strconv.Itoa(w.LastCitedBy)+" استشهاد")
	}
	return lines
}

func writeSection(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	b.WriteString("\n" + title + ":\n" + strings.Join(lines, "\n") + "\n")
}

// digestKey is the per-ISO-week dedup key.
func digestKey(now time.Time) string {
	year, week := now.ISOWeek()
	return "digest:" + strconv.Itoa(year) + "-W" + strconv.Itoa(week)
}

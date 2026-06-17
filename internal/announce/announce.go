// Package announce is the core logic for the academic announcements feed
// (conferences, CFPs, grants, fellowships, jobs). Content is admin-curated and
// loaded from data/announcements.yaml; this package filters, sorts, and reports
// deadline status. It holds no Telegram or HTTP concerns.
package announce

import (
	"strings"
	"time"
)

// Kind classifies an announcement.
type Kind string

const (
	KindConference Kind = "conference"
	KindCFP        Kind = "cfp"
	KindGrant      Kind = "grant"
	KindFellowship Kind = "fellowship"
	KindJob        Kind = "job"
)

// Announcement is one curated item in the feed.
type Announcement struct {
	ID          string     `yaml:"id"`
	Kind        Kind       `yaml:"kind"`
	Title       string     `yaml:"title"`
	Body        string     `yaml:"body"`
	Disciplines []string   `yaml:"disciplines"`          // tags; empty => general (everyone)
	Deadline    *time.Time `yaml:"deadline"`             // optional
	Link        string     `yaml:"link"`                 // optional URL button
	Image       string     `yaml:"image,omitempty"`      // optional image URL
	PublishAt   *time.Time `yaml:"publish_at,omitempty"` // optional schedule; hidden until then
	PostedAt    time.Time  `yaml:"posted_at"`
}

// HasDeadline reports whether the item carries a deadline.
func (a Announcement) HasDeadline() bool { return a.Deadline != nil }

// Visible reports whether a scheduled item is due to appear yet. An item with
// no PublishAt is always visible.
func (a Announcement) Visible(now time.Time) bool {
	return a.PublishAt == nil || !now.Before(*a.PublishAt)
}

// Expired reports whether the deadline has passed.
func (a Announcement) Expired(now time.Time) bool {
	return a.Deadline != nil && now.After(*a.Deadline)
}

// DaysLeft returns the whole days remaining until the deadline (clamped at 0)
// and whether a deadline exists.
func (a Announcement) DaysLeft(now time.Time) (int, bool) {
	if a.Deadline == nil {
		return 0, false
	}
	diff := a.Deadline.Sub(now)
	if diff < 0 {
		return 0, true
	}
	return int(diff.Hours()) / 24, true
}

// MatchesDiscipline reports whether the item is relevant to a user's field.
// An item with no discipline tags is general and matches everyone; an empty
// field (user hasn't chosen one) matches everything.
func (a Announcement) MatchesDiscipline(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" || len(a.Disciplines) == 0 {
		return true
	}
	for _, d := range a.Disciplines {
		if strings.EqualFold(strings.TrimSpace(d), field) {
			return true
		}
	}
	return false
}

// Filter narrows the feed. A zero Filter returns all active items.
type Filter struct {
	Kinds          []Kind // empty => all kinds
	Discipline     string // "" => no discipline restriction
	IncludeExpired bool   // false => hide items past their deadline
}

func (f Filter) matchesKind(k Kind) bool {
	if len(f.Kinds) == 0 {
		return true
	}
	for _, want := range f.Kinds {
		if want == k {
			return true
		}
	}
	return false
}

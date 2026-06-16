package alerts

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/domain"
)

// AnnounceLister provides the active announcements feed (implemented by
// announce.Repo).
type AnnounceLister interface {
	List(now time.Time, f announce.Filter) []announce.Announcement
}

// UserLister enumerates registered users (implemented by the store).
type UserLister interface {
	ListUsers(ctx context.Context) ([]domain.User, error)
}

// ReminderLog deduplicates reminders so each (user, announcement) fires once.
type ReminderLog interface {
	WasReminded(ctx context.Context, userID int64, key string) (bool, error)
	MarkReminded(ctx context.Context, userID int64, key string) error
}

// DeadlineReminder proactively notifies users about announcements whose deadline
// is approaching, filtered by each user's discipline (#3).
type DeadlineReminder struct {
	announce   AnnounceLister
	users      UserLister
	log        ReminderLog
	notify     Notifier
	windowDays int
	interval   time.Duration
}

// NewDeadlineReminder builds a DeadlineReminder. windowDays is how many days
// before a deadline to start reminding; a non-positive interval disables it.
func NewDeadlineReminder(a AnnounceLister, u UserLister, l ReminderLog, n Notifier, windowDays int, interval time.Duration) *DeadlineReminder {
	return &DeadlineReminder{announce: a, users: u, log: l, notify: n, windowDays: windowDays, interval: interval}
}

// Run checks deadlines on the configured interval until ctx is cancelled.
func (d *DeadlineReminder) Run(ctx context.Context) {
	if d.interval <= 0 || d.announce == nil || d.users == nil || d.log == nil || d.notify == nil {
		log.Println("deadline reminder disabled")
		return
	}
	log.Printf("deadline reminder running every %s (window %d days)", d.interval, d.windowDays)
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

// RunOnce performs one deadline-reminder pass.
func (d *DeadlineReminder) RunOnce(ctx context.Context, now time.Time) {
	items := d.announce.List(now, announce.Filter{}) // active, not expired
	users, err := d.users.ListUsers(ctx)
	if err != nil {
		log.Printf("deadline reminder: list users: %v", err)
		return
	}

	for _, item := range items {
		days, ok := item.DaysLeft(now)
		if !ok || days > d.windowDays {
			continue // no deadline, or still far away
		}
		for _, u := range users {
			if !item.MatchesDiscipline(u.Field) {
				continue
			}
			reminded, err := d.log.WasReminded(ctx, u.TelegramID, item.ID)
			if err != nil || reminded {
				continue
			}
			if err := d.notify.Notify(u.TelegramID, deadlineText(item, days)); err != nil {
				log.Printf("deadline reminder: notify %d: %v", u.TelegramID, err)
				continue // don't mark; retry next pass
			}
			if err := d.log.MarkReminded(ctx, u.TelegramID, item.ID); err != nil {
				log.Printf("deadline reminder: mark %d/%s: %v", u.TelegramID, item.ID, err)
			}
		}
	}
}

// deadlineText renders a reminder for an approaching deadline.
func deadlineText(item announce.Announcement, days int) string {
	when := ""
	if item.Deadline != nil {
		when = " (" + item.Deadline.Format("2006-01-02") + ")"
	}
	left := "متبقّي " + strconv.Itoa(days) + " يوم"
	if days == 0 {
		left = "ينتهي اليوم ⚠️"
	}
	msg := "⏰ تذكير بموعد قريب:\n«" + item.Title + "»\n🗓️ " + left + when
	if item.Link != "" {
		msg += "\n🔗 " + item.Link
	}
	return msg
}

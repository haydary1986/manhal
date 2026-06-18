package alerts

import (
	"context"
	"log"
	"strconv"
	"time"
)

// RenewalReminder notifies premium users a few days before their subscription
// expires so they can renew before losing access. Each (user, expiry-date) is
// reminded once via the dedup log, so a renewal (new expiry) re-arms the nudge.
type RenewalReminder struct {
	users      UserLister
	log        ReminderLog
	notify     Notifier
	windowDays int
	interval   time.Duration
}

// NewRenewalReminder builds the reminder; a non-positive interval disables it.
// windowDays is how many days before expiry to start nudging.
func NewRenewalReminder(u UserLister, l ReminderLog, n Notifier, windowDays int, interval time.Duration) *RenewalReminder {
	return &RenewalReminder{users: u, log: l, notify: n, windowDays: windowDays, interval: interval}
}

// Run checks for upcoming expiries on the configured interval until ctx ends.
func (d *RenewalReminder) Run(ctx context.Context) {
	if d.interval <= 0 || d.users == nil || d.log == nil || d.notify == nil {
		log.Println("renewal reminder disabled")
		return
	}
	if d.windowDays <= 0 {
		d.windowDays = 3
	}
	log.Printf("renewal reminder running every %s (window %d days)", d.interval, d.windowDays)
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

// RunOnce performs one renewal-reminder pass.
func (d *RenewalReminder) RunOnce(ctx context.Context, now time.Time) {
	users, err := d.users.ListUsers(ctx)
	if err != nil {
		log.Printf("renewal reminder: list users: %v", err)
		return
	}
	for i := range users {
		u := users[i]
		days, ok := u.PremiumDaysLeft(now)
		if !ok || days < 1 || days > d.windowDays {
			continue // free, permanent, expired, or still far away
		}
		key := "renew|" + u.PremiumUntil.Format("2006-01-02")
		if reminded, err := d.log.WasReminded(ctx, u.TelegramID, key); err != nil || reminded {
			continue
		}
		if err := d.notify.Notify(u.TelegramID, renewalText(days, *u.PremiumUntil)); err != nil {
			log.Printf("renewal reminder: notify %d: %v", u.TelegramID, err)
			continue // don't mark; retry next pass
		}
		if err := d.log.MarkReminded(ctx, u.TelegramID, key); err != nil {
			log.Printf("renewal reminder: mark %d: %v", u.TelegramID, err)
		}
	}
}

func renewalText(days int, until time.Time) string {
	left := "بعد " + strconv.Itoa(days) + " أيام"
	if days == 1 {
		left = "غداً"
	}
	return "🔔 اشتراكك المميّز في منهل ينتهي " + left + " (" + until.Format("2006-01-02") + ").\n" +
		"جدّده عبر زر 💎 الاشتراك لتستمر كل المزايا دون انقطاع 🌟"
}

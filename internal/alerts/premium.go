package alerts

import (
	"context"
	"log"
	"time"

	"github.com/erticaz/manhal/internal/domain"
)

// UserSaver persists user changes (implemented by the store).
type UserSaver interface {
	SaveUser(ctx context.Context, u *domain.User) error
}

// PremiumExpiry periodically downgrades users whose time-limited premium grant
// has lapsed back to the free tier, keeping tier data accurate and notifying
// them so they can renew. Permanent grants (no expiry) are never touched.
type PremiumExpiry struct {
	users    UserLister
	saver    UserSaver
	notify   Notifier
	interval time.Duration
}

// NewPremiumExpiry builds the expiry sweeper; a non-positive interval disables it.
func NewPremiumExpiry(users UserLister, saver UserSaver, notify Notifier, interval time.Duration) *PremiumExpiry {
	return &PremiumExpiry{users: users, saver: saver, notify: notify, interval: interval}
}

// Run sweeps expired grants on the configured interval until ctx is cancelled.
func (p *PremiumExpiry) Run(ctx context.Context) {
	if p.interval <= 0 || p.users == nil || p.saver == nil {
		log.Println("premium expiry sweeper disabled")
		return
	}
	log.Printf("premium expiry sweeper running every %s", p.interval)
	// Run once promptly so a restart cleans up immediately, then on the ticker.
	p.RunOnce(ctx, time.Now())
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.RunOnce(ctx, time.Now())
		}
	}
}

// RunOnce performs a single expiry pass.
func (p *PremiumExpiry) RunOnce(ctx context.Context, now time.Time) {
	users, err := p.users.ListUsers(ctx)
	if err != nil {
		log.Printf("premium expiry: list users: %v", err)
		return
	}
	for i := range users {
		u := users[i]
		if !u.IsExpired(now) {
			continue
		}
		u.RevokePremium()
		if err := p.saver.SaveUser(ctx, &u); err != nil {
			log.Printf("premium expiry: save %d: %v", u.TelegramID, err)
			continue
		}
		if p.notify != nil {
			_ = p.notify.Notify(u.TelegramID,
				"⏳ انتهت صلاحية اشتراكك المميّز في منهل.\nنشكرك على ثقتك 🌟 — يمكنك التجديد في أي وقت عبر زر 💎 الاشتراك.")
		}
		log.Printf("premium expiry: downgraded user %d to free", u.TelegramID)
	}
}

// Package store defines the persistence boundary and its implementations.
// A Postgres implementation will be added later; the in-memory implementation
// is used for development.
package store

import (
	"context"
	"errors"

	"github.com/erticaz/manhal/internal/domain"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// ErrCodeUsed is returned when redeeming an already-claimed gift code.
var ErrCodeUsed = errors.New("gift code already used")

// Store is the persistence interface.
type Store interface {
	GetUser(ctx context.Context, telegramID int64) (*domain.User, error)
	SaveUser(ctx context.Context, u *domain.User) error
	// ListUsers returns all registered users (for broadcast/reminders).
	ListUsers(ctx context.Context) ([]domain.User, error)

	// WasReminded reports whether a user was already reminded about a key
	// (e.g. an announcement id), so reminders are sent once.
	WasReminded(ctx context.Context, userID int64, key string) (bool, error)
	// MarkReminded records that a reminder was sent.
	MarkReminded(ctx context.Context, userID int64, key string) error

	// AddLibraryItem saves a reference for a user, ignoring duplicates (by ID).
	AddLibraryItem(ctx context.Context, userID int64, item domain.LibraryItem) error
	// ListLibrary returns the user's saved references, newest first.
	ListLibrary(ctx context.Context, userID int64) ([]domain.LibraryItem, error)
	// RemoveLibraryItem deletes a saved reference by ID; returns ErrNotFound if absent.
	RemoveLibraryItem(ctx context.Context, userID int64, itemID string) error

	// AddTicket stores a new support request.
	AddTicket(ctx context.Context, t domain.Ticket) error
	// ListTickets returns all support requests, open first then newest.
	ListTickets(ctx context.Context) ([]domain.Ticket, error)
	// AnswerTicket sets a reply and marks the ticket answered, returning it.
	AnswerTicket(ctx context.Context, id, reply string) (domain.Ticket, error)

	// AddSubscription stores a topic subscription for a user.
	AddSubscription(ctx context.Context, s domain.Subscription) error
	// ListSubscriptions returns a user's subscriptions, newest first.
	ListSubscriptions(ctx context.Context, userID int64) ([]domain.Subscription, error)
	// ListAllSubscriptions returns every subscription (for the alerts scheduler).
	ListAllSubscriptions(ctx context.Context) ([]domain.Subscription, error)
	// RemoveSubscription deletes a subscription by ID; ErrNotFound if absent.
	RemoveSubscription(ctx context.Context, userID int64, id string) error
	// UpdateSubscriptionSeen replaces the dedup memory of a subscription.
	UpdateSubscriptionSeen(ctx context.Context, id string, seen []string) error

	// AddGiftCode stores a new redeemable gift code.
	AddGiftCode(ctx context.Context, g domain.GiftCode) error
	// ListGiftCodes returns all gift codes, newest first.
	ListGiftCodes(ctx context.Context) ([]domain.GiftCode, error)
	// RedeemGiftCode atomically claims an unused code for a user, returning it.
	// Returns ErrNotFound if absent, ErrCodeUsed if already claimed.
	RedeemGiftCode(ctx context.Context, code string, userID int64) (domain.GiftCode, error)

	// RecordUsage increments the usage counter for (user, action). Best-effort
	// analytics: callers ignore the error so tracking never breaks a feature.
	RecordUsage(ctx context.Context, userID int64, action string) error
	// FeatureUsage returns per-action totals, most-used first.
	FeatureUsage(ctx context.Context) ([]domain.FeatureCount, error)
	// TopUsers returns the most active users by total actions, capped at limit.
	TopUsers(ctx context.Context, limit int) ([]domain.UserUsage, error)
	// UsageTotals returns the total recorded actions and the number of distinct
	// users who have used at least one feature.
	UsageTotals(ctx context.Context) (actions int, activeUsers int, err error)
	// UsageByWeekday returns action counts per weekday in Baghdad time, indexed
	// by Go's time.Weekday (Sunday=0 .. Saturday=6).
	UsageByWeekday(ctx context.Context) ([7]int, error)
	// UsageByHour returns action counts per hour-of-day (0..23) in Baghdad time.
	UsageByHour(ctx context.Context) ([24]int, error)

	// AddCitationWatch stores a citation watch for a user.
	AddCitationWatch(ctx context.Context, w domain.CitationWatch) error
	// ListCitationWatches returns a user's citation watches, newest first.
	ListCitationWatches(ctx context.Context, userID int64) ([]domain.CitationWatch, error)
	// ListAllCitationWatches returns every watch (for the alerts scheduler).
	ListAllCitationWatches(ctx context.Context) ([]domain.CitationWatch, error)
	// RemoveCitationWatch deletes a watch by ID; ErrNotFound if absent.
	RemoveCitationWatch(ctx context.Context, userID int64, id string) error
	// UpdateCitationCount records a new last-known citation count.
	UpdateCitationCount(ctx context.Context, id string, count int) error
}

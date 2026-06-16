package store

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/erticaz/manhal/internal/domain"
)

// Memory is an in-memory Store implementation for development.
type Memory struct {
	mu        sync.RWMutex
	users     map[int64]domain.User
	library   map[int64][]domain.LibraryItem
	tickets   []domain.Ticket
	subs      []domain.Subscription
	reminders map[string]bool // "userID:key" -> sent
	watches   []domain.CitationWatch
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		users:     make(map[int64]domain.User),
		library:   make(map[int64][]domain.LibraryItem),
		reminders: make(map[string]bool),
	}
}

// GetUser returns a copy of the stored user, or ErrNotFound.
func (m *Memory) GetUser(_ context.Context, telegramID int64) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[telegramID]
	if !ok {
		return nil, ErrNotFound
	}
	return &u, nil
}

// SaveUser stores a copy of the user (immutable storage semantics).
func (m *Memory) SaveUser(_ context.Context, u *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	m.users[u.TelegramID] = *u
	return nil
}

// ListUsers returns a copy of all registered users.
func (m *Memory) ListUsers(_ context.Context) ([]domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}

// WasReminded reports whether (user, key) was already reminded.
func (m *Memory) WasReminded(_ context.Context, userID int64, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reminders[reminderKey(userID, key)], nil
}

// MarkReminded records a sent reminder.
func (m *Memory) MarkReminded(_ context.Context, userID int64, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reminders[reminderKey(userID, key)] = true
	return nil
}

func reminderKey(userID int64, key string) string {
	return strconv.FormatInt(userID, 10) + ":" + key
}

// AddLibraryItem prepends an item (newest first), skipping duplicate IDs.
func (m *Memory) AddLibraryItem(_ context.Context, userID int64, item domain.LibraryItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item.SavedAt.IsZero() {
		item.SavedAt = time.Now()
	}
	for _, existing := range m.library[userID] {
		if existing.ID == item.ID {
			return nil // already saved
		}
	}
	m.library[userID] = append([]domain.LibraryItem{item}, m.library[userID]...)
	return nil
}

// ListLibrary returns a copy of the user's saved references (newest first).
func (m *Memory) ListLibrary(_ context.Context, userID int64) ([]domain.LibraryItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.library[userID]
	out := make([]domain.LibraryItem, len(items))
	copy(out, items)
	return out, nil
}

// RemoveLibraryItem deletes an item by ID.
func (m *Memory) RemoveLibraryItem(_ context.Context, userID int64, itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.library[userID]
	for i, it := range items {
		if it.ID == itemID {
			m.library[userID] = append(items[:i:i], items[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// AddTicket appends a support request (stamping CreatedAt and Open status).
func (m *Memory) AddTicket(_ context.Context, t domain.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if t.Status == "" {
		t.Status = domain.TicketOpen
	}
	m.tickets = append(m.tickets, t)
	return nil
}

// ListTickets returns a copy of all tickets, open ones first then newest.
func (m *Memory) ListTickets(_ context.Context) ([]domain.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Ticket, len(m.tickets))
	copy(out, m.tickets)
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := out[i].Status == domain.TicketOpen, out[j].Status == domain.TicketOpen
		if oi != oj {
			return oi // open tickets first
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// AnswerTicket records a reply and marks the ticket answered.
func (m *Memory) AnswerTicket(_ context.Context, id, reply string) (domain.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.tickets {
		if m.tickets[i].ID == id {
			now := time.Now()
			m.tickets[i].Reply = reply
			m.tickets[i].Status = domain.TicketAnswered
			m.tickets[i].AnsweredAt = &now
			return m.tickets[i], nil
		}
	}
	return domain.Ticket{}, ErrNotFound
}

// AddSubscription appends a subscription, ignoring duplicate IDs.
func (m *Memory) AddSubscription(_ context.Context, s domain.Subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.subs {
		if existing.ID == s.ID {
			return nil
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	m.subs = append(m.subs, s)
	return nil
}

// ListSubscriptions returns a user's subscriptions (newest first).
func (m *Memory) ListSubscriptions(_ context.Context, userID int64) ([]domain.Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.Subscription
	for i := len(m.subs) - 1; i >= 0; i-- {
		if m.subs[i].UserID == userID {
			out = append(out, m.subs[i])
		}
	}
	return out, nil
}

// ListAllSubscriptions returns a copy of every subscription.
func (m *Memory) ListAllSubscriptions(_ context.Context) ([]domain.Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Subscription, len(m.subs))
	copy(out, m.subs)
	return out, nil
}

// RemoveSubscription deletes a user's subscription by ID.
func (m *Memory) RemoveSubscription(_ context.Context, userID int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.subs {
		if s.ID == id && s.UserID == userID {
			m.subs = append(m.subs[:i:i], m.subs[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// UpdateSubscriptionSeen replaces a subscription's dedup memory.
func (m *Memory) UpdateSubscriptionSeen(_ context.Context, id string, seen []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.subs {
		if m.subs[i].ID == id {
			m.subs[i].SeenDOIs = seen
			return nil
		}
	}
	return ErrNotFound
}

// AddCitationWatch appends a watch, ignoring duplicate IDs.
func (m *Memory) AddCitationWatch(_ context.Context, w domain.CitationWatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.watches {
		if existing.ID == w.ID {
			return nil
		}
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now()
	}
	m.watches = append(m.watches, w)
	return nil
}

// ListCitationWatches returns a user's watches (newest first).
func (m *Memory) ListCitationWatches(_ context.Context, userID int64) ([]domain.CitationWatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.CitationWatch
	for i := len(m.watches) - 1; i >= 0; i-- {
		if m.watches[i].UserID == userID {
			out = append(out, m.watches[i])
		}
	}
	return out, nil
}

// ListAllCitationWatches returns a copy of every watch.
func (m *Memory) ListAllCitationWatches(_ context.Context) ([]domain.CitationWatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.CitationWatch, len(m.watches))
	copy(out, m.watches)
	return out, nil
}

// RemoveCitationWatch deletes a user's watch by ID.
func (m *Memory) RemoveCitationWatch(_ context.Context, userID int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, w := range m.watches {
		if w.ID == id && w.UserID == userID {
			m.watches = append(m.watches[:i:i], m.watches[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// UpdateCitationCount records the latest citation count for a watch.
func (m *Memory) UpdateCitationCount(_ context.Context, id string, count int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.watches {
		if m.watches[i].ID == id {
			m.watches[i].LastCitedBy = count
			return nil
		}
	}
	return ErrNotFound
}

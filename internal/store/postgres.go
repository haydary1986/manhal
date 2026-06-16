package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// schema is applied on connect; every statement is idempotent.
const schema = `
CREATE TABLE IF NOT EXISTS users (
  telegram_id   BIGINT PRIMARY KEY,
  name          TEXT NOT NULL DEFAULT '',
  field         TEXT NOT NULL DEFAULT '',
  tier          TEXT NOT NULL DEFAULT 'free',
  premium_until TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS library_items (
  user_id  BIGINT NOT NULL,
  item_id  TEXT NOT NULL,
  work     JSONB NOT NULL,
  tags     TEXT[] NOT NULL DEFAULT '{}',
  vector   JSONB NOT NULL DEFAULT '[]',
  saved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, item_id)
);
CREATE TABLE IF NOT EXISTS tickets (
  id          TEXT PRIMARY KEY,
  user_id     BIGINT NOT NULL,
  user_name   TEXT NOT NULL DEFAULT '',
  message     TEXT NOT NULL,
  reply       TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'open',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  answered_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS subscriptions (
  id         TEXT PRIMARY KEY,
  user_id    BIGINT NOT NULL,
  topic      TEXT NOT NULL,
  seen_dois  TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS subscriptions_user_idx ON subscriptions (user_id);
CREATE TABLE IF NOT EXISTS reminders (
  user_id BIGINT NOT NULL,
  key     TEXT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, key)
);
CREATE TABLE IF NOT EXISTS citation_watches (
  id            TEXT PRIMARY KEY,
  user_id       BIGINT NOT NULL,
  author_name   TEXT NOT NULL,
  last_cited_by INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS citation_watches_user_idx ON citation_watches (user_id);`

// Postgres is a Store backed by PostgreSQL (pgx/v5).
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres connects to the database and applies the schema.
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres schema: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases the connection pool.
func (p *Postgres) Close() { p.pool.Close() }

// --- users ---

func (p *Postgres) GetUser(ctx context.Context, telegramID int64) (*domain.User, error) {
	const q = `SELECT telegram_id, name, field, tier, premium_until, created_at
	           FROM users WHERE telegram_id = $1`
	var u domain.User
	var tier string
	err := p.pool.QueryRow(ctx, q, telegramID).Scan(
		&u.TelegramID, &u.Name, &u.Field, &tier, &u.PremiumUntil, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.Tier = domain.Tier(tier)
	return &u, nil
}

func (p *Postgres) SaveUser(ctx context.Context, u *domain.User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	const q = `INSERT INTO users (telegram_id, name, field, tier, premium_until, created_at)
	           VALUES ($1, $2, $3, $4, $5, $6)
	           ON CONFLICT (telegram_id) DO UPDATE
	             SET name = EXCLUDED.name, field = EXCLUDED.field,
	                 tier = EXCLUDED.tier, premium_until = EXCLUDED.premium_until`
	_, err := p.pool.Exec(ctx, q, u.TelegramID, u.Name, u.Field, string(u.Tier), u.PremiumUntil, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	return nil
}

func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
	const q = `SELECT telegram_id, name, field, tier, premium_until, created_at FROM users`
	rows, err := p.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		var tier string
		if err := rows.Scan(&u.TelegramID, &u.Name, &u.Field, &tier, &u.PremiumUntil, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Tier = domain.Tier(tier)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) WasReminded(ctx context.Context, userID int64, key string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM reminders WHERE user_id = $1 AND key = $2)`, userID, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("was reminded: %w", err)
	}
	return exists, nil
}

func (p *Postgres) MarkReminded(ctx context.Context, userID int64, key string) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO reminders (user_id, key) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, key)
	if err != nil {
		return fmt.Errorf("mark reminded: %w", err)
	}
	return nil
}

// --- library ---

func (p *Postgres) AddLibraryItem(ctx context.Context, userID int64, item domain.LibraryItem) error {
	if item.SavedAt.IsZero() {
		item.SavedAt = time.Now()
	}
	work, err := json.Marshal(item.Work)
	if err != nil {
		return fmt.Errorf("marshal work: %w", err)
	}
	tags := item.Tags
	if tags == nil {
		tags = []string{} // a nil slice would become SQL NULL (column is NOT NULL)
	}
	vec, err := json.Marshal(item.Vector) // nil marshals to "null"; coerce below
	if err != nil {
		return fmt.Errorf("marshal vector: %w", err)
	}
	if item.Vector == nil {
		vec = []byte("[]")
	}
	const q = `INSERT INTO library_items (user_id, item_id, work, tags, vector, saved_at)
	           VALUES ($1, $2, $3, $4, $5, $6)
	           ON CONFLICT (user_id, item_id) DO NOTHING`
	if _, err := p.pool.Exec(ctx, q, userID, item.ID, work, tags, vec, item.SavedAt); err != nil {
		return fmt.Errorf("add library item: %w", err)
	}
	return nil
}

func (p *Postgres) ListLibrary(ctx context.Context, userID int64) ([]domain.LibraryItem, error) {
	const q = `SELECT item_id, work, tags, vector, saved_at FROM library_items
	           WHERE user_id = $1 ORDER BY saved_at DESC`
	rows, err := p.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list library: %w", err)
	}
	defer rows.Close()

	var out []domain.LibraryItem
	for rows.Next() {
		var it domain.LibraryItem
		var work, vec []byte
		if err := rows.Scan(&it.ID, &work, &it.Tags, &vec, &it.SavedAt); err != nil {
			return nil, fmt.Errorf("scan library: %w", err)
		}
		if err := json.Unmarshal(work, &it.Work); err != nil {
			return nil, fmt.Errorf("unmarshal work: %w", err)
		}
		if len(vec) > 0 {
			_ = json.Unmarshal(vec, &it.Vector)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (p *Postgres) RemoveLibraryItem(ctx context.Context, userID int64, itemID string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM library_items WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	if err != nil {
		return fmt.Errorf("remove library item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- tickets ---

func (p *Postgres) AddTicket(ctx context.Context, t domain.Ticket) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if t.Status == "" {
		t.Status = domain.TicketOpen
	}
	const q = `INSERT INTO tickets (id, user_id, user_name, message, reply, status, created_at, answered_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := p.pool.Exec(ctx, q, t.ID, t.UserID, t.UserName, t.Message, t.Reply, string(t.Status), t.CreatedAt, t.AnsweredAt)
	if err != nil {
		return fmt.Errorf("add ticket: %w", err)
	}
	return nil
}

func (p *Postgres) ListTickets(ctx context.Context) ([]domain.Ticket, error) {
	const q = `SELECT id, user_id, user_name, message, reply, status, created_at, answered_at
	           FROM tickets ORDER BY (status = 'open') DESC, created_at DESC`
	rows, err := p.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	var out []domain.Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (p *Postgres) AnswerTicket(ctx context.Context, id, reply string) (domain.Ticket, error) {
	const q = `UPDATE tickets SET reply = $2, status = 'answered', answered_at = now()
	           WHERE id = $1
	           RETURNING id, user_id, user_name, message, reply, status, created_at, answered_at`
	t, err := scanTicket(p.pool.QueryRow(ctx, q, id, reply))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ticket{}, ErrNotFound
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("answer ticket: %w", err)
	}
	return t, nil
}

// --- subscriptions ---

func (p *Postgres) AddSubscription(ctx context.Context, s domain.Subscription) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	seen := s.SeenDOIs
	if seen == nil {
		seen = []string{}
	}
	const q = `INSERT INTO subscriptions (id, user_id, topic, seen_dois, created_at)
	           VALUES ($1, $2, $3, $4, $5)
	           ON CONFLICT (id) DO NOTHING`
	if _, err := p.pool.Exec(ctx, q, s.ID, s.UserID, s.Topic, seen, s.CreatedAt); err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	return nil
}

func (p *Postgres) ListSubscriptions(ctx context.Context, userID int64) ([]domain.Subscription, error) {
	const q = `SELECT id, user_id, topic, seen_dois, created_at FROM subscriptions
	           WHERE user_id = $1 ORDER BY created_at DESC`
	return p.querySubs(ctx, q, userID)
}

func (p *Postgres) ListAllSubscriptions(ctx context.Context) ([]domain.Subscription, error) {
	const q = `SELECT id, user_id, topic, seen_dois, created_at FROM subscriptions ORDER BY created_at`
	return p.querySubs(ctx, q)
}

func (p *Postgres) querySubs(ctx context.Context, q string, args ...any) ([]domain.Subscription, error) {
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()
	var out []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Topic, &s.SeenDOIs, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *Postgres) RemoveSubscription(ctx context.Context, userID int64, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("remove subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UpdateSubscriptionSeen(ctx context.Context, id string, seen []string) error {
	if seen == nil {
		seen = []string{}
	}
	tag, err := p.pool.Exec(ctx, `UPDATE subscriptions SET seen_dois = $2 WHERE id = $1`, id, seen)
	if err != nil {
		return fmt.Errorf("update subscription seen: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- citation watches ---

func (p *Postgres) AddCitationWatch(ctx context.Context, w domain.CitationWatch) error {
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now()
	}
	const q = `INSERT INTO citation_watches (id, user_id, author_name, last_cited_by, created_at)
	           VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`
	if _, err := p.pool.Exec(ctx, q, w.ID, w.UserID, w.AuthorName, w.LastCitedBy, w.CreatedAt); err != nil {
		return fmt.Errorf("add citation watch: %w", err)
	}
	return nil
}

func (p *Postgres) ListCitationWatches(ctx context.Context, userID int64) ([]domain.CitationWatch, error) {
	const q = `SELECT id, user_id, author_name, last_cited_by, created_at FROM citation_watches
	           WHERE user_id = $1 ORDER BY created_at DESC`
	return p.queryWatches(ctx, q, userID)
}

func (p *Postgres) ListAllCitationWatches(ctx context.Context) ([]domain.CitationWatch, error) {
	const q = `SELECT id, user_id, author_name, last_cited_by, created_at FROM citation_watches ORDER BY created_at`
	return p.queryWatches(ctx, q)
}

func (p *Postgres) queryWatches(ctx context.Context, q string, args ...any) ([]domain.CitationWatch, error) {
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list citation watches: %w", err)
	}
	defer rows.Close()
	var out []domain.CitationWatch
	for rows.Next() {
		var w domain.CitationWatch
		if err := rows.Scan(&w.ID, &w.UserID, &w.AuthorName, &w.LastCitedBy, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan citation watch: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (p *Postgres) RemoveCitationWatch(ctx context.Context, userID int64, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM citation_watches WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("remove citation watch: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UpdateCitationCount(ctx context.Context, id string, count int) error {
	tag, err := p.pool.Exec(ctx, `UPDATE citation_watches SET last_cited_by = $2 WHERE id = $1`, id, count)
	if err != nil {
		return fmt.Errorf("update citation count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface{ Scan(dest ...any) error }

func scanTicket(row rowScanner) (domain.Ticket, error) {
	var t domain.Ticket
	var status string
	if err := row.Scan(&t.ID, &t.UserID, &t.UserName, &t.Message, &t.Reply, &status, &t.CreatedAt, &t.AnsweredAt); err != nil {
		return domain.Ticket{}, err
	}
	t.Status = domain.TicketStatus(status)
	return t, nil
}

// compile-time check that Postgres satisfies Store.
var _ Store = (*Postgres)(nil)

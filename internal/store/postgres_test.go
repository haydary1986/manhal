package store

import (
	"context"
	"os"
	"testing"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/domain"
)

// newTestPostgres connects to TEST_DATABASE_URL and truncates the tables. The
// test is skipped when the env var is unset, so the suite stays green without a
// database.
func newTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	pg, err := NewPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := pg.pool.Exec(context.Background(),
		"TRUNCATE users, library_items, tickets"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(pg.Close)
	return pg
}

func TestPostgres_Users(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()

	if _, err := pg.GetUser(ctx, 1); err != ErrNotFound {
		t.Errorf("missing user err = %v, want ErrNotFound", err)
	}
	if err := pg.SaveUser(ctx, &domain.User{TelegramID: 1, Name: "أحمد", Field: "cs", Tier: domain.TierStudent}); err != nil {
		t.Fatal(err)
	}
	u, err := pg.GetUser(ctx, 1)
	if err != nil || u.Name != "أحمد" || u.Field != "cs" || u.Tier != domain.TierStudent {
		t.Fatalf("GetUser = (%+v, %v)", u, err)
	}
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should be stamped")
	}
	// Upsert.
	_ = pg.SaveUser(ctx, &domain.User{TelegramID: 1, Name: "أحمد علي", Field: "med"})
	u2, _ := pg.GetUser(ctx, 1)
	if u2.Name != "أحمد علي" || u2.Field != "med" {
		t.Errorf("upsert failed: %+v", u2)
	}
}

func TestPostgres_Library(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()
	const uid int64 = 7

	a := domain.LibraryItem{ID: "a", Work: cite.Work{Title: "First", Year: 2020, DOI: "10.1/a"}, Tags: []string{"ai", "nlp"}, Vector: []float32{0.1, 0.2, 0.3}}
	b := domain.LibraryItem{ID: "b", Work: cite.Work{Title: "Second"}}
	_ = pg.AddLibraryItem(ctx, uid, a)
	_ = pg.AddLibraryItem(ctx, uid, b)
	_ = pg.AddLibraryItem(ctx, uid, a) // duplicate -> DO NOTHING

	items, _ := pg.ListLibrary(ctx, uid)
	if len(items) != 2 {
		t.Fatalf("library size = %d, want 2", len(items))
	}
	if items[0].ID != "b" {
		t.Errorf("newest first expected, got %q", items[0].ID)
	}
	// JSONB round-trip of the Work.
	var found domain.LibraryItem
	for _, it := range items {
		if it.ID == "a" {
			found = it
		}
	}
	if found.Work.Title != "First" || found.Work.DOI != "10.1/a" || len(found.Tags) != 2 {
		t.Errorf("work/tags round-trip wrong: %+v", found)
	}
	if len(found.Vector) != 3 || found.Vector[2] != 0.3 {
		t.Errorf("vector round-trip wrong: %+v", found.Vector)
	}

	if err := pg.RemoveLibraryItem(ctx, uid, "a"); err != nil {
		t.Fatal(err)
	}
	if err := pg.RemoveLibraryItem(ctx, uid, "missing"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
	if items, _ := pg.ListLibrary(ctx, uid); len(items) != 1 {
		t.Errorf("after remove size = %d, want 1", len(items))
	}
}

func TestPostgres_Tickets(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()

	_ = pg.AddTicket(ctx, domain.Ticket{ID: "t1", UserID: 9, UserName: "سارة", Message: "استفسار"})
	_ = pg.AddTicket(ctx, domain.Ticket{ID: "t2", UserID: 9, Message: "آخر"})

	got, err := pg.AnswerTicket(ctx, "t1", "تم الحل")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.TicketAnswered || got.Reply != "تم الحل" || got.AnsweredAt == nil {
		t.Errorf("answer wrong: %+v", got)
	}

	list, _ := pg.ListTickets(ctx)
	if len(list) != 2 {
		t.Fatalf("tickets = %d, want 2", len(list))
	}
	// Open tickets sort before answered ones.
	if list[0].ID != "t2" {
		t.Errorf("open ticket should come first, got %q", list[0].ID)
	}

	if _, err := pg.AnswerTicket(ctx, "missing", "x"); err != ErrNotFound {
		t.Errorf("answer missing = %v, want ErrNotFound", err)
	}
}

func TestPostgres_Subscriptions(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()

	_ = pg.AddSubscription(ctx, domain.Subscription{ID: "s1", UserID: 1, Topic: "ai", SeenDOIs: []string{"10.1/a"}})
	_ = pg.AddSubscription(ctx, domain.Subscription{ID: "s1", UserID: 1, Topic: "ai"}) // ON CONFLICT DO NOTHING
	_ = pg.AddSubscription(ctx, domain.Subscription{ID: "s2", UserID: 2, Topic: "nlp"})

	if subs, _ := pg.ListSubscriptions(ctx, 1); len(subs) != 1 || subs[0].Topic != "ai" || len(subs[0].SeenDOIs) != 1 {
		t.Fatalf("user 1 subs wrong: %+v", subs)
	}
	if all, _ := pg.ListAllSubscriptions(ctx); len(all) != 2 {
		t.Errorf("all subs = %d, want 2", len(all))
	}

	if err := pg.UpdateSubscriptionSeen(ctx, "s2", []string{"10.1/x", "10.1/y"}); err != nil {
		t.Fatal(err)
	}
	subs, _ := pg.ListSubscriptions(ctx, 2)
	if len(subs[0].SeenDOIs) != 2 {
		t.Errorf("seen not updated: %+v", subs[0])
	}

	if err := pg.RemoveSubscription(ctx, 1, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := pg.RemoveSubscription(ctx, 1, "missing"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

func TestPostgres_UsersListAndReminders(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()

	_ = pg.SaveUser(ctx, &domain.User{TelegramID: 1, Field: "cs"})
	_ = pg.SaveUser(ctx, &domain.User{TelegramID: 2, Field: "med"})
	users, _ := pg.ListUsers(ctx)
	if len(users) != 2 {
		t.Fatalf("ListUsers = %d, want 2", len(users))
	}

	if ok, _ := pg.WasReminded(ctx, 1, "conf"); ok {
		t.Error("should not be reminded yet")
	}
	if err := pg.MarkReminded(ctx, 1, "conf"); err != nil {
		t.Fatal(err)
	}
	_ = pg.MarkReminded(ctx, 1, "conf") // ON CONFLICT DO NOTHING
	if ok, _ := pg.WasReminded(ctx, 1, "conf"); !ok {
		t.Error("should be reminded after MarkReminded")
	}
	if ok, _ := pg.WasReminded(ctx, 2, "conf"); ok {
		t.Error("reminder is per-user")
	}
}

func TestPostgres_CitationWatches(t *testing.T) {
	pg := newTestPostgres(t)
	ctx := context.Background()

	_ = pg.AddCitationWatch(ctx, domain.CitationWatch{ID: "w1", UserID: 1, AuthorName: "Yann LeCun", LastCitedBy: 100})
	_ = pg.AddCitationWatch(ctx, domain.CitationWatch{ID: "w1", UserID: 1, AuthorName: "Yann LeCun"}) // ON CONFLICT
	_ = pg.AddCitationWatch(ctx, domain.CitationWatch{ID: "w2", UserID: 2, AuthorName: "Bengio"})

	if ws, _ := pg.ListCitationWatches(ctx, 1); len(ws) != 1 || ws[0].LastCitedBy != 100 {
		t.Fatalf("user 1 watches wrong: %+v", ws)
	}
	if all, _ := pg.ListAllCitationWatches(ctx); len(all) != 2 {
		t.Errorf("all watches = %d, want 2", len(all))
	}

	if err := pg.UpdateCitationCount(ctx, "w1", 175); err != nil {
		t.Fatal(err)
	}
	ws, _ := pg.ListCitationWatches(ctx, 1)
	if ws[0].LastCitedBy != 175 {
		t.Errorf("count not updated: %+v", ws[0])
	}

	if err := pg.RemoveCitationWatch(ctx, 1, "w1"); err != nil {
		t.Fatal(err)
	}
	if err := pg.RemoveCitationWatch(ctx, 1, "missing"); err != ErrNotFound {
		t.Errorf("remove missing = %v, want ErrNotFound", err)
	}
}

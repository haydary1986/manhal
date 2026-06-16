package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/store"
)

// fakeNotifier records pushed messages.
type fakeNotifier struct {
	userID int64
	text   string
	calls  int
}

func (f *fakeNotifier) Notify(userID int64, text string) error {
	f.userID, f.text, f.calls = userID, text, f.calls+1
	return nil
}

func testServer(t *testing.T) *Server {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), []menu.Item{
		{ID: "announcements", Label: "📢 الإعلانات", Action: "announcements"},
		{ID: "refs", Label: "المراجع", Children: []menu.Item{
			{ID: "search", Label: "🔍 بحث", Action: "search"},
		}},
	})
	return NewServer(mgr, store.NewMemory(), &fakeNotifier{}, map[string]string{"admin": "secret"})
}

func TestHealthz_NoAuth(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("healthz = %d %q, want 200 ok", rec.Code, rec.Body.String())
	}
}

func TestAdmin_RequiresAuth(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth => %d, want 401", rec.Code)
	}
}

func TestAdmin_RejectsWrongPassword(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "wrong")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password => %d, want 401", rec.Code)
	}
}

func TestAdmin_RendersTree(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"المراجع", "🔍 بحث", "[search]", "إضافة زر"} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestAdd_CreatesButton(t *testing.T) {
	s := testServer(t)
	form := url.Values{"parent": {"refs"}, "label": {"📝 اقتباس"}, "action": {"cite"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/menu/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add status = %d, want 303", rec.Code)
	}
	kids, _ := s.menu.Children("refs")
	if len(kids) != 2 {
		t.Errorf("refs children = %d, want 2 after add", len(kids))
	}
	if _, ok := s.menu.Find("cite"); !ok {
		t.Error("cite button should exist after add")
	}
}

func TestAdd_SubmenuCreatesContainer(t *testing.T) {
	s := testServer(t)
	form := url.Values{"parent": {menu.RootID}, "label": {"أدوات"}, "action": {"submenu"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/menu/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	sub, ok := s.menu.Find("sub")
	if !ok || !sub.IsSubmenu() {
		t.Errorf("submenu container not created: %+v ok=%v", sub, ok)
	}
}

func TestDelete_RemovesButton(t *testing.T) {
	s := testServer(t)
	form := url.Values{"id": {"announcements"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/menu/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if _, ok := s.menu.Find("announcements"); ok {
		t.Error("announcements should be gone after delete")
	}
}

func TestAuth_MultipleAccounts(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	s := NewServer(mgr, store.NewMemory(), nil, map[string]string{"alice": "pw1", "bob": "pw2"})

	for _, c := range []struct {
		user, pass string
		want       int
	}{
		{"alice", "pw1", http.StatusOK},
		{"bob", "pw2", http.StatusOK},
		{"alice", "pw2", http.StatusUnauthorized}, // wrong password
		{"carol", "pw1", http.StatusUnauthorized}, // unknown user
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.SetBasicAuth(c.user, c.pass)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s/%s => %d, want %d", c.user, c.pass, rec.Code, c.want)
		}
	}
}

func TestSupport_ListAndReplyPushes(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	notifier := &fakeNotifier{}
	s := NewServer(mgr, st, notifier, map[string]string{"admin": "secret"})

	_ = st.AddTicket(context.Background(), domain.Ticket{
		ID: "t1", UserID: 555, UserName: "باحث", Message: "كيف أصدّر مكتبتي؟",
	})

	// List shows the ticket.
	req := httptest.NewRequest(http.MethodGet, "/admin/support", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "كيف أصدّر مكتبتي؟") {
		t.Fatalf("support list wrong: %d\n%s", rec.Code, rec.Body.String())
	}

	// Reply persists and pushes to the user.
	form := url.Values{"id": {"t1"}, "reply": {"من زر «تصدير BibTeX»."}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/support/reply", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("reply status = %d", rec2.Code)
	}
	if notifier.calls != 1 || notifier.userID != 555 {
		t.Errorf("expected push to user 555, got calls=%d user=%d", notifier.calls, notifier.userID)
	}
	if !strings.Contains(notifier.text, "تصدير BibTeX") {
		t.Errorf("push text missing reply: %q", notifier.text)
	}

	tickets, _ := st.ListTickets(context.Background())
	if tickets[0].Status != domain.TicketAnswered {
		t.Errorf("ticket should be answered, got %q", tickets[0].Status)
	}
}

func TestSupportReply_NilNotifierStillSaves(t *testing.T) {
	st := store.NewMemory()
	s := NewServer(menu.NewManager(t.TempDir(), nil), st, nil, map[string]string{"admin": "secret"})
	_ = st.AddTicket(context.Background(), domain.Ticket{ID: "x", UserID: 1, Message: "q"})

	form := url.Values{"id": {"x"}, "reply": {"ans"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/support/reply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req) // must not panic on nil notifier

	tickets, _ := st.ListTickets(context.Background())
	if tickets[0].Reply != "ans" {
		t.Error("reply should be saved even without a notifier")
	}
}

func TestAdd_DuplicateShowsError(t *testing.T) {
	s := testServer(t)
	// "search" already exists, GenID makes it unique, so duplicate can't happen
	// via the form path; instead delete a missing id to exercise the error path.
	form := url.Values{"id": {"does-not-exist"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/menu/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

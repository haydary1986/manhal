package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/announce"
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

func (f *fakeNotifier) SendRich(userID int64, text, imageURL, buttonLabel, buttonURL string) error {
	f.userID, f.text, f.calls = userID, text, f.calls+1
	return nil
}

// fakeSettings is an in-memory Settings double for tests.
type fakeSettings struct {
	channel        string
	require        bool
	premiumInfo    string
	paymentDetails string
	paymentLink    string
	freeLimit      int
	premiumLimit   int
	deepSeekKey    string
	welcome        string
	botName        string
	botDesc        string
}

func (f *fakeSettings) RequiredChannel() string   { return f.channel }
func (f *fakeSettings) RequireSubscription() bool { return f.require }
func (f *fakeSettings) SetGate(channel string, require bool) error {
	if require && channel == "" {
		return errTestGate
	}
	f.channel, f.require = channel, require
	return nil
}
func (f *fakeSettings) PremiumInfo() string    { return f.premiumInfo }
func (f *fakeSettings) PaymentDetails() string { return f.paymentDetails }
func (f *fakeSettings) PaymentLink() string    { return f.paymentLink }
func (f *fakeSettings) SetPayment(premiumInfo, paymentDetails, paymentLink string) error {
	f.premiumInfo, f.paymentDetails, f.paymentLink = premiumInfo, paymentDetails, paymentLink
	return nil
}
func (f *fakeSettings) FreeAILimit() int    { return f.freeLimit }
func (f *fakeSettings) PremiumAILimit() int { return f.premiumLimit }
func (f *fakeSettings) SetLimits(free, premium int) error {
	f.freeLimit, f.premiumLimit = free, premium
	return nil
}
func (f *fakeSettings) DeepSeekKey() string             { return f.deepSeekKey }
func (f *fakeSettings) SetDeepSeekKey(key string) error { f.deepSeekKey = key; return nil }
func (f *fakeSettings) WelcomeMessage() string          { return f.welcome }
func (f *fakeSettings) BotName() string                 { return f.botName }
func (f *fakeSettings) BotDescription() string          { return f.botDesc }
func (f *fakeSettings) SetIdentity(welcome, name, description string) error {
	f.welcome, f.botName, f.botDesc = welcome, name, description
	return nil
}

var errTestGate = errorString("channel required when gate enabled")

type errorString string

func (e errorString) Error() string { return string(e) }

func testServer(t *testing.T) *Server {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), []menu.Item{
		{ID: "announcements", Label: "📢 الإعلانات", Action: "announcements"},
		{ID: "refs", Label: "المراجع", Children: []menu.Item{
			{ID: "search", Label: "🔍 بحث", Action: "search"},
		}},
	})
	return NewServer(mgr, store.NewMemory(), &fakeNotifier{}, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))
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

func TestMenuPage_RendersTree(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/menu", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"المراجع", "🔍 بحث", "search", "إضافة زر"} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestDashboard_RendersStats(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	// Seed a user and some usage so the analytics cards have data.
	_ = st.SaveUser(context.Background(), &domain.User{TelegramID: 7, Name: "باحث"})
	_ = st.RecordUsage(context.Background(), 7, "search")
	_ = st.RecordUsage(context.Background(), 7, "search")
	_ = st.RecordUsage(context.Background(), 7, "cite")
	s := NewServer(mgr, st, nil, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"لوحة التحكم", "أكثر الميزات استخداماً", "أنشط المستخدمين", "توقيت بغداد"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
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
	s := NewServer(mgr, store.NewMemory(), nil, map[string]string{"alice": "pw1", "bob": "pw2"}, &fakeSettings{}, announce.NewRepo(nil))

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
	s := NewServer(mgr, st, notifier, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

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

func TestSupport_GroupsByUser(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	s := NewServer(mgr, st, &fakeNotifier{}, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))
	ctx := context.Background()
	_ = st.AddTicket(ctx, domain.Ticket{ID: "a", UserID: 7, UserName: "علي", Message: "سؤال أول"})
	_ = st.AddTicket(ctx, domain.Ticket{ID: "b", UserID: 7, UserName: "علي", Message: "سؤال ثانٍ"})
	_ = st.AddTicket(ctx, domain.Ticket{ID: "c", UserID: 9, UserName: "سارة", Message: "استفسار"})

	req := httptest.NewRequest(http.MethodGet, "/admin/support", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"علي", "سؤال أول", "سؤال ثانٍ", "سارة", "استفسار"} {
		if !strings.Contains(body, want) {
			t.Errorf("threaded support missing %q", want)
		}
	}
	// User 7's two messages must appear under a SINGLE thread header.
	if c := strings.Count(body, "#7</span>"); c != 1 {
		t.Errorf("user 7 should be one thread header, got %d occurrences", c)
	}
}

func TestSupportReply_NilNotifierStillSaves(t *testing.T) {
	st := store.NewMemory()
	s := NewServer(menu.NewManager(t.TempDir(), nil), st, nil, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))
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

func TestUsers_ListMessageTier(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	notifier := &fakeNotifier{}
	_ = st.SaveUser(context.Background(), &domain.User{TelegramID: 50, Name: "علي", Tier: domain.TierFree})
	_ = st.RecordUsage(context.Background(), 50, "search")
	s := NewServer(mgr, st, notifier, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

	// The list page shows the user.
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "علي") {
		t.Fatalf("users list wrong: %d", rec.Code)
	}

	// Messaging a user pushes through the notifier.
	form := url.Values{"id": {"50"}, "text": {"مرحباً بك"}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/users/message", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusSeeOther || notifier.calls != 1 || notifier.userID != 50 {
		t.Fatalf("message not pushed: code=%d calls=%d user=%d", rec2.Code, notifier.calls, notifier.userID)
	}
	if !strings.Contains(notifier.text, "مرحباً بك") {
		t.Errorf("pushed text missing message: %q", notifier.text)
	}

	// Granting a 1-month premium persists, makes the user premium, and sets an
	// expiry roughly one month out.
	form2 := url.Values{"id": {"50"}, "action": {"grant"}, "tier": {"researcher"}, "months": {"1"}}
	req3 := httptest.NewRequest(http.MethodPost, "/admin/users/premium", strings.NewReader(form2.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.SetBasicAuth("admin", "secret")
	rec3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusSeeOther {
		t.Fatalf("premium status = %d", rec3.Code)
	}
	u, _ := st.GetUser(context.Background(), 50)
	if u.Tier != domain.TierResearcher || !u.IsPremium(time.Now()) {
		t.Errorf("user not premium after grant: tier=%q", u.Tier)
	}
	if u.PremiumUntil == nil {
		t.Error("1-month grant should set an expiry, got permanent")
	}

	// Revoking returns the user to free.
	form3 := url.Values{"id": {"50"}, "action": {"revoke"}}
	req4 := httptest.NewRequest(http.MethodPost, "/admin/users/premium", strings.NewReader(form3.Encode()))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req4.SetBasicAuth("admin", "secret")
	rec4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec4, req4)
	u2, _ := st.GetUser(context.Background(), 50)
	if u2.Tier != domain.TierFree || u2.IsPremium(time.Now()) {
		t.Errorf("user still premium after revoke: tier=%q", u2.Tier)
	}
}

func TestAnnounce_AddAndDelete(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	repo := announce.NewRepo(nil)
	s := NewServer(mgr, store.NewMemory(), nil, map[string]string{"admin": "secret"}, &fakeSettings{}, repo)

	// Publish.
	form := url.Values{"kind": {"grant"}, "title": {"منحة بحثية"}, "body": {"تفاصيل"}, "link": {"https://x.org"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/announcements/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add status = %d, want 303", rec.Code)
	}
	all := repo.All()
	if len(all) != 1 || all[0].Title != "منحة بحثية" {
		t.Fatalf("announcement not stored: %+v", all)
	}

	// The list page shows it.
	req2 := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "منحة بحثية") {
		t.Fatalf("list page wrong: %d", rec2.Code)
	}

	// Delete.
	del := url.Values{"id": {all[0].ID}}
	req3 := httptest.NewRequest(http.MethodPost, "/admin/announcements/delete", strings.NewReader(del.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.SetBasicAuth("admin", "secret")
	rec3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusSeeOther || repo.Len() != 0 {
		t.Errorf("delete failed: code=%d len=%d", rec3.Code, repo.Len())
	}
}

func TestUserActivity_Page(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 50, Name: "علي"})
	_ = st.RecordUsage(ctx, 50, "search")
	_ = st.RecordUsage(ctx, 50, "cite")
	s := NewServer(menu.NewManager(t.TempDir(), nil), st, &fakeNotifier{},
		map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

	req := httptest.NewRequest(http.MethodGet, "/admin/users/activity?id=50", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"علي", "سجلّ نشاط", "آخر نشاط"} {
		if !strings.Contains(body, want) {
			t.Errorf("activity page missing %q", want)
		}
	}
}

func TestSettings_SaveGate(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	fs := &fakeSettings{}
	s := NewServer(mgr, store.NewMemory(), nil, map[string]string{"admin": "secret"}, fs, announce.NewRepo(nil))

	// Page renders.
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "الاشتراك الإجباري") {
		t.Fatalf("settings page wrong: %d", rec.Code)
	}

	// Saving a channel persists through the Settings interface.
	form := url.Values{"channel": {"@manhal_channel"}, "require": {"on"}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/settings/gate", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d, want 303", rec2.Code)
	}
	if fs.channel != "@manhal_channel" || !fs.require {
		t.Errorf("gate not saved: %+v", fs)
	}

	// Enabling the gate without a channel is rejected (error redirect).
	form2 := url.Values{"channel": {""}, "require": {"on"}}
	req3 := httptest.NewRequest(http.MethodPost, "/admin/settings/gate", strings.NewReader(form2.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.SetBasicAuth("admin", "secret")
	rec3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec3, req3)
	if !strings.Contains(rec3.Header().Get("Location"), "err=") {
		t.Errorf("expected error redirect, got %q", rec3.Header().Get("Location"))
	}
}

func TestAdd_URLButton(t *testing.T) {
	s := testServer(t)
	form := url.Values{"parent": {menu.RootID}, "label": {"📢 قناتنا"}, "action": {"url"}, "url": {"@manhal_channel"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/menu/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}

	kids, _ := s.menu.Children(menu.RootID)
	var link *menu.Item
	for i := range kids {
		if kids[i].IsLink() {
			link = &kids[i]
		}
	}
	if link == nil {
		t.Fatal("link button not created")
	}
	if link.URL != "https://t.me/manhal_channel" {
		t.Errorf("url = %q, want normalized t.me link", link.URL)
	}
	if link.IsSubmenu() {
		t.Error("link should not be a submenu")
	}

	// An empty/invalid URL is rejected.
	bad := url.Values{"parent": {menu.RootID}, "label": {"x"}, "action": {"url"}, "url": {""}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/menu/add", strings.NewReader(bad.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if !strings.Contains(rec2.Header().Get("Location"), "err=") {
		t.Errorf("empty url should redirect with err, got %q", rec2.Header().Get("Location"))
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

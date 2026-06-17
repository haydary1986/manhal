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

func TestBroadcastTargets(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	users := []domain.User{
		{TelegramID: 1, Tier: domain.TierFree},
		{TelegramID: 2, Tier: domain.TierResearcher, PremiumUntil: &future},
		{TelegramID: 3, Tier: domain.TierFree},
	}
	if got := broadcastTargets(users, "all", now); len(got) != 3 {
		t.Errorf("all => %d, want 3", len(got))
	}
	prem := broadcastTargets(users, "premium", now)
	if len(prem) != 1 || prem[0] != 2 {
		t.Errorf("premium => %v, want [2]", prem)
	}
	free := broadcastTargets(users, "free", now)
	if len(free) != 2 {
		t.Errorf("free => %v, want 2 users", free)
	}
}

func TestGiftCodes_Generate(t *testing.T) {
	st := store.NewMemory()
	s := NewServer(menu.NewManager(t.TempDir(), nil), st, &fakeNotifier{},
		map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

	form := url.Values{"tier": {"researcher"}, "days": {"30"}, "quantity": {"5"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/giftcodes/generate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("generate status = %d, want 303", rec.Code)
	}
	codes, _ := st.ListGiftCodes(context.Background())
	if len(codes) != 5 {
		t.Errorf("generated %d codes, want 5", len(codes))
	}
	for _, g := range codes {
		if g.Tier != domain.TierResearcher || g.Days != 30 {
			t.Errorf("bad code: %+v", g)
		}
	}

	// A free tier is rejected.
	bad := url.Values{"tier": {"free"}, "quantity": {"3"}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/giftcodes/generate", strings.NewReader(bad.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if !strings.Contains(rec2.Header().Get("Location"), "err=") {
		t.Error("free tier should be rejected")
	}
}

func TestBroadcastPage_Renders(t *testing.T) {
	st := store.NewMemory()
	_ = st.SaveUser(context.Background(), &domain.User{TelegramID: 9, Tier: domain.TierFree})
	s := NewServer(menu.NewManager(t.TempDir(), nil), st, &fakeNotifier{},
		map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))

	req := httptest.NewRequest(http.MethodGet, "/admin/broadcast", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"البث الجماعي", "المستهدَفون", "البريميم فقط"} {
		if !strings.Contains(body, want) {
			t.Errorf("broadcast page missing %q", want)
		}
	}
}

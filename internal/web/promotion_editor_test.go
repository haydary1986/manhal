package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/promotion"
	"github.com/erticaz/manhal/internal/store"
)

// promoServer wires the real promotion.Manager as the editor (it satisfies
// PromotionEditor), so the test exercises validation end to end.
func promoServer(t *testing.T) (*Server, *promotion.Manager) {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), nil)
	pm := promotion.NewManager(t.TempDir(), promotion.DefaultRules())
	s := NewServer(mgr, store.NewMemory(), &fakeNotifier{}, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil)).
		WithEditors(nil, nil, pm)
	return s, pm
}

func promoRequest(t *testing.T, s *Server, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPromotionEditor_Render(t *testing.T) {
	s, _ := promoServer(t)
	rec := promoRequest(t, s, http.MethodGet, "/admin/promotion", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"activities:", "ranks:", `name="rules"`} {
		if !strings.Contains(body, want) {
			t.Errorf("render missing %q", want)
		}
	}
}

func TestPromotionEditor_SaveValidApplies(t *testing.T) {
	s, pm := promoServer(t)
	doc := "ranks:\n" +
		"  - {key: lecturer, label: مدرس, next_label: أستاذ مساعد, required_total: 111, required_table1: 60, required_table2: 30, min_service_years: 4}\n" +
		"activities:\n" +
		"  - {key: if_first, label: بحث, table: 1, points: {lecturer: 25}}\n"
	rec := promoRequest(t, s, http.MethodPost, "/admin/promotion/save", url.Values{"rules": {doc}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	rk, ok := pm.FindRank("lecturer")
	if !ok || rk.RequiredTotal != 111 {
		t.Errorf("edit not applied to live rules: %+v ok=%v", rk, ok)
	}
}

func TestPromotionEditor_SaveInvalidRejected(t *testing.T) {
	s, pm := promoServer(t)
	before := len(pm.Ranks())
	rec := promoRequest(t, s, http.MethodPost, "/admin/promotion/save", url.Values{"rules": {"not: [valid"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
	if len(pm.Ranks()) != before {
		t.Errorf("live rules mutated by an invalid save")
	}
}

func TestPromotionEditor_Reset(t *testing.T) {
	s, pm := promoServer(t)
	if err := pm.SetYAML("ranks:\n  - {key: x, label: y}\nactivities:\n  - {key: a, label: b, table: 1, points: {\"*\": 1}}\n"); err != nil {
		t.Fatal(err)
	}
	rec := promoRequest(t, s, http.MethodPost, "/admin/promotion/reset", url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := len(pm.Ranks()); got != 3 {
		t.Errorf("reset did not restore defaults: %d ranks", got)
	}
}

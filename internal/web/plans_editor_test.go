package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/plans"
	"github.com/erticaz/manhal/internal/store"
)

func plansServer(t *testing.T) (*Server, *plans.Manager) {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), nil)
	pm := plans.NewManager(t.TempDir(), plans.DefaultPlans())
	s := NewServer(mgr, store.NewMemory(), &fakeNotifier{}, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil)).
		WithPlans(pm)
	return s, pm
}

func TestPlansEditor_RenderListsSeed(t *testing.T) {
	s, _ := plansServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/plans", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "شهري") {
		t.Error("plans page should list the seed monthly plan")
	}
}

func TestPlansEditor_UpsertAndDelete(t *testing.T) {
	s, pm := plansServer(t)

	// Upsert (update the monthly price).
	form := url.Values{"id": {"monthly"}, "name": {"شهري"}, "months": {"1"}, "price": {"7000"}, "tier": {"researcher"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/plans/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d", rec.Code)
	}
	if p, ok := pm.Get("monthly"); !ok || p.Price != 7000 {
		t.Errorf("price not updated: %+v ok=%v", p, ok)
	}

	// Delete the annual plan.
	form2 := url.Values{"id": {"annual"}}
	req2 := httptest.NewRequest(http.MethodPost, "/admin/plans/delete", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("admin", "secret")
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if _, ok := pm.Get("annual"); ok {
		t.Error("annual plan should be deleted")
	}
}

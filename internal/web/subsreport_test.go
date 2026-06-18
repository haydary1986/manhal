package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/store"
)

func TestSubsReport_RevenueAndCounts(t *testing.T) {
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	s := NewServer(mgr, st, &fakeNotifier{}, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))
	ctx := context.Background()
	now := time.Now()

	// Two approved requests this month → revenue 50000; one pending; one rejected.
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{ID: "a1", UserID: 1, PlanName: "شهري", Price: 5000, Status: domain.SubReqApproved, DecidedAt: &now})
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{ID: "a2", UserID: 2, PlanName: "سنوي", Price: 45000, Status: domain.SubReqApproved, DecidedAt: &now})
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{ID: "p1", UserID: 3, PlanName: "شهري", Price: 5000, Status: domain.SubReqPending})
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{ID: "x1", UserID: 4, PlanName: "شهري", Price: 5000, Status: domain.SubReqRejected})

	req := httptest.NewRequest(http.MethodGet, "/admin/subscriptions", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "50000") {
		t.Errorf("report should show total revenue 50000:\n%s", body)
	}
	if !strings.Contains(body, "تقرير الاشتراكات") {
		t.Error("report heading missing")
	}
}

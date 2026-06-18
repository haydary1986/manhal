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

func requestsServer(t *testing.T) (*Server, *store.Memory, *fakeNotifier) {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), nil)
	st := store.NewMemory()
	notifier := &fakeNotifier{}
	s := NewServer(mgr, st, notifier, map[string]string{"admin": "secret"}, &fakeSettings{}, announce.NewRepo(nil))
	return s, st, notifier
}

func postForm(t *testing.T, s *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestRequestQueue_ApproveActivatesUser(t *testing.T) {
	s, st, notifier := requestsServer(t)
	ctx := context.Background()
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 99, Name: "سعد"})
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{
		ID: "rq1", UserID: 99, PlanID: "monthly", PlanName: "شهري", Months: 1,
		Tier: domain.TierResearcher, Price: 5000, Proof: "زين كاش 777",
	})

	rec := postForm(t, s, "/admin/requests/approve", url.Values{"id": {"rq1"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("approve status = %d", rec.Code)
	}
	u, _ := st.GetUser(ctx, 99)
	if !u.IsPremium(time.Now()) || u.Tier != domain.TierResearcher {
		t.Errorf("user not premium after approve: %+v", u)
	}
	if u.PremiumUntil == nil {
		t.Error("1-month plan should set an expiry")
	}
	got, _ := st.GetSubscriptionRequest(ctx, "rq1")
	if got.Status != domain.SubReqApproved {
		t.Errorf("request status = %q, want approved", got.Status)
	}
	if notifier.calls != 1 || notifier.userID != 99 {
		t.Errorf("user not notified: calls=%d to=%d", notifier.calls, notifier.userID)
	}
}

func TestRequestQueue_RejectNotifiesWithReason(t *testing.T) {
	s, st, notifier := requestsServer(t)
	ctx := context.Background()
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{
		ID: "rq2", UserID: 5, PlanName: "سنوي", Months: 12, Tier: domain.TierResearcher,
	})

	rec := postForm(t, s, "/admin/requests/reject", url.Values{"id": {"rq2"}, "note": {"لم يصل التحويل"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reject status = %d", rec.Code)
	}
	got, _ := st.GetSubscriptionRequest(ctx, "rq2")
	if got.Status != domain.SubReqRejected || got.Note != "لم يصل التحويل" {
		t.Errorf("reject not recorded: %+v", got)
	}
	u, _ := st.GetUser(ctx, 5)
	if u != nil && u.IsPremium(time.Now()) {
		t.Error("rejected user should not be premium")
	}
	if notifier.calls != 1 || !strings.Contains(notifier.text, "لم يصل التحويل") {
		t.Errorf("reject notification missing reason: calls=%d text=%q", notifier.calls, notifier.text)
	}
}

func TestRequestQueue_RenderListsPending(t *testing.T) {
	s, st, _ := requestsServer(t)
	ctx := context.Background()
	_ = st.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{
		ID: "rq3", UserID: 8, UserName: "ليلى", PlanName: "شهري", Months: 1, Proof: "وصل #321",
	})
	req := httptest.NewRequest(http.MethodGet, "/admin/requests", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "ليلى") || !strings.Contains(body, "وصل #321") {
		t.Errorf("queue render missing pending request: %d", rec.Code)
	}
}

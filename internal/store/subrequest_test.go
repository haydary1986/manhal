package store

import (
	"context"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
)

func TestMemory_SubscriptionRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	if err := m.AddSubscriptionRequest(ctx, domain.SubscriptionRequest{
		ID: "r1", UserID: 7, PlanID: "monthly", PlanName: "شهري", Months: 1,
		Tier: domain.TierResearcher, Price: 5000, Proof: "زين كاش 12345",
	}); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Pending list contains it; status defaulted.
	pending, _ := m.ListSubscriptionRequests(ctx, domain.SubReqPending)
	if len(pending) != 1 || pending[0].Status != domain.SubReqPending {
		t.Fatalf("pending = %+v", pending)
	}

	// Approve it.
	got, err := m.GetSubscriptionRequest(ctx, "r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Status = domain.SubReqApproved
	if err := m.UpdateSubscriptionRequest(ctx, *got); err != nil {
		t.Fatalf("update: %v", err)
	}

	// No longer pending; appears under approved.
	if p, _ := m.ListSubscriptionRequests(ctx, domain.SubReqPending); len(p) != 0 {
		t.Errorf("still pending: %+v", p)
	}
	if a, _ := m.ListSubscriptionRequests(ctx, domain.SubReqApproved); len(a) != 1 {
		t.Errorf("approved = %+v", a)
	}

	// Unknown id errors.
	if _, err := m.GetSubscriptionRequest(ctx, "nope"); err != ErrNotFound {
		t.Errorf("get missing = %v, want ErrNotFound", err)
	}
}

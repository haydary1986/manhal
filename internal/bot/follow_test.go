package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/store"
)

func followApp() *App {
	return &App{store: store.NewMemory(), sessions: newSessions()}
}

func TestSubscriptionID_StableAndCaseInsensitive(t *testing.T) {
	if subscriptionID(1, "Deep Learning") != subscriptionID(1, "deep learning ") {
		t.Error("id should be case/space-insensitive for the same topic")
	}
	if subscriptionID(1, "a") == subscriptionID(2, "a") {
		t.Error("different users should differ")
	}
}

func TestFollowScreen_EmptyAndPopulated(t *testing.T) {
	a := followApp()
	ctx := context.Background()
	const uid int64 = 5

	if !strings.Contains(a.followScreen(ctx, uid).Text, "لا تتابع") {
		t.Error("empty follow screen should say no topics")
	}

	_ = a.store.AddSubscription(ctx, domain.Subscription{ID: "s1", UserID: uid, Topic: "transformers"})
	scr := a.followScreen(ctx, uid)
	if !strings.Contains(scr.Text, "transformers") {
		t.Errorf("populated screen missing topic:\n%s", scr.Text)
	}
	var hasAdd, hasRemove bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "sub:add" {
				hasAdd = true
			}
			if b.Data == "sub:rm:s1" {
				hasRemove = true
			}
		}
	}
	if !hasAdd || !hasRemove {
		t.Errorf("screen should offer add and remove: add=%v remove=%v", hasAdd, hasRemove)
	}
}

func TestSubscription_IdempotentByID(t *testing.T) {
	a := followApp()
	ctx := context.Background()
	const uid int64 = 7

	id := subscriptionID(uid, "graph neural networks")
	_ = a.store.AddSubscription(ctx, domain.Subscription{ID: id, UserID: uid, Topic: "graph neural networks"})
	// Same topic, different casing -> same id -> ignored.
	id2 := subscriptionID(uid, "Graph Neural Networks")
	_ = a.store.AddSubscription(ctx, domain.Subscription{ID: id2, UserID: uid, Topic: "Graph Neural Networks"})

	subs, _ := a.store.ListSubscriptions(ctx, uid)
	if len(subs) != 1 {
		t.Errorf("duplicate topic should be ignored, got %d", len(subs))
	}
}

func TestBaselineDOIs_NilSearchSafe(t *testing.T) {
	a := followApp() // search is nil
	if got := a.baselineDOIs(context.Background(), "x"); got != nil {
		t.Errorf("nil search should yield nil baseline, got %v", got)
	}
}

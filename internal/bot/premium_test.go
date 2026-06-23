package bot

import (
	"context"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/store"
)

func TestIsPremiumAction(t *testing.T) {
	premium := []string{"pdfchat", "slides", "viva", "litreview", "gap", "websearch", "ytsummary", "pdf2word"}
	for _, a := range premium {
		if _, ok := isPremiumAction(a); !ok {
			t.Errorf("%q should be premium", a)
		}
	}
	free := []string{"search", "cite", "journal", "oa", "author", "promotion", "extsearch", "ai", "library", "home"}
	for _, a := range free {
		if _, ok := isPremiumAction(a); ok {
			t.Errorf("%q should be free", a)
		}
	}
}

func TestRequirePremium_AllowsPremiumUserAndFreeActions(t *testing.T) {
	st := store.NewMemory()
	ctx := context.Background()
	until := time.Now().AddDate(0, 1, 0)
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 7, Tier: domain.TierResearcher, PremiumUntil: &until})
	_ = st.SaveUser(ctx, &domain.User{TelegramID: 8, Tier: domain.TierFree})
	a := &App{store: st, sessions: newSessions()}

	// Premium subscriber may use a premium feature (no send/bot needed).
	if !a.requirePremium(ctx, 7, "pdfchat") {
		t.Error("premium user should be allowed into a premium feature")
	}
	// Any user may use a free feature.
	if !a.requirePremium(ctx, 8, "search") {
		t.Error("free feature should be open to everyone")
	}
}

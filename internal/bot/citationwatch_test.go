package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/store"
)

func cwApp() *App {
	return &App{store: store.NewMemory(), sessions: newSessions()}
}

func TestWatchID_StableAndCaseInsensitive(t *testing.T) {
	if watchID(1, "Yann LeCun") != watchID(1, "yann lecun ") {
		t.Error("watch id should be case/space-insensitive")
	}
	if watchID(1, "a") == watchID(2, "a") {
		t.Error("different users should differ")
	}
}

func TestFollowScreen_IncludesCitationWatches(t *testing.T) {
	a := cwApp()
	ctx := context.Background()
	const uid int64 = 9

	_ = a.store.AddCitationWatch(ctx, domain.CitationWatch{
		ID: watchID(uid, "Yann LeCun"), UserID: uid, AuthorName: "Yann LeCun", LastCitedBy: 300000,
	})

	scr := a.followScreen(ctx, uid)
	if !strings.Contains(scr.Text, "تنبيهات الاستشهاد") || !strings.Contains(scr.Text, "Yann LeCun") {
		t.Errorf("follow screen should list the citation watch:\n%s", scr.Text)
	}
	var hasRemove bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "cwatch:rm:") {
				hasRemove = true
			}
		}
	}
	if !hasRemove {
		t.Error("each watch should have a remove button")
	}
}

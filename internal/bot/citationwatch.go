package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/domain"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleCitationWatch routes "cwatch:" callbacks (add / remove).
func (a *App) handleCitationWatch(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings))
		return
	}

	data := strings.TrimPrefix(cq.Data, "cwatch:")
	switch {
	case data == "add":
		a.addCitationWatch(ctx, cq.From.ID)
	case strings.HasPrefix(data, "rm:"):
		_ = a.store.RemoveCitationWatch(ctx, cq.From.ID, strings.TrimPrefix(data, "rm:"))
		a.send(ctx, cq.From.ID, a.followScreen(ctx, cq.From.ID))
	default:
		a.send(ctx, cq.From.ID, a.followScreen(ctx, cq.From.ID))
	}
}

// addCitationWatch starts watching the citations of the last-viewed author.
func (a *App) addCitationWatch(ctx context.Context, userID int64) {
	au := a.sessions.lastAuthor(userID)
	if au == nil || au.Name == "" {
		a.send(ctx, userID, Screen{
			Text:     "افتح ملف باحث أولاً (👤 ملف باحث) ثم اضغط «نبّهني».",
			Keyboard: followNav(),
		})
		return
	}
	watch := domain.CitationWatch{
		ID:          watchID(userID, au.Name),
		UserID:      userID,
		AuthorName:  au.Name,
		LastCitedBy: au.CitedBy, // baseline: alert only on future increases
	}
	if err := a.store.AddCitationWatch(ctx, watch); err != nil {
		a.logf("citation watch add: %v", err)
		a.send(ctx, userID, Screen{Text: "❌ تعذّرت المتابعة ⚠️", Keyboard: followNav()})
		return
	}
	a.send(ctx, userID, Screen{
		Text:     "🔔 سأنبّهك عند أي استشهاد جديد لـ «" + au.Name + "».",
		Keyboard: followNav(),
	})
}

// watchID derives a short stable id from the user and author name.
func watchID(userID int64, name string) string {
	seed := strconv.FormatInt(userID, 10) + "|cw|" + strings.ToLower(strings.TrimSpace(name))
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:10]
}

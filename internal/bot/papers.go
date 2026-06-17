package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// RelatedSource finds papers related to a DOI (implemented by scholar.OpenAlex).
type RelatedSource interface {
	Related(ctx context.Context, doi string, limit int) ([]scholar.SearchResult, error)
}

// RetractionChecker reports whether a DOI has been retracted (scholar.Crossref).
type RetractionChecker interface {
	CheckRetraction(ctx context.Context, doi string) (scholar.Retraction, error)
}

// --- similar papers ---

func similarPromptScreen() Screen {
	return Screen{
		Text: "🔗 أوراق مشابهة\n\nأرسل DOI ورقة لأجد لك أوراقاً ذات صلة (عبر OpenAlex).",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

func (a *App) handleSimilarDOI(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	if a.related == nil {
		a.send(ctx, chatID, Screen{Text: "الخدمة غير متاحة حالياً.", Keyboard: paperNav("similar")})
		return
	}
	a.send(ctx, chatID, Screen{Text: "🔎 جاري البحث عن أوراق مشابهة..."})

	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	results, err := a.related.Related(cctx, strings.TrimSpace(msg.Text), 6)
	switch {
	case errors.Is(err, scholar.ErrInvalidDOI):
		a.send(ctx, chatID, Screen{Text: "هذا لا يبدو DOI صحيحاً 🤔 يبدأ بـ «10.».", Keyboard: paperNav("similar")})
	case err != nil:
		a.logf("related: %v", err)
		a.send(ctx, chatID, Screen{Text: "صار خلل ⚠️ جرّب لاحقاً.", Keyboard: paperNav("similar")})
	case len(results) == 0:
		a.send(ctx, chatID, Screen{Text: "لم أجد أوراقاً مشابهة لهذا الـ DOI.", Keyboard: paperNav("similar")})
	default:
		a.sessions.setResults(msg.From.ID, results) // enables the inline اقتباس buttons
		a.send(ctx, chatID, searchResultsScreen("🔗 أوراق مشابهة", results))
	}
}

// --- retraction check ---

func retractPromptScreen() Screen {
	return Screen{
		Text: "🚫 كشف الأوراق المسحوبة (Retracted)\n\nأرسل DOI ورقة للتحقّق إن كانت قد سُحبت.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

func (a *App) handleRetractDOI(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	if a.retraction == nil {
		a.send(ctx, chatID, Screen{Text: "الخدمة غير متاحة حالياً.", Keyboard: paperNav("retracted")})
		return
	}
	a.send(ctx, chatID, Screen{Text: "🔍 جاري التحقّق عبر Crossref / Retraction Watch..."})

	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r, err := a.retraction.CheckRetraction(cctx, strings.TrimSpace(msg.Text))
	switch {
	case errors.Is(err, scholar.ErrInvalidDOI):
		a.send(ctx, chatID, Screen{Text: "هذا لا يبدو DOI صحيحاً 🤔 يبدأ بـ «10.».", Keyboard: paperNav("retracted")})
	case err != nil:
		a.logf("retraction: %v", err)
		a.send(ctx, chatID, Screen{Text: "صار خلل ⚠️ جرّب لاحقاً.", Keyboard: paperNav("retracted")})
	case r.Retracted:
		msg := "⚠️ تحذير: هذه الورقة مسحوبة (Retracted)!\nلا تعتمد عليها كمرجع."
		if r.Date != "" {
			msg += "\n🗓️ تاريخ السحب: " + r.Date
		}
		if r.NoticeDOI != "" {
			msg += "\n📄 إشعار السحب: " + r.NoticeDOI
		}
		a.send(ctx, chatID, Screen{Text: msg, Keyboard: paperNav("retracted")})
	default:
		a.send(ctx, chatID, Screen{
			Text:     "✅ لم يُعثَر على سحب لهذه الورقة.\nℹ️ تقدير إرشادي عبر Crossref/Retraction Watch — تحقّق دائماً من المصدر.",
			Keyboard: paperNav("retracted"),
		})
	}
}

func paperNav(action string) *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔁 DOI آخر", Data: "menu:" + action}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

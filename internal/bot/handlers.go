package bot

import (
	"context"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStart greets the user, applying the subscription gate.
func (a *App) handleStart(ctx context.Context, _ *tg.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	from := update.Message.From
	chatID := update.Message.Chat.ID
	a.ensureUser(ctx, from)
	if from != nil {
		a.sessions.clear(from.ID) // /start cancels any active wizard
	}

	if from != nil && !a.isSubscribed(ctx, from.ID) {
		a.send(ctx, chatID, gateScreen(a.settings.Get()))
		return
	}
	a.send(ctx, chatID, a.mainMenuScreen())
}

// handleGateCheck re-checks membership after the user taps «✅ تحقّقت».
func (a *App) handleGateCheck(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	if a.isSubscribed(ctx, cq.From.ID) {
		_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
		a.send(ctx, cq.From.ID, a.mainMenuScreen())
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "لسه ما اشتركت بالقناة 🔒",
		ShowAlert:       true,
	})
}

// handleMenu routes the main-menu buttons. Most are placeholders for now.
func (a *App) handleMenu(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	// Enforce the subscription gate on every action.
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	// Any menu navigation cancels an in-progress wizard.
	a.sessions.clear(cq.From.ID)

	action := strings.TrimPrefix(cq.Data, "menu:")
	if action != "home" {
		a.recordUsage(ctx, cq.From.ID, action) // analytics; best-effort
	}

	switch action {
	case "home":
		a.send(ctx, cq.From.ID, a.mainMenuScreen())
	case "announcements":
		a.send(ctx, cq.From.ID, a.announcementsScreen(ctx, cq.From.ID, "all"))
	case "search":
		a.sessions.set(cq.From.ID, stateAwaitQuery)
		a.send(ctx, cq.From.ID, searchPromptScreen())
	case "extsearch":
		a.send(ctx, cq.From.ID, extSearchScreen())
	case "websearch":
		a.sessions.set(cq.From.ID, stateAwaitWebSearch)
		a.send(ctx, cq.From.ID, a.webSearchPromptScreen())
	case "ytsummary":
		a.sessions.set(cq.From.ID, stateAwaitYouTube)
		a.send(ctx, cq.From.ID, a.youtubePromptScreen())
	case "cite":
		a.sessions.set(cq.From.ID, stateAwaitDOI)
		a.send(ctx, cq.From.ID, citePromptScreen())
	case "journal":
		a.sessions.set(cq.From.ID, stateAwaitJournal)
		a.send(ctx, cq.From.ID, journalPromptScreen())
	case "author":
		a.sessions.set(cq.From.ID, stateAwaitAuthor)
		a.send(ctx, cq.From.ID, authorPromptScreen())
	case "oa":
		a.sessions.set(cq.From.ID, stateAwaitOADOI)
		a.send(ctx, cq.From.ID, oaPromptScreen())
	case "ai":
		a.send(ctx, cq.From.ID, a.aiMenuScreen(cq.From.ID))
	case "promotion":
		a.send(ctx, cq.From.ID, a.promotionIntroScreen())
	case "publish":
		a.sessions.set(cq.From.ID, stateAwaitPublish)
		a.send(ctx, cq.From.ID, publishPromptScreen())
	case "litreview":
		a.sessions.set(cq.From.ID, stateAwaitLitReview)
		a.send(ctx, cq.From.ID, litReviewPromptScreen())
	case "gap":
		a.sessions.set(cq.From.ID, stateAwaitGap)
		a.send(ctx, cq.From.ID, gapPromptScreen())
	case "similarity":
		a.sessions.set(cq.From.ID, stateAwaitSimilarity)
		a.send(ctx, cq.From.ID, similarityPromptScreen())
	case "stats":
		a.send(ctx, cq.From.ID, statsMenuScreen())
	case "latex":
		a.sessions.set(cq.From.ID, stateAwaitLatexFile)
		a.send(ctx, cq.From.ID, latexPromptScreen())
	case "viva":
		a.sessions.set(cq.From.ID, stateAwaitVivaFile)
		a.send(ctx, cq.From.ID, vivaPromptScreen())
	case "pdfchat":
		if a.embed == nil {
			a.send(ctx, cq.From.ID, Screen{
				Text:     "📄 محادثة الـ PDF غير مفعّلة حالياً (يلزم نموذج embeddings).",
				Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع", Data: "menu:home"}}}},
			})
			return
		}
		a.sessions.set(cq.From.ID, stateAwaitPdfFile)
		a.send(ctx, cq.From.ID, pdfChatPromptScreen())
	case "library":
		a.send(ctx, cq.From.ID, a.libraryScreen(ctx, cq.From.ID))
	case "semantic":
		if a.embed == nil {
			a.send(ctx, cq.From.ID, Screen{
				Text:     "🧠 البحث الدلالي غير مفعّل حالياً (يلزم نموذج embeddings).",
				Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع", Data: "menu:home"}}}},
			})
			return
		}
		a.sessions.set(cq.From.ID, stateAwaitSemantic)
		a.send(ctx, cq.From.ID, semanticPromptScreen())
	case "follow":
		a.send(ctx, cq.From.ID, a.followScreen(ctx, cq.From.ID))
	case "trends":
		a.sessions.set(cq.From.ID, stateAwaitTrend)
		a.send(ctx, cq.From.ID, trendsPromptScreen())
	case "radar":
		a.sessions.set(cq.From.ID, stateAwaitRadar)
		a.send(ctx, cq.From.ID, radarPromptScreen())
	case "humanize":
		a.sessions.set(cq.From.ID, stateAwaitHumanize)
		a.send(ctx, cq.From.ID, humanizePromptScreen(a.isPremiumUser(ctx, cq.From.ID)))
	case "pdf2word":
		a.sessions.set(cq.From.ID, stateAwaitPdf2Word)
		a.send(ctx, cq.From.ID, pdf2wordPromptScreen())
	case "slides":
		a.sessions.set(cq.From.ID, stateAwaitSlides)
		a.send(ctx, cq.From.ID, slidesPromptScreen())
	case "similar":
		a.sessions.set(cq.From.ID, stateAwaitSimilarDOI)
		a.send(ctx, cq.From.ID, similarPromptScreen())
	case "retracted":
		a.sessions.set(cq.From.ID, stateAwaitRetractDOI)
		a.send(ctx, cq.From.ID, retractPromptScreen())
	case "subscribe":
		a.send(ctx, cq.From.ID, subscribeScreen(a.settings.Get(), a.plans.List()))
	case "support":
		a.sessions.set(cq.From.ID, stateAwaitSupport)
		a.send(ctx, cq.From.ID, supportPromptScreen())
	case "help":
		a.send(ctx, cq.From.ID, helpScreen())
	default:
		a.send(ctx, cq.From.ID, a.mainMenuScreen())
	}
}

// ensureUser registers a first-time user in the store.
func (a *App) ensureUser(ctx context.Context, from *models.User) {
	if from == nil {
		return
	}
	if _, err := a.store.GetUser(ctx, from.ID); err == nil {
		return
	}
	name := strings.TrimSpace(from.FirstName + " " + from.LastName)
	_ = a.store.SaveUser(ctx, &domain.User{
		TelegramID: from.ID,
		Name:       name,
		Tier:       domain.TierFree,
		CreatedAt:  time.Now(),
	})
}

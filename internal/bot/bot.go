// Package bot is the Telegram adapter: it renders screens and routes updates to
// the core services. It is a thin layer over a shared core (web adapter later).
package bot

import (
	"context"
	"log"
	"strings"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/embed"
	"github.com/erticaz/manhal/internal/journal"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/predator"
	"github.com/erticaz/manhal/internal/promotion"
	"github.com/erticaz/manhal/internal/store"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Deps groups everything the Telegram adapter needs. Using a struct keeps the
// constructor readable as the adapter grows.
type Deps struct {
	Config      *config.Config
	Settings    *config.SettingsManager
	Store       store.Store
	AI          ai.Provider
	Citations   CitationSource
	Search      PaperSearch
	Authors     AuthorSearch
	AuthorWorks AuthorWorksSource
	Trends      TrendSource
	OA          OAFinder
	Journals    *journal.Index
	Predators   *predator.List
	Promotion   *promotion.Rules
	Announce    *announce.Repo
	Disciplines []config.Discipline
	Menu        *menu.Manager
	Embed       embed.Provider // nil disables semantic features
}

// App wires the Telegram adapter with the core dependencies.
type App struct {
	bot          *tg.Bot
	cfg          *config.Config
	settings     *config.SettingsManager
	store        store.Store
	ai           ai.Provider
	cite         CitationSource
	search       PaperSearch
	authorSearch AuthorSearch
	authorWorks  AuthorWorksSource
	trends       TrendSource
	oa           OAFinder
	journals     *journal.Index
	predators    *predator.List
	promotion    *promotion.Rules
	announce     *announce.Repo
	disciplines  []config.Discipline
	menu         *menu.Manager
	embed        embed.Provider
	usage        *usageLimiter
	sessions     *sessions
}

// New constructs the bot application and registers handlers.
func New(d Deps) (*App, error) {
	a := &App{
		cfg:          d.Config,
		settings:     d.Settings,
		store:        d.Store,
		ai:           d.AI,
		cite:         d.Citations,
		search:       d.Search,
		authorSearch: d.Authors,
		authorWorks:  d.AuthorWorks,
		trends:       d.Trends,
		oa:           d.OA,
		journals:     d.Journals,
		predators:    d.Predators,
		promotion:    d.Promotion,
		announce:     d.Announce,
		disciplines:  d.Disciplines,
		menu:         d.Menu,
		embed:        d.Embed,
		sessions:     newSessions(),
	}
	// Tier-aware daily AI quota (free vs premium), resolved per user at call time.
	a.usage = newUsageLimiter(a.aiLimit)

	b, err := tg.New(d.Config.BotToken, tg.WithDefaultHandler(a.defaultHandler))
	if err != nil {
		return nil, err
	}
	a.bot = b

	b.RegisterHandler(tg.HandlerTypeMessageText, "/start", tg.MatchTypeExact, a.handleStart)
	b.RegisterHandler(tg.HandlerTypeMessageText, "/admin", tg.MatchTypeExact, a.handleAdminCommand)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "gate:check", tg.MatchTypeExact, a.handleGateCheck)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "premium:request", tg.MatchTypeExact, a.handlePremiumRequest)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "menu:", tg.MatchTypePrefix, a.handleMenu)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "nav:", tg.MatchTypePrefix, a.handleNav)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "admin:", tg.MatchTypePrefix, a.handleAdmin)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "ann:", tg.MatchTypePrefix, a.handleAnnouncements)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "field:", tg.MatchTypePrefix, a.handleField)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "search:cite:", tg.MatchTypePrefix, a.handleSearchCite)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "author:profile:", tg.MatchTypePrefix, a.handleAuthorProfile)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "ai:tool:", tg.MatchTypePrefix, a.handleAITool)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "promo:", tg.MatchTypePrefix, a.handlePromotion)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "stats:test:", tg.MatchTypePrefix, a.handleStats)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "lib:", tg.MatchTypePrefix, a.handleLibrary)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "sub:", tg.MatchTypePrefix, a.handleFollow)
	b.RegisterHandler(tg.HandlerTypeCallbackQueryData, "cwatch:", tg.MatchTypePrefix, a.handleCitationWatch)

	return a, nil
}

// Run starts long polling until the context is cancelled.
func (a *App) Run(ctx context.Context) {
	a.logf("منهل bot starting (ai provider=%s)", a.ai.Name())
	a.bot.Start(ctx)
}

func (a *App) logf(format string, args ...any) {
	log.Printf(format, args...)
}

// send delivers a Screen to a chat. We send plain text (no parse_mode) for
// reliability, so any Markdown the AI emits (**bold**, ### headings) is cleaned
// to avoid showing raw symbols.
func (a *App) send(ctx context.Context, chatID int64, s Screen) {
	params := &tg.SendMessageParams{ChatID: chatID, Text: cleanMarkdown(s.Text)}
	if m := s.Keyboard.markup(); m != nil {
		params.ReplyMarkup = m
	}
	if _, err := a.bot.SendMessage(ctx, params); err != nil {
		a.logf("send error: %v", err)
	}
}

// cleanMarkdown removes Markdown emphasis/heading markers that Telegram shows
// literally in plain-text mode: **bold** -> bold, "### Title" -> "Title". Only
// "#" runs followed by a space are treated as headings, so "#1" is left intact.
func cleanMarkdown(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " ")
		h := 0
		for h < len(trimmed) && trimmed[h] == '#' {
			h++
		}
		if h > 0 && h < len(trimmed) && trimmed[h] == ' ' {
			lines[i] = strings.TrimSpace(trimmed[h+1:])
		}
	}
	return strings.Join(lines, "\n")
}

// defaultHandler routes any unrouted message: it either feeds an active wizard
// (e.g. DOI entry) or falls back to showing the main menu.
func (a *App) defaultHandler(ctx context.Context, _ *tg.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	a.ensureUser(ctx, msg.From)

	if !a.isSubscribed(ctx, msg.From.ID) {
		a.send(ctx, msg.Chat.ID, gateScreen(a.settings.Get()))
		return
	}

	// Uploaded documents are meaningful only to the file-based tools.
	if msg.Document != nil {
		switch a.sessions.get(msg.From.ID) {
		case stateAwaitLatexFile:
			a.sessions.clear(msg.From.ID)
			a.handleLatexDocument(ctx, msg)
		case stateAwaitVivaFile:
			a.sessions.clear(msg.From.ID)
			a.handleVivaDocument(ctx, msg)
		case stateAwaitPdfFile:
			a.handlePdfUpload(ctx, msg) // keeps the session (enters chat mode)
		default:
			a.send(ctx, msg.Chat.ID, a.mainMenuScreen())
		}
		return
	}

	switch a.sessions.get(msg.From.ID) {
	case stateAwaitLatexFile:
		a.send(ctx, msg.Chat.ID, Screen{Text: "أرسل ملفاً ‎.tex‎ أو ‎.docx‎ (وليس نصاً).", Keyboard: latexNav()})
		return
	case stateAwaitVivaFile:
		a.send(ctx, msg.Chat.ID, Screen{Text: "ارفع ملف الأطروحة ‎.docx‎ أو ‎.tex‎ (وليس نصاً).", Keyboard: vivaNav()})
		return
	case stateAwaitPdfFile:
		a.send(ctx, msg.Chat.ID, Screen{Text: "ارفع ملف ‎.pdf‎ (وليس نصاً).", Keyboard: pdfNav()})
		return
	case statePdfChat:
		a.handlePdfQuestion(ctx, msg) // stays in chat mode
		return
	case stateAwaitDOI:
		a.sessions.clear(msg.From.ID)
		a.handleCiteDOI(ctx, msg)
		return
	case stateAwaitQuery:
		a.handleSearchQuery(ctx, msg) // stores results; keeps the session alive
		return
	case stateAwaitJournal:
		a.sessions.clear(msg.From.ID)
		a.handleJournalQuery(ctx, msg)
		return
	case stateAwaitAuthor:
		a.handleAuthorQuery(ctx, msg) // stores results; keeps the session alive
		return
	case stateAwaitOADOI:
		a.sessions.clear(msg.From.ID)
		a.handleOADOI(ctx, msg)
		return
	case stateAwaitAIInput:
		a.handleAIInput(ctx, msg)
		return
	case stateAwaitPromotion:
		a.handlePromotionActivities(ctx, msg)
		return
	case stateAwaitPromoCount:
		a.handlePromoCount(ctx, msg)
		return
	case stateAwaitPublish:
		a.sessions.clear(msg.From.ID)
		a.handlePublishAbstract(ctx, msg)
		return
	case stateAwaitLitReview:
		a.sessions.clear(msg.From.ID)
		a.handleLitReviewTopic(ctx, msg)
		return
	case stateAwaitStats:
		a.handleStatsData(ctx, msg)
		return
	case stateAwaitSupport:
		a.sessions.clear(msg.From.ID)
		a.handleSupportMessage(ctx, msg)
		return
	case stateAwaitFollowTopic:
		a.sessions.clear(msg.From.ID)
		a.handleFollowTopic(ctx, msg)
		return
	case stateAwaitTrend:
		a.sessions.clear(msg.From.ID)
		a.handleTrendTopic(ctx, msg)
		return
	case stateAwaitGap:
		a.sessions.clear(msg.From.ID)
		a.handleGapTopic(ctx, msg)
		return
	case stateAwaitRadar:
		a.sessions.clear(msg.From.ID)
		a.handleRadar(ctx, msg)
		return
	case stateAwaitSimilarity:
		a.sessions.clear(msg.From.ID)
		a.handleSimilarityText(ctx, msg)
		return
	case stateAwaitSemantic:
		a.sessions.clear(msg.From.ID)
		a.handleSemanticQuery(ctx, msg)
		return
	case stateAwaitAdminLabel:
		if a.cfg.IsAdmin(msg.From.ID) {
			a.handleAdminLabel(ctx, msg)
			return
		}
		a.sessions.clear(msg.From.ID)
	}

	a.send(ctx, msg.Chat.ID, a.mainMenuScreen())
}

package bot

import (
	"context"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// extSource pairs a picker button with its searcher and the state that awaits
// the user's query for it.
type extSource struct {
	key   string
	label string
	state sessionState
}

var extSources = []extSource{
	{"pubmed", "🧬 PubMed (طبّي/حيوي)", stateAwaitPubmed},
	{"arxiv", "🧪 arXiv (علوم/هندسة)", stateAwaitArxiv},
	{"s2", "🧠 Semantic Scholar (ملخّص TLDR)", stateAwaitS2},
}

// extSearchScreen lets the user pick an additional scholarly source.
func extSearchScreen() Screen {
	rows := make([][]Button, 0, len(extSources)+1)
	for _, s := range extSources {
		rows = append(rows, []Button{{Text: s.label, Data: "ext:" + s.key}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})
	return Screen{
		Text: "🔬 بحث في مصادر علمية إضافية\n\n" +
			"اختر المصدر ثم أرسل كلماتك المفتاحية. هذه المصادر تكمّل البحث الأساسي:\n" +
			"• PubMed: الطب والعلوم الحياتية.\n" +
			"• arXiv: الحوسبة والفيزياء والهندسة والرياضيات (Preprints).\n" +
			"• Semantic Scholar: يضيف ملخّصاً ذكياً (TLDR) لكل ورقة.",
		Keyboard: &Keyboard{Rows: rows},
	}
}

// handleExtSource records the chosen source and asks for the query.
func (a *App) handleExtSource(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}
	key := strings.TrimPrefix(cq.Data, "ext:")
	for _, s := range extSources {
		if s.key == key {
			a.sessions.set(cq.From.ID, s.state)
			a.send(ctx, cq.From.ID, Screen{
				Text: s.label + "\n\nأرسل كلماتك المفتاحية (عنوان/موضوع/مؤلف):",
				Keyboard: &Keyboard{Rows: [][]Button{
					{{Text: "⬅️ المصادر", Data: "menu:extsearch"}},
				}},
			})
			return
		}
	}
	a.send(ctx, cq.From.ID, extSearchScreen())
}

// sourceFor maps a session state to its searcher and display label.
func (a *App) sourceFor(state sessionState) (PaperSearch, string) {
	switch state {
	case stateAwaitPubmed:
		return a.pubmed, "PubMed"
	case stateAwaitArxiv:
		return a.arxiv, "arXiv"
	case stateAwaitS2:
		return a.s2, "Semantic Scholar"
	}
	return nil, ""
}

// runExtSearch searches the chosen external source and renders the results,
// reusing the standard result screen (so the per-result "cite" buttons work).
func (a *App) runExtSearch(ctx context.Context, msg *models.Message, state sessionState) {
	chatID := msg.Chat.ID
	src, label := a.sourceFor(state)
	a.sessions.clear(msg.From.ID)
	if src == nil {
		a.send(ctx, chatID, extSearchScreen())
		return
	}
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل كلمات بحث غير فارغة."})
		return
	}
	a.send(ctx, chatID, Screen{Text: "⏳ جاري البحث في " + label + "..."})

	searchCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	results, err := src.Search(searchCtx, query, searchResultLimit)
	if err != nil {
		a.logf("extsearch %s: %q: %v", label, query, err)
		a.send(ctx, chatID, searchErrorScreen())
		return
	}
	a.sessions.setResults(msg.From.ID, results)
	a.send(ctx, chatID, searchResultsScreen(label+" — "+query, results))
}

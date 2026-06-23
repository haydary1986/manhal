package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/scholar"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// PaperSearch searches scholarly works by free text. The bot depends on this
// small interface so the concrete source (OpenAlex) can be swapped.
type PaperSearch interface {
	Search(ctx context.Context, query string, limit int) ([]scholar.SearchResult, error)
}

// searchResultLimit caps how many results a search returns.
const searchResultLimit = 5

// searchPromptScreen asks the user for a search query.
func searchPromptScreen() Screen {
	return Screen{
		Text: "🔍 بحث عن ورقة\n\n" +
			"أرسل عنوان الورقة أو اسم الباحث أو كلمات مفتاحية،\n" +
			"وسأرجّع لك أقرب النتائج مع إمكانية توليد الاقتباس فوراً.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleSearchQuery runs the search for the text the user just sent.
func (a *App) handleSearchQuery(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	query := strings.TrimSpace(msg.Text)
	a.send(ctx, chatID, Screen{Text: "⏳ جاري البحث..."})

	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	results, err := a.search.Search(searchCtx, query, searchResultLimit)
	if err != nil {
		a.logf("search: %q: %v", query, err)
		a.send(ctx, chatID, searchErrorScreen())
		return
	}
	a.sessions.setResults(msg.From.ID, results)
	a.send(ctx, chatID, searchResultsScreen(query, results))
}

// searchErrorScreen is shown when the search backend fails.
func searchErrorScreen() Screen {
	return Screen{
		Text: "❌ صار خلل بالبحث ⚠️\nجرّب بعد لحظات أو بصياغة مختلفة.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔁 بحث جديد", Data: "menu:search"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// searchResultsScreen renders the result list with a "cite" button per result
// that carries a DOI.
func searchResultsScreen(query string, results []scholar.SearchResult) Screen {
	if len(results) == 0 {
		return Screen{
			Text: "🔍 لا توجد نتائج لـ «" + query + "».\nجرّب كلمات أوسع أو تهجئة مختلفة.",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "🔁 بحث جديد", Data: "menu:search"}},
				{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
			}},
		}
	}

	var b strings.Builder
	b.WriteString("🔍 نتائج «" + query + "»:\n")
	rows := [][]Button{}
	for i, r := range results {
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + renderSearchResult(r) + "\n")
		if r.DOI != "" {
			rows = append(rows, []Button{{
				Text: "📝 اقتبس #" + strconv.Itoa(i+1),
				Data: "search:cite:" + strconv.Itoa(i),
			}})
		}
	}
	rows = append(rows, []Button{{Text: "🔁 بحث جديد", Data: "menu:search"}})
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})

	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// renderSearchResult formats one result as a text block.
func renderSearchResult(r scholar.SearchResult) string {
	var b strings.Builder
	b.WriteString(r.Title + "\n")

	meta := []string{}
	if authors := authorsLine(r.Authors); authors != "" {
		meta = append(meta, authors)
	}
	if r.Year > 0 {
		meta = append(meta, strconv.Itoa(r.Year))
	}
	if r.Venue != "" {
		meta = append(meta, r.Venue)
	}
	if len(meta) > 0 {
		b.WriteString("   " + strings.Join(meta, " · ") + "\n")
	}
	if r.Summary != "" {
		b.WriteString("   💡 " + r.Summary + "\n")
	}

	b.WriteString("   📊 الاستشهادات: " + strconv.Itoa(r.CitedBy))
	if r.DOI == "" {
		if r.URL != "" {
			b.WriteString("\n   🔗 " + r.URL)
		} else {
			b.WriteString("\n   ⚠️ بدون DOI — الاقتباس غير متاح")
		}
	}
	return b.String()
}

// authorsLine renders up to three author names, then "وآخرون".
func authorsLine(authors []string) string {
	switch len(authors) {
	case 0:
		return ""
	case 1, 2, 3:
		return strings.Join(authors, "، ")
	default:
		return strings.Join(authors[:3], "، ") + " وآخرون"
	}
}

// handleSearchCite generates a citation for a chosen search result.
func (a *App) handleSearchCite(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	idx, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "search:cite:"))
	if err != nil {
		return
	}
	result, ok := a.sessions.resultAt(cq.From.ID, idx)
	if !ok || result.DOI == "" {
		a.send(ctx, cq.From.ID, Screen{
			Text: "انتهت صلاحية النتائج 🔄\nأعد البحث من جديد.",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "🔁 بحث جديد", Data: "menu:search"}},
				{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
			}},
		})
		return
	}
	a.fetchAndRenderCitation(ctx, cq.From.ID, result.DOI)
}

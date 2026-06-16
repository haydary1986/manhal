package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// CitationSource resolves a DOI to bibliographic metadata. The bot depends on
// this small interface so the concrete fetcher (Crossref) can be swapped.
type CitationSource interface {
	FetchByDOI(ctx context.Context, doi string) (*cite.Work, error)
}

// citePromptScreen asks the user to send a DOI.
func citePromptScreen() Screen {
	return Screen{
		Text: "📝 مولّد الاقتباس التلقائي\n\n" +
			"أرسل لي معرّف الورقة (DOI) وسأرجّع لك الاقتباس بكل الصيغ:\n" +
			"APA · MLA · Chicago · Harvard · IEEE · Vancouver + BibTeX.\n\n" +
			"مثال:\n10.1038/nphys1170\nأو https://doi.org/10.1038/nphys1170",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// citeResultScreen renders all citation styles plus a BibTeX block.
func citeResultScreen(w *cite.Work) Screen {
	var b strings.Builder
	title := strings.TrimSpace(w.Title)
	if title == "" {
		title = w.DOI
	}
	b.WriteString("📚 اقتباسات الورقة:\n«" + title + "»\n")

	for _, s := range cite.All(*w) {
		b.WriteString("\n🔹 " + s.Name + "\n" + s.Text + "\n")
	}
	b.WriteString("\n🔹 BibTeX\n" + cite.BibTeX(*w) + "\n")

	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⭐ احفظ بمكتبتي", Data: "lib:save"}},
			{{Text: "📝 اقتباس آخر", Data: "menu:cite"}, {Text: "⬅️ القائمة", Data: "menu:home"}},
		}},
	}
}

// citeErrorScreen maps a fetch error to a friendly Arabic message.
func citeErrorScreen(err error) Screen {
	var msg string
	switch {
	case errors.Is(err, scholar.ErrInvalidDOI):
		msg = "هذا لا يبدو معرّف DOI صحيحاً 🤔\n" +
			"المعرّف يبدأ بـ «10.» ويحتوي «/». جرّب مرّة أخرى."
	case errors.Is(err, scholar.ErrNotFound):
		msg = "ما لقيت ورقة بهذا الـ DOI 🔍\n" +
			"تأكّد من المعرّف أو جرّب نسخه من صفحة الناشر."
	default:
		msg = "صار خلل بجلب البيانات ⚠️\nجرّب بعد لحظات."
	}
	return Screen{
		Text: "❌ " + msg,
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔁 جرّب DOI آخر", Data: "menu:cite"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleCiteDOI fetches and renders citations for the DOI the user just sent.
func (a *App) handleCiteDOI(ctx context.Context, msg *models.Message) {
	a.fetchAndRenderCitation(ctx, msg.Chat.ID, strings.TrimSpace(msg.Text))
}

// fetchAndRenderCitation resolves a DOI and sends the citation screen. Shared by
// the DOI wizard and the "cite this search result" button.
func (a *App) fetchAndRenderCitation(ctx context.Context, chatID int64, doi string) {
	a.send(ctx, chatID, Screen{Text: "⏳ جاري الجلب..."})

	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	work, err := a.cite.FetchByDOI(fetchCtx, doi)
	if err != nil {
		a.logf("cite: fetch %q: %v", doi, err)
		a.send(ctx, chatID, citeErrorScreen(err))
		return
	}
	a.sessions.setLastWork(chatID, work) // enable "save to library"
	a.send(ctx, chatID, citeResultScreen(work))
}

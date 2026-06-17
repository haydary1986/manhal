package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/embed"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// libraryDisplayLimit caps how many items the list screen shows with controls.
const libraryDisplayLimit = 8

// handleLibrary routes "lib:" callbacks (save / export / remove / open).
func (a *App) handleLibrary(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	data := strings.TrimPrefix(cq.Data, "lib:")
	switch {
	case data == "save":
		a.saveToLibrary(ctx, cq.From.ID)
	case data == "export:bibtex":
		a.exportLibrary(ctx, cq.From.ID, "bibtex")
	case data == "export:ris":
		a.exportLibrary(ctx, cq.From.ID, "ris")
	case strings.HasPrefix(data, "rm:"):
		_ = a.store.RemoveLibraryItem(ctx, cq.From.ID, strings.TrimPrefix(data, "rm:"))
		a.send(ctx, cq.From.ID, a.libraryScreen(ctx, cq.From.ID))
	default:
		a.send(ctx, cq.From.ID, a.libraryScreen(ctx, cq.From.ID))
	}
}

// saveToLibrary stores the most recently fetched citation.
func (a *App) saveToLibrary(ctx context.Context, userID int64) {
	work := a.sessions.lastWork(userID)
	if work == nil {
		a.send(ctx, userID, Screen{Text: "لا يوجد اقتباس لحفظه. ولّد اقتباساً أولاً.", Keyboard: libraryNav()})
		return
	}
	item := newLibraryItem(*work)
	if a.embed != nil {
		if v, err := embed.EmbedOne(ctx, a.embed, libraryEmbedText(item)); err == nil {
			item.Vector = v
		} else {
			a.logf("library embed: %v", err)
		}
	}
	if err := a.store.AddLibraryItem(ctx, userID, item); err != nil {
		a.logf("library save: %v", err)
		a.send(ctx, userID, Screen{Text: "❌ تعذّر الحفظ ⚠️", Keyboard: libraryNav()})
		return
	}
	a.send(ctx, userID, Screen{
		Text:     "⭐ تم حفظ المرجع في مكتبتك.",
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⭐ افتح مكتبتي", Data: "lib:open"}}, {{Text: "⬅️ القائمة", Data: "menu:home"}}}},
	})
}

// libraryScreen lists the user's saved references with export and remove actions.
func (a *App) libraryScreen(ctx context.Context, userID int64) Screen {
	items, _ := a.store.ListLibrary(ctx, userID)
	if len(items) == 0 {
		return Screen{
			Text: "⭐ مكتبتك فارغة.\n\nولّد اقتباساً (📝) ثم اضغط «احفظ بمكتبتي».",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "📝 توليد اقتباس", Data: "menu:cite"}},
				{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
			}},
		}
	}

	var b strings.Builder
	b.WriteString("⭐ مكتبتي (" + strconv.Itoa(len(items)) + " مرجع)\n")
	rows := [][]Button{}
	for i, it := range items {
		if i == libraryDisplayLimit {
			b.WriteString("\n…و" + strconv.Itoa(len(items)-libraryDisplayLimit) + " مرجعاً آخر (تظهر بالتصدير).")
			break
		}
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + libraryItemLine(it))
		rows = append(rows, []Button{{Text: "🗑️ حذف #" + strconv.Itoa(i+1), Data: "lib:rm:" + it.ID}})
	}
	rows = append(rows,
		[]Button{{Text: "📥 تصدير BibTeX", Data: "lib:export:bibtex"}, {Text: "📥 تصدير RIS", Data: "lib:export:ris"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// libraryItemLine renders one saved reference as a compact line.
func libraryItemLine(it domain.LibraryItem) string {
	title := strings.TrimSpace(it.Work.Title)
	if title == "" {
		title = it.Work.DOI
	}
	line := title
	if it.Work.Year > 0 {
		line += " (" + strconv.Itoa(it.Work.Year) + ")"
	}
	if len(it.Tags) > 0 {
		line += "\n   🏷️ " + strings.Join(it.Tags, "، ")
	}
	return line
}

// exportLibrary builds a BibTeX or RIS file from the whole library and sends it.
func (a *App) exportLibrary(ctx context.Context, userID int64, format string) {
	items, _ := a.store.ListLibrary(ctx, userID)
	if len(items) == 0 {
		a.send(ctx, userID, Screen{Text: "مكتبتك فارغة — لا شيء للتصدير.", Keyboard: libraryNav()})
		return
	}

	entries := make([]string, 0, len(items))
	for _, it := range items {
		if format == "ris" {
			entries = append(entries, cite.RIS(it.Work))
		} else {
			entries = append(entries, cite.BibTeX(it.Work))
		}
	}
	content := strings.Join(entries, "\n\n") + "\n"
	filename, caption := "library.bib", "📥 مكتبتك بصيغة BibTeX"
	if format == "ris" {
		filename, caption = "library.ris", "📥 مكتبتك بصيغة RIS"
	}
	a.sendDocument(ctx, userID, filename, []byte(content), caption)
}

// newLibraryItem builds a savable item from a work, with a stable id and
// auto-keywords (#21).
func newLibraryItem(w cite.Work) domain.LibraryItem {
	return domain.LibraryItem{ID: libraryID(w), Work: w, Tags: cite.Keywords(w)}
}

// libraryID derives a short stable id from the DOI (or title).
func libraryID(w cite.Work) string {
	seed := strings.TrimSpace(w.DOI)
	if seed == "" {
		seed = strings.ToLower(strings.TrimSpace(w.Title))
	}
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:10]
}

func libraryNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "⭐ مكتبتي", Data: "lib:open"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

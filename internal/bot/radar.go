package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// AuthorWorksSource fetches a researcher's most-cited works (#8).
type AuthorWorksSource interface {
	AuthorWorks(ctx context.Context, authorID string, limit int) ([]scholar.SearchResult, error)
}

const radarWorksLimit = 5

// radarPromptScreen asks for a researcher name.
func radarPromptScreen() Screen {
	return Screen{
		Text: "📡 رادار البحث — لوحة تأثيرك\n\n" +
			"أرسل اسم الباحث (بالإنجليزية غالباً)، وسأعرض\n" +
			"مؤشّرات تأثيره وأبرز أعماله الأعلى استشهاداً.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleRadar builds the impact dashboard for the named researcher.
func (a *App) handleRadar(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	name := strings.TrimSpace(msg.Text)
	a.send(ctx, chatID, Screen{Text: "⏳ جاري بناء رادار البحث..."})

	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	authors, err := a.authorSearch.SearchAuthors(searchCtx, name, 1)
	if err != nil {
		a.logf("radar author: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خلل بالبحث ⚠️ جرّب لاحقاً.", Keyboard: radarNav()})
		return
	}
	if len(authors) == 0 {
		a.send(ctx, chatID, Screen{Text: "🔍 ما لقيت باحثاً بهذا الاسم. جرّب بالإنجليزية.", Keyboard: radarNav()})
		return
	}

	author := authors[0]
	a.sessions.setLastAuthor(msg.From.ID, &author) // enable "follow citations"

	var works []scholar.SearchResult
	if a.authorWorks != nil && author.ID != "" {
		works, _ = a.authorWorks.AuthorWorks(searchCtx, author.ID, radarWorksLimit)
	}
	a.send(ctx, chatID, radarScreen(author, works))
}

// radarScreen renders the impact dashboard plus the researcher's top works.
func radarScreen(au scholar.Author, works []scholar.SearchResult) Screen {
	var b strings.Builder
	b.WriteString("📡 رادار البحث — " + au.Name + "\n")
	if au.Institution != "" {
		b.WriteString("🏛️ " + au.Institution + "\n")
	}

	b.WriteString("\n📊 لوحة التأثير:\n")
	b.WriteString("   • h-index: " + strconv.Itoa(au.HIndex))
	if au.I10Index > 0 {
		b.WriteString(" · i10: " + strconv.Itoa(au.I10Index))
	}
	b.WriteString("\n   • الأبحاث: " + strconv.Itoa(au.WorksCount) +
		" · الاستشهادات: " + strconv.Itoa(au.CitedBy) + "\n")
	if len(au.Concepts) > 0 {
		b.WriteString("🏷️ مجالات: " + strings.Join(au.Concepts, "، ") + "\n")
	}
	if au.ORCID != "" {
		b.WriteString("🆔 ORCID: " + au.ORCID + "\n")
	}

	if len(works) > 0 {
		b.WriteString("\n📚 أبرز أعماله (الأعلى استشهاداً):\n")
		for i, w := range works {
			b.WriteString(strconv.Itoa(i+1) + ") " + radarWorkLine(w) + "\n")
		}
	}
	b.WriteString("\nℹ️ بيانات من OpenAlex — قد تختلف عن Scopus/WoS.")

	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔔 تابع استشهاداته", Data: "cwatch:add"}},
			{{Text: "📡 باحث آخر", Data: "menu:radar"}, {Text: "⬅️ القائمة", Data: "menu:home"}},
		}},
	}
}

// radarWorkLine renders a compact "title (year) — N citations" line.
func radarWorkLine(w scholar.SearchResult) string {
	line := w.Title
	if w.Year > 0 {
		line += " (" + strconv.Itoa(w.Year) + ")"
	}
	line += " — 📊 " + strconv.Itoa(w.CitedBy)
	if w.DOI != "" {
		line += "\n   https://doi.org/" + w.DOI
	}
	return line
}

func radarNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📡 باحث آخر", Data: "menu:radar"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

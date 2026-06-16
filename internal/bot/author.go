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

// AuthorSearch finds researcher profiles by name. The bot depends on this small
// interface so the concrete source (OpenAlex) can be swapped.
type AuthorSearch interface {
	SearchAuthors(ctx context.Context, name string, limit int) ([]scholar.Author, error)
}

const authorResultLimit = 5

// authorPromptScreen asks for a researcher name.
func authorPromptScreen() Screen {
	return Screen{
		Text: "👤 ملف الباحث (h-index)\n\n" +
			"أرسل اسم الباحث (بالإنجليزية غالباً كما يظهر بأبحاثه)،\n" +
			"وسأرجّع لك مؤشّر h-index وعدد الأبحاث والاستشهادات.\n\n" +
			"مثال:\nYann LeCun",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleAuthorQuery searches for the name the user just sent.
func (a *App) handleAuthorQuery(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	name := strings.TrimSpace(msg.Text)
	a.send(ctx, chatID, Screen{Text: "⏳ جاري البحث عن الباحث..."})

	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	authors, err := a.authorSearch.SearchAuthors(searchCtx, name, authorResultLimit)
	if err != nil {
		a.logf("author search: %q: %v", name, err)
		a.send(ctx, chatID, authorErrorScreen())
		return
	}
	a.sessions.setAuthors(msg.From.ID, authors)

	switch len(authors) {
	case 0:
		a.send(ctx, chatID, authorNotFoundScreen(name))
	case 1:
		a.sessions.setLastAuthor(msg.From.ID, &authors[0])
		a.send(ctx, chatID, authorProfileScreen(authors[0]))
	default:
		a.send(ctx, chatID, authorChoicesScreen(name, authors))
	}
}

// handleAuthorProfile shows the profile for a chosen disambiguation entry.
func (a *App) handleAuthorProfile(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	idx, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "author:profile:"))
	if err != nil {
		return
	}
	author, ok := a.sessions.authorAt(cq.From.ID, idx)
	if !ok {
		a.send(ctx, cq.From.ID, Screen{
			Text:     "انتهت صلاحية النتائج 🔄\nأعد البحث من جديد.",
			Keyboard: authorNavKeyboard(),
		})
		return
	}
	a.sessions.setLastAuthor(cq.From.ID, &author)
	a.send(ctx, cq.From.ID, authorProfileScreen(author))
}

// authorChoicesScreen lists same-name matches for disambiguation.
func authorChoicesScreen(name string, authors []scholar.Author) Screen {
	var b strings.Builder
	b.WriteString("👥 عدّة باحثين بالاسم «" + name + "». اختر الملف:\n")
	rows := [][]Button{}
	for i, au := range authors {
		desc := au.Name
		if au.Institution != "" {
			desc += " — " + au.Institution
		}
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + desc +
			"\n   📄 أبحاث: " + strconv.Itoa(au.WorksCount) + " · h-index: " + strconv.Itoa(au.HIndex) + "\n")
		rows = append(rows, []Button{{
			Text: "📊 الملف #" + strconv.Itoa(i+1),
			Data: "author:profile:" + strconv.Itoa(i),
		}})
	}
	rows = append(rows, []Button{{Text: "🔁 بحث جديد", Data: "menu:author"}})
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})
	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// authorProfileScreen renders a single researcher's metrics card.
func authorProfileScreen(au scholar.Author) Screen {
	var b strings.Builder
	b.WriteString("👤 " + au.Name + "\n")
	if au.Institution != "" {
		b.WriteString("🏛️ " + au.Institution + "\n")
	}
	b.WriteString("\n📈 المؤشّرات:\n")
	b.WriteString("   • h-index: " + strconv.Itoa(au.HIndex) + "\n")
	if au.I10Index > 0 {
		b.WriteString("   • i10-index: " + strconv.Itoa(au.I10Index) + "\n")
	}
	b.WriteString("   • عدد الأبحاث: " + strconv.Itoa(au.WorksCount) + "\n")
	b.WriteString("   • الاستشهادات: " + strconv.Itoa(au.CitedBy) + "\n")
	if len(au.Concepts) > 0 {
		b.WriteString("🏷️ مجالات البحث: " + strings.Join(au.Concepts, "، ") + "\n")
	}
	if au.ORCID != "" {
		b.WriteString("🆔 ORCID: " + au.ORCID + "\n")
	}
	b.WriteString("\nℹ️ بيانات من OpenAlex — قد تختلف عن قواعد أخرى (Scopus/WoS).")
	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔔 نبّهني عند استشهاد جديد", Data: "cwatch:add"}},
			{{Text: "🔁 بحث عن باحث آخر", Data: "menu:author"}, {Text: "⬅️ القائمة", Data: "menu:home"}},
		}},
	}
}

func authorNotFoundScreen(name string) Screen {
	return Screen{
		Text: "🔍 ما لقيت باحثاً باسم «" + name + "».\n" +
			"جرّب الاسم بالإنجليزية أو بصيغة مختلفة.",
		Keyboard: authorNavKeyboard(),
	}
}

func authorErrorScreen() Screen {
	return Screen{
		Text:     "❌ صار خلل بالبحث ⚠️\nجرّب بعد لحظات.",
		Keyboard: authorNavKeyboard(),
	}
}

func authorNavKeyboard() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔁 بحث عن باحث آخر", Data: "menu:author"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

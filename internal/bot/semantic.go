package bot

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/embed"
	"github.com/go-telegram/bot/models"
)

// semanticResultLimit caps how many matches a semantic search shows.
const semanticResultLimit = 5

// libraryEmbedText is the text embedded for a saved reference (title + tags +
// venue), so semantic search matches on meaning.
func libraryEmbedText(it domain.LibraryItem) string {
	parts := []string{it.Work.Title}
	if len(it.Tags) > 0 {
		parts = append(parts, strings.Join(it.Tags, " "))
	}
	if it.Work.ContainerTitle != "" {
		parts = append(parts, it.Work.ContainerTitle)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// semanticPromptScreen asks for a natural-language query.
func semanticPromptScreen() Screen {
	return Screen{
		Text: "🧠 بحث دلالي في مكتبتك\n\n" +
			"اكتب وصفاً بالمعنى (وليس بالكلمات الحرفية)،\n" +
			"وسأرجّع أقرب مراجعك المحفوظة دلالياً.\n\n" +
			"مثال: «الذكاء الاصطناعي في التشخيص الطبي»",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleSemanticQuery embeds the query and ranks the user's library by cosine.
func (a *App) handleSemanticQuery(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		a.send(ctx, chatID, Screen{Text: "اكتب وصفاً غير فارغ.", Keyboard: semanticNav()})
		return
	}

	items, _ := a.store.ListLibrary(ctx, msg.From.ID)
	if !hasVectors(items) {
		a.send(ctx, chatID, Screen{
			Text:     "مكتبتك لا تحتوي مراجع مفهرسة دلالياً بعد. احفظ مراجع جديدة لتُفهرَس.",
			Keyboard: semanticNav(),
		})
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري البحث الدلالي..."})
	embCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	qv, err := embed.EmbedOne(embCtx, a.embed, query)
	if err != nil {
		a.logf("semantic embed: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خدمة الفهرسة غير متاحة الآن ⚠️", Keyboard: semanticNav()})
		return
	}

	ranked := rankBySimilarity(items, qv, semanticResultLimit)
	a.send(ctx, chatID, semanticResultScreen(query, ranked))
}

// scoredItem pairs a library item with its similarity to the query.
type scoredItem struct {
	item  domain.LibraryItem
	score float32
}

// rankBySimilarity returns the top items by cosine similarity to qv (items
// without a vector are skipped).
func rankBySimilarity(items []domain.LibraryItem, qv []float32, limit int) []scoredItem {
	scored := make([]scoredItem, 0, len(items))
	for _, it := range items {
		if len(it.Vector) == 0 {
			continue
		}
		scored = append(scored, scoredItem{item: it, score: embed.Cosine(qv, it.Vector)})
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

// semanticResultScreen renders the ranked matches.
func semanticResultScreen(query string, ranked []scoredItem) Screen {
	if len(ranked) == 0 {
		return Screen{Text: "لا توجد مراجع مفهرسة للمطابقة.", Keyboard: semanticNav()}
	}
	var b strings.Builder
	b.WriteString("🧠 أقرب المراجع لـ «" + query + "»:\n")
	for i, s := range ranked {
		title := strings.TrimSpace(s.item.Work.Title)
		if title == "" {
			title = s.item.Work.DOI
		}
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + title)
		if s.item.Work.Year > 0 {
			b.WriteString(" (" + strconv.Itoa(s.item.Work.Year) + ")")
		}
		b.WriteString("\n   🎯 تطابق: " + strconv.Itoa(int(s.score*100)) + "%")
		if s.item.Work.DOI != "" {
			b.WriteString("\n   https://doi.org/" + s.item.Work.DOI)
		}
		b.WriteString("\n")
	}
	return Screen{Text: b.String(), Keyboard: semanticNav()}
}

func hasVectors(items []domain.LibraryItem) bool {
	for _, it := range items {
		if len(it.Vector) > 0 {
			return true
		}
	}
	return false
}

func semanticNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🧠 بحث آخر", Data: "menu:semantic"}},
		{{Text: "⭐ مكتبتي", Data: "lib:open"}, {Text: "⬅️ القائمة", Data: "menu:home"}},
	}}
}

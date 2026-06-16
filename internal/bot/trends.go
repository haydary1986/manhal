package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// TrendSource returns the most-cited recent works for a topic (#6).
type TrendSource interface {
	Trending(ctx context.Context, topic string, limit int) ([]scholar.SearchResult, error)
}

const trendResultLimit = 6

// trendsPromptScreen asks for a field/topic.
func trendsPromptScreen() Screen {
	return Screen{
		Text: "🔥 ترندات المجال\n\n" +
			"أرسل مجالك أو موضوعك، وسأعرض الأكثر استشهاداً\n" +
			"من الأوراق المنشورة خلال آخر ٣ سنوات.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleTrendTopic shows the trending papers for the topic the user sent.
func (a *App) handleTrendTopic(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	topic := strings.TrimSpace(msg.Text)
	a.send(ctx, chatID, Screen{Text: "⏳ جاري جلب الأكثر استشهاداً..."})

	findCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	results, err := a.trends.Trending(findCtx, topic, trendResultLimit)
	if err != nil {
		a.logf("trends: %q: %v", topic, err)
		a.send(ctx, chatID, Screen{Text: "❌ خلل بجلب الترندات ⚠️ جرّب لاحقاً.", Keyboard: trendsNav()})
		return
	}
	a.send(ctx, chatID, trendsResultScreen(topic, results))
}

// trendsResultScreen renders the top-cited recent papers.
func trendsResultScreen(topic string, results []scholar.SearchResult) Screen {
	if len(results) == 0 {
		return Screen{
			Text:     "🔍 لا توجد نتائج لـ «" + topic + "». جرّب كلمات أوسع أو بالإنجليزية.",
			Keyboard: trendsNav(),
		}
	}
	var b strings.Builder
	b.WriteString("🔥 الأكثر استشهاداً في «" + topic + "» (آخر ٣ سنوات):\n")
	for i, r := range results {
		b.WriteString("\n" + strconv.Itoa(i+1) + ") " + renderSearchResult(r) + "\n")
	}
	b.WriteString("\nℹ️ مرتّبة تنازلياً حسب عدد الاستشهادات (OpenAlex).")
	return Screen{Text: b.String(), Keyboard: trendsNav()}
}

func trendsNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔥 مجال آخر", Data: "menu:trends"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

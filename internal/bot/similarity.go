package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// similaritySourceCount is how many related open sources are surfaced.
const similaritySourceCount = 5

// similaritySystemPrompt keeps the tool firmly advisory and anti-evasion (P12).
const similaritySystemPrompt = "أنت مستشار نزاهة أكاديمية. راجع النص المُعطى واكتب تقريراً استرشادياً بالعربية يتضمّن: " +
	"١) المقاطع التي تحتاج استشهاداً (ادعاءات/إحصاءات/تعريفات مقدّمة كحقائق). " +
	"٢) الصياغات العامة أو النمطية التي قد تتشابه مع مصادر شائعة. " +
	"٣) اقتراحات لإعادة الصياغة بأسلوبك الخاص أو لإضافة الاستشهاد المناسب. " +
	"أكّد بوضوح أن هذا ليس فحص انتحال مقابل قواعد بيانات وليس بديلاً عن أدوات مثل Turnitin، " +
	"وأن الهدف تحسين الأصالة وتصحيح التشابه لا إخفاؤه."

// similarityPromptScreen asks for the text to review.
func similarityPromptScreen() Screen {
	return Screen{
		Text: "🔍 فحص التشابه المبدئي (استرشادي)\n\n" +
			"ألصق المقطع الذي تريد مراجعته. سأعرض:\n" +
			"• مصادر مفتوحة ذات صلة لمراجعتها/الاستشهاد بها.\n" +
			"• ملاحظات أصالة واستشهاد.\n\n" +
			"⚠️ ليس بديلاً عن Turnitin — الهدف الإصلاح لا الإخفاء.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleSimilarityText reviews the passage for originality and related sources.
func (a *App) handleSimilarityText(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: similarityNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}
	text, _ = truncateRunes(text, 6000)
	if text == "" {
		a.send(ctx, chatID, Screen{Text: "ألصق نصاً غير فارغ.", Keyboard: similarityNav()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "🔎 جاري البحث عن مصادر ذات صلة والمراجعة..."})

	// Related open sources (best-effort; advisory).
	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	related, _ := a.search.Search(searchCtx, text, similaritySourceCount)
	cancel()

	callCtx, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	defer cancel2()
	advisory, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: similaritySystemPrompt},
		{Role: "user", Content: text},
	})
	if err != nil {
		a.logf("similarity ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, similarityResultScreen(advisory, related))
}

// similarityResultScreen renders the advisory plus related open sources.
func similarityResultScreen(advisory string, related []scholar.SearchResult) Screen {
	var b strings.Builder
	b.WriteString("🔍 مراجعة التشابه والأصالة:\n\n")
	b.WriteString(strings.TrimSpace(advisory))

	if len(related) > 0 {
		b.WriteString("\n\n📚 مصادر مفتوحة ذات صلة (راجعها واستشهد بما يلزم):\n")
		for i, r := range related {
			b.WriteString(strconv.Itoa(i+1) + ". " + referenceLine(r) + "\n")
		}
	}
	b.WriteString("\n⚠️ فحص مبدئي استرشادي مقابل مصادر مفتوحة فقط — ليس بديلاً عن Turnitin.")
	return Screen{Text: b.String(), Keyboard: similarityNav()}
}

func similarityNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔍 نص آخر", Data: "menu:similarity"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

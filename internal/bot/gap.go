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

// gapPaperCount is how many real papers ground the gap analysis.
const gapPaperCount = 8

// gapSystemPrompt steers the model to reason only from the supplied papers (#25).
const gapSystemPrompt = "أنت مستشار بحثي. بناءً على قائمة الأوراق المرفقة حول الموضوع: " +
	"١) لخّص أبرز ما دُرس فعلاً (ما هو مُشبع بحثياً). " +
	"٢) حدّد ٣–٥ فجوات بحثية أو مجالات قليلة الدراسة يستنتجها من غياب التغطية. " +
	"٣) اقترح اتجاهات/أسئلة بحثية مستقبلية لكل فجوة. " +
	"استند إلى الأوراق المُعطاة فقط ولا تختلق مراجع أو نتائج. أجب بالعربية بإيجاز منظّم."

// gapPromptScreen asks for the research topic.
func gapPromptScreen() Screen {
	return Screen{
		Text: "🧩 كاشف الفجوة البحثية\n\n" +
			"أرسل موضوع بحثك، وسأحلّل أوراقاً حقيقية حوله\n" +
			"لاقتراح الفجوات البحثية والاتجاهات غير المطروقة.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleGapTopic performs a grounded research-gap analysis for the topic.
func (a *App) handleGapTopic(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	topic := strings.TrimSpace(msg.Text)

	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: gapNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "🔎 جاري تحليل الأوراق وكشف الفجوات..."})
	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	papers, err := a.search.Search(searchCtx, topic, gapPaperCount)
	if err != nil {
		a.logf("gap search: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خلل بالبحث عن الأوراق ⚠️ جرّب لاحقاً.", Keyboard: gapNav()})
		return
	}
	if len(papers) == 0 {
		a.send(ctx, chatID, Screen{Text: "🔍 ما لقيت أوراقاً لهذا الموضوع. جرّب كلمات أوسع أو بالإنجليزية.", Keyboard: gapNav()})
		return
	}

	callCtx, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	defer cancel2()
	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: gapSystemPrompt},
		{Role: "user", Content: gapUserPrompt(topic, papers)},
	})
	if err != nil {
		a.logf("gap ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, gapResultScreen(reply, papers))
}

// gapUserPrompt embeds the real papers the model must reason from.
func gapUserPrompt(topic string, papers []scholar.SearchResult) string {
	var b strings.Builder
	b.WriteString("الموضوع: " + topic + "\n\nالأوراق المتاحة (استند إليها حصراً):\n")
	for i, p := range papers {
		b.WriteString("[" + strconv.Itoa(i+1) + "] " + referenceLine(p) + "\n")
	}
	return b.String()
}

// gapResultScreen renders the analysis plus the considered papers.
func gapResultScreen(analysis string, papers []scholar.SearchResult) Screen {
	var b strings.Builder
	b.WriteString("🧩 تحليل الفجوة البحثية:\n\n")
	b.WriteString(strings.TrimSpace(analysis))
	b.WriteString("\n\n📚 الأوراق المُحلَّلة:\n")
	for i, p := range papers {
		b.WriteString(strconv.Itoa(i+1) + ". " + referenceLine(p) + "\n")
	}
	b.WriteString("\nℹ️ تحليل استرشادي مبني على عيّنة محدودة — وسّع البحث قبل اعتماد أي فجوة.")
	return Screen{Text: b.String(), Keyboard: gapNav()}
}

func gapNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🧩 موضوع آخر", Data: "menu:gap"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

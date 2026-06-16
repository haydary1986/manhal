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

// litReviewPaperCount is how many real papers ground the review.
const litReviewPaperCount = 8

// litReviewSystemPrompt forces the model to stay grounded in the supplied papers
// so it cannot invent references (P2).
const litReviewSystemPrompt = "أنت باحث أكاديمي. اكتب مسودة مراجعة أدبيات (Literature Review) بالعربية " +
	"حول الموضوع المُعطى، مستنداً حصراً إلى قائمة الأوراق المرفقة. أشر إلى الدراسات بصيغة (المؤلف، السنة). " +
	"نظّم المراجعة في فقرات مترابطة: تمهيد، المحاور الرئيسية، ثم الفجوة البحثية. " +
	"لا تذكر أي مرجع غير وارد في القائمة، ولا تختلق بيانات أو أرقاماً."

// litReviewPromptScreen asks for the research topic.
func litReviewPromptScreen() Screen {
	return Screen{
		Text: "📖 مولّد مراجعة الأدبيات\n\n" +
			"أرسل موضوع بحثك، وسأبحث عن أوراق حقيقية ذات صلة\n" +
			"ثم أصيغ مسودة مراجعة أدبيات مستندة إليها مع قائمة مراجع.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleLitReviewTopic builds a grounded literature review for the topic.
func (a *App) handleLitReviewTopic(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	topic := strings.TrimSpace(msg.Text)

	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: litReviewNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "🔎 جاري البحث عن أوراق ذات صلة..."})
	searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	papers, err := a.search.Search(searchCtx, topic, litReviewPaperCount)
	if err != nil {
		a.logf("litreview search: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خلل بالبحث عن الأوراق ⚠️ جرّب لاحقاً.", Keyboard: litReviewNav()})
		return
	}
	if len(papers) == 0 {
		a.send(ctx, chatID, Screen{
			Text:     "🔍 ما لقيت أوراقاً لهذا الموضوع. جرّب كلمات أوسع أو بالإنجليزية.",
			Keyboard: litReviewNav(),
		})
		return
	}

	a.send(ctx, chatID, Screen{Text: "✍️ جاري صياغة مراجعة الأدبيات..."})
	callCtx, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	defer cancel2()

	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: litReviewSystemPrompt},
		{Role: "user", Content: litReviewUserPrompt(topic, papers)},
	})
	if err != nil {
		a.logf("litreview ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, litReviewResultScreen(reply, papers))
}

// litReviewUserPrompt embeds the real papers the model must rely on.
func litReviewUserPrompt(topic string, papers []scholar.SearchResult) string {
	var b strings.Builder
	b.WriteString("الموضوع: " + topic + "\n\nالأوراق المتاحة (استند إليها حصراً):\n")
	for i, p := range papers {
		b.WriteString("[" + strconv.Itoa(i+1) + "] " + referenceLine(p) + "\n")
	}
	return b.String()
}

// litReviewResultScreen renders the review followed by the real reference list.
func litReviewResultScreen(review string, papers []scholar.SearchResult) Screen {
	var b strings.Builder
	b.WriteString("📖 مسودة مراجعة الأدبيات:\n\n")
	b.WriteString(strings.TrimSpace(review))
	b.WriteString("\n\n📚 المراجع (أوراق حقيقية):\n")
	for i, p := range papers {
		b.WriteString(strconv.Itoa(i+1) + ". " + referenceLine(p) + "\n")
	}
	b.WriteString("\nℹ️ مسودة استرشادية — راجع الاستشهادات ووسّع القائمة قبل الاعتماد.")
	return Screen{Text: b.String(), Keyboard: litReviewNav()}
}

// referenceLine formats one paper as a short reference.
func referenceLine(p scholar.SearchResult) string {
	parts := []string{}
	if au := authorsLine(p.Authors); au != "" {
		parts = append(parts, au)
	}
	if p.Year > 0 {
		parts = append(parts, strconv.Itoa(p.Year))
	}
	head := strings.Join(parts, " · ")
	line := p.Title
	if head != "" {
		line = head + ". " + p.Title
	}
	if p.DOI != "" {
		line += " — https://doi.org/" + p.DOI
	}
	return line
}

func litReviewNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📖 موضوع آخر", Data: "menu:litreview"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

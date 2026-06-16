package bot

import (
	"context"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/predator"
	"github.com/go-telegram/bot/models"
)

// publishSystemPrompt steers DeepSeek to act as a publication advisor (P1).
const publishSystemPrompt = "أنت مستشار نشر أكاديمي خبير. بناءً على الملخّص المُعطى: " +
	"١) حدّد المجال الدقيق للبحث. " +
	"٢) اقترح ٥–٧ مجلات علمية مفهرسة (Scopus/Web of Science) مناسبة للموضوع، ولكلٍّ منها: " +
	"تقدير الربع المتوقّع (Q1–Q4) ونطاق المجلة باختصار. " +
	"٣) نبّه إلى تجنّب المجلات المفترسة، وذكّر بالتحقق من رسوم النشر (APC) ونطاق المجلة قبل التقديم. " +
	"أجب بالعربية بقائمة مرتّبة وواضحة."

// publishPromptScreen asks for the abstract.
func publishPromptScreen() Screen {
	return Screen{
		Text: "🧭 مرشّح المجلات — «وين أنشر؟»\n\n" +
			"ألصق ملخّص بحثك (Abstract) وسأقترح أنسب المجلات لمجالك\n" +
			"مع تقدير الربع (Quartile) وتنبيهات النشر.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handlePublishAbstract recommends journals for the abstract the user sent.
func (a *App) handlePublishAbstract(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: aiNavKeyboard()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	abstract, _ := truncateRunes(strings.TrimSpace(msg.Text), 6000)
	if abstract == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل ملخّصاً غير فارغ.", Keyboard: aiNavKeyboard()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري تحليل الملخّص واقتراح المجلات..."})
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: publishSystemPrompt},
		{Role: "user", Content: abstract},
	})
	if err != nil {
		a.logf("publish: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)

	// Cross-check the AI's suggestions against the predatory watch list.
	var flags []predator.Flag
	if a.predators != nil {
		flags = a.predators.Check(reply)
	}
	a.send(ctx, chatID, publishResultScreen(reply, flags))
}

// publishResultScreen renders the recommendations plus advisories.
func publishResultScreen(reply string, flags []predator.Flag) Screen {
	var b strings.Builder
	b.WriteString("🧭 مجلات مقترحة لبحثك:\n\n")
	b.WriteString(strings.TrimSpace(reply))

	if len(flags) > 0 {
		b.WriteString("\n\n⛔ تنبيه استرشادي: ظهر في الاقتراحات اسم/ناشر مُدرج في قائمة التحذير:")
		for _, f := range flags {
			b.WriteString("\n   • " + f.Reason)
		}
	}

	b.WriteString("\n\nℹ️ هذه اقتراحات استرشادية من الذكاء الاصطناعي — تحقّق من:")
	b.WriteString("\n• تصنيف المجلة عبر «🛡️ فحص مجلة».")
	b.WriteString("\n• رسوم النشر (APC) ونطاق المجلة من موقع الناشر الرسمي.")

	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🛡️ فحص مجلة", Data: "menu:journal"}},
			{{Text: "🧭 ملخّص آخر", Data: "menu:publish"}, {Text: "⬅️ القائمة", Data: "menu:home"}},
		}},
	}
}

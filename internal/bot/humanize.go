package bot

import (
	"context"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/docx"
	"github.com/go-telegram/bot/models"
)

// humanizeMaxRunes caps the text sent to the model.
const humanizeMaxRunes = 12000

// humanizeSystemPrompt rewrites AI-sounding text into natural human prose,
// preserving meaning and the original language. The guidance distils common
// AI-writing "tells" (significance inflation, vague attribution, filler,
// synonym cycling, rule-of-three, em-dashes, robotic openers/closers).
const humanizeSystemPrompt = "أنت محرّر لغوي بشري خبير. أعد كتابة النص ليبدو طبيعياً ككتابة إنسان، مع " +
	"الحفاظ التامّ على المعنى وعلى لغة النص الأصلية (العربي يبقى عربياً والإنجليزي إنجليزياً). " +
	"أزِل علامات الكتابة الآلية: المبالغة في الأهمية («لحظة محورية»)، والنسب الغامضة («يعتقد الخبراء»)، " +
	"والحشو («في الواقع»، «تجدر الإشارة»، additionally، moreover، furthermore)، وتكرار المرادفات بلا داعٍ، " +
	"وقاعدة الثلاثيات المصطنعة، والشرطات الطويلة (—)، والعناوين المزخرفة والإيموجي، وعبارات الافتتاح («دعنا نتعمّق») " +
	"والختام الآلية («أتمنى أن يفيدك هذا»). استبدل «يُعدّ/يمثّل/يُشكّل/يتميّز بـ» بأفعال مباشرة مثل «هو/له»، " +
	"واختصر «من أجل أن»→«لـ» و«نظراً لحقيقة أن»→«لأن». نوّع طول الجمل وإيقاعها بشكل طبيعي. " +
	"لا تُضِف معلومات جديدة ولا تحذف جوهرياً. أعِد النصّ المُعاد صياغته فقط، دون أي مقدمة أو تعليق."

// humanizePromptScreen invites text (everyone) or a Word file (premium).
func humanizePromptScreen(premium bool) Screen {
	text := "✍️ إعادة الكتابة بأسلوب بشري\n\n" +
		"أُزيل علامات الكتابة الآلية وأجعل النص طبيعياً مع الحفاظ على المعنى.\n\n"
	if premium {
		text += "أرسل النص مباشرةً، أو ارفع ملف Word (‎.docx‎) وسأعيد لك ملفاً مُعدّلاً. 💎"
	} else {
		text += "أرسل مقطعاً نصّياً وسأعيد صياغته.\n" +
			"💎 المشتركون يرفعون ملف Word كاملاً ويستلمون ملفاً مُعدّلاً — فعّل اشتراكك من «💎 الاشتراك»."
	}
	return Screen{Text: text, Keyboard: humanizeNav()}
}

// handleHumanizeText rewrites a text snippet (available to everyone).
func (a *App) handleHumanizeText(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	if !a.aiEnabled() {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: humanizeNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}
	text, _ := truncateRunes(strings.TrimSpace(msg.Text), humanizeMaxRunes)
	if text == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل نصاً لإعادة صياغته.", Keyboard: humanizeNav()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "✍️ جاري إعادة الصياغة بأسلوب بشري..."})
	out, err := a.humanize(ctx, text)
	if err != nil {
		a.logf("humanize text: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, Screen{
		Text:     "✍️ النص بأسلوب بشري:\n\n" + strings.TrimSpace(out),
		Keyboard: humanizeNav(),
	})
}

// handleHumanizeDoc rewrites an uploaded Word file and returns an edited file
// (premium only).
func (a *App) handleHumanizeDoc(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	doc := msg.Document

	if !a.isPremiumUser(ctx, msg.From.ID) {
		a.send(ctx, chatID, premiumGateScreen("معالجة ملفات Word كاملة متاحة للمشتركين — يمكنك إرسال النص مباشرةً مجاناً."))
		return
	}
	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".docx") {
		a.send(ctx, chatID, Screen{Text: "❌ ارفع ملف ‎.docx‎ (Word).", Keyboard: humanizeNav()})
		return
	}
	if doc.FileSize > maxCheckFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ٢ ميغابايت).", Keyboard: humanizeNav()})
		return
	}
	if !a.aiEnabled() {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: humanizeNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري معالجة الملف وإعادة صياغته..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("humanize download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: humanizeNav()})
		return
	}
	text, err := docx.ExtractText(data)
	if err != nil || strings.TrimSpace(text) == "" {
		a.send(ctx, chatID, Screen{Text: "❌ تعذّرت قراءة الملف أو أنه فارغ.", Keyboard: humanizeNav()})
		return
	}
	text, _ = truncateRunes(text, humanizeMaxRunes)

	out, err := a.humanize(ctx, text)
	if err != nil {
		a.logf("humanize doc ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)

	outDoc, err := docx.Build(out)
	if err != nil {
		a.logf("humanize build docx: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر إنشاء الملف، إليك النص:\n\n" + strings.TrimSpace(out), Keyboard: humanizeNav()})
		return
	}
	a.sendDocument(ctx, chatID, "humanized.docx", outDoc, "✅ تمّت إعادة الصياغة — هذه نسختك المُعدّلة:")
}

// humanize runs the rewrite through the AI provider.
func (a *App) humanize(ctx context.Context, input string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	return a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: humanizeSystemPrompt},
		{Role: "user", Content: input},
	})
}

// isPremiumUser reports whether the user currently holds a paid tier.
func (a *App) isPremiumUser(ctx context.Context, userID int64) bool {
	if u, err := a.store.GetUser(ctx, userID); err == nil && u != nil {
		return u.IsPremium(time.Now())
	}
	return false
}

func humanizeNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "✍️ نصّ آخر", Data: "menu:humanize"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

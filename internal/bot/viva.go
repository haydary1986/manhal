package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/docx"
	"github.com/go-telegram/bot/models"
)

// vivaMaxInputRunes caps the thesis text sent to the model.
const vivaMaxInputRunes = 12000

// errUnsupportedDoc is returned for a file type we cannot read as text.
var errUnsupportedDoc = errors.New("unsupported document type")

const vivaSystemPrompt = "أنت أستاذ جامعي خبير في مناقشة الأطاريح. بناءً على نص الأطروحة المُعطى: " +
	"١) اكتب هيكل عرض تقديمي للدفاع (٨–١٠ شرائح بعناوين ونقاط). " +
	"٢) ولّد ٢٠ سؤالاً متوقعاً من لجنة المناقشة مرتبة حسب المحاور (المشكلة، المنهجية، النتائج، المساهمة)، " +
	"مع إجابة نموذجية موجزة لكل سؤال، وركّز على نقاط الضعف المنهجية المحتملة. أجب بالعربية بتنظيم واضح."

// vivaPromptScreen asks for a thesis file.
func vivaPromptScreen() Screen {
	return Screen{
		Text: "🎤 تحضير المناقشة (Viva)\n\n" +
			"ارفع أطروحتك (‎.docx‎ أو ‎.tex‎) وسأجهّز لك:\n" +
			"• هيكل عرض تقديمي للدفاع.\n" +
			"• أسئلة لجنة متوقّعة بإجابات نموذجية.\n" +
			"(الحد الأقصى ٢ ميغابايت)",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleVivaDocument reads the uploaded thesis and prepares viva material.
func (a *App) handleVivaDocument(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	doc := msg.Document

	if _, err := supportedDocExt(doc.FileName); err != nil {
		a.send(ctx, chatID, Screen{Text: "❌ الصيغة غير مدعومة. ارفع ملف ‎.docx‎ أو ‎.tex‎.", Keyboard: vivaNav()})
		return
	}
	if doc.FileSize > maxCheckFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ٢ ميغابايت).", Keyboard: vivaNav()})
		return
	}
	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: vivaNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري قراءة الأطروحة وتحضير المناقشة..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("viva download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: vivaNav()})
		return
	}

	text, err := extractDocText(doc.FileName, data)
	if err != nil {
		a.send(ctx, chatID, Screen{Text: "❌ تعذّرت قراءة الملف ⚠️", Keyboard: vivaNav()})
		return
	}
	text, _ = truncateRunes(strings.TrimSpace(text), vivaMaxInputRunes)
	if text == "" {
		a.send(ctx, chatID, Screen{Text: "الملف لا يحتوي نصاً قابلاً للقراءة.", Keyboard: vivaNav()})
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: vivaSystemPrompt},
		{Role: "user", Content: text},
	})
	if err != nil {
		a.logf("viva ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, vivaResultScreen(reply))
}

// vivaResultScreen renders the prepared viva material.
func vivaResultScreen(reply string) Screen {
	return Screen{
		Text: "🎤 تحضير المناقشة (Viva):\n\n" + strings.TrimSpace(reply) +
			"\n\nℹ️ مادة استرشادية — راجعها وكيّفها حسب أطروحتك ولجنتك.",
		Keyboard: vivaNav(),
	}
}

// extractDocText returns the prose of a supported document.
func extractDocText(filename string, data []byte) (string, error) {
	ext, err := supportedDocExt(filename)
	if err != nil {
		return "", err
	}
	if ext == ".docx" {
		return docx.ExtractText(data)
	}
	return string(data), nil // .tex/.txt are already text
}

// supportedDocExt reports the file's text-extractable extension.
func supportedDocExt(filename string) (string, error) {
	name := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(name, ".docx"):
		return ".docx", nil
	case strings.HasSuffix(name, ".tex"):
		return ".tex", nil
	case strings.HasSuffix(name, ".txt"):
		return ".txt", nil
	default:
		return "", errUnsupportedDoc
	}
}

func vivaNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🎤 أطروحة أخرى", Data: "menu:viva"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

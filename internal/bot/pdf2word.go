package bot

import (
	"context"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/docx"
	"github.com/erticaz/manhal/internal/ocr"
	"github.com/erticaz/manhal/internal/pdf"
	"github.com/go-telegram/bot/models"
)

// pdf2wordMaxFileSize caps uploads (no AI cost, so larger than the AI tools).
const pdf2wordMaxFileSize = 12 << 20 // 12 MB

// minTextForBornDigital is the text length below which we assume the PDF is
// scanned and route it through OCR.
const minTextForBornDigital = 80

// pdf2wordPromptScreen asks for a PDF to convert.
func pdf2wordPromptScreen() Screen {
	return Screen{
		Text: "📄➡️ تحويل PDF إلى Word\n\n" +
			"ارفع ملف ‎.pdf‎ وسأحوّله إلى ملف Word (‎.docx‎) قابل للتعديل.\n" +
			"الملفات الممسوحة ضوئياً تُقرأ تلقائياً بـ OCR (عربي/إنجليزي).",
		Keyboard: pdf2wordNav(),
	}
}

// handlePdf2Word converts an uploaded PDF to a Word file.
func (a *App) handlePdf2Word(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	doc := msg.Document

	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".pdf") {
		a.send(ctx, chatID, Screen{Text: "❌ ارفع ملف ‎.pdf‎.", Keyboard: pdf2wordNav()})
		return
	}
	if doc.FileSize > pdf2wordMaxFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ١٢ ميغابايت).", Keyboard: pdf2wordNav()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري تحويل الملف..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("pdf2word download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: pdf2wordNav()})
		return
	}

	text, _ := pdf.ExtractText(data)
	if len(strings.TrimSpace(text)) < minTextForBornDigital {
		// Likely a scanned PDF — try OCR.
		a.send(ctx, chatID, Screen{Text: "📷 الملف يبدو ممسوحاً ضوئياً — جاري قراءته بـ OCR (قد يستغرق دقيقة)..."})
		octx, cancel := context.WithTimeout(ctx, 240*time.Second)
		ocrText, oerr := ocr.ExtractPDF(octx, data)
		cancel()
		switch {
		case oerr == ocr.ErrUnavailable:
			a.send(ctx, chatID, Screen{Text: "⚠️ هذا ملف ممسوح ضوئياً ويحتاج OCR غير متاح حالياً على الخادم.", Keyboard: pdf2wordNav()})
			return
		case oerr != nil || strings.TrimSpace(ocrText) == "":
			a.send(ctx, chatID, Screen{Text: "❌ تعذّرت قراءة محتوى الملف.", Keyboard: pdf2wordNav()})
			return
		}
		text = ocrText
	}

	out, err := docx.Build(text)
	if err != nil {
		a.logf("pdf2word build: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر إنشاء ملف Word.", Keyboard: pdf2wordNav()})
		return
	}
	a.sendDocument(ctx, chatID, convertedName(doc.FileName), out, "✅ تمّ التحويل إلى Word:")
}

// convertedName swaps a .pdf name for .docx.
func convertedName(pdfName string) string {
	base := strings.TrimSpace(pdfName)
	if i := strings.LastIndex(strings.ToLower(base), ".pdf"); i >= 0 {
		base = base[:i]
	}
	if base == "" {
		base = "converted"
	}
	return base + ".docx"
}

func pdf2wordNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📄 ملف آخر", Data: "menu:pdf2word"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

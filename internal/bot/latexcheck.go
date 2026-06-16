package bot

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/docx"
	"github.com/erticaz/manhal/internal/latex"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// maxCheckFileSize caps the document size accepted for checking (cost control).
const maxCheckFileSize = 2 << 20 // 2 MiB

const latexSystemPrompt = "أنت مدقّق لغوي. صحّح الأخطاء اللغوية والنحوية والإملائية في النص فقط. " +
	"قواعد صارمة: (١) لا تغيّر أبداً أي رمز محاط بـ ⟦⟦…⟧⟧ — أبقِه حرفياً بمكانه (معادلات/جداول/أكواد). " +
	"(٢) لا تغيّر أوامر LaTeX أو بنيتها. (٣) أعد النص كاملاً مصحّحاً دون أي شرح إضافي."

const docxSystemPrompt = "أنت مدقّق لغوي أكاديمي. دقّق النص التالي لغوياً ونحوياً وإملائياً. " +
	"أعد النص المصحّح، ثم قائمة موجزة بأبرز التصحيحات. اكتب بنفس لغة النص."

// latexPromptScreen asks for a .tex or .docx file.
func latexPromptScreen() Screen {
	return Screen{
		Text: "📐 مدقّق LaTeX / Word الآمن\n\n" +
			"أرسل ملف ‎.tex‎ أو ‎.docx‎ وسأدقّقه لغوياً\n" +
			"دون المساس بالمعادلات أو الجداول أو الأكواد.\n" +
			"(الحد الأقصى ٢ ميغابايت)",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleLatexDocument downloads the uploaded file and language-checks it safely.
func (a *App) handleLatexDocument(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	doc := msg.Document

	name := strings.ToLower(doc.FileName)
	isTex := strings.HasSuffix(name, ".tex")
	isDocx := strings.HasSuffix(name, ".docx")
	if !isTex && !isDocx {
		a.send(ctx, chatID, Screen{Text: "❌ الصيغة غير مدعومة. أرسل ملف ‎.tex‎ أو ‎.docx‎.", Keyboard: latexNav()})
		return
	}
	if doc.FileSize > maxCheckFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ٢ ميغابايت).", Keyboard: latexNav()})
		return
	}
	if a.cfg.DeepSeekKey == "" {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: latexNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري تنزيل الملف وتدقيقه..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("latexcheck download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: latexNav()})
		return
	}

	if isTex {
		a.checkTex(ctx, chatID, msg.From.ID, doc.FileName, string(data))
		return
	}
	a.checkDocx(ctx, chatID, msg.From.ID, data)
}

// checkTex masks math/tables/code, proofreads the prose, restores them, and
// returns a corrected .tex file.
func (a *App) checkTex(ctx context.Context, chatID, userID int64, filename, src string) {
	masked, tokens := latex.Protect(src)

	callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: latexSystemPrompt},
		{Role: "user", Content: masked},
	})
	if err != nil {
		a.logf("latexcheck tex ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(userID)

	corrected := latex.Restore(reply, tokens)
	caption := "✅ تم التدقيق اللغوي مع الحفاظ على المعادلات والجداول والأكواد."
	if !latex.PlaceholdersIntact(reply, tokens) {
		caption = "⚠️ تم التدقيق، لكن قد تكون بعض المعادلات تأثّرت — راجع الملف قبل الاعتماد."
	}
	a.sendDocument(ctx, chatID, checkedName(filename, ".tex"), []byte(corrected), caption)
}

// checkDocx extracts the prose and returns proofreading suggestions as text
// (a safe docx round-trip is out of scope; we never rewrite the file).
func (a *App) checkDocx(ctx context.Context, chatID, userID int64, data []byte) {
	text, err := docx.ExtractText(data)
	if err != nil {
		a.send(ctx, chatID, Screen{Text: "❌ تعذّرت قراءة ملف Word ⚠️", Keyboard: latexNav()})
		return
	}
	text, _ = truncateRunes(strings.TrimSpace(text), 6000)
	if text == "" {
		a.send(ctx, chatID, Screen{Text: "الملف لا يحتوي نصاً قابلاً للتدقيق.", Keyboard: latexNav()})
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: docxSystemPrompt},
		{Role: "user", Content: text},
	})
	if err != nil {
		a.logf("latexcheck docx ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(userID)
	a.send(ctx, chatID, Screen{Text: "📝 نتيجة التدقيق:\n\n" + strings.TrimSpace(reply), Keyboard: latexNav()})
}

// downloadFile fetches a Telegram file's bytes by file id.
func (a *App) downloadFile(ctx context.Context, fileID string) ([]byte, error) {
	f, err := a.bot.GetFile(ctx, &tg.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}
	link := a.bot.FileDownloadLink(f)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, maxCheckFileSize+1))
}

// sendDocument uploads bytes as a document.
func (a *App) sendDocument(ctx context.Context, chatID int64, filename string, data []byte, caption string) {
	_, err := a.bot.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID:   chatID,
		Document: &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(data)},
		Caption:  caption,
	})
	if err != nil {
		a.logf("sendDocument: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر إرسال الملف المصحّح ⚠️", Keyboard: latexNav()})
	}
}

// checkedName inserts "-checked" before the extension.
func checkedName(filename, ext string) string {
	base := strings.TrimSuffix(filename, ext)
	if base == filename { // extension not found, append
		return filename + "-checked"
	}
	return base + "-checked" + ext
}

func latexNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📐 ملف آخر", Data: "menu:latex"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

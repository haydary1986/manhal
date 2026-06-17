package bot

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/embed"
	"github.com/erticaz/manhal/internal/pdf"
	"github.com/go-telegram/bot/models"
)

const (
	pdfChunkRunes   = 1000 // chunk size in runes
	pdfChunkOverlap = 150  // overlap between chunks
	pdfMaxChunks    = 150  // cap embeddings per document
	pdfTopK         = 8    // chunks retrieved per question
	pdfAllChunksMax = 16   // when a doc has ≤ this many chunks, send them all
	pdfMaxFileSize  = 8 << 20
)

// pdfChunk is one embedded passage of the active PDF.
type pdfChunk struct {
	text   string
	vector []float32
}

const ragSystemPrompt = "أنت مساعد يجيب عن أسئلة حول ورقة بحثية بالاعتماد على المقاطع المرفقة منها. " +
	"استخرج الإجابة من المقاطع، ويمكنك الاستنتاج المعقول والتلخيص مما ورد فيها. " +
	"إن كانت المعلومة غائبة تماماً عن كل المقاطع فقل: «لا تتوفّر هذه المعلومة في المقاطع المتاحة». " +
	"لا تختلق حقائق أو أرقاماً من خارج الورقة. أجب بالعربية بإيجاز ودقّة."

// pdfChatPromptScreen asks for a PDF to chat with.
func pdfChatPromptScreen() Screen {
	return Screen{
		Text: "📄 محادثة الـ PDF\n\n" +
			"ارفع ملف PDF لورقة بحثية، ثم اسألني عنها\n" +
			"وسأجيب من محتواها مباشرة (دون اختلاق).\n" +
			"(الحد الأقصى ٨ ميغابايت)",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handlePdfUpload extracts, chunks, and embeds a PDF, entering chat mode.
func (a *App) handlePdfUpload(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	doc := msg.Document

	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".pdf") {
		a.send(ctx, chatID, Screen{Text: "❌ ارفع ملف ‎.pdf‎.", Keyboard: pdfNav()})
		return
	}
	if doc.FileSize > pdfMaxFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ٨ ميغابايت).", Keyboard: pdfNav()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري قراءة الـ PDF وفهرسته..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("pdf download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: pdfNav()})
		return
	}

	text, err := pdf.ExtractText(data)
	if err != nil || strings.TrimSpace(text) == "" {
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر استخراج نص قابل للقراءة من الـ PDF (قد يكون صورة ممسوحة).", Keyboard: pdfNav()})
		return
	}

	texts := chunkText(text, pdfChunkRunes, pdfChunkOverlap)
	truncated := false
	if len(texts) > pdfMaxChunks {
		texts = texts[:pdfMaxChunks]
		truncated = true
	}

	embCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	vectors, err := a.embed.Embed(embCtx, texts)
	if err != nil {
		a.logf("pdf embed: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خدمة الفهرسة غير متاحة الآن ⚠️", Keyboard: pdfNav()})
		return
	}

	chunks := make([]pdfChunk, 0, len(texts))
	for i, t := range texts {
		if i < len(vectors) {
			chunks = append(chunks, pdfChunk{text: t, vector: vectors[i]})
		}
	}
	a.sessions.startPdfChat(msg.From.ID, chunks)

	note := ""
	if truncated {
		note = "\n(فُهرست أوائل الورقة فقط لطولها.)"
	}
	a.send(ctx, chatID, Screen{
		Text: "✅ تمت فهرسة الورقة في " + strconv.Itoa(len(chunks)) + " مقطعاً." + note +
			"\n\nاسألني الآن أي سؤال عن محتواها 👇",
		Keyboard: pdfNav(),
	})
}

// handlePdfQuestion answers a question via retrieval over the active PDF chunks.
func (a *App) handlePdfQuestion(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	chunks := a.sessions.pdfChunks(msg.From.ID)
	if len(chunks) == 0 {
		a.send(ctx, chatID, Screen{Text: "لا توجد ورقة نشطة. ارفع PDF أولاً.", Keyboard: pdfNav()})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	question := strings.TrimSpace(msg.Text)
	embCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	qv, err := embed.EmbedOne(embCtx, a.embed, question)
	cancel()
	if err != nil {
		a.logf("pdf q embed: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ خدمة الفهرسة غير متاحة الآن ⚠️", Keyboard: pdfNav()})
		return
	}

	// Small documents: hand the model everything (avoids retrieval misses,
	// especially with lighter embedding models). Larger ones: retrieve top-K.
	top := chunks
	if len(chunks) > pdfAllChunksMax {
		top = topChunks(chunks, qv, pdfTopK)
	}
	a.send(ctx, chatID, Screen{Text: "⏳ أبحث في الورقة..."})

	callCtx, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	defer cancel2()
	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: ragSystemPrompt},
		{Role: "user", Content: ragUserPrompt(question, top)},
	})
	if err != nil {
		a.logf("pdf rag ai: %v", err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, Screen{
		Text:     "📄 " + strings.TrimSpace(reply) + "\n\nℹ️ مبني على محتوى ورقتك. اسأل سؤالاً آخر أو ارجع للقائمة.",
		Keyboard: pdfNav(),
	})
}

// ragUserPrompt embeds the retrieved passages and the question.
func ragUserPrompt(question string, chunks []pdfChunk) string {
	var b strings.Builder
	b.WriteString("مقاطع من الورقة:\n")
	for _, c := range chunks {
		b.WriteString("---\n" + c.text + "\n")
	}
	b.WriteString("---\n\nالسؤال: " + question)
	return b.String()
}

// chunkText splits text into overlapping windows of `size` runes.
func chunkText(text string, size, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" || size <= 0 {
		return nil
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}
	r := []rune(text)
	var chunks []string
	for start := 0; start < len(r); start += size - overlap {
		end := start + size
		if end > len(r) {
			end = len(r)
		}
		if c := strings.TrimSpace(string(r[start:end])); c != "" {
			chunks = append(chunks, c)
		}
		if end == len(r) {
			break
		}
	}
	return chunks
}

// topChunks returns the top-K chunks by cosine similarity to qv.
func topChunks(chunks []pdfChunk, qv []float32, k int) []pdfChunk {
	type scored struct {
		c pdfChunk
		s float32
	}
	ranked := make([]scored, 0, len(chunks))
	for _, c := range chunks {
		ranked = append(ranked, scored{c: c, s: embed.Cosine(qv, c.vector)})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].s > ranked[j].s })
	if len(ranked) > k {
		ranked = ranked[:k]
	}
	out := make([]pdfChunk, len(ranked))
	for i, r := range ranked {
		out[i] = r.c
	}
	return out
}

func pdfNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📄 ورقة أخرى", Data: "menu:pdfchat"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

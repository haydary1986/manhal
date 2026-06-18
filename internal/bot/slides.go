package bot

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/docx"
	"github.com/erticaz/manhal/internal/pdf"
	"github.com/erticaz/manhal/internal/pptx"
	"github.com/go-telegram/bot/models"
)

const slidesMaxRunes = 12000

// PhotoSource fetches a stock photo for a query (implemented by pexels.Client).
type PhotoSource interface {
	Enabled() bool
	Photo(ctx context.Context, query string) ([]byte, error)
}

const slidesSystemPrompt = "أنت مصمّم عروض تقديمية أكاديمية. من النص المُعطى أنشئ مخطّط عرض تقديمي مترابط. " +
	"أعد JSON فقط بهذا الشكل بالضبط دون أي نص خارجه:\n" +
	`{"title":"عنوان العرض","slides":[{"title":"عنوان الشريحة","bullets":["نقطة موجزة","نقطة موجزة"],"image":"english search keyword"}]}` + "\n" +
	"من 6 إلى 10 شرائح، كل شريحة 3–5 نقاط قصيرة وواضحة بنفس لغة النص. " +
	"حقل image: كلمة/عبارة بحث بالإنجليزية تمثّل موضوع الشريحة بصرياً لجلب صورة مناسبة. اعتمد على النص فقط."

// slidesPromptScreen invites text or a document to turn into a deck.
func slidesPromptScreen() Screen {
	return Screen{
		Text: "🎬 إنشاء عرض تقديمي\n\n" +
			"أرسل نصّاً، أو ارفع ملف PDF/Word، وسأحوّله إلى عرض PowerPoint (‎.pptx‎) قابل للتعديل.",
		Keyboard: slidesNav(),
	}
}

func (a *App) handleSlidesText(ctx context.Context, msg *models.Message) {
	a.sessions.clear(msg.From.ID)
	text, _ := truncateRunes(strings.TrimSpace(msg.Text), slidesMaxRunes)
	if text == "" {
		a.send(ctx, msg.Chat.ID, Screen{Text: "أرسل نصاً لإنشاء العرض منه.", Keyboard: slidesNav()})
		return
	}
	a.makeSlides(ctx, msg.Chat.ID, msg.From.ID, text)
}

func (a *App) handleSlidesDoc(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	doc := msg.Document
	name := strings.ToLower(doc.FileName)

	if !strings.HasSuffix(name, ".pdf") && !strings.HasSuffix(name, ".docx") {
		a.send(ctx, chatID, Screen{Text: "❌ ارفع ملف ‎.pdf‎ أو ‎.docx‎ (أو أرسل نصاً).", Keyboard: slidesNav()})
		return
	}
	if doc.FileSize > pdf2wordMaxFileSize {
		a.send(ctx, chatID, Screen{Text: "❌ الملف كبير (الحد ١٢ ميغابايت).", Keyboard: slidesNav()})
		return
	}
	a.send(ctx, chatID, Screen{Text: "⏳ جاري قراءة الملف..."})
	data, err := a.downloadFile(ctx, doc.FileID)
	if err != nil {
		a.logf("slides download: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر تنزيل الملف ⚠️", Keyboard: slidesNav()})
		return
	}
	var text string
	if strings.HasSuffix(name, ".pdf") {
		text, err = pdf.ExtractText(data)
	} else {
		text, err = docx.ExtractText(data)
	}
	if err != nil {
		a.logf("slides extract (%s, %d bytes): %v", doc.FileName, len(data), err)
	} else if strings.TrimSpace(text) == "" {
		a.logf("slides extract (%s, %d bytes): parsed empty", doc.FileName, len(data))
	}
	if strings.TrimSpace(text) == "" {
		a.send(ctx, chatID, Screen{
			Text: "❌ تعذّر استخراج نص من الملف.\n" +
				"تأكّد أنه ملف Word حديث بصيغة ‎.docx‎ (وليس ‎.doc‎ القديم أو ملفاً ممسوحاً ضوئياً). " +
				"في Word: «حفظ باسم» ← Word Document (‎.docx‎). أو أرسل النص مباشرةً.",
			Keyboard: slidesNav(),
		})
		return
	}
	text, _ = truncateRunes(text, slidesMaxRunes)
	a.makeSlides(ctx, chatID, msg.From.ID, text)
}

// makeSlides builds the AI outline and sends the generated .pptx.
func (a *App) makeSlides(ctx context.Context, chatID, userID int64, text string) {
	if !a.aiEnabled() {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek).", Keyboard: slidesNav()})
		return
	}
	if !a.usage.allow(userID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	a.send(ctx, chatID, Screen{Text: "🎬 جاري إنشاء العرض التقديمي..."})
	octx, cancel := context.WithTimeout(ctx, 150*time.Second)
	title, specs, err := a.slideOutline(octx, text)
	cancel()
	if err != nil || len(specs) == 0 {
		a.logf("slides outline: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر إنشاء مخطّط العرض، جرّب نصاً أوضح.", Keyboard: slidesNav()})
		return
	}
	a.usage.record(userID)

	withImages := a.pexels != nil && a.pexels.Enabled()
	if withImages {
		a.send(ctx, chatID, Screen{Text: "🎨 جاري جلب الصور المناسبة..."})
	}
	slides := make([]pptx.Slide, 0, len(specs))
	for _, sp := range specs {
		ps := pptx.Slide{Title: sp.Title, Bullets: sp.Bullets}
		if withImages {
			query := sp.Image
			if strings.TrimSpace(query) == "" {
				query = sp.Title
			}
			ictx, c := context.WithTimeout(ctx, 20*time.Second)
			if img, ierr := a.pexels.Photo(ictx, query); ierr == nil {
				ps.Image = img
			}
			c()
		}
		slides = append(slides, ps)
	}

	deck, err := pptx.Build(title, slides)
	if err != nil {
		a.logf("slides build: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر إنشاء ملف العرض.", Keyboard: slidesNav()})
		return
	}
	a.sendDocument(ctx, chatID, "presentation.pptx", deck, "✅ عرضك التقديمي جاهز (افتحه في PowerPoint للتعديل):")
}

// slideSpec is one slide from the AI outline, with an English image keyword.
type slideSpec struct {
	Title   string
	Bullets []string
	Image   string
}

// slideOutline asks the model for a JSON deck outline and parses it.
func (a *App) slideOutline(ctx context.Context, text string) (string, []slideSpec, error) {
	out, err := a.ai.Chat(ctx, []ai.Message{
		{Role: "system", Content: slidesSystemPrompt},
		{Role: "user", Content: text},
	})
	if err != nil {
		return "", nil, err
	}
	var doc struct {
		Title  string `json:"title"`
		Slides []struct {
			Title   string   `json:"title"`
			Bullets []string `json:"bullets"`
			Image   string   `json:"image"`
		} `json:"slides"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(out)), &doc); err != nil {
		return "", nil, err
	}
	specs := make([]slideSpec, 0, len(doc.Slides))
	for _, s := range doc.Slides {
		specs = append(specs, slideSpec{Title: s.Title, Bullets: s.Bullets, Image: s.Image})
	}
	if doc.Title == "" {
		doc.Title = "العرض التقديمي"
	}
	return doc.Title, specs, nil
}

// extractJSONObject returns the substring from the first { to the last },
// tolerating any prose the model adds around the JSON.
func extractJSONObject(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

func slidesNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🎬 عرض آخر", Data: "menu:slides"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

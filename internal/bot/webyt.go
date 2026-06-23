package bot

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/websearch"
	"github.com/erticaz/manhal/internal/youtube"
	"github.com/go-telegram/bot/models"
)

// WebSearcher performs an AI web search (implemented by websearch.Tavily).
type WebSearcher interface {
	Enabled() bool
	Search(ctx context.Context, query string, maxResults int) (websearch.Answer, error)
}

// Transcriber fetches a video transcript (implemented by youtube.Client).
type Transcriber interface {
	Transcript(ctx context.Context, videoID string) (string, error)
}

const ytTranscriptMaxRunes = 14000

// ---------- AI web search ----------

func (a *App) webSearchPromptScreen() Screen {
	if a.web == nil || !a.web.Enabled() {
		return Screen{
			Text: "🌐 البحث الذكي في الويب\n\n⚙️ الخدمة غير مفعّلة حالياً.\n" +
				"يلزم ضبط مفتاح Tavily لتشغيلها.",
			Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}}}},
		}
	}
	return Screen{
		Text: "🌐 البحث الذكي في الويب\n\n" +
			"أرسل سؤالك أو موضوعك، وسأبحث في الويب وأعطيك إجابة موجزة مع المصادر.",
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}}}},
	}
}

func (a *App) handleWebSearch(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	if a.web == nil || !a.web.Enabled() {
		a.send(ctx, chatID, a.webSearchPromptScreen())
		return
	}
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل سؤالاً غير فارغ."})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}
	a.send(ctx, chatID, Screen{Text: "🌐 جاري البحث في الويب..."})

	sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ans, err := a.web.Search(sctx, query, 5)
	if err != nil {
		a.logf("websearch %q: %v", query, err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر البحث الآن ⚠️ جرّب لاحقاً.", Keyboard: webYTNav("menu:websearch")})
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, webSearchResultScreen(query, ans))
}

func webSearchResultScreen(query string, ans websearch.Answer) Screen {
	var b strings.Builder
	b.WriteString("🌐 «" + query + "»\n\n")
	if ans.Text != "" {
		b.WriteString(ans.Text + "\n")
	} else {
		b.WriteString("لم أجد إجابة مركّزة، لكن هذه أبرز المصادر:\n")
	}
	if len(ans.Results) > 0 {
		b.WriteString("\n🔗 المصادر:\n")
		for i, r := range ans.Results {
			b.WriteString(strconv.Itoa(i+1) + ") " + r.Title + "\n" + r.URL + "\n")
		}
	}
	b.WriteString("\nℹ️ نتائج من الويب — تحقّق من المصادر قبل الاعتماد عليها.")
	return Screen{Text: b.String(), Keyboard: webYTNav("menu:websearch")}
}

// ---------- YouTube summarization ----------

func (a *App) youtubePromptScreen() Screen {
	return Screen{
		Text: "▶️ تلخيص فيديو يوتيوب\n\n" +
			"أرسل رابط فيديو (محاضرة/مؤتمر) وسألخّص محتواه بالعربية من نصّ ترجمته.\n" +
			"يعمل مع الفيديوهات التي تتوفّر لها ترجمة/تفريغ.",
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}}}},
	}
}

func (a *App) handleYouTube(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	a.sessions.clear(msg.From.ID)
	if a.youtube == nil {
		a.send(ctx, chatID, a.youtubePromptScreen())
		return
	}
	id, ok := youtube.VideoID(strings.TrimSpace(msg.Text))
	if !ok {
		a.send(ctx, chatID, Screen{Text: "❌ هذا لا يبدو رابط يوتيوب صحيحاً. أرسل رابطاً مثل https://youtu.be/...", Keyboard: webYTNav("menu:ytsummary")})
		return
	}
	if !a.aiEnabled() {
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة (يلزم مفتاح DeepSeek).", Keyboard: webYTNav("menu:ytsummary")})
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}
	a.send(ctx, chatID, Screen{Text: "⏳ جاري جلب نصّ الفيديو..."})

	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	transcript, err := a.youtube.Transcript(tctx, id)
	cancel()
	if err != nil {
		a.logf("youtube transcript %s: %v", id, err)
		msgTxt := "❌ تعذّر جلب نصّ الفيديو ⚠️ جرّب فيديو آخر."
		if errors.Is(err, youtube.ErrNoTranscript) {
			msgTxt = "❌ هذا الفيديو لا يحتوي ترجمة/تفريغاً نصّياً، فلا يمكن تلخيصه."
		}
		a.send(ctx, chatID, Screen{Text: msgTxt, Keyboard: webYTNav("menu:ytsummary")})
		return
	}
	transcript, _ = truncateRunes(transcript, ytTranscriptMaxRunes)

	a.send(ctx, chatID, Screen{Text: "🧠 جاري التلخيص..."})
	sctx, cancel2 := context.WithTimeout(ctx, 120*time.Second)
	reply, err := a.ai.Chat(sctx, []ai.Message{
		{Role: "system", Content: "أنت مساعد بحثي. لخّص نصّ هذا الفيديو بالعربية تلخيصاً أكاديمياً واضحاً: " +
			"الفكرة الرئيسية، أبرز النقاط في صورة قائمة، والخلاصة. اعتمد على النص فقط ولا تختلق."},
		{Role: "user", Content: transcript},
	})
	cancel2()
	if err != nil {
		a.logf("youtube summarize %s: %v", id, err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, Screen{
		Text:     "▶️ ملخّص الفيديو:\n\n" + strings.TrimSpace(reply),
		Keyboard: webYTNav("menu:ytsummary"),
	})
}

func webYTNav(retry string) *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔁 مرّة أخرى", Data: retry}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

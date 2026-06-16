package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/assist"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// aiMenuScreen lists the AI tools and the user's remaining daily quota.
func (a *App) aiMenuScreen(userID int64) Screen {
	if a.cfg.DeepSeekKey == "" {
		return Screen{
			Text: "🤖 المساعد الذكي\n\n⚙️ الخدمة غير مفعّلة حالياً.\n" +
				"يلزم ضبط مفتاح DeepSeek لتشغيل أدوات الذكاء.",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
			}},
		}
	}

	var b strings.Builder
	b.WriteString("🤖 المساعد الذكي\n\nاختر الأداة المناسبة:")
	if r := a.usage.remaining(userID); r >= 0 {
		b.WriteString("\nمتبقّي لك اليوم: " + strconv.Itoa(r) + " استخدام.")
	}

	rows := [][]Button{}
	for _, tool := range assist.Tools() {
		rows = append(rows, []Button{{Text: tool.Label, Data: "ai:tool:" + tool.Key}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})
	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// handleAITool starts a tool: it records the choice and asks for input.
func (a *App) handleAITool(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings))
		return
	}

	tool, ok := assist.Find(strings.TrimPrefix(cq.Data, "ai:tool:"))
	if !ok {
		a.send(ctx, cq.From.ID, a.aiMenuScreen(cq.From.ID))
		return
	}
	a.sessions.startAITool(cq.From.ID, tool.Key)
	a.send(ctx, cq.From.ID, Screen{
		Text: tool.Label + "\n\n" + tool.Prompt,
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ إلغاء", Data: "menu:ai"}},
		}},
	})
}

// handleAIInput runs the active tool over the text the user just sent.
func (a *App) handleAIInput(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	toolKey := a.sessions.aiTool(msg.From.ID)
	a.sessions.clear(msg.From.ID)

	tool, ok := assist.Find(toolKey)
	if !ok {
		a.send(ctx, chatID, a.mainMenuScreen())
		return
	}
	if !a.usage.allow(msg.From.ID) {
		a.send(ctx, chatID, aiLimitScreen())
		return
	}

	input, truncated := truncateRunes(strings.TrimSpace(msg.Text), assist.MaxInputRunes)
	if input == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل نصاً غير فارغ.", Keyboard: aiNavKeyboard()})
		return
	}

	a.send(ctx, chatID, Screen{Text: "⏳ جاري المعالجة..."})
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	reply, err := a.ai.Chat(callCtx, []ai.Message{
		{Role: "system", Content: tool.System},
		{Role: "user", Content: input},
	})
	if err != nil {
		a.logf("ai %s: %v", tool.Key, err)
		a.send(ctx, chatID, aiErrorScreen())
		return
	}
	a.usage.record(msg.From.ID)
	a.send(ctx, chatID, aiResultScreen(tool, reply, truncated))
}

// aiResultScreen shows the tool's output.
func aiResultScreen(tool assist.Tool, reply string, truncated bool) Screen {
	var b strings.Builder
	b.WriteString(tool.Label + "\n")
	if truncated {
		b.WriteString("⚠️ تم اقتصار النص الطويل قبل المعالجة.\n")
	}
	b.WriteString("\n" + strings.TrimSpace(reply))
	return Screen{Text: b.String(), Keyboard: aiNavKeyboard()}
}

func aiLimitScreen() Screen {
	return Screen{
		Text: "🚦 وصلت حدّك اليومي من استخدامات الذكاء.\n" +
			"عُد غداً، أو رقّ لخطة موسّعة لاحقاً.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

func aiErrorScreen() Screen {
	return Screen{
		Text:     "❌ صار خلل بمعالجة طلبك ⚠️\nجرّب بعد لحظات أو بنص أقصر.",
		Keyboard: aiNavKeyboard(),
	}
}

func aiNavKeyboard() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🤖 أداة أخرى", Data: "menu:ai"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

// truncateRunes shortens s to at most max runes, reporting whether it was cut.
func truncateRunes(s string, max int) (string, bool) {
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	return string(r[:max]), true
}

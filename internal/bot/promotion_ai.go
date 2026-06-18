package bot

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/promotion"
	"github.com/go-telegram/bot/models"
)

// promoAISystem instructs the model to turn a free-text achievements description
// into per-activity counts plus service years, using the live activity keys.
const promoAISystem = "أنت مساعد يحوّل وصف الباحث لإنجازاته إلى أعداد بنود الترقية العلمية. " +
	"ستحصل على قائمة البنود المتاحة بصيغة «المفتاح — الوصف». " +
	"اقرأ نص المستخدم واستخرج عدد كل بند ينطبق عليه وعدد سنوات خدمته منذ آخر ترقية. " +
	"أعد JSON فقط بهذا الشكل بالضبط دون أي نص خارجه:\n" +
	`{"counts":{"key":number},"years":number}` + "\n" +
	"استخدم المفاتيح الإنجليزية من القائمة حصراً، وتجاهل ما لا ينطبق. " +
	"إن لم تُذكر سنوات الخدمة اجعل years = 0. لا تخترع بنوداً غير مذكورة."

// promoAIPromptScreen invites a natural-language description of achievements.
func promoAIPromptScreen(rank promotion.Rank) Screen {
	return Screen{
		Text: "🤖 صِف إنجازاتك بالعربية\n" +
			"الترقية: " + rank.Label + " ← " + rank.NextLabel + "\n\n" +
			"اكتب فقرة تصف فيها أبحاثك وكتبك ولجانك وإشرافك وسنوات خدمتك، وسأحسب نقاطك تلقائياً.\n\n" +
			"مثال:\n«لديّ بحثان IF أنا الباحث الأول فيهما، وكتاب محلي منفرد، وعضوية لجنتين علميتين، وخدمتي ٤ سنوات».",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ طريقة أخرى", Data: "promo:rank:" + rank.Key}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// promoAISchema lists the available activity keys and labels for the prompt.
func (a *App) promoAISchema() string {
	var b strings.Builder
	for _, t := range []int{1, 2} {
		for _, act := range a.promotion.ActivitiesByTable(t) {
			b.WriteString(act.Key + " — " + act.Label + "\n")
		}
	}
	return b.String()
}

// handlePromotionAI extracts indicators from the user's prose via the model and
// shows the official result. Falls back gracefully when nothing is recognized.
func (a *App) handlePromotionAI(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	rankKey := a.sessions.promoteRank(userID)
	rank, ok := a.promotion.FindRank(rankKey)
	if !ok {
		a.sessions.clear(userID)
		a.send(ctx, chatID, a.promotionRankScreen())
		return
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		a.send(ctx, chatID, promoAIPromptScreen(rank))
		return
	}
	if !a.aiEnabled() {
		a.sessions.clear(userID)
		a.send(ctx, chatID, Screen{Text: "⚙️ الخدمة غير مفعّلة حالياً (يلزم مفتاح DeepSeek). استخدم طريقة الأزرار.", Keyboard: &Keyboard{Rows: [][]Button{{{Text: "🔘 المنشئ التفاعلي", Data: "promo:build:" + rank.Key}}}}})
		return
	}
	if !a.usage.allow(userID) {
		a.sessions.clear(userID)
		a.send(ctx, chatID, aiLimitScreen())
		return
	}
	a.sessions.clear(userID)
	a.send(ctx, chatID, Screen{Text: "🤖 جاري تحليل وصفك واستخراج البنود..."})

	octx, cancel := context.WithTimeout(ctx, 90*time.Second)
	out, err := a.ai.Chat(octx, []ai.Message{
		{Role: "system", Content: promoAISystem + "\n\nالبنود المتاحة:\n" + a.promoAISchema()},
		{Role: "user", Content: text},
	})
	cancel()
	if err != nil {
		a.logf("promo ai: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر التحليل، حاول مجدداً أو استخدم طريقة الأزرار.", Keyboard: &Keyboard{Rows: [][]Button{{{Text: "🔘 المنشئ التفاعلي", Data: "promo:build:" + rank.Key}}}}})
		return
	}

	counts, years := parsePromoAIJSON(out)
	known := a.promoActivityMap()
	filtered := make(map[string]float64, len(counts))
	for k, v := range counts {
		if _, ok := known[k]; ok && v > 0 {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 && years == 0 {
		a.send(ctx, chatID, Screen{
			Text: "🤔 لم أتمكّن من استخراج بنود واضحة من وصفك.\nجرّب وصفاً أدقّ، أو استخدم المنشئ التفاعلي بالأزرار.",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "🔘 المنشئ التفاعلي", Data: "promo:build:" + rank.Key}},
				{{Text: "🔁 إعادة الوصف", Data: "promo:ai:" + rank.Key}},
			}},
		})
		return
	}
	a.usage.record(userID)
	res, _ := a.promotion.Compute(promotion.Input{RankKey: rankKey, Counts: filtered, ServiceYears: years})
	a.send(ctx, chatID, promotionResultScreen(res))
}

// parsePromoAIJSON tolerantly parses the {"counts":{..},"years":N} object.
func parsePromoAIJSON(s string) (map[string]float64, int) {
	var doc struct {
		Counts map[string]float64 `json:"counts"`
		Years  float64            `json:"years"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(s)), &doc); err != nil {
		return nil, 0
	}
	return doc.Counts, int(doc.Years)
}

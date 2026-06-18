package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/promotion"
	"github.com/go-telegram/bot/models"
)

// promoYearsKey is the pseudo-key for service years inside the interactive draft.
const promoYearsKey = "years"

// promoCategory groups related activity keys so the interactive builder shows a
// short, navigable set of buttons instead of ~50 items at once.
type promoCategory struct {
	Label string
	Keys  []string
}

var promoCategories = []promoCategory{
	{"📄 بحوث IF/CS", []string{"if_first", "if_second", "if_third", "if_fourth", "if_fifth", "if_sixthplus"}},
	{"📄 بحوث عراقية", []string{"iq_first", "iq_second", "iq_third", "iq_fourth", "iq_fifth", "iq_sixthplus"}},
	{"📚 كتب ومؤتمرات", []string{"book_local_solo", "book_local_two", "book_local_multi", "book_intl_solo", "book_intl_two", "book_intl_multi", "book_chapter", "conference_paper"}},
	{"🔬 دراسات ومقالات", []string{"community_study_in", "community_study_out", "citations10", "review_article", "popular_article"}},
	{"🏅 براءات وأوسمة", []string{"patent_intl", "patent_local", "award_intl", "award_local"}},
	{"⚖️ لجان وعضويات", []string{"exam_committee", "thesis_committee", "scientific_committee", "conf_committee", "ministerial_committee", "union_academics", "union_professional"}},
	{"🎓 إشراف وتحرير", []string{"supervise_master", "supervise_phd", "editor_chief", "editor_member"}},
}

// promoActivityMap indexes all activities by key for quick label/points lookup.
func (a *App) promoActivityMap() map[string]promotion.Activity {
	m := map[string]promotion.Activity{}
	for _, t := range []int{1, 2} {
		for _, act := range a.promotion.ActivitiesByTable(t) {
			m[act.Key] = act
		}
	}
	return m
}

// promoEffectiveCategories returns the configured categories filtered to keys
// that actually exist, with any uncategorized activity appended under "أخرى".
func (a *App) promoEffectiveCategories() []promoCategory {
	acts := a.promoActivityMap()
	seen := map[string]bool{}
	var cats []promoCategory
	for _, c := range promoCategories {
		var keys []string
		for _, k := range c.Keys {
			if _, ok := acts[k]; ok {
				keys = append(keys, k)
				seen[k] = true
			}
		}
		if len(keys) > 0 {
			cats = append(cats, promoCategory{Label: c.Label, Keys: keys})
		}
	}
	var extra []string
	for _, t := range []int{1, 2} {
		for _, act := range a.promotion.ActivitiesByTable(t) {
			if !seen[act.Key] {
				extra = append(extra, act.Key)
				seen[act.Key] = true
			}
		}
	}
	if len(extra) > 0 {
		cats = append(cats, promoCategory{Label: "➕ بنود أخرى", Keys: extra})
	}
	return cats
}

// computeDraft runs the official calculation over an interactive draft, pulling
// service years out of the pseudo-key.
func (a *App) computeDraft(rankKey string, draft map[string]float64) promotion.Result {
	counts := make(map[string]float64, len(draft))
	years := 0
	for k, v := range draft {
		if k == promoYearsKey {
			years = int(v)
			continue
		}
		counts[k] = v
	}
	res, _ := a.promotion.Compute(promotion.Input{RankKey: rankKey, Counts: counts, ServiceYears: years})
	return res
}

// promoMethodScreen lets the user pick how to enter their indicators.
func (a *App) promoMethodScreen(rank promotion.Rank) Screen {
	return Screen{
		Text: "🎓 حاسبة الترقية: " + rank.Label + " ← " + rank.NextLabel + "\n\n" +
			"اختر طريقة الإدخال الأنسب لك:",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔘 منشئ تفاعلي بالأزرار (موصى به)", Data: "promo:build:" + rank.Key}},
			{{Text: "🤖 صِف إنجازاتك (ذكاء اصطناعي)", Data: "promo:ai:" + rank.Key}},
			{{Text: "✍️ كتابة المؤشّرات نصّاً", Data: "promo:text:" + rank.Key}},
			{{Text: "⬅️ رجوع", Data: "menu:promotion"}},
		}},
	}
}

// promoHomeScreen is the builder hub: live totals + category navigation.
func (a *App) promoHomeScreen(userID int64, rank promotion.Rank) Screen {
	draft := a.sessions.promoDraftCounts(userID)
	res := a.computeDraft(rank.Key, draft)

	var b strings.Builder
	b.WriteString("🎓 " + rank.Label + " ← " + rank.NextLabel + "\n")
	b.WriteString("🧮 مجموعك: " + fmtNum(res.Total) + "/" + fmtNum(rank.RequiredTotal) +
		"  (ج١ " + fmtNum(res.Table1) + "/" + fmtNum(rank.RequiredTable1) +
		" · ج٢ " + fmtNum(res.Table2) + "/" + fmtNum(rank.RequiredTable2) + ")\n")
	b.WriteString("⏱️ الخدمة: " + strconv.Itoa(int(draft[promoYearsKey])) + " سنوات\n\n")
	b.WriteString("اختر فئة لإضافة مؤشّراتها 👇")

	rows := [][]Button{}
	var pair []Button
	for i, c := range a.promoEffectiveCategories() {
		pair = append(pair, Button{Text: c.Label, Data: "promo:cat:" + strconv.Itoa(i)})
		if len(pair) == 2 {
			rows = append(rows, pair)
			pair = nil
		}
	}
	if len(pair) > 0 {
		rows = append(rows, pair)
	}
	rows = append(rows,
		[]Button{{Text: "⏱️ سنوات الخدمة", Data: "promo:add:" + promoYearsKey}},
		[]Button{{Text: "✅ احسب النتيجة", Data: "promo:done"}, {Text: "🔄 تصفير", Data: "promo:reset"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// promoCategoryScreen lists one category's indicators as buttons, showing any
// already-entered counts.
func (a *App) promoCategoryScreen(userID int64, rank promotion.Rank, idx int) Screen {
	cats := a.promoEffectiveCategories()
	if idx < 0 || idx >= len(cats) {
		return a.promoHomeScreen(userID, rank)
	}
	c := cats[idx]
	acts := a.promoActivityMap()
	draft := a.sessions.promoDraftCounts(userID)

	rows := [][]Button{}
	for _, k := range c.Keys {
		act := acts[k]
		label := act.Label + " (" + fmtNum(act.PointsFor(rank.Key)) + ")"
		if n := draft[k]; n > 0 {
			label = "✅ " + label + " ×" + fmtNum(n)
		}
		rows = append(rows, []Button{{Text: label, Data: "promo:add:" + k}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع للفئات", Data: "promo:home"}})
	return Screen{
		Text:     "— " + c.Label + " —\nاضغط المؤشّر ثم أرسل عدده:",
		Keyboard: &Keyboard{Rows: rows},
	}
}

func promoCancelKb() *Keyboard {
	return &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع للفئات", Data: "promo:home"}}}}
}

// promoAskCount prompts for an indicator's count and enters the awaiting state.
func (a *App) promoAskCount(ctx context.Context, userID int64, key string) {
	if key == promoYearsKey {
		a.sessions.promoAwaitCount(userID, key)
		a.send(ctx, userID, Screen{Text: "⏱️ أدخل سنوات الخدمة منذ آخر ترقية:\nأرسل رقماً.", Keyboard: promoCancelKb()})
		return
	}
	act, ok := a.promoActivityMap()[key]
	if !ok {
		return
	}
	rankKey := a.sessions.promoteRank(userID)
	a.sessions.promoAwaitCount(userID, key)
	a.send(ctx, userID, Screen{
		Text: "أدخل العدد لـ:\n«" + act.Label + "» (" + fmtNum(act.PointsFor(rankKey)) + " نقطة للوحدة)\n\n" +
			"أرسل رقماً (أرسل 0 لإلغاء هذا البند).",
		Keyboard: promoCancelKb(),
	})
}

// handlePromoCount records a number the user sent for the pending indicator.
func (a *App) handlePromoCount(ctx context.Context, msg *models.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	key := a.sessions.promoPendingKey(userID)
	rank, ok := a.promotion.FindRank(a.sessions.promoteRank(userID))
	if key == "" || !ok {
		a.sessions.clear(userID)
		a.send(ctx, chatID, a.promotionRankScreen())
		return
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(msg.Text), 64)
	if err != nil || n < 0 {
		a.send(ctx, chatID, Screen{Text: "أرسل رقماً صحيحاً (مثل 1 أو 2)، أو 0 للإلغاء.", Keyboard: promoCancelKb()})
		return // stay awaiting
	}
	a.sessions.promoSetCount(userID, key, n)
	a.send(ctx, chatID, a.promoHomeScreen(userID, rank))
}

package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/promotion"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// promotionRankScreen asks the user to pick their current rank.
func (a *App) promotionRankScreen() Screen {
	rows := [][]Button{}
	for _, rk := range a.promotion.Ranks {
		rows = append(rows, []Button{{Text: rk.Label + " ← " + rk.NextLabel, Data: "promo:rank:" + rk.Key}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})
	return Screen{
		Text: "🎓 حاسبة نقاط الترقية العلمية\n" +
			"(تعليمات رقم ١٠ لسنة ٢٠٢٥)\n\n" +
			"اختر رتبتك الحالية:",
		Keyboard: &Keyboard{Rows: rows},
	}
}

// handlePromotion routes "promo:" callbacks (rank selection).
func (a *App) handlePromotion(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	rankKey := strings.TrimPrefix(cq.Data, "promo:rank:")
	rank, ok := a.promotion.FindRank(rankKey)
	if !ok {
		a.send(ctx, cq.From.ID, a.promotionRankScreen())
		return
	}
	a.sessions.startPromotion(cq.From.ID, rankKey)
	a.send(ctx, cq.From.ID, a.promotionInputScreen(rank))
}

// promotionInputScreen lists the official requirements and the activity keys
// (with this rank's point values) for the user to fill in.
func (a *App) promotionInputScreen(rank promotion.Rank) Screen {
	var b strings.Builder
	b.WriteString("📊 " + rank.Label + " ← " + rank.NextLabel + "\n")
	b.WriteString("المطلوب: المجموع ≥ " + fmtNum(rank.RequiredTotal) +
		" · جدول١ ≥ " + fmtNum(rank.RequiredTable1) +
		" · جدول٢ ≥ " + fmtNum(rank.RequiredTable2) +
		" · الخدمة ≥ " + strconv.Itoa(rank.MinServiceYears) + " سنوات\n\n")
	b.WriteString("أرسل أعدادك (كل سطر «المفتاح: العدد»):\n")

	b.WriteString("\n— الجدول ١: البحوث —\n")
	for _, act := range a.promotion.ActivitiesByTable(1) {
		b.WriteString(act.Key + ": 0  (" + act.Label + " = " + fmtNum(act.PointsFor(rank.Key)) + ")\n")
	}
	b.WriteString("\n— الجدول ٢: النشاطات —\n")
	for _, act := range a.promotion.ActivitiesByTable(2) {
		line := act.Key + ": 0  (" + act.Label + " = " + fmtNum(act.PointsFor(rank.Key))
		if act.Cap > 0 {
			line += "، سقف " + fmtNum(act.Cap)
		}
		b.WriteString(line + ")\n")
	}
	b.WriteString("\nyears: 0  (سنوات الخدمة منذ آخر ترقية)")

	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ إلغاء", Data: "menu:promotion"}},
		}},
	}
}

// handlePromotionActivities parses the activities message and shows the result.
func (a *App) handlePromotionActivities(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	rankKey := a.sessions.promoteRank(msg.From.ID)
	a.sessions.clear(msg.From.ID)

	counts, years := promotion.ParseActivities(msg.Text)
	res, ok := a.promotion.Compute(promotion.Input{RankKey: rankKey, Counts: counts, ServiceYears: years})
	if !ok {
		a.send(ctx, chatID, a.promotionRankScreen())
		return
	}
	a.send(ctx, chatID, promotionResultScreen(res))
}

// gate renders a ✅/⛔ line "label: got / need".
func gate(label string, got, need float64, met bool) string {
	icon := "⛔"
	if met {
		icon = "✅"
	}
	return icon + " " + label + ": " + fmtNum(got) + " / " + fmtNum(need)
}

// promotionResultScreen renders the four-gate verdict and the points breakdown.
func promotionResultScreen(res promotion.Result) Screen {
	var b strings.Builder
	b.WriteString("📊 نتيجة الترقية\n" + res.Rank.Label + " ← " + res.Rank.NextLabel + "\n\n")

	b.WriteString(gate("الجدول ١ (بحوث)", res.Table1, res.Rank.RequiredTable1, res.Table1Met) + "\n")
	b.WriteString(gate("الجدول ٢ (نشاطات)", res.Table2, res.Rank.RequiredTable2, res.Table2Met) + "\n")
	b.WriteString(gate("المجموع الكلي", res.Total, res.Rank.RequiredTotal, res.TotalMet) + "\n")
	serviceIcon := "⛔"
	if res.ServiceMet {
		serviceIcon = "✅"
	}
	b.WriteString(serviceIcon + " الخدمة: " + strconv.Itoa(res.ServiceYears) +
		" / " + strconv.Itoa(res.Rank.MinServiceYears) + " سنوات\n\n")

	if res.Eligible {
		b.WriteString("🎉 مستوفٍ لشروط التقديم.\n")
	} else {
		b.WriteString("⏳ لم تكتمل الشروط بعد — راجع البنود المعلّمة بـ ⛔.\n")
	}

	if lines := breakdownText(res); lines != "" {
		b.WriteString("\nالتفصيل:\n" + lines)
	}
	b.WriteString("\nℹ️ تقدير استرشادي وفق تعليمات ١٠/٢٠٢٥ — القرار النهائي للجنة الترقيات.")

	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "🔁 إعادة الحساب", Data: "menu:promotion"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// breakdownText lists the entered items and their (capped) points.
func breakdownText(res promotion.Result) string {
	var b strings.Builder
	write := func(items []promotion.LineItem) {
		for _, li := range items {
			b.WriteString("• " + li.Label + ": " + fmtNum(li.Count) + " → " + fmtNum(li.Points))
			if li.Capped {
				b.WriteString(" (بلغ السقف)")
			}
			b.WriteString("\n")
		}
	}
	write(res.Breakdown1)
	write(res.Breakdown2)
	return b.String()
}

// fmtNum formats a number without a trailing ".0" for whole values.
func fmtNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

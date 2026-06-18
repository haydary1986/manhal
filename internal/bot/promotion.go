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
	for _, rk := range a.promotion.Ranks() {
		rows = append(rows, []Button{{Text: rk.Label + " ← " + rk.NextLabel, Data: "promo:rank:" + rk.Key}})
	}
	rows = append(rows,
		[]Button{{Text: "📊 الجدول الأول (البحوث)", Data: "promo:table1"}, {Text: "📋 الجدول الثاني (النشاطات)", Data: "promo:table2"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{
		Text: "🎓 حاسبة نقاط الترقية العلمية\n" +
			"(تعليمات الترقيات العلمية 2025-2026)\n\n" +
			"📊 اطّلع على الجدولين الرسميين، أو اختر رتبتك الحالية لحساب نقاطك:",
		Keyboard: &Keyboard{Rows: rows},
	}
}

// handlePromotion routes all "promo:" callbacks: rank choice, the method
// chooser, and the interactive builder (categories/items/compute).
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
	userID := cq.From.ID
	data := strings.TrimPrefix(cq.Data, "promo:")

	switch {
	case data == "table1":
		a.send(ctx, userID, promotionTable1Screen())
	case data == "table2":
		a.send(ctx, userID, promotionTable2Screen())
	case strings.HasPrefix(data, "rank:"):
		rank, ok := a.promotion.FindRank(strings.TrimPrefix(data, "rank:"))
		if !ok {
			a.send(ctx, userID, a.promotionRankScreen())
			return
		}
		a.sessions.promoBegin(userID, rank.Key)
		a.send(ctx, userID, a.promoMethodScreen(rank))
	case strings.HasPrefix(data, "build:"):
		rank, ok := a.promotion.FindRank(strings.TrimPrefix(data, "build:"))
		if !ok {
			return
		}
		a.sessions.promoBegin(userID, rank.Key)
		a.send(ctx, userID, a.promoHomeScreen(userID, rank))
	case strings.HasPrefix(data, "text:"):
		rank, ok := a.promotion.FindRank(strings.TrimPrefix(data, "text:"))
		if !ok {
			return
		}
		a.sessions.startPromotion(userID, rank.Key)
		a.send(ctx, userID, a.promotionInputScreen(rank))
	case strings.HasPrefix(data, "ai:"):
		rank, ok := a.promotion.FindRank(strings.TrimPrefix(data, "ai:"))
		if !ok {
			return
		}
		a.sessions.startPromotionAI(userID, rank.Key)
		a.send(ctx, userID, promoAIPromptScreen(rank))
	case strings.HasPrefix(data, "cat:"):
		rank, ok := a.promotion.FindRank(a.sessions.promoteRank(userID))
		if !ok {
			a.send(ctx, userID, a.promotionRankScreen())
			return
		}
		idx, _ := strconv.Atoi(strings.TrimPrefix(data, "cat:"))
		a.send(ctx, userID, a.promoCategoryScreen(userID, rank, idx))
	case strings.HasPrefix(data, "add:"):
		a.promoAskCount(ctx, userID, strings.TrimPrefix(data, "add:"))
	case data == "home", data == "reset":
		if data == "reset" {
			a.sessions.promoResetDraft(userID)
		}
		rank, ok := a.promotion.FindRank(a.sessions.promoteRank(userID))
		if !ok {
			a.send(ctx, userID, a.promotionRankScreen())
			return
		}
		a.send(ctx, userID, a.promoHomeScreen(userID, rank))
	case data == "done":
		res := a.computeDraft(a.sessions.promoteRank(userID), a.sessions.promoDraftCounts(userID))
		a.sessions.clear(userID)
		a.send(ctx, userID, promotionResultScreen(res))
	default:
		a.send(ctx, userID, a.promotionRankScreen())
	}
}

// promotionInputScreen lists the official requirements and the activity keys
// (with this rank's point values) for the user to fill in.
func (a *App) promotionInputScreen(rank promotion.Rank) Screen {
	var b strings.Builder
	b.WriteString("📊 الترقية: " + rank.Label + " ← " + rank.NextLabel + "\n")
	b.WriteString("🎯 المطلوب: المجموع ≥ " + fmtNum(rank.RequiredTotal) +
		" · الجدول ١ ≥ " + fmtNum(rank.RequiredTable1) +
		" · الجدول ٢ ≥ " + fmtNum(rank.RequiredTable2) +
		" · الخدمة ≥ " + strconv.Itoa(rank.MinServiceYears) + " سنوات\n\n")
	b.WriteString("✍️ الطريقة: أرسل البنود التي تنطبق عليك فقط، كل بند في سطر بصيغة «المفتاح: العدد».\n")
	b.WriteString("لا حاجة لإرسال ما قيمته صفر.\n\n")
	b.WriteString("مثال على الإرسال:\n")
	b.WriteString("if_first: 1\n")
	b.WriteString("iq_second: 2\n")
	b.WriteString("years: 4\n")

	b.WriteString("\n━━━ الجدول ١: البحوث ━━━\n")
	for _, act := range a.promotion.ActivitiesByTable(1) {
		b.WriteString("🔹 " + act.Key + " — " + act.Label + " (" + fmtNum(act.PointsFor(rank.Key)) + " نقطة)\n")
	}
	b.WriteString("\n━━━ الجدول ٢: النشاطات ━━━\n")
	for _, act := range a.promotion.ActivitiesByTable(2) {
		line := "🔸 " + act.Key + " — " + act.Label + " (" + fmtNum(act.PointsFor(rank.Key)) + " نقطة"
		if act.Cap > 0 {
			line += "، السقف " + fmtNum(act.Cap)
		}
		b.WriteString(line + ")\n")
	}
	b.WriteString("\n⏱️ years — سنوات الخدمة منذ آخر ترقية\n")
	b.WriteString("\nأرسل بنودك الآن 👇")

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

	counts, years := promotion.ParseActivities(msg.Text)
	if len(counts) == 0 && years == 0 {
		// Nothing recognized — guide the user and keep the session so they can
		// simply retype without restarting the flow.
		a.send(ctx, chatID, Screen{
			Text: "🤔 لم أتعرّف على أي بند في رسالتك.\n\n" +
				"أرسل كل بند في سطر بصيغة «المفتاح: العدد»، مثل:\n" +
				"if_first: 1\niq_second: 2\nyears: 4\n\n" +
				"انسخ المفاتيح من القائمة بالأعلى وأعِد الإرسال.",
			Keyboard: &Keyboard{Rows: [][]Button{
				{{Text: "🔁 عرض القائمة من جديد", Data: "menu:promotion"}},
				{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}},
			}},
		})
		return
	}
	a.sessions.clear(msg.From.ID)

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

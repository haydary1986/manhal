package bot

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/plans"
	"github.com/erticaz/manhal/internal/store"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// premiumBenefits is the single source of truth for what a paid subscription
// unlocks. It is shown on the subscribe screen and on the premium gate so the
// value is always clear and consistent.
func premiumBenefits() []string {
	return []string{
		"حدود استخدام يومية أعلى للأدوات الذكية (أو بلا حدود)",
		"إعادة الكتابة البشرية لملفات Word كاملة (لا النص فقط)",
		"أولوية في الدعم الفني",
	}
}

// plansList renders the plan catalogue as bullet lines with price + duration.
func plansList(pl []plans.Plan) string {
	var b strings.Builder
	for _, p := range pl {
		b.WriteString("• " + p.Name + " — " + strconv.Itoa(p.Price) + " د.ع / " + p.DurationLabel() + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// benefitsList renders the benefits as bullet lines.
func benefitsList() string {
	var b strings.Builder
	for _, x := range premiumBenefits() {
		b.WriteString("• " + x + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// premiumGateScreen is shown when a free user reaches a premium-only feature.
func premiumGateScreen(reason string) Screen {
	text := "💎 هذه ميزة مخصّصة للمشتركين."
	if strings.TrimSpace(reason) != "" {
		text += "\n" + reason
	}
	text += "\n\n✨ مزايا الاشتراك:\n" + benefitsList()
	return Screen{
		Text: text,
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "💎 الاشتراك / الترقية", Data: "menu:subscribe"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// subscribeScreen shows the premium benefits and plans plus the manual payment
// details the admin configured, with a button to register a paid request.
func subscribeScreen(bs config.BotSettings, pl []plans.Plan) Screen {
	text := "💎 الاشتراك المميّز في منهل\n\n✨ ماذا تكسب:\n" + benefitsList()
	if len(pl) > 0 {
		text += "\n\n📦 الباقات المتاحة:\n" + plansList(pl)
	}
	if info := strings.TrimSpace(bs.PremiumInfo); info != "" {
		text += "\n\n" + info
	}
	if pay := strings.TrimSpace(bs.PaymentDetails); pay != "" {
		text += "\n\n💳 طريقة الدفع:\n" + pay
	}
	text += "\n\nبعد إتمام التحويل اضغط الزر أدناه ليصل طلبك، وسنفعّل اشتراكك بعد تأكيد الدفع."

	var rows [][]Button
	if link := strings.TrimSpace(bs.PaymentLink); link != "" {
		rows = append(rows, []Button{{Text: "💳 ادفع عبر التطبيق", URL: link}})
	}
	rows = append(rows,
		[]Button{{Text: "✅ طلبت الاشتراك / دفعت", Data: "premium:request"}},
		[]Button{{Text: "🎁 لديّ كود هدية", Data: "premium:code"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: text, Keyboard: &Keyboard{Rows: rows}}
}

// handlePremium routes the "premium:" callbacks (paid request or gift-code entry).
func (a *App) handlePremium(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	if strings.TrimPrefix(cq.Data, "premium:") == "code" {
		a.sessions.set(cq.From.ID, stateAwaitGiftCode)
		a.send(ctx, cq.From.ID, Screen{
			Text:     "🎁 أرسل كود الهدية الذي حصلت عليه:",
			Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع", Data: "menu:home"}}}},
		})
		return
	}

	const msg = "💎 طلب اشتراك بريميم — يرجى تأكيد الدفع وتفعيل الحساب."
	ticket := domain.Ticket{
		ID:       supportID(cq.From.ID, msg),
		UserID:   cq.From.ID,
		UserName: userDisplayName(&cq.From),
		Message:  msg,
		Status:   domain.TicketOpen,
	}
	if err := a.store.AddTicket(ctx, ticket); err != nil {
		a.logf("premium request add: %v", err)
	}
	a.send(ctx, cq.From.ID, Screen{
		Text:     "✅ وصل طلب اشتراكك!\nسيتواصل معك فريقنا لتأكيد الدفع وتفعيل المزايا المميّزة قريباً. شكراً لك 🌟",
		Keyboard: supportNav(),
	})
}

// handleGiftCode redeems a gift code the user sent and grants its premium tier.
func (a *App) handleGiftCode(ctx context.Context, msg *models.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	a.sessions.clear(userID)

	code := strings.ToUpper(strings.TrimSpace(msg.Text))
	if code == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل كوداً صحيحاً.", Keyboard: giftNav()})
		return
	}

	g, err := a.store.RedeemGiftCode(ctx, code, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		a.send(ctx, chatID, Screen{Text: "❌ كود غير صحيح. تأكّد منه وحاول مجدداً.", Keyboard: giftNav()})
		return
	case errors.Is(err, store.ErrCodeUsed):
		a.send(ctx, chatID, Screen{Text: "❌ هذا الكود مُستخدَم مسبقاً.", Keyboard: giftNav()})
		return
	case err != nil:
		a.logf("redeem gift: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر تفعيل الكود الآن، جرّب لاحقاً.", Keyboard: giftNav()})
		return
	}

	u, gerr := a.store.GetUser(ctx, userID)
	if gerr != nil || u == nil {
		u = &domain.User{TelegramID: userID, Name: userDisplayName(msg.From)}
	}
	u.Tier = g.Tier
	if g.Days > 0 {
		until := time.Now().AddDate(0, 0, g.Days)
		u.PremiumUntil = &until
	} else {
		u.PremiumUntil = nil
	}
	if err := a.store.SaveUser(ctx, u); err != nil {
		a.logf("save user after gift: %v", err)
	}

	dur := "بشكل دائم"
	if g.Days > 0 {
		dur = "لمدة " + strconv.Itoa(g.Days) + " يوم"
	}
	a.send(ctx, chatID, Screen{
		Text:     "🎉 تم تفعيل اشتراكك المميّز " + dur + "! استمتع بكامل الميزات 🌟",
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}}}},
	})
}

func giftNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🎁 جرّب كوداً آخر", Data: "premium:code"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

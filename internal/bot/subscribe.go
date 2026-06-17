package bot

import (
	"context"
	"strings"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// subscribeScreen shows the premium plans and the manual payment details the
// admin configured, plus a button to register a paid request.
func subscribeScreen(bs config.BotSettings) Screen {
	info := strings.TrimSpace(bs.PremiumInfo)
	if info == "" {
		info = "الاشتراك المميّز يرفع حدود الاستخدام اليومية ويفتح كامل الأدوات."
	}
	text := "💎 الاشتراك المميّز في منهل\n\n" + info
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
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: text, Keyboard: &Keyboard{Rows: rows}}
}

// handlePremiumRequest records a premium request as a ticket so the admin can
// confirm the manual payment and grant the tier from the dashboard.
func (a *App) handlePremiumRequest(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

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

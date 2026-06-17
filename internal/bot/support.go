package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// supportPromptScreen invites the user to write to support directly.
func supportPromptScreen() Screen {
	return Screen{
		Text: "📞 تواصل مع الدعم الفني\n\n" +
			"لم تجد ما تريد؟ اكتب طلبك أو استفسارك هنا،\n" +
			"وسيراجعه فريقنا ويرد عليك مباشرة عبر البوت.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleSupportMessage stores the user's support request as a ticket.
func (a *App) handleSupportMessage(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	text, _ := truncateRunes(strings.TrimSpace(msg.Text), 3000)
	if text == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل رسالة غير فارغة.", Keyboard: supportNav()})
		return
	}

	ticket := domain.Ticket{
		ID:       supportID(msg.From.ID, text),
		UserID:   msg.From.ID,
		UserName: userDisplayName(msg.From),
		Message:  text,
		Status:   domain.TicketOpen,
	}
	if err := a.store.AddTicket(ctx, ticket); err != nil {
		a.logf("support add: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّر إرسال الطلب ⚠️ جرّب لاحقاً.", Keyboard: supportNav()})
		return
	}
	a.send(ctx, chatID, Screen{
		Text: "✅ وصلت رسالتك إلى فريق الدعم.\n" +
			"أرسل المزيد لمتابعة المحادثة، وسنرد عليك هنا. اضغط «⬅️ رجوع» عند الانتهاء.",
		Keyboard: supportNav(),
	})
}

// Notify pushes a message to a user (implements web.Notifier so admin replies
// reach the user through the bot).
func (a *App) Notify(userID int64, text string) error {
	_, err := a.bot.SendMessage(context.Background(), &tg.SendMessageParams{ChatID: userID, Text: text})
	return err
}

// userDisplayName builds a readable name from a Telegram user.
func userDisplayName(u *models.User) string {
	if u == nil {
		return ""
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	return name
}

// supportID derives a short stable id for a ticket.
func supportID(userID int64, msg string) string {
	seed := strconv.FormatInt(userID, 10) + "|" + msg + "|" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:10]
}

func supportNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

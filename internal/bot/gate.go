package bot

import (
	"context"
	"strings"

	"github.com/erticaz/manhal/internal/config"
	tg "github.com/go-telegram/bot"
)

// gateScreen is shown when the user must join the required channel first.
func gateScreen(bs config.BotSettings) Screen {
	return Screen{
		Text: "🔒 لتفعيل منهل، اشترك أولاً في القناة:\n" + bs.RequiredChannel +
			"\n\nبعد الاشتراك اضغط «✅ تحقّقت».",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "📢 فتح القناة", URL: channelURL(bs.RequiredChannel)}},
			{{Text: "✅ تحقّقت", Data: "gate:check"}},
		}},
	}
}

func channelURL(ch string) string {
	switch {
	case ch == "":
		return "https://t.me"
	case strings.HasPrefix(ch, "http"):
		return ch
	case strings.HasPrefix(ch, "@"):
		return "https://t.me/" + strings.TrimPrefix(ch, "@")
	default:
		return "https://t.me/" + ch
	}
}

// channelChatID converts whatever the admin entered (a t.me link, a @handle, a
// bare username, or a numeric -100… id) into the ChatID form that the Telegram
// API's GetChatMember accepts ("@username" or a numeric id). Without this, a
// link like "https://t.me/foo" makes the membership check fail and the gate
// silently lets everyone through.
func channelChatID(ch string) string {
	ch = strings.TrimSpace(ch)
	for _, p := range []string{"https://t.me/", "http://t.me/", "https://telegram.me/", "t.me/"} {
		ch = strings.TrimPrefix(ch, p)
	}
	ch = strings.TrimSpace(strings.TrimPrefix(ch, "@"))
	switch {
	case ch == "":
		return ""
	case strings.HasPrefix(ch, "-"): // numeric private-channel id
		return ch
	default:
		return "@" + ch
	}
}

// isSubscribed reports whether the user is a member of the required channel.
// When subscription is not required it always returns true.
func (a *App) isSubscribed(ctx context.Context, userID int64) bool {
	bs := a.settings.Get()
	if !bs.RequireSubscription || bs.RequiredChannel == "" {
		return true
	}
	member, err := a.bot.GetChatMember(ctx, &tg.GetChatMemberParams{
		ChatID: channelChatID(bs.RequiredChannel),
		UserID: userID,
	})
	if err != nil {
		// Misconfiguration (bot not admin, bad channel) — don't lock users out.
		a.logf("gate: GetChatMember error: %v", err)
		return true
	}
	switch {
	case member.Owner != nil, member.Administrator != nil, member.Member != nil:
		return true
	case member.Restricted != nil:
		return member.Restricted.IsMember
	default:
		return false
	}
}

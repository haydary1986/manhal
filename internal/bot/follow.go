package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/domain"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// followBaselineCount is how many current papers are marked "seen" at subscribe
// time, so the user is later alerted only to genuinely new papers.
const followBaselineCount = 10

// handleFollow routes "sub:" callbacks (add / remove / open).
func (a *App) handleFollow(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	data := strings.TrimPrefix(cq.Data, "sub:")
	switch {
	case data == "add":
		a.sessions.set(cq.From.ID, stateAwaitFollowTopic)
		a.send(ctx, cq.From.ID, followPromptScreen())
	case strings.HasPrefix(data, "rm:"):
		_ = a.store.RemoveSubscription(ctx, cq.From.ID, strings.TrimPrefix(data, "rm:"))
		a.send(ctx, cq.From.ID, a.followScreen(ctx, cq.From.ID))
	default:
		a.send(ctx, cq.From.ID, a.followScreen(ctx, cq.From.ID))
	}
}

// followScreen lists the user's followed topics with add/remove controls.
func (a *App) followScreen(ctx context.Context, userID int64) Screen {
	subs, _ := a.store.ListSubscriptions(ctx, userID)

	var b strings.Builder
	b.WriteString("🔔 متابعاتي\n")
	if len(subs) == 0 {
		b.WriteString("\nلا تتابع أي موضوع بعد. أضِف موضوعاً لتصلك أوراقه الجديدة.")
	} else {
		b.WriteString("\nسأنبّهك بالأوراق الجديدة في:")
	}

	rows := [][]Button{}
	for _, s := range subs {
		b.WriteString("\n• " + s.Topic)
		rows = append(rows, []Button{{Text: "🗑️ إلغاء «" + truncLabel(s.Topic) + "»", Data: "sub:rm:" + s.ID}})
	}

	// Citation watches (#5): researchers followed for new-citation alerts.
	watches, _ := a.store.ListCitationWatches(ctx, userID)
	if len(watches) > 0 {
		b.WriteString("\n\n📈 تنبيهات الاستشهاد:")
		for _, w := range watches {
			b.WriteString("\n• " + w.AuthorName)
			rows = append(rows, []Button{{Text: "🗑️ إيقاف «" + truncLabel(w.AuthorName) + "»", Data: "cwatch:rm:" + w.ID}})
		}
	}

	rows = append(rows,
		[]Button{{Text: "➕ تابع موضوعاً", Data: "sub:add"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: b.String(), Keyboard: &Keyboard{Rows: rows}}
}

// followPromptScreen asks for a topic to follow.
func followPromptScreen() Screen {
	return Screen{
		Text: "➕ تابع موضوعاً\n\n" +
			"أرسل موضوعاً أو كلمات مفتاحية (إنجليزية غالباً)،\n" +
			"وسأنبّهك تلقائياً بالأوراق الجديدة التي تُنشَر فيه.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع لمتابعاتي", Data: "sub:open"}},
		}},
	}
}

// handleFollowTopic creates a subscription, baselining current papers as "seen".
func (a *App) handleFollowTopic(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	topic := strings.TrimSpace(msg.Text)
	if topic == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل موضوعاً غير فارغ.", Keyboard: followNav()})
		return
	}

	// Baseline: mark currently-known papers as seen so only newer ones alert.
	seen := a.baselineDOIs(ctx, topic)

	sub := domain.Subscription{
		ID:       subscriptionID(msg.From.ID, topic),
		UserID:   msg.From.ID,
		Topic:    topic,
		SeenDOIs: seen,
	}
	if err := a.store.AddSubscription(ctx, sub); err != nil {
		a.logf("follow add: %v", err)
		a.send(ctx, chatID, Screen{Text: "❌ تعذّرت المتابعة ⚠️", Keyboard: followNav()})
		return
	}
	a.send(ctx, chatID, Screen{
		Text:     "🔔 تابعت «" + topic + "».\nسأنبّهك بالأوراق الجديدة فور نشرها.",
		Keyboard: followNav(),
	})
}

// baselineDOIs returns the DOIs currently published for a topic (best effort).
func (a *App) baselineDOIs(ctx context.Context, topic string) []string {
	if a.search == nil {
		return nil
	}
	results, err := a.search.Search(ctx, topic, followBaselineCount)
	if err != nil {
		return nil // an empty baseline just means the first poll may alert existing papers
	}
	seen := make([]string, 0, len(results))
	for _, r := range results {
		if r.DOI != "" {
			seen = append(seen, r.DOI)
		}
	}
	return seen
}

// subscriptionID derives a short stable id from the user and topic (so the same
// topic isn't followed twice).
func subscriptionID(userID int64, topic string) string {
	seed := strconv.FormatInt(userID, 10) + "|" + strings.ToLower(strings.TrimSpace(topic))
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:10]
}

func truncLabel(s string) string {
	r := []rune(s)
	if len(r) <= 18 {
		return s
	}
	return string(r[:18]) + "…"
}

func followNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔔 متابعاتي", Data: "sub:open"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// maxAnnouncements caps how many items one screen shows.
const maxAnnouncements = 6

// uiTab maps a UI filter tab to the kinds it includes.
var uiTabs = map[string][]announce.Kind{
	"all":   nil,
	"conf":  {announce.KindConference, announce.KindCFP},
	"grant": {announce.KindGrant, announce.KindFellowship},
	"job":   {announce.KindJob},
}

var kindLabels = map[announce.Kind]string{
	announce.KindConference: "🎓 مؤتمر",
	announce.KindCFP:        "📝 دعوة أبحاث",
	announce.KindGrant:      "💰 منحة",
	announce.KindFellowship: "🌍 زمالة",
	announce.KindJob:        "💼 وظيفة",
}

// handleAnnouncements renders the announcements feed for an "ann:<tab>" tap.
func (a *App) handleAnnouncements(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	tab := strings.TrimPrefix(cq.Data, "ann:")
	if tab == "field" {
		a.send(ctx, cq.From.ID, a.disciplinePicker(ctx, cq.From.ID))
		return
	}
	if _, ok := uiTabs[tab]; !ok {
		tab = "all"
	}
	a.send(ctx, cq.From.ID, a.announcementsScreen(ctx, cq.From.ID, tab))
}

// handleField sets or clears the user's discipline, then re-shows the feed.
func (a *App) handleField(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	id := strings.TrimPrefix(cq.Data, "field:")
	if id == "clear" {
		id = ""
	}
	a.setUserField(ctx, cq.From.ID, id)
	a.send(ctx, cq.From.ID, a.announcementsScreen(ctx, cq.From.ID, "all"))
}

// announcementsScreen builds the feed filtered by the chosen tab and the user's
// saved discipline.
func (a *App) announcementsScreen(ctx context.Context, userID int64, tab string) Screen {
	field := a.userField(ctx, userID)
	now := time.Now()
	items := a.announce.List(now, announce.Filter{
		Kinds:      uiTabs[tab],
		Discipline: field,
	})

	var b strings.Builder
	b.WriteString("📢 الإعلانات الأكاديمية\n")
	b.WriteString("🎯 تخصصك: " + a.fieldLabel(field) + "\n")

	switch {
	case len(items) == 0:
		b.WriteString("\nلا توجد إعلانات مطابقة حالياً.\nجرّب تبويباً آخر أو غيّر تخصّصك.")
	default:
		shown := items
		if len(shown) > maxAnnouncements {
			shown = shown[:maxAnnouncements]
		}
		for i, item := range shown {
			b.WriteString("\n" + strconv.Itoa(i+1) + ") " + renderAnnouncement(item, now) + "\n")
		}
		if len(items) > maxAnnouncements {
			b.WriteString("\n…و" + strconv.Itoa(len(items)-maxAnnouncements) + " إعلاناً آخر.")
		}
	}

	return Screen{Text: b.String(), Keyboard: announcementsKeyboard(tab)}
}

// renderAnnouncement formats one item as a text block.
func renderAnnouncement(item announce.Announcement, now time.Time) string {
	label := kindLabels[item.Kind]
	if label == "" {
		label = "📌 إعلان"
	}
	var b strings.Builder
	b.WriteString(label + " — " + item.Title + "\n")
	if days, ok := item.DaysLeft(now); ok && item.Deadline != nil {
		line := "   🗓️ آخر موعد: " + item.Deadline.Format("2006-01-02")
		switch {
		case days == 0:
			line += " · ينتهي اليوم ⚠️"
		case days <= 7:
			line += " · متبقّي " + strconv.Itoa(days) + " يوم ⏰"
		default:
			line += " · متبقّي " + strconv.Itoa(days) + " يوم"
		}
		b.WriteString(line + "\n")
	}
	if item.Body != "" {
		b.WriteString("   " + item.Body + "\n")
	}
	if item.Image != "" {
		b.WriteString("   🖼️ " + item.Image + "\n")
	}
	if item.Link != "" {
		b.WriteString("   🔗 " + item.Link)
	}
	return strings.TrimRight(b.String(), "\n")
}

// announcementsKeyboard renders the filter tabs (active one marked) plus nav.
func announcementsKeyboard(active string) *Keyboard {
	tab := func(id, label string) Button {
		if id == active {
			label = "• " + label
		}
		return Button{Text: label, Data: "ann:" + id}
	}
	return &Keyboard{Rows: [][]Button{
		{tab("all", "📋 الكل"), tab("conf", "🎓 مؤتمرات")},
		{tab("grant", "💰 منح"), tab("job", "💼 وظائف")},
		{{Text: "🎯 تخصّصي", Data: "ann:field"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

// disciplinePicker builds the discipline-selection screen.
func (a *App) disciplinePicker(ctx context.Context, userID int64) Screen {
	current := a.userField(ctx, userID)
	rows := [][]Button{}
	var pair []Button
	for _, d := range a.disciplines {
		label := d.Label
		if d.ID == current {
			label = "✅ " + label
		}
		pair = append(pair, Button{Text: label, Data: "field:" + d.ID})
		if len(pair) == 2 {
			rows = append(rows, pair)
			pair = nil
		}
	}
	if len(pair) > 0 {
		rows = append(rows, pair)
	}
	rows = append(rows, []Button{{Text: "🌐 كل التخصصات", Data: "field:clear"}})
	rows = append(rows, []Button{{Text: "⬅️ رجوع للإعلانات", Data: "ann:all"}})

	return Screen{
		Text:     "🎯 اختر تخصّصك لتصفية الإعلانات المناسبة لك:",
		Keyboard: &Keyboard{Rows: rows},
	}
}

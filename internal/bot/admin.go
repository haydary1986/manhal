package bot

import (
	"context"
	"strings"

	"github.com/erticaz/manhal/internal/menu"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// adminActions lists the action keys an admin can attach to a new button, plus
// the "submenu" option for a container. Order is the display order.
var adminActions = []struct{ Key, Label string }{
	{"submenu", "📁 قائمة فرعية (تحتوي أزراراً)"},
	{"announcements", "📢 الإعلانات"},
	{"follow", "🔔 متابعاتي"},
	{"search", "🔍 بحث عن ورقة"},
	{"cite", "📝 توليد اقتباس"},
	{"journal", "🛡️ فحص مجلة"},
	{"oa", "🔓 نسخة مجانية"},
	{"author", "👤 ملف باحث"},
	{"radar", "📡 رادار البحث"},
	{"trends", "🔥 ترندات المجال"},
	{"promotion", "🎓 حاسبة الترقيات"},
	{"publish", "🧭 وين أنشر؟"},
	{"litreview", "📖 مراجعة الأدبيات"},
	{"gap", "🧩 كاشف الفجوة البحثية"},
	{"similarity", "🔍 فحص التشابه المبدئي"},
	{"stats", "📊 المساعد الإحصائي"},
	{"latex", "📐 مدقّق LaTeX/Word"},
	{"viva", "🎤 تحضير المناقشة"},
	{"pdfchat", "📄 محادثة الـ PDF"},
	{"ai", "🤖 المساعد الذكي"},
	{"library", "⭐ مكتبتي"},
	{"semantic", "🧠 بحث دلالي بمكتبتي"},
	{"support", "📞 الدعم الفني"},
	{"help", "ℹ️ مساعدة"},
}

// handleAdminCommand opens the admin panel for the /admin command.
func (a *App) handleAdminCommand(ctx context.Context, _ *tg.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	if !a.cfg.IsAdmin(msg.From.ID) {
		a.send(ctx, msg.Chat.ID, Screen{Text: "هذا الأمر للمشرفين فقط 🔒"})
		return
	}
	a.send(ctx, msg.Chat.ID, a.adminPanelScreen())
}

// handleAdmin routes the "admin:" callbacks (admin-gated).
func (a *App) handleAdmin(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	if !a.cfg.IsAdmin(cq.From.ID) {
		_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID, Text: "للمشرفين فقط 🔒", ShowAlert: true,
		})
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	data := strings.TrimPrefix(cq.Data, "admin:")
	switch {
	case data == "panel":
		a.send(ctx, cq.From.ID, a.adminPanelScreen())
	case data == "add":
		a.send(ctx, cq.From.ID, a.adminParentPickerScreen())
	case data == "del":
		a.send(ctx, cq.From.ID, a.adminDeleteListScreen())
	case strings.HasPrefix(data, "parent:"):
		parent := strings.TrimPrefix(data, "parent:")
		a.sessions.startAdminLabel(cq.From.ID, parent)
		a.send(ctx, cq.From.ID, Screen{Text: "✍️ أرسل نص الزر الجديد (مثلاً: «📊 إحصائياتي»):"})
	case strings.HasPrefix(data, "action:"):
		a.finishAddButton(ctx, cq.From.ID, strings.TrimPrefix(data, "action:"))
	case strings.HasPrefix(data, "delete:"):
		a.finishDeleteButton(ctx, cq.From.ID, strings.TrimPrefix(data, "delete:"))
	default:
		a.send(ctx, cq.From.ID, a.adminPanelScreen())
	}
}

// handleAdminLabel captures the button label the admin just typed and asks for
// the action to attach. Invoked from defaultHandler on stateAwaitAdminLabel.
func (a *App) handleAdminLabel(ctx context.Context, msg *models.Message) {
	label := strings.TrimSpace(msg.Text)
	if label == "" {
		a.send(ctx, msg.Chat.ID, Screen{Text: "النص فارغ، جرّب مرة أخرى."})
		return
	}
	a.sessions.captureAdminLabel(msg.From.ID, label)
	a.send(ctx, msg.Chat.ID, adminActionPickerScreen())
}

// finishAddButton creates the button from the wizard draft and persists it.
func (a *App) finishAddButton(ctx context.Context, userID int64, actionKey string) {
	parent, label := a.sessions.adminDraft(userID)
	if label == "" {
		a.send(ctx, userID, Screen{Text: "انتهت الجلسة. ابدأ الإضافة من جديد.", Keyboard: adminBackKeyboard()})
		return
	}

	item := menu.Item{Label: label}
	if actionKey == "submenu" {
		item.ID = a.uniqueID("sub")
	} else {
		item.ID = a.uniqueID(actionKey)
		item.Action = actionKey
	}

	if err := a.menu.Add(parent, item); err != nil {
		a.logf("admin add: %v", err)
		a.send(ctx, userID, Screen{Text: "❌ تعذّرت الإضافة: " + adminErr(err), Keyboard: adminBackKeyboard()})
		return
	}
	a.send(ctx, userID, Screen{
		Text:     "✅ تمت إضافة الزر «" + label + "».",
		Keyboard: adminBackKeyboard(),
	})
}

// finishDeleteButton removes a button and persists the tree.
func (a *App) finishDeleteButton(ctx context.Context, userID int64, id string) {
	if err := a.menu.Remove(id); err != nil {
		a.send(ctx, userID, Screen{Text: "❌ تعذّر الحذف: " + adminErr(err), Keyboard: adminBackKeyboard()})
		return
	}
	a.send(ctx, userID, Screen{Text: "🗑️ تم حذف الزر.", Keyboard: adminBackKeyboard()})
}

// uniqueID returns a tree-unique id derived from base.
func (a *App) uniqueID(base string) string {
	return a.menu.GenID(base)
}

// adminPanelScreen shows the current menu tree and the edit actions.
func (a *App) adminPanelScreen() Screen {
	var b strings.Builder
	b.WriteString("🛠️ لوحة إدارة القائمة\n\nالشجرة الحالية:\n")
	b.WriteString(renderTree(a.menu.Root(), 0))
	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "➕ إضافة زر", Data: "admin:add"}, {Text: "🗑️ حذف زر", Data: "admin:del"}},
			{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}},
		}},
	}
}

// renderTree renders the menu hierarchy with indentation and ids.
func renderTree(items []menu.Item, depth int) string {
	var b strings.Builder
	indent := strings.Repeat("   ", depth)
	bullet := "•"
	if depth > 0 {
		bullet = "↳"
	}
	for _, it := range items {
		b.WriteString(indent + bullet + " " + it.Label + "  [" + it.ID + "]\n")
		if it.IsSubmenu() {
			b.WriteString(renderTree(it.Children, depth+1))
		}
	}
	return b.String()
}

// adminParentPickerScreen lets the admin choose where the new button goes.
func (a *App) adminParentPickerScreen() Screen {
	rows := [][]Button{
		{{Text: "🏠 القائمة الرئيسية", Data: "admin:parent:" + menu.RootID}},
	}
	for _, sub := range collectSubmenus(a.menu.Root()) {
		rows = append(rows, []Button{{Text: "📁 " + sub.Label, Data: "admin:parent:" + sub.ID}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع", Data: "admin:panel"}})
	return Screen{Text: "📍 أين تريد إضافة الزر؟", Keyboard: &Keyboard{Rows: rows}}
}

// adminActionPickerScreen lets the admin pick the button's action or a submenu.
func adminActionPickerScreen() Screen {
	rows := [][]Button{}
	for _, act := range adminActions {
		rows = append(rows, []Button{{Text: act.Label, Data: "admin:action:" + act.Key}})
	}
	rows = append(rows, []Button{{Text: "⬅️ إلغاء", Data: "admin:panel"}})
	return Screen{Text: "🎯 اختر وظيفة الزر:", Keyboard: &Keyboard{Rows: rows}}
}

// adminDeleteListScreen lists every button with a delete action.
func (a *App) adminDeleteListScreen() Screen {
	rows := [][]Button{}
	for _, it := range flatten(a.menu.Root(), 0) {
		prefix := strings.Repeat("· ", it.depth)
		rows = append(rows, []Button{{
			Text: "🗑️ " + prefix + it.item.Label,
			Data: "admin:delete:" + it.item.ID,
		}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع", Data: "admin:panel"}})
	text := "🗑️ اختر الزر المراد حذفه:"
	if len(rows) == 1 {
		text = "لا توجد أزرار للحذف."
	}
	return Screen{Text: text, Keyboard: &Keyboard{Rows: rows}}
}

// collectSubmenus returns every submenu item in the tree (depth-first).
func collectSubmenus(items []menu.Item) []menu.Item {
	var out []menu.Item
	for _, it := range items {
		if it.IsSubmenu() {
			out = append(out, it)
			out = append(out, collectSubmenus(it.Children)...)
		}
	}
	return out
}

type flatItem struct {
	item  menu.Item
	depth int
}

// flatten returns every item with its depth, for listing.
func flatten(items []menu.Item, depth int) []flatItem {
	var out []flatItem
	for _, it := range items {
		out = append(out, flatItem{item: it, depth: depth})
		if it.IsSubmenu() {
			out = append(out, flatten(it.Children, depth+1)...)
		}
	}
	return out
}

func adminBackKeyboard() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🛠️ لوحة الإدارة", Data: "admin:panel"}},
		{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}},
	}}
}

func adminErr(err error) string {
	switch err {
	case menu.ErrDuplicateID:
		return "المعرّف مكرّر."
	case menu.ErrParentNotFound:
		return "القائمة الأم غير موجودة."
	case menu.ErrParentIsAction:
		return "لا يمكن الإضافة تحت زر وظيفي."
	case menu.ErrNotFound:
		return "الزر غير موجود."
	default:
		return "خطأ غير متوقّع."
	}
}

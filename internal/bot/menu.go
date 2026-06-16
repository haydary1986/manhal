package bot

import (
	"context"
	"strings"

	"github.com/erticaz/manhal/internal/menu"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// mainMenuScreen renders the root menu from the editable menu tree.
func (a *App) mainMenuScreen() Screen {
	return Screen{
		Text:     a.settings.WelcomeMessage,
		Keyboard: menuKeyboard(a.menuItems(menu.RootID), false),
	}
}

// submenuScreen renders a submenu's children with a back-to-home button.
func (a *App) submenuScreen(title string, items []menu.Item) Screen {
	return Screen{
		Text:     title,
		Keyboard: menuKeyboard(items, true),
	}
}

// menuItems returns the children of a container id (root or a submenu).
func (a *App) menuItems(id string) []menu.Item {
	if a.menu == nil {
		return menu.DefaultTree()
	}
	items, _ := a.menu.Children(id)
	return items
}

// menuKeyboard turns menu items into an inline keyboard. Submenus open with a
// "nav:" callback; action leaves keep the existing "menu:" dispatch so all
// feature screens continue to work unchanged. Two buttons per row.
func menuKeyboard(items []menu.Item, withBack bool) *Keyboard {
	rows := [][]Button{}
	var row []Button
	for _, it := range items {
		var b Button
		if it.IsSubmenu() {
			b = Button{Text: "📁 " + it.Label, Data: "nav:" + it.ID}
		} else {
			b = Button{Text: it.Label, Data: "menu:" + it.Action}
		}
		row = append(row, b)
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	if withBack {
		rows = append(rows, []Button{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}})
	}
	return &Keyboard{Rows: rows}
}

// handleNav opens a submenu by id.
func (a *App) handleNav(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings))
		return
	}
	a.sessions.clear(cq.From.ID)

	id := strings.TrimPrefix(cq.Data, "nav:")
	item, ok := a.menu.Find(id)
	if !ok {
		a.send(ctx, cq.From.ID, a.mainMenuScreen())
		return
	}
	children, _ := a.menu.Children(id)
	a.send(ctx, cq.From.ID, a.submenuScreen(item.Label, children))
}

// placeholder is shown for features that are not implemented yet.
func placeholder(title string) Screen {
	return Screen{
		Text: title + "\n\n🚧 هذه الميزة قيد الإنشاء — قريباً!",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// helpScreen describes the bot.
func helpScreen() Screen {
	return Screen{
		Text: "ℹ️ منهل — مساعدك الأكاديمي\n\n" +
			"بوت يجمع أدوات البحث والدراسة بمكان واحد.\n" +
			"الميزات تُضاف تدريجياً — تابع القناة للتحديثات.",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

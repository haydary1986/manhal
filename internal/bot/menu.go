package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/menu"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// mainMenuScreen renders the root menu from the editable menu tree.
func (a *App) mainMenuScreen() Screen {
	return Screen{
		Text:     a.settings.Get().WelcomeMessage,
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
	for i, it := range items {
		num := strconv.Itoa(i+1) + ". " // sequential number so users can refer to a service by its number
		var b Button
		switch {
		case it.IsSubmenu():
			b = Button{Text: num + "📁 " + it.Label, Data: "nav:" + it.ID}
		case it.IsLink():
			b = Button{Text: num + it.Label, URL: it.URL}
		default:
			b = Button{Text: num + it.Label, Data: "menu:" + it.Action}
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
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
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

// helpTool is one entry in the numbered usage guide.
type helpTool struct {
	Icon string
	Name string
	Desc string
}

// helpGroups is the full, categorized catalog of features shown on the help
// screen — numbered with a one-line explanation each.
var helpGroups = []struct {
	Title string
	Tools []helpTool
}{
	{"🔎 البحث والمراجع", []helpTool{
		{"🔍", "بحث عن ورقة", "ابحث بالعنوان أو الكلمات، وتصفّح النتائج مع الاستشهادات وأزرار اقتباس فوري."},
		{"📝", "توليد اقتباس", "أدخل DOI واحصل على ٦ صيغ (APA, MLA, Chicago, Harvard, IEEE, Vancouver) + BibTeX."},
		{"🔓", "نسخة مجانية", "أدخل DOI لإيجاد نسخة قانونية مفتوحة الوصول عبر Unpaywall."},
		{"🛡️", "فحص مجلة", "تحقّق من تصنيف المجلة (Quartile, SJR, h-index) وكشف المجلات المفترسة."},
		{"🔗", "أوراق مشابهة", "أرسل DOI لإيجاد أوراق ذات صلة (OpenAlex)."},
		{"🚫", "كشف المسحوبة", "تحقّق إن كانت ورقة (DOI) قد سُحبت (Retracted)."},
	}},
	{"👤 الباحثون والمجال", []helpTool{
		{"👤", "ملف باحث", "h-index وعدد الأبحاث والاستشهادات من OpenAlex."},
		{"📡", "رادار البحث", "نظرة سريعة على نشاط باحث وإنتاجه العلمي."},
		{"🔥", "ترندات المجال", "أبرز الأوراق الحديثة الأكثر تأثيراً في موضوعك."},
	}},
	{"🎓 أدوات أكاديمية", []helpTool{
		{"🎓", "حاسبة الترقيات", "احسب نقاطك وفق تعليمات ١٠/٢٠٢٥ عبر منشئ تفاعلي بالأزرار."},
		{"📊", "المساعد الإحصائي", "اختبارات (t, ANOVA, ارتباط…) بحساب دقيق وقيم p."},
		{"📐", "مدقّق LaTeX/Word", "ارفع ملفاً وافحص أخطاء التنسيق الشائعة."},
		{"📄➡️", "PDF إلى Word", "حوّل أي PDF (حتى الممسوح ضوئياً عبر OCR) إلى ملف Word قابل للتعديل."},
	}},
	{"🤖 أدوات الذكاء — حدّ يومي للحساب المجاني", []helpTool{
		{"🤖", "المساعد الذكي", "كتابة وتلخيص وترجمة وتحسين صياغة… (١١ أداة)."},
		{"✍️", "إعادة كتابة بشرية", "أعيد صياغة نصّك بأسلوب طبيعي وأزيل علامات الذكاء (البريميم: ملف Word كامل)."},
		{"🧭", "وين أنشر؟", "ألصق ملخّصك واحصل على مجلات مناسبة مع نصائح APC والنطاق."},
		{"📖", "مراجعة الأدبيات", "مسودّة مراجعة أدبيات منظّمة لموضوعك."},
		{"🧩", "كاشف الفجوة البحثية", "اكتشف الفجوات والفرص في موضوعك."},
		{"🔍", "فحص التشابه المبدئي", "تقدير مبدئي للتشابه قبل التدقيق الرسمي."},
		{"🎤", "تحضير المناقشة", "ارفع أطروحتك (PDF/Word/LaTeX) واحصل على هيكل دفاع وأسئلة لجنة متوقّعة بإجابات."},
		{"📄", "محادثة PDF", "ارفع PDF واسأل عن محتواه (يلزم تفعيل التضمين)."},
	}},
	{"⭐ مكتبتك ومتابعاتك", []helpTool{
		{"⭐", "مكتبتي", "احفظ المراجع وصدّرها لاحقاً."},
		{"🧠", "بحث دلالي بمكتبتي", "ابحث في مكتبتك بالمعنى لا بالكلمات (يلزم التضمين)."},
		{"🔔", "متابعاتي", "تابع مواضيع وتنبّه فور صدور أوراق جديدة."},
		{"📢", "الإعلانات", "مؤتمرات ومنح ودعوات أبحاث محدّثة."},
	}},
	{"💎 أخرى", []helpTool{
		{"💎", "الاشتراك / الترقية", "ارفع حدودك اليومية وافتح كل الأدوات."},
		{"📞", "الدعم الفني", "تواصل معنا مباشرة لأي استفسار."},
	}},
}

// helpScreen renders the full, numbered guide to every tool.
func helpScreen() Screen {
	var b strings.Builder
	b.WriteString("ℹ️ دليل منهل — كل الأدوات\n")
	b.WriteString("مساعدك الأكاديمي في مكان واحد. إليك كل أداة وما تفعله:\n")
	n := 0
	for _, g := range helpGroups {
		b.WriteString("\n" + g.Title + "\n")
		for _, t := range g.Tools {
			n++
			b.WriteString(strconv.Itoa(n) + ". " + t.Icon + " " + t.Name + " — " + t.Desc + "\n")
		}
	}
	b.WriteString("\n🤖 أدوات الذكاء لها حدّ استخدام يومي للحساب المجاني؛ الاشتراك المميّز يرفع الحدّ ويفتح كل الأدوات.\n")
	b.WriteString("اكتب /start للعودة للقائمة في أي وقت.")
	return Screen{
		Text: b.String(),
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "💎 الاشتراك / الترقية", Data: "menu:subscribe"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

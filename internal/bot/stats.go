package bot

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/stats"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// statsTests lists the available tests with their input instructions.
var statsTests = []struct{ Key, Label, Prompt string }{
	{"describe", "📈 إحصاء وصفي", "أرسل الأرقام في سطر واحد (مفصولة بمسافات أو فواصل):\nمثال: 2 4 4 5 5 7 9"},
	{"ttest", "🆚 اختبار T (مجموعتين)", "أرسل مجموعتين، كل مجموعة بسطر:\n5 6 7 8 9\n3 4 5 6 7"},
	{"correlation", "🔗 ارتباط بيرسون", "أرسل متغيّرين متوازيين، كل واحد بسطر:\n1 2 3 4 5\n2 4 5 4 5"},
	{"anova", "📊 تحليل التباين ANOVA", "أرسل المجموعات، كل مجموعة بسطر (سطران فأكثر):\n1 2 3\n4 5 6\n7 8 9"},
	{"cronbach", "🔁 ثبات كرونباخ ألفا", "أرسل درجات كل فقرة بسطر (فقرتان فأكثر، نفس عدد المستجيبين):\n2 3 3 4\n3 3 4 5"},
}

// statsMenuScreen lists the statistical tests.
func statsMenuScreen() Screen {
	rows := [][]Button{}
	for _, st := range statsTests {
		rows = append(rows, []Button{{Text: st.Label, Data: "stats:test:" + st.Key}})
	}
	rows = append(rows, []Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}})
	return Screen{
		Text:     "📊 المساعد الإحصائي\n\nاختر التحليل المطلوب (حساب دقيق بلا ذكاء اصطناعي):",
		Keyboard: &Keyboard{Rows: rows},
	}
}

// handleStats routes "stats:test:" callbacks (test selection).
func (a *App) handleStats(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	if !a.isSubscribed(ctx, cq.From.ID) {
		a.send(ctx, cq.From.ID, gateScreen(a.settings.Get()))
		return
	}

	key := strings.TrimPrefix(cq.Data, "stats:test:")
	st, ok := findStatsTest(key)
	if !ok {
		a.send(ctx, cq.From.ID, statsMenuScreen())
		return
	}
	a.sessions.startStats(cq.From.ID, key)
	a.send(ctx, cq.From.ID, Screen{
		Text: st.Label + "\n\n" + st.Prompt,
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ إلغاء", Data: "menu:stats"}},
		}},
	})
}

// handleStatsData parses the data and runs the chosen test.
func (a *App) handleStatsData(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	key := a.sessions.statsTest(msg.From.ID)
	a.sessions.clear(msg.From.ID)

	groups := parseStatsGroups(msg.Text)
	text, err := runStatsTest(key, groups)
	if err != nil {
		a.send(ctx, chatID, Screen{Text: "❌ " + statsErr(err), Keyboard: statsNav()})
		return
	}
	a.send(ctx, chatID, Screen{Text: text, Keyboard: statsNav()})
}

// runStatsTest dispatches to the engine and formats an APA-style result.
func runStatsTest(key string, groups [][]float64) (string, error) {
	switch key {
	case "describe":
		d, err := stats.Describe(flattenNums(groups))
		if err != nil {
			return "", err
		}
		return "📈 إحصاء وصفي\n" +
			"N = " + strconv.Itoa(d.N) + "\n" +
			"المتوسط M = " + f2(d.Mean) + "\n" +
			"الانحراف SD = " + f2(d.SD) + "\n" +
			"الوسيط = " + f2(d.Median) + "\n" +
			"المدى = [" + f2(d.Min) + " ، " + f2(d.Max) + "]" + statsFooter(), nil

	case "ttest":
		if len(groups) < 2 {
			return "", stats.ErrInsufficientData
		}
		r, err := stats.IndependentTTest(groups[0], groups[1])
		if err != nil {
			return "", err
		}
		return "🆚 اختبار T المستقل (Welch)\n" +
			"متوسط ١ = " + f2(r.Mean1) + " · متوسط ٢ = " + f2(r.Mean2) + "\n" +
			"t(" + f1(r.DF) + ") = " + f2(r.T) + "، " + fmtP(r.P) + "\n" +
			sig(r.P) + statsFooter(), nil

	case "correlation":
		if len(groups) < 2 {
			return "", stats.ErrInsufficientData
		}
		r, err := stats.Pearson(groups[0], groups[1])
		if err != nil {
			return "", err
		}
		return "🔗 ارتباط بيرسون\n" +
			"r(" + strconv.Itoa(r.N-2) + ") = " + f2(r.R) + "، " + fmtP(r.P) + "\n" +
			sig(r.P) + statsFooter(), nil

	case "anova":
		r, err := stats.OneWayANOVA(groups)
		if err != nil {
			return "", err
		}
		return "📊 تحليل التباين الأحادي\n" +
			"F(" + f1(r.DFBetween) + " ، " + f1(r.DFWithin) + ") = " + f2(r.F) + "، " + fmtP(r.P) + "\n" +
			sig(r.P) + statsFooter(), nil

	case "cronbach":
		alpha, err := stats.CronbachAlpha(groups)
		if err != nil {
			return "", err
		}
		return "🔁 معامل ثبات كرونباخ ألفا\n" +
			"α = " + f2(alpha) + " (" + strconv.Itoa(len(groups)) + " فقرات)\n" +
			cronbachVerdict(alpha) + statsFooter(), nil
	}
	return "", stats.ErrInsufficientData
}

// --- formatting helpers ---

func statsFooter() string {
	return "\n\nℹ️ حساب دقيق وفق الصيغ الإحصائية المعتمدة."
}

func sig(p float64) string {
	if p < 0.05 {
		return "النتيجة دالة إحصائياً (p < 0.05)."
	}
	return "النتيجة غير دالة إحصائياً (p ≥ 0.05)."
}

func cronbachVerdict(a float64) string {
	switch {
	case a >= 0.9:
		return "ثبات ممتاز."
	case a >= 0.8:
		return "ثبات جيد."
	case a >= 0.7:
		return "ثبات مقبول."
	default:
		return "ثبات ضعيف — راجع الفقرات."
	}
}

// fmtP renders a p-value in APA style ("p < .001" / "p = .045").
func fmtP(p float64) string {
	if math.IsNaN(p) {
		return "p = غير معرّف"
	}
	if p < 0.001 {
		return "p < .001"
	}
	s := strconv.FormatFloat(p, 'f', 3, 64)
	return "p = " + strings.TrimPrefix(s, "0")
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func statsErr(err error) string {
	switch err {
	case stats.ErrInsufficientData:
		return "بيانات غير كافية لهذا التحليل."
	case stats.ErrLengthMismatch:
		return "يجب أن تتساوى المجموعات/المتغيّرات بعدد القيم."
	case stats.ErrNoVariance:
		return "لا يوجد تباين في البيانات (كل القيم متطابقة)."
	default:
		return "تعذّر إجراء التحليل."
	}
}

func statsNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "📊 تحليل آخر", Data: "menu:stats"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

func findStatsTest(key string) (struct{ Key, Label, Prompt string }, bool) {
	for _, st := range statsTests {
		if st.Key == key {
			return st, true
		}
	}
	return struct{ Key, Label, Prompt string }{}, false
}

// parseStatsGroups parses each non-empty line into a slice of numbers.
func parseStatsGroups(text string) [][]float64 {
	var groups [][]float64
	for _, line := range strings.Split(text, "\n") {
		if nums := parseNumbers(line); len(nums) > 0 {
			groups = append(groups, nums)
		}
	}
	return groups
}

// parseNumbers extracts numbers from a line (space/comma/semicolon separated,
// Arabic-Indic digits accepted).
func parseNumbers(line string) []float64 {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ',' || r == '،' || r == ';'
	})
	var out []float64
	for _, fld := range fields {
		if v, err := strconv.ParseFloat(normalizeArabicDigits(fld), 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func flattenNums(groups [][]float64) []float64 {
	var out []float64
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func normalizeArabicDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '٠' && r <= '٩':
			b.WriteRune('0' + (r - '٠'))
		case r == '٫': // Arabic decimal separator
			b.WriteRune('.')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

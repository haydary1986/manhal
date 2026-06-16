package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/scholar"
	"github.com/go-telegram/bot/models"
)

// OAFinder resolves a DOI to its legal open-access copies.
type OAFinder interface {
	FindOA(ctx context.Context, doi string) (*scholar.OAResult, error)
}

// oaMaxLocations caps how many OA links one screen shows.
const oaMaxLocations = 4

// oaPromptScreen asks the user for a DOI.
func oaPromptScreen() Screen {
	return Screen{
		Text: "🔓 البحث عن نسخة مجانية قانونية\n\n" +
			"أرسل معرّف الورقة (DOI) وسأبحث عن نسخة Open Access قانونية\n" +
			"(مستودعات + نسخ المؤلفين). لا روابط قرصنة.\n\n" +
			"مثال:\n10.1038/nphys1170",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleOADOI looks up the open-access status of the DOI the user just sent.
func (a *App) handleOADOI(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	doi := strings.TrimSpace(msg.Text)
	a.send(ctx, chatID, Screen{Text: "⏳ جاري البحث عن نسخة مجانية..."})

	findCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res, err := a.oa.FindOA(findCtx, doi)
	if err != nil {
		a.logf("oa: %q: %v", doi, err)
		a.send(ctx, chatID, oaErrorScreen(err))
		return
	}
	a.send(ctx, chatID, oaResultScreen(res))
}

// oaResultScreen renders the open-access copies, or a not-available message.
func oaResultScreen(res *scholar.OAResult) Screen {
	if !res.IsOA || len(res.Locations) == 0 {
		return oaClosedScreen()
	}

	var b strings.Builder
	if title := strings.TrimSpace(res.Title); title != "" {
		b.WriteString("🔓 نسخة مجانية متاحة لـ:\n«" + title + "»\n")
	} else {
		b.WriteString("🔓 نسخة مجانية متاحة ✅\n")
	}

	for i, loc := range dedupeLocations(res.Locations) {
		if i == oaMaxLocations {
			break
		}
		b.WriteString("\n" + oaLocationLabel(loc) + "\n   " + loc.URL + "\n")
	}
	b.WriteString("\nℹ️ روابط Open Access قانونية عبر Unpaywall.")

	return Screen{Text: b.String(), Keyboard: oaNavKeyboard()}
}

// oaLocationLabel renders an Arabic label for a copy's type and version.
func oaLocationLabel(loc scholar.OALocation) string {
	icon := "🔗"
	if loc.IsPDF {
		icon = "📄"
	}
	host := map[string]string{
		"repository": "مستودع",
		"publisher":  "الناشر",
	}[loc.HostType]
	version := map[string]string{
		"publishedVersion": "النسخة المنشورة",
		"acceptedVersion":  "النسخة المقبولة (بعد التحكيم)",
		"submittedVersion": "مسودة preprint",
	}[loc.Version]

	parts := []string{}
	if host != "" {
		parts = append(parts, host)
	}
	if version != "" {
		parts = append(parts, version)
	}
	label := icon
	if len(parts) > 0 {
		label += " " + strings.Join(parts, " · ")
	}
	return label
}

// dedupeLocations drops repeated URLs while preserving order.
func dedupeLocations(locs []scholar.OALocation) []scholar.OALocation {
	seen := make(map[string]bool, len(locs))
	out := make([]scholar.OALocation, 0, len(locs))
	for _, l := range locs {
		if l.URL == "" || seen[l.URL] {
			continue
		}
		seen[l.URL] = true
		out = append(out, l)
	}
	return out
}

func oaClosedScreen() Screen {
	return Screen{
		Text: "🔒 ما لقيت نسخة مجانية قانونية لهذه الورقة.\n\n" +
			"خيارات مشروعة:\n" +
			"• اطلب نسخة من المؤلف مباشرة (ResearchGate/البريد).\n" +
			"• تحقّق من مكتبة جامعتك أو اشتراكاتها.\n" +
			"• ابحث عن preprint على arXiv إن وُجد.",
		Keyboard: oaNavKeyboard(),
	}
}

func oaErrorScreen(err error) Screen {
	var msg string
	switch {
	case errors.Is(err, scholar.ErrInvalidDOI):
		msg = "هذا لا يبدو DOI صحيحاً 🤔 المعرّف يبدأ بـ «10.» ويحتوي «/»."
	case errors.Is(err, scholar.ErrNotFound):
		msg = "ما لقيت بيانات لهذا الـ DOI 🔍 تأكّد منه."
	case errors.Is(err, scholar.ErrNotConfigured):
		msg = "خدمة البحث عن النسخ المجانية غير مفعّلة حالياً ⚙️"
	default:
		msg = "صار خلل بالبحث ⚠️ جرّب بعد لحظات."
	}
	return Screen{Text: "❌ " + msg, Keyboard: oaNavKeyboard()}
}

func oaNavKeyboard() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🔓 DOI آخر", Data: "menu:oa"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

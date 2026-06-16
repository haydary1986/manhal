package bot

import (
	"context"
	"strings"

	"github.com/erticaz/manhal/internal/journal"
	"github.com/erticaz/manhal/internal/predator"
	"github.com/go-telegram/bot/models"
)

// journalSuggestionLimit caps how many near-matches a miss shows.
const journalSuggestionLimit = 5

// journalPromptScreen asks for a journal name or ISSN.
func journalPromptScreen() Screen {
	return Screen{
		Text: "🛡️ فحص تصنيف المجلة\n\n" +
			"أرسل اسم المجلة أو رقم الـ ISSN، وسأرجّع لك تصنيف Quartile و SJR.\n\n" +
			"مثال:\nNature\nأو 0028-0836",
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// handleJournalQuery looks up the journal the user just named and attaches an
// advisory predatory-risk assessment.
func (a *App) handleJournalQuery(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	query := strings.TrimSpace(msg.Text)

	if j, ok := a.journals.Lookup(query); ok {
		adv := predator.Assess(true, a.checkPredator(j.Title, j.Publisher))
		a.send(ctx, chatID, journalCardScreen(j, adv))
		return
	}
	if matches := a.journals.Search(query, journalSuggestionLimit); len(matches) > 0 {
		a.send(ctx, chatID, journalSuggestionsScreen(query, matches))
		return
	}
	adv := predator.Assess(false, a.checkPredator(query))
	a.send(ctx, chatID, journalNotFoundScreen(query, adv))
}

// checkPredator runs the watch list, tolerating a nil list (feature disabled).
func (a *App) checkPredator(fields ...string) []predator.Flag {
	if a.predators == nil {
		return nil
	}
	return a.predators.Check(fields...)
}

// advisoryBlock renders the predatory-risk advisory shown under a journal result.
func advisoryBlock(adv predator.Advisory) string {
	var b strings.Builder
	switch adv.Risk {
	case predator.RiskHigh:
		b.WriteString("\n⛔ تحذير استرشادي — مؤشرات على احتمال كون المجلة مفترِسة:\n")
		for _, f := range adv.Flags {
			b.WriteString("   • " + f.Reason + "\n")
		}
	case predator.RiskLow:
		b.WriteString("\n✅ إشارة موثوقية: المجلة مفهرسة في Scimago.\n")
	default: // RiskMedium
		b.WriteString("\n⚠️ غير موجودة في قاعدتنا (Scimago). هذا لا يعني أنها مفترِسة — تحقّق يدوياً عبر DOAJ أو موقع الناشر الرسمي.\n")
	}
	b.WriteString("ℹ️ تقييم استرشادي وليس حكماً نهائياً.")
	return b.String()
}

// journalCardScreen renders a journal's ranking card plus its risk advisory.
func journalCardScreen(j journal.Journal, adv predator.Advisory) Screen {
	var b strings.Builder
	b.WriteString("🛡️ " + j.Title + "\n")
	b.WriteString("🏆 التصنيف: " + journal.QuartileLabel(j.Quartile) + "\n")

	stats := []string{}
	if j.SJR != "" {
		stats = append(stats, "SJR: "+j.SJR)
	}
	if j.HIndex != "" {
		stats = append(stats, "h-index: "+j.HIndex)
	}
	if len(stats) > 0 {
		b.WriteString("📈 " + strings.Join(stats, " · ") + "\n")
	}
	if len(j.Categories) > 0 {
		b.WriteString("🏷️ المجالات: " + strings.Join(j.Categories, "، ") + "\n")
	}
	if j.Publisher != "" {
		line := "🏢 الناشر: " + j.Publisher
		if j.Country != "" {
			line += " (" + j.Country + ")"
		}
		b.WriteString(line + "\n")
	}
	if len(j.ISSNs) > 0 {
		b.WriteString("🔖 ISSN: " + strings.Join(j.ISSNs, ", ") + "\n")
	}
	b.WriteString("\nℹ️ بيانات استرشادية مبنية على تصنيف Scimago — قد تتغيّر سنوياً.\n")
	b.WriteString(advisoryBlock(adv))

	return Screen{Text: b.String(), Keyboard: journalNavKeyboard()}
}

// journalSuggestionsScreen lists near-matches when there is no exact hit.
func journalSuggestionsScreen(query string, matches []journal.Journal) Screen {
	var b strings.Builder
	b.WriteString("🔍 ما لقيت تطابقاً دقيقاً لـ «" + query + "».\nأقرب المجلات:\n")
	for _, j := range matches {
		q := strings.TrimSpace(j.Quartile)
		if q == "" {
			q = "غير مصنّفة"
		}
		b.WriteString("\n• " + j.Title + " — " + q)
	}
	b.WriteString("\n\nأعد إرسال الاسم بالضبط أو الـ ISSN للحصول على البطاقة الكاملة.")
	return Screen{Text: b.String(), Keyboard: journalNavKeyboard()}
}

// journalNotFoundScreen is shown when nothing matches at all. It still carries
// the advisory, which is the key signal when a flagged name isn't indexed.
func journalNotFoundScreen(query string, adv predator.Advisory) Screen {
	return Screen{
		Text: "❌ ما لقيت مجلة باسم «" + query + "» في قاعدة البيانات.\n" +
			"تأكّد من الاسم أو جرّب الـ ISSN.\n" +
			advisoryBlock(adv),
		Keyboard: journalNavKeyboard(),
	}
}

func journalNavKeyboard() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🛡️ فحص مجلة أخرى", Data: "menu:journal"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}

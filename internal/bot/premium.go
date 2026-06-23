package bot

import "context"

// premiumActions are the menu actions reserved for premium subscribers. The
// split keeps a rich free tier (search, citations, journal/predatory checks,
// the promotion calculator, basic AI tools with a daily quota, alerts…) while
// charging for the compute-heavy or external-cost power tools. Edit this map to
// move a feature between tiers — nothing else needs to change.
var premiumActions = map[string]string{
	"pdfchat":   "📄 محادثة الـ PDF",
	"slides":    "🎬 توليد العرض التقديمي",
	"viva":      "🎤 تحضير مناقشة الرسالة",
	"litreview": "📖 مراجعة الأدبيات",
	"gap":       "🧩 كاشف الفجوة البحثية",
	"websearch": "🌐 البحث الذكي في الويب",
	"ytsummary": "▶️ تلخيص فيديو يوتيوب",
	"pdf2word":  "📄↔Word مع OCR",
}

// isPremiumAction reports whether an action is gated behind premium.
func isPremiumAction(action string) (label string, premium bool) {
	label, premium = premiumActions[action]
	return label, premium
}

// requirePremium allows free actions through, and for premium actions lets
// subscribers through while showing everyone else an upsell screen. It returns
// true when the caller may proceed.
func (a *App) requirePremium(ctx context.Context, userID int64, action string) bool {
	label, premium := isPremiumAction(action)
	if !premium || a.isPremiumUser(ctx, userID) {
		return true
	}
	a.send(ctx, userID, premiumGateScreen("🔒 «"+label+"» متاحة لمشتركي البريميم فقط."))
	return false
}

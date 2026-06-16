package menu

// DefaultTree is the seed menu used when data/menu.yaml is absent. It mirrors
// the original hardcoded menu as a flat list of action buttons; admins can then
// reorganize it into submenus.
func DefaultTree() []Item {
	return []Item{
		{ID: "announcements", Label: "📢 الإعلانات والمؤتمرات", Action: "announcements"},
		{ID: "follow", Label: "🔔 متابعاتي", Action: "follow"},
		{ID: "search", Label: "🔍 بحث عن ورقة", Action: "search"},
		{ID: "cite", Label: "📝 توليد اقتباس", Action: "cite"},
		{ID: "journal", Label: "🛡️ فحص مجلة", Action: "journal"},
		{ID: "oa", Label: "🔓 نسخة مجانية", Action: "oa"},
		{ID: "author", Label: "👤 ملف باحث (h-index)", Action: "author"},
		{ID: "radar", Label: "📡 رادار البحث", Action: "radar"},
		{ID: "trends", Label: "🔥 ترندات المجال", Action: "trends"},
		{ID: "promotion", Label: "🎓 حاسبة الترقيات", Action: "promotion"},
		{ID: "publish", Label: "🧭 وين أنشر؟", Action: "publish"},
		{ID: "litreview", Label: "📖 مراجعة الأدبيات", Action: "litreview"},
		{ID: "gap", Label: "🧩 كاشف الفجوة البحثية", Action: "gap"},
		{ID: "similarity", Label: "🔍 فحص التشابه المبدئي", Action: "similarity"},
		{ID: "stats", Label: "📊 المساعد الإحصائي", Action: "stats"},
		{ID: "latex", Label: "📐 مدقّق LaTeX/Word", Action: "latex"},
		{ID: "viva", Label: "🎤 تحضير المناقشة", Action: "viva"},
		{ID: "pdfchat", Label: "📄 محادثة الـ PDF", Action: "pdfchat"},
		{ID: "ai", Label: "🤖 المساعد الذكي", Action: "ai"},
		{ID: "library", Label: "⭐ مكتبتي", Action: "library"},
		{ID: "semantic", Label: "🧠 بحث دلالي بمكتبتي", Action: "semantic"},
		{ID: "support", Label: "📞 الدعم الفني", Action: "support"},
		{ID: "help", Label: "ℹ️ مساعدة", Action: "help"},
	}
}

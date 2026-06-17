// Package web is the HTTP admin adapter. It edits the same menu.Manager the bot
// uses, so changes made on the web page appear in the bot immediately. Access is
// protected by HTTP Basic Auth against an admin token.
package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
)

// Data is the persistence the admin panel reads from: support tickets plus the
// aggregates powering the analytics dashboard.
type Data interface {
	ListTickets(ctx context.Context) ([]domain.Ticket, error)
	AnswerTicket(ctx context.Context, id, reply string) (domain.Ticket, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	GetUser(ctx context.Context, telegramID int64) (*domain.User, error)
	SaveUser(ctx context.Context, u *domain.User) error
	ListLibrary(ctx context.Context, userID int64) ([]domain.LibraryItem, error)
	ListAllSubscriptions(ctx context.Context) ([]domain.Subscription, error)
	ListAllCitationWatches(ctx context.Context) ([]domain.CitationWatch, error)
	FeatureUsage(ctx context.Context) ([]domain.FeatureCount, error)
	TopUsers(ctx context.Context, limit int) ([]domain.UserUsage, error)
	UsageTotals(ctx context.Context) (actions int, activeUsers int, err error)
	UsageByWeekday(ctx context.Context) ([7]int, error)
	UsageByHour(ctx context.Context) ([24]int, error)
}

// Notifier pushes an admin reply back to a user (implemented by the bot).
type Notifier interface {
	Notify(userID int64, text string) error
}

// Settings is the editable bot configuration the admin manages (the
// subscription gate). Implemented by config.SettingsManager.
type Settings interface {
	RequiredChannel() string
	RequireSubscription() bool
	SetGate(channel string, require bool) error
	PremiumInfo() string
	PaymentDetails() string
	PaymentLink() string
	SetPayment(premiumInfo, paymentDetails, paymentLink string) error
}

// actionOptions are the leaf actions an admin can attach to a button, plus the
// "submenu" container option. Keys match the bot's menu dispatch.
var actionOptions = []struct{ Key, Label string }{
	{"submenu", "📁 قائمة فرعية"},
	{"url", "🔗 رابط خارجي"},
	{"subscribe", "💎 الاشتراك / الترقية"},
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

// Server is the admin web adapter.
type Server struct {
	menu     *menu.Manager
	data     Data
	notifier Notifier
	settings Settings
	accounts map[string]string // username -> password
}

// NewServer builds the admin server over the shared menu manager, data store
// and settings. notifier may be nil (replies are saved but not pushed until the
// bot is up). accounts maps usernames to passwords for Basic Auth.
func NewServer(mgr *menu.Manager, data Data, notifier Notifier, accounts map[string]string, settings Settings) *Server {
	return &Server{menu: mgr, data: data, notifier: notifier, settings: settings, accounts: accounts}
}

// Handler returns the routed, auth-protected HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Unauthenticated liveness probe for orchestrators (Coolify/Docker health).
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", s.auth(s.handleIndex))
	mux.HandleFunc("/admin", s.auth(s.handleDashboard))
	mux.HandleFunc("/admin/users", s.auth(s.handleUsers))
	mux.HandleFunc("/admin/users/message", s.auth(s.handleUserMessage))
	mux.HandleFunc("/admin/users/tier", s.auth(s.handleUserTier))
	mux.HandleFunc("/admin/menu", s.auth(s.handleMenuPage))
	mux.HandleFunc("/admin/menu/add", s.auth(s.handleAdd))
	mux.HandleFunc("/admin/menu/delete", s.auth(s.handleDelete))
	mux.HandleFunc("/admin/support", s.auth(s.handleSupport))
	mux.HandleFunc("/admin/support/reply", s.auth(s.handleSupportReply))
	mux.HandleFunc("/admin/settings", s.auth(s.handleSettings))
	mux.HandleFunc("/admin/settings/gate", s.auth(s.handleSetGate))
	mux.HandleFunc("/admin/settings/payment", s.auth(s.handleSetPayment))
	return mux
}

// Run starts the HTTP server and shuts it down when ctx is cancelled.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// auth wraps a handler with HTTP Basic Auth against the configured accounts.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || !s.validCredentials(user, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Manhal Admin"`)
			http.Error(w, "غير مصرّح", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// validCredentials checks a username/password pair in constant time.
func (s *Server) validCredentials(user, pass string) bool {
	want, ok := s.accounts[user]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pass), []byte(want)) == 1
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleDashboard renders the analytics overview (the admin home).
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.renderDashboard(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleMenuPage renders the button-management page.
func (s *Server) handleMenuPage(w http.ResponseWriter, r *http.Request) {
	s.renderMenu(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleUsers renders the user-management page (activity, message, premium).
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	s.renderUsers(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleUserMessage pushes an admin message to a user via the bot.
func (s *Server) handleUserMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("id")), 10, 64)
	text := strings.TrimSpace(r.FormValue("text"))
	if err != nil || text == "" {
		http.Redirect(w, r, "/admin/users?err="+urlencode("المعرّف والرسالة مطلوبان"), http.StatusSeeOther)
		return
	}
	if s.notifier == nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("البوت غير متصل — تعذّر الإرسال"), http.StatusSeeOther)
		return
	}
	if err := s.notifier.Notify(id, "📩 رسالة من فريق منهل:\n\n"+text); err != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("تعذّر إرسال الرسالة"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?msg="+urlencode("تم إرسال الرسالة للمستخدم"), http.StatusSeeOther)
}

// handleUserTier sets a user's subscription tier (manual premium grant/revoke).
func (s *Server) handleUserTier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("id")), 10, 64)
	tier := domain.Tier(strings.TrimSpace(r.FormValue("tier")))
	if err != nil || !validTier(tier) {
		http.Redirect(w, r, "/admin/users?err="+urlencode("بيانات غير صالحة"), http.StatusSeeOther)
		return
	}
	user, gerr := s.data.GetUser(r.Context(), id)
	if gerr != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("المستخدم غير موجود"), http.StatusSeeOther)
		return
	}
	updated := *user
	updated.Tier = tier
	updated.PremiumUntil = nil // manual grant is permanent until changed
	if serr := s.data.SaveUser(r.Context(), &updated); serr != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("تعذّر حفظ الاشتراك"), http.StatusSeeOther)
		return
	}
	// Best-effort: notify the user of their new tier.
	if s.notifier != nil && tier != domain.TierFree {
		_ = s.notifier.Notify(id, "🎉 تم تفعيل اشتراكك المميّز في منهل! استمتع بكامل الميزات.")
	}
	http.Redirect(w, r, "/admin/users?msg="+urlencode("تم تحديث اشتراك المستخدم"), http.StatusSeeOther)
}

// validTier reports whether t is one of the known subscription tiers.
func validTier(t domain.Tier) bool {
	return t == domain.TierFree || t == domain.TierStudent || t == domain.TierResearcher
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectErr(w, r, "نموذج غير صالح")
		return
	}
	parent := strings.TrimSpace(r.FormValue("parent"))
	label := strings.TrimSpace(r.FormValue("label"))
	action := strings.TrimSpace(r.FormValue("action"))
	if label == "" || action == "" {
		redirectErr(w, r, "النص والوظيفة مطلوبان")
		return
	}

	item := menu.Item{Label: label}
	switch action {
	case "submenu":
		item.ID = s.menu.GenID("sub")
	case "url":
		link := normalizeLink(r.FormValue("url"))
		if !validLink(link) {
			redirectErr(w, r, "أدخل رابطاً صحيحاً يبدأ بـ http أو https أو https://t.me")
			return
		}
		item.ID = s.menu.GenID("link")
		item.URL = link
	default:
		item.ID = s.menu.GenID(action)
		item.Action = action
	}
	if err := s.menu.Add(parent, item); err != nil {
		redirectErr(w, r, adminErr(err))
		return
	}
	http.Redirect(w, r, "/admin/menu?msg="+urlencode("تمت إضافة «"+label+"»"), http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectErr(w, r, "نموذج غير صالح")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if err := s.menu.Remove(id); err != nil {
		redirectErr(w, r, adminErr(err))
		return
	}
	http.Redirect(w, r, "/admin/menu?msg="+urlencode("تم حذف الزر"), http.StatusSeeOther)
}

func redirectErr(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/admin/menu?err="+urlencode(msg), http.StatusSeeOther)
}

// handleSupport lists support tickets.
func (s *Server) handleSupport(w http.ResponseWriter, r *http.Request) {
	tickets, err := s.data.ListTickets(r.Context())
	if err != nil {
		http.Error(w, "خطأ بقراءة الطلبات", http.StatusInternalServerError)
		return
	}
	s.renderSupport(w, tickets, r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleSupportReply records an admin reply and pushes it to the user.
func (s *Server) handleSupportReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/support?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	reply := strings.TrimSpace(r.FormValue("reply"))
	if reply == "" {
		http.Redirect(w, r, "/admin/support?err="+urlencode("الرد فارغ"), http.StatusSeeOther)
		return
	}

	ticket, err := s.data.AnswerTicket(r.Context(), id, reply)
	if err != nil {
		http.Redirect(w, r, "/admin/support?err="+urlencode("الطلب غير موجود"), http.StatusSeeOther)
		return
	}

	msg := "تم حفظ الرد وإرساله للمستخدم."
	if s.notifier == nil {
		msg = "تم حفظ الرد (سيُرسَل عند تشغيل البوت)."
	} else if err := s.notifier.Notify(ticket.UserID, "📨 رد فريق الدعم على طلبك:\n\n"+reply); err != nil {
		msg = "تم حفظ الرد، لكن تعذّر إرساله الآن."
	}
	http.Redirect(w, r, "/admin/support?msg="+urlencode(msg), http.StatusSeeOther)
}

// handleSettings renders the bot-settings page (subscription gate).
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.renderSettings(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleSetGate saves the required-channel subscription gate.
func (s *Server) handleSetGate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.settings == nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("الإعدادات غير متاحة"), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	channel := strings.TrimSpace(r.FormValue("channel"))
	require := r.FormValue("require") == "on"
	if err := s.settings.SetGate(channel, require); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("لتفعيل الاشتراك الإجباري أدخِل معرّف القناة أولاً"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/settings?msg="+urlencode("تم حفظ إعدادات الاشتراك الإجباري"), http.StatusSeeOther)
}

// handleSetPayment saves the premium plans + manual payment details.
func (s *Server) handleSetPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.settings == nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("الإعدادات غير متاحة"), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	info := strings.TrimSpace(r.FormValue("premium_info"))
	details := strings.TrimSpace(r.FormValue("payment_details"))
	link := normalizeLink(r.FormValue("payment_link"))
	if link != "" && !validLink(link) {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("رابط الدفع غير صالح"), http.StatusSeeOther)
		return
	}
	if err := s.settings.SetPayment(info, details, link); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("تعذّر حفظ إعدادات الدفع"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/settings?msg="+urlencode("تم حفظ إعدادات الاشتراك والدفع"), http.StatusSeeOther)
}

func adminErr(err error) string {
	switch {
	case errors.Is(err, menu.ErrDuplicateID):
		return "المعرّف مكرّر"
	case errors.Is(err, menu.ErrParentNotFound):
		return "القائمة الأم غير موجودة"
	case errors.Is(err, menu.ErrParentIsAction):
		return "لا يمكن الإضافة تحت زر وظيفي"
	case errors.Is(err, menu.ErrNotFound):
		return "الزر غير موجود"
	default:
		return "خطأ غير متوقّع"
	}
}

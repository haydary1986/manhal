// Package web is the HTTP admin adapter. It edits the same menu.Manager the bot
// uses, so changes made on the web page appear in the bot immediately. Access is
// protected by HTTP Basic Auth against an admin token.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/plans"
	"github.com/erticaz/manhal/internal/predator"
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
	AddGiftCode(ctx context.Context, g domain.GiftCode) error
	ListGiftCodes(ctx context.Context) ([]domain.GiftCode, error)
	ListSubscriptionRequests(ctx context.Context, status domain.SubReqStatus) ([]domain.SubscriptionRequest, error)
	GetSubscriptionRequest(ctx context.Context, id string) (*domain.SubscriptionRequest, error)
	UpdateSubscriptionRequest(ctx context.Context, r domain.SubscriptionRequest) error
	FeatureUsage(ctx context.Context) ([]domain.FeatureCount, error)
	UserEvents(ctx context.Context, userID int64, limit int) ([]domain.UsageEvent, error)
	TopUsers(ctx context.Context, limit int) ([]domain.UserUsage, error)
	UsageTotals(ctx context.Context) (actions int, activeUsers int, err error)
	UsageByWeekday(ctx context.Context) ([7]int, error)
	UsageByHour(ctx context.Context) ([24]int, error)
}

// Notifier pushes messages from the admin to users (implemented by the bot).
type Notifier interface {
	Notify(userID int64, text string) error
	// SendRich delivers a broadcast: optional image (URL) + optional URL button.
	SendRich(userID int64, text, imageURL, buttonLabel, buttonURL string) error
}

// DisciplinesEditor is the editable academic-discipline taxonomy (implemented by
// config.DisciplinesManager).
type DisciplinesEditor interface {
	List() []config.Discipline
	Add(id, label string) error
	Remove(id string) error
}

// PredatorEditor is the editable predatory-journal watch list (implemented by
// predator.List).
type PredatorEditor interface {
	All() []predator.Flag
	Add(pattern, reason string) error
	Remove(pattern string) error
}

// PromotionEditor is the editable promotion rule set (implemented by
// promotion.Manager). Rules are edited as a validated YAML document so the admin
// can track ministry amendments without a code change.
type PromotionEditor interface {
	YAML() (string, error)
	SetYAML(text string) error
	ResetDefault() error
}

// PlansEditor is the editable subscription-plan catalogue (implemented by
// plans.Manager). Plans drive the subscribe screen and one-click activation.
type PlansEditor interface {
	List() []plans.Plan
	Upsert(p plans.Plan) error
	Remove(id string) error
}

// Announcements is the editable announcements store (implemented by
// announce.Repo): the bot reads it, the admin web publishes/removes items.
type Announcements interface {
	All() []announce.Announcement
	Add(a announce.Announcement) error
	Remove(id string) error
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
	FreeAILimit() int
	PremiumAILimit() int
	SetLimits(free, premium int) error
	DeepSeekKey() string
	SetDeepSeekKey(key string) error
	WelcomeMessage() string
	BotName() string
	BotDescription() string
	SetIdentity(welcome, name, description string) error
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
	{"humanize", "✍️ إعادة كتابة بشرية"},
	{"pdf2word", "📄➡️ PDF إلى Word"},
	{"slides", "🎬 إنشاء عرض تقديمي"},
	{"similar", "🔗 أوراق مشابهة"},
	{"retracted", "🚫 كشف الأوراق المسحوبة"},
	{"library", "⭐ مكتبتي"},
	{"semantic", "🧠 بحث دلالي بمكتبتي"},
	{"support", "📞 الدعم الفني"},
	{"help", "ℹ️ مساعدة"},
}

// Server is the admin web adapter.
type Server struct {
	menu        *menu.Manager
	data        Data
	notifier    Notifier
	settings    Settings
	announce    Announcements
	disciplines DisciplinesEditor
	predators   PredatorEditor
	promotion   PromotionEditor
	plans       PlansEditor
	proofImages ProofImages
	accounts    map[string]string // username -> password
}

// ProofImages fetches a payment-proof photo by its Telegram file id (implemented
// by the bot). Optional: the queue still works without it (proof text only).
type ProofImages interface {
	ProofImage(ctx context.Context, fileID string) ([]byte, string, error)
}

// WithPlans attaches the subscription-plan editor (set from main; tests that
// don't exercise the plans page leave it nil).
func (s *Server) WithPlans(p PlansEditor) *Server {
	s.plans = p
	return s
}

// WithProofImages wires the proof-photo fetcher so the queue can preview
// receipts uploaded in Telegram.
func (s *Server) WithProofImages(p ProofImages) *Server {
	s.proofImages = p
	return s
}

// WithEditors attaches optional reference-table editors (set from main; tests
// that don't exercise these pages leave them nil).
func (s *Server) WithEditors(disciplines DisciplinesEditor, predators PredatorEditor, promotion PromotionEditor) *Server {
	s.disciplines = disciplines
	s.predators = predators
	s.promotion = promotion
	return s
}

// NewServer builds the admin server over the shared managers and data store.
// notifier may be nil (replies are saved but not pushed until the bot is up).
// accounts maps usernames to passwords for Basic Auth.
func NewServer(mgr *menu.Manager, data Data, notifier Notifier, accounts map[string]string, settings Settings, ann Announcements) *Server {
	return &Server{menu: mgr, data: data, notifier: notifier, settings: settings, announce: ann, accounts: accounts}
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
	mux.HandleFunc("/admin/users/activity", s.auth(s.handleUserActivity))
	mux.HandleFunc("/admin/users/premium", s.auth(s.handleUserPremium))
	mux.HandleFunc("/admin/announcements", s.auth(s.handleAnnouncements))
	mux.HandleFunc("/admin/announcements/add", s.auth(s.handleAnnounceAdd))
	mux.HandleFunc("/admin/announcements/delete", s.auth(s.handleAnnounceDelete))
	mux.HandleFunc("/admin/menu", s.auth(s.handleMenuPage))
	mux.HandleFunc("/admin/menu/add", s.auth(s.handleAdd))
	mux.HandleFunc("/admin/menu/delete", s.auth(s.handleDelete))
	mux.HandleFunc("/admin/support", s.auth(s.handleSupport))
	mux.HandleFunc("/admin/support/reply", s.auth(s.handleSupportReply))
	mux.HandleFunc("/admin/disciplines", s.auth(s.handleDisciplines))
	mux.HandleFunc("/admin/disciplines/add", s.auth(s.handleDisciplineAdd))
	mux.HandleFunc("/admin/disciplines/delete", s.auth(s.handleDisciplineDelete))
	mux.HandleFunc("/admin/predatory", s.auth(s.handlePredatory))
	mux.HandleFunc("/admin/predatory/add", s.auth(s.handlePredatoryAdd))
	mux.HandleFunc("/admin/predatory/delete", s.auth(s.handlePredatoryDelete))
	mux.HandleFunc("/admin/promotion", s.auth(s.handlePromotionEditor))
	mux.HandleFunc("/admin/promotion/save", s.auth(s.handlePromotionSave))
	mux.HandleFunc("/admin/promotion/reset", s.auth(s.handlePromotionReset))
	mux.HandleFunc("/admin/plans", s.auth(s.handlePlans))
	mux.HandleFunc("/admin/plans/save", s.auth(s.handlePlanSave))
	mux.HandleFunc("/admin/plans/delete", s.auth(s.handlePlanDelete))
	mux.HandleFunc("/admin/requests", s.auth(s.handleRequests))
	mux.HandleFunc("/admin/requests/approve", s.auth(s.handleRequestApprove))
	mux.HandleFunc("/admin/requests/reject", s.auth(s.handleRequestReject))
	mux.HandleFunc("/admin/requests/proof", s.auth(s.handleRequestProof))
	mux.HandleFunc("/admin/giftcodes", s.auth(s.handleGiftCodes))
	mux.HandleFunc("/admin/giftcodes/generate", s.auth(s.handleGenerateCodes))
	mux.HandleFunc("/admin/broadcast", s.auth(s.handleBroadcast))
	mux.HandleFunc("/admin/broadcast/send", s.auth(s.handleBroadcastSend))
	mux.HandleFunc("/admin/logs", s.auth(s.handleLogs))
	mux.HandleFunc("/admin/settings", s.auth(s.handleSettings))
	mux.HandleFunc("/admin/settings/gate", s.auth(s.handleSetGate))
	mux.HandleFunc("/admin/settings/payment", s.auth(s.handleSetPayment))
	mux.HandleFunc("/admin/settings/limits", s.auth(s.handleSetLimits))
	mux.HandleFunc("/admin/settings/apikey", s.auth(s.handleSetAPIKey))
	mux.HandleFunc("/admin/settings/identity", s.auth(s.handleSetIdentity))
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

// handleSetLimits saves the per-tier daily AI usage limits.
func (s *Server) handleSetLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.settings == nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("الإعدادات غير متاحة"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	free, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("free_limit")))
	premium, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("premium_limit")))
	if err := s.settings.SetLimits(free, premium); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("تعذّر حفظ الحدود"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/settings?msg="+urlencode("تم حفظ حدود الاستخدام"), http.StatusSeeOther)
}

// handleAnnouncements renders the announcement composer + list.
func (s *Server) handleAnnouncements(w http.ResponseWriter, r *http.Request) {
	s.renderAnnouncements(w, r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleAnnounceAdd publishes (or schedules) a new announcement.
func (s *Server) handleAnnounceAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.announce == nil {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("الإعلانات غير متاحة"), http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	kind := announce.Kind(strings.TrimSpace(r.FormValue("kind")))
	if title == "" || !validKind(kind) {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("العنوان والنوع مطلوبان"), http.StatusSeeOther)
		return
	}
	link := normalizeLink(r.FormValue("link"))
	if r.FormValue("link") != "" && !validLink(link) {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("رابط الزر غير صالح"), http.StatusSeeOther)
		return
	}
	item := announce.Announcement{
		ID:          annID(kind),
		Kind:        kind,
		Title:       title,
		Body:        strings.TrimSpace(r.FormValue("body")),
		Link:        link,
		Image:       normalizeLink(r.FormValue("image")),
		Disciplines: splitCSV(r.FormValue("disciplines")),
		PostedAt:    time.Now(),
	}
	if d := strings.TrimSpace(r.FormValue("deadline")); d != "" {
		if t, err := time.ParseInLocation("2006-01-02", d, webBaghdad); err == nil {
			item.Deadline = &t
		}
	}
	if p := strings.TrimSpace(r.FormValue("publish_at")); p != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", p, webBaghdad); err == nil {
			item.PublishAt = &t
		}
	}
	if err := s.announce.Add(item); err != nil {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("تعذّر نشر الإعلان"), http.StatusSeeOther)
		return
	}
	msg := "تم نشر الإعلان"
	if item.PublishAt != nil {
		msg = "تمت جدولة الإعلان"
	}
	http.Redirect(w, r, "/admin/announcements?msg="+urlencode(msg), http.StatusSeeOther)
}

// handleAnnounceDelete removes an announcement.
func (s *Server) handleAnnounceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.announce == nil {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("الإعلانات غير متاحة"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.announce.Remove(strings.TrimSpace(r.FormValue("id"))); err != nil {
		http.Redirect(w, r, "/admin/announcements?err="+urlencode("الإعلان غير موجود"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/announcements?msg="+urlencode("تم حذف الإعلان"), http.StatusSeeOther)
}

// validKind reports whether k is a known announcement kind.
func validKind(k announce.Kind) bool {
	switch k {
	case announce.KindConference, announce.KindCFP, announce.KindGrant, announce.KindFellowship, announce.KindJob:
		return true
	}
	return false
}

// handleUsers renders the user-management page (activity, message, premium).
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	s.renderUsers(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleUserActivity renders a single user's action timeline for diagnosis.
func (s *Server) handleUserActivity(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("id")), 10, 64)
	s.renderUserActivity(w, r.Context(), id)
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

// handleUserPremium activates, extends, or revokes a user's premium tier. A
// grant takes a plan in months (0 = permanent); extending an active grant adds
// to the remaining time. action="revoke" returns the user to free.
func (s *Server) handleUserPremium(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("نموذج غير صالح"), http.StatusSeeOther)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("id")), 10, 64)
	if err != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("بيانات غير صالحة"), http.StatusSeeOther)
		return
	}
	user, gerr := s.data.GetUser(r.Context(), id)
	if gerr != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("المستخدم غير موجود"), http.StatusSeeOther)
		return
	}
	updated := *user

	if r.FormValue("action") == "revoke" {
		updated.RevokePremium()
		if serr := s.data.SaveUser(r.Context(), &updated); serr != nil {
			http.Redirect(w, r, "/admin/users?err="+urlencode("تعذّر الحفظ"), http.StatusSeeOther)
			return
		}
		if s.notifier != nil {
			_ = s.notifier.Notify(id, "انتهى اشتراكك المميّز في منهل. يمكنك التجديد في أي وقت عبر زر 💎 الاشتراك.")
		}
		http.Redirect(w, r, "/admin/users?msg="+urlencode("تم إلغاء البريميم"), http.StatusSeeOther)
		return
	}

	// Grant / extend. Default to the full (researcher) tier; months 0 = permanent.
	tier := domain.Tier(strings.TrimSpace(r.FormValue("tier")))
	if !validTier(tier) || tier == domain.TierFree {
		tier = domain.TierResearcher
	}
	months, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("months")))
	updated.GrantPremium(tier, months, time.Now())
	if serr := s.data.SaveUser(r.Context(), &updated); serr != nil {
		http.Redirect(w, r, "/admin/users?err="+urlencode("تعذّر حفظ الاشتراك"), http.StatusSeeOther)
		return
	}
	if s.notifier != nil {
		_ = s.notifier.Notify(id, premiumGrantedText(&updated))
	}
	http.Redirect(w, r, "/admin/users?msg="+urlencode("تم تفعيل/تمديد اشتراك المستخدم"), http.StatusSeeOther)
}

// premiumGrantedText is the in-Telegram confirmation a user receives on grant.
func premiumGrantedText(u *domain.User) string {
	if u.PremiumUntil == nil {
		return "🎉 تم تفعيل اشتراكك المميّز في منهل بشكل دائم! استمتع بكامل الأدوات 🌟"
	}
	return "🎉 تم تفعيل اشتراكك المميّز في منهل حتى " + u.PremiumUntil.Format("2006-01-02") +
		"! استمتع بكامل الأدوات 🌟"
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
	} else if err := s.notifier.Notify(ticket.UserID, "📨 رد فريق الدعم:\n\n"+reply+"\n\n💬 للمتابعة، افتح «📞 الدعم الفني» واكتب ردّك."); err != nil {
		msg = "تم حفظ الرد، لكن تعذّر إرساله الآن."
	}
	http.Redirect(w, r, "/admin/support?msg="+urlencode(msg), http.StatusSeeOther)
}

// handleSetAPIKey saves the admin-managed DeepSeek API key.
func (s *Server) handleSetAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.settings == nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("الإعدادات غير متاحة"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	key := strings.TrimSpace(r.FormValue("deepseek_key"))
	if key == "" {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("المفتاح فارغ"), http.StatusSeeOther)
		return
	}
	if err := s.settings.SetDeepSeekKey(key); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("تعذّر حفظ المفتاح"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/settings?msg="+urlencode("تم حفظ مفتاح DeepSeek — فعّال فوراً"), http.StatusSeeOther)
}

// handleSetIdentity saves the bot greeting + name/description.
func (s *Server) handleSetIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.settings == nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("الإعدادات غير متاحة"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	welcome := strings.TrimSpace(r.FormValue("welcome"))
	name := strings.TrimSpace(r.FormValue("bot_name"))
	desc := strings.TrimSpace(r.FormValue("bot_description"))
	if err := s.settings.SetIdentity(welcome, name, desc); err != nil {
		http.Redirect(w, r, "/admin/settings?err="+urlencode("تعذّر حفظ الهوية"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/settings?msg="+urlencode("تم حفظ هوية البوت ورسالته (الاسم/الوصف يُطبَّقان عند النشر)"), http.StatusSeeOther)
}

// codeAlphabet excludes easily-confused characters (0/O, 1/I, etc.).
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// genCode returns a random, readable gift code like "MNHL-AB7K2QPD".
func genCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, len(b))
	for i, x := range b {
		out[i] = codeAlphabet[int(x)%len(codeAlphabet)]
	}
	return "MNHL-" + string(out)
}

// handleDisciplines renders the discipline taxonomy editor.
func (s *Server) handleDisciplines(w http.ResponseWriter, r *http.Request) {
	s.renderDisciplines(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleDisciplineAdd adds a discipline.
func (s *Server) handleDisciplineAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.disciplines == nil {
		http.Redirect(w, r, "/admin/disciplines?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.disciplines.Add(r.FormValue("id"), r.FormValue("label")); err != nil {
		http.Redirect(w, r, "/admin/disciplines?err="+urlencode("تعذّرت الإضافة (معرّف مكرّر أو ناقص)"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/disciplines?msg="+urlencode("تمت إضافة التخصص"), http.StatusSeeOther)
}

// handleDisciplineDelete removes a discipline.
func (s *Server) handleDisciplineDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.disciplines == nil {
		http.Redirect(w, r, "/admin/disciplines?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.disciplines.Remove(strings.TrimSpace(r.FormValue("id"))); err != nil {
		http.Redirect(w, r, "/admin/disciplines?err="+urlencode("التخصص غير موجود"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/disciplines?msg="+urlencode("تم حذف التخصص"), http.StatusSeeOther)
}

// handlePredatory renders the predatory-journal watch-list editor.
func (s *Server) handlePredatory(w http.ResponseWriter, r *http.Request) {
	s.renderPredatory(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handlePredatoryAdd adds a watch-list pattern.
func (s *Server) handlePredatoryAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.predators == nil {
		http.Redirect(w, r, "/admin/predatory?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.predators.Add(r.FormValue("pattern"), r.FormValue("reason")); err != nil {
		http.Redirect(w, r, "/admin/predatory?err="+urlencode("تعذّرت الإضافة (نمط مكرّر أو فارغ)"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/predatory?msg="+urlencode("تمت إضافة النمط"), http.StatusSeeOther)
}

// handlePredatoryDelete removes a watch-list pattern.
func (s *Server) handlePredatoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.predators == nil {
		http.Redirect(w, r, "/admin/predatory?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.predators.Remove(r.FormValue("pattern")); err != nil {
		http.Redirect(w, r, "/admin/predatory?err="+urlencode("النمط غير موجود"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/predatory?msg="+urlencode("تم حذف النمط"), http.StatusSeeOther)
}

// handlePromotionEditor renders the promotion rules (YAML) editor.
func (s *Server) handlePromotionEditor(w http.ResponseWriter, r *http.Request) {
	s.renderPromotion(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handlePromotionSave validates and persists an edited rule set. Invalid YAML is
// rejected with the parser's message; the live rules stay intact.
func (s *Server) handlePromotionSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.promotion == nil {
		http.Redirect(w, r, "/admin/promotion?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.promotion.SetYAML(r.FormValue("rules")); err != nil {
		http.Redirect(w, r, "/admin/promotion?err="+urlencode("خطأ في القواعد: "+err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/promotion?msg="+urlencode("تم حفظ قواعد الترقية وتطبيقها فوراً"), http.StatusSeeOther)
}

// handlePromotionReset restores the ministry defaults.
func (s *Server) handlePromotionReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.promotion == nil {
		http.Redirect(w, r, "/admin/promotion?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	if err := s.promotion.ResetDefault(); err != nil {
		http.Redirect(w, r, "/admin/promotion?err="+urlencode("تعذّرت الاستعادة"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/promotion?msg="+urlencode("تمت استعادة القواعد الافتراضية"), http.StatusSeeOther)
}

// handlePlans renders the subscription-plan catalogue editor.
func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	s.renderPlans(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handlePlanSave upserts a plan (add when the id is new, update otherwise).
func (s *Server) handlePlanSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.plans == nil {
		http.Redirect(w, r, "/admin/plans?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	months, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("months")))
	price, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("price")))
	tier := domain.Tier(strings.TrimSpace(r.FormValue("tier")))
	if !validTier(tier) || tier == domain.TierFree {
		tier = domain.TierResearcher
	}
	p := plans.Plan{
		ID:     strings.TrimSpace(r.FormValue("id")),
		Name:   strings.TrimSpace(r.FormValue("name")),
		Months: months,
		Price:  price,
		Tier:   tier,
	}
	if err := s.plans.Upsert(p); err != nil {
		http.Redirect(w, r, "/admin/plans?err="+urlencode("تعذّر الحفظ (المعرّف والاسم مطلوبان)"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/plans?msg="+urlencode("تم حفظ الباقة"), http.StatusSeeOther)
}

// handlePlanDelete removes a plan by id.
func (s *Server) handlePlanDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.plans == nil {
		http.Redirect(w, r, "/admin/plans?err="+urlencode("غير متاح"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	if err := s.plans.Remove(strings.TrimSpace(r.FormValue("id"))); err != nil {
		http.Redirect(w, r, "/admin/plans?err="+urlencode("الباقة غير موجودة"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/plans?msg="+urlencode("تم حذف الباقة"), http.StatusSeeOther)
}

// handleRequests renders the subscription-request queue.
func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	s.renderRequests(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleRequestApprove activates the requested plan for the user in one click,
// marks the request approved, and notifies the user.
func (s *Server) handleRequestApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	req, err := s.data.GetSubscriptionRequest(r.Context(), strings.TrimSpace(r.FormValue("id")))
	if err != nil {
		http.Redirect(w, r, "/admin/requests?err="+urlencode("الطلب غير موجود"), http.StatusSeeOther)
		return
	}
	user, gerr := s.data.GetUser(r.Context(), req.UserID)
	if gerr != nil || user == nil {
		user = &domain.User{TelegramID: req.UserID, Name: req.UserName}
	}
	updated := *user
	updated.GrantPremium(req.Tier, req.Months, time.Now())
	if serr := s.data.SaveUser(r.Context(), &updated); serr != nil {
		http.Redirect(w, r, "/admin/requests?err="+urlencode("تعذّر تفعيل المستخدم"), http.StatusSeeOther)
		return
	}
	now := time.Now()
	req.Status = domain.SubReqApproved
	req.DecidedAt = &now
	if uerr := s.data.UpdateSubscriptionRequest(r.Context(), *req); uerr != nil {
		http.Redirect(w, r, "/admin/requests?err="+urlencode("فُعّل المستخدم لكن تعذّر تحديث الطلب"), http.StatusSeeOther)
		return
	}
	if s.notifier != nil {
		_ = s.notifier.Notify(req.UserID, premiumGrantedText(&updated))
	}
	http.Redirect(w, r, "/admin/requests?msg="+urlencode("تم تفعيل الاشتراك وإشعار المستخدم"), http.StatusSeeOther)
}

// handleRequestReject marks a request rejected with an optional reason and
// notifies the user.
func (s *Server) handleRequestReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	req, err := s.data.GetSubscriptionRequest(r.Context(), strings.TrimSpace(r.FormValue("id")))
	if err != nil {
		http.Redirect(w, r, "/admin/requests?err="+urlencode("الطلب غير موجود"), http.StatusSeeOther)
		return
	}
	now := time.Now()
	req.Status = domain.SubReqRejected
	req.Note = strings.TrimSpace(r.FormValue("note"))
	req.DecidedAt = &now
	if uerr := s.data.UpdateSubscriptionRequest(r.Context(), *req); uerr != nil {
		http.Redirect(w, r, "/admin/requests?err="+urlencode("تعذّر تحديث الطلب"), http.StatusSeeOther)
		return
	}
	if s.notifier != nil {
		msg := "تعذّر تأكيد دفعتك لطلب الاشتراك."
		if req.Note != "" {
			msg += "\nالسبب: " + req.Note
		}
		msg += "\nيمكنك المحاولة مجدداً أو التواصل مع الدعم."
		_ = s.notifier.Notify(req.UserID, msg)
	}
	http.Redirect(w, r, "/admin/requests?msg="+urlencode("تم رفض الطلب وإشعار المستخدم"), http.StatusSeeOther)
}

// handleRequestProof streams a request's payment-receipt photo through the bot
// (when a proof-image fetcher is wired). Returns 404 when unavailable.
func (s *Server) handleRequestProof(w http.ResponseWriter, r *http.Request) {
	if s.proofImages == nil {
		http.NotFound(w, r)
		return
	}
	req, err := s.data.GetSubscriptionRequest(r.Context(), strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil || req.ProofFileID == "" {
		http.NotFound(w, r)
		return
	}
	data, ctype, ferr := s.proofImages.ProofImage(r.Context(), req.ProofFileID)
	if ferr != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	if ctype == "" {
		ctype = "image/jpeg"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(data)
}

// handleGiftCodes renders the gift-code generator and list.
func (s *Server) handleGiftCodes(w http.ResponseWriter, r *http.Request) {
	s.renderGiftCodes(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleGenerateCodes mints a batch of premium gift codes.
func (s *Server) handleGenerateCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	tier := domain.Tier(strings.TrimSpace(r.FormValue("tier")))
	if !validTier(tier) || tier == domain.TierFree {
		http.Redirect(w, r, "/admin/giftcodes?err="+urlencode("اختر باقة مميّزة (طالب/باحث)"), http.StatusSeeOther)
		return
	}
	days, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("days")))
	if days < 0 {
		days = 0
	}
	qty, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("quantity")))
	if qty < 1 {
		qty = 1
	}
	if qty > 100 {
		qty = 100
	}
	n := 0
	for i := 0; i < qty; i++ {
		if err := s.data.AddGiftCode(r.Context(), domain.GiftCode{Code: genCode(), Tier: tier, Days: days}); err == nil {
			n++
		}
	}
	http.Redirect(w, r, "/admin/giftcodes?msg="+urlencode("تم توليد "+strconv.Itoa(n)+" كوداً"), http.StatusSeeOther)
}

// handleBroadcast renders the broadcast composer.
func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	s.renderBroadcast(w, r.Context(), r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
}

// handleBroadcastSend pushes a message to all (or a tier of) users.
func (s *Server) handleBroadcastSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.notifier == nil {
		http.Redirect(w, r, "/admin/broadcast?err="+urlencode("البوت غير متصل — تعذّر البث"), http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Redirect(w, r, "/admin/broadcast?err="+urlencode("نص الرسالة مطلوب"), http.StatusSeeOther)
		return
	}
	image := normalizeLink(r.FormValue("image"))
	btnLabel := strings.TrimSpace(r.FormValue("button_label"))
	btnURL := normalizeLink(r.FormValue("button_url"))
	if btnURL != "" && !validLink(btnURL) {
		http.Redirect(w, r, "/admin/broadcast?err="+urlencode("رابط الزر غير صالح"), http.StatusSeeOther)
		return
	}
	target := r.FormValue("target")

	users, _ := s.data.ListUsers(r.Context())
	ids := broadcastTargets(users, target, time.Now())

	// Fire-and-forget with light rate limiting to respect Telegram's limits.
	notifier := s.notifier
	go func() {
		for _, id := range ids {
			_ = notifier.SendRich(id, text, image, btnLabel, btnURL)
			time.Sleep(50 * time.Millisecond)
		}
	}()
	http.Redirect(w, r, "/admin/broadcast?msg="+urlencode("بدأ البث إلى "+strconv.Itoa(len(ids))+" مستخدم"), http.StatusSeeOther)
}

// broadcastTargets filters users to the chosen tier ("premium"/"free"/else all).
func broadcastTargets(users []domain.User, target string, now time.Time) []int64 {
	var ids []int64
	for _, u := range users {
		prem := u.IsPremium(now)
		if (target == "premium" && !prem) || (target == "free" && prem) {
			continue
		}
		ids = append(ids, u.TelegramID)
	}
	return ids
}

// handleLogs renders recent system logs for diagnosis.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	s.renderLogs(w, r.Context())
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

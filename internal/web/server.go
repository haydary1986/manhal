// Package web is the HTTP admin adapter. It edits the same menu.Manager the bot
// uses, so changes made on the web page appear in the bot immediately. Access is
// protected by HTTP Basic Auth against an admin token.
package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
)

// Tickets is the persistence the support panel needs.
type Tickets interface {
	ListTickets(ctx context.Context) ([]domain.Ticket, error)
	AnswerTicket(ctx context.Context, id, reply string) (domain.Ticket, error)
}

// Notifier pushes an admin reply back to a user (implemented by the bot).
type Notifier interface {
	Notify(userID int64, text string) error
}

// actionOptions are the leaf actions an admin can attach to a button, plus the
// "submenu" container option. Keys match the bot's menu dispatch.
var actionOptions = []struct{ Key, Label string }{
	{"submenu", "📁 قائمة فرعية"},
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
	tickets  Tickets
	notifier Notifier
	accounts map[string]string // username -> password
}

// NewServer builds the admin server over the shared menu manager and ticket
// store. notifier may be nil (replies are saved but not pushed until the bot is
// up). accounts maps usernames to passwords for Basic Auth.
func NewServer(mgr *menu.Manager, tickets Tickets, notifier Notifier, accounts map[string]string) *Server {
	return &Server{menu: mgr, tickets: tickets, notifier: notifier, accounts: accounts}
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
	mux.HandleFunc("/admin", s.auth(s.handleAdmin))
	mux.HandleFunc("/admin/menu/add", s.auth(s.handleAdd))
	mux.HandleFunc("/admin/menu/delete", s.auth(s.handleDelete))
	mux.HandleFunc("/admin/support", s.auth(s.handleSupport))
	mux.HandleFunc("/admin/support/reply", s.auth(s.handleSupportReply))
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

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	s.render(w, r.URL.Query().Get("msg"), r.URL.Query().Get("err"))
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
	if action == "submenu" {
		item.ID = s.menu.GenID("sub")
	} else {
		item.ID = s.menu.GenID(action)
		item.Action = action
	}
	if err := s.menu.Add(parent, item); err != nil {
		redirectErr(w, r, adminErr(err))
		return
	}
	http.Redirect(w, r, "/admin?msg="+urlencode("تمت إضافة «"+label+"»"), http.StatusSeeOther)
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
	http.Redirect(w, r, "/admin?msg="+urlencode("تم حذف الزر"), http.StatusSeeOther)
}

func redirectErr(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/admin?err="+urlencode(msg), http.StatusSeeOther)
}

// handleSupport lists support tickets.
func (s *Server) handleSupport(w http.ResponseWriter, r *http.Request) {
	tickets, err := s.tickets.ListTickets(r.Context())
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

	ticket, err := s.tickets.AnswerTicket(r.Context(), id, reply)
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

package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
)

func urlencode(s string) string { return url.QueryEscape(s) }

// layout holds the fields every page needs for the shared shell (title bar,
// sidebar active state, open-tickets badge). It is embedded in each page VM so
// its fields are promoted in templates ({{.Heading}}, {{.Active}}, ...).
type layout struct {
	Title     string
	Heading   string
	Sub       string
	Active    string // "dashboard" | "menu" | "support"
	OpenBadge string // open-ticket count for the sidebar pill ("" hides it)
}

// openBadge returns the open-ticket count as a string for the sidebar pill.
func (s *Server) openBadge(ctx context.Context) string {
	tickets, err := s.data.ListTickets(ctx)
	if err != nil {
		return ""
	}
	n := 0
	for _, t := range tickets {
		if t.Status != domain.TicketAnswered {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// ---------- dashboard (analytics) ----------

type statBlock struct {
	Users, Active, Actions, Premium int
	OpenTickets, Buttons, Library   int
	Subscriptions, Watches          int
}

type featRowVM struct {
	Label string
	Count int
	Pct   int // bar width 0..100 relative to the most-used feature
}

type userRowVM struct {
	Rank  int
	Name  string
	ID    string
	Count int
	Extra string // e.g. premium tier
}

type barVM struct {
	Label string
	Count int
	Pct   int
	Peak  bool
}

type dashboardVM struct {
	layout
	Stats    statBlock
	Features []featRowVM
	TopUsers []userRowVM
	Premium  []userRowVM
	Weekdays []barVM
	Hours    []barVM
	PeakDay  string
	PeakHour string
	Msg, Err string
}

// arabicWeekdays maps Go's time.Weekday index (Sunday=0) to Arabic names.
var arabicWeekdays = [7]string{"الأحد", "الاثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة", "السبت"}

// weekdayDisplayOrder shows the week starting Saturday (Arabic convention).
var weekdayDisplayOrder = [7]int{6, 0, 1, 2, 3, 4, 5}

// renderDashboard gathers and renders the analytics overview.
func (s *Server) renderDashboard(w http.ResponseWriter, ctx context.Context, msg, errMsg string) {
	vm := dashboardVM{Msg: msg, Err: errMsg}
	now := time.Now()

	users, _ := s.data.ListUsers(ctx)
	vm.Stats.Users = len(users)
	for _, u := range users {
		if u.IsPremium(now) {
			vm.Stats.Premium++
			if len(vm.Premium) < 10 {
				vm.Premium = append(vm.Premium, userRowVM{
					Rank:  len(vm.Premium) + 1,
					Name:  nameOr(u.Name),
					ID:    strconv.FormatInt(u.TelegramID, 10),
					Extra: string(u.Tier),
				})
			}
		}
		if items, err := s.data.ListLibrary(ctx, u.TelegramID); err == nil {
			vm.Stats.Library += len(items)
		}
	}

	vm.Stats.Actions, vm.Stats.Active, _ = s.data.UsageTotals(ctx)

	tickets, _ := s.data.ListTickets(ctx)
	for _, t := range tickets {
		if t.Status != domain.TicketAnswered {
			vm.Stats.OpenTickets++
		}
	}

	vm.Stats.Buttons = countButtons(s.menu.Root())
	if subs, err := s.data.ListAllSubscriptions(ctx); err == nil {
		vm.Stats.Subscriptions = len(subs)
	}
	if watches, err := s.data.ListAllCitationWatches(ctx); err == nil {
		vm.Stats.Watches = len(watches)
	}

	// Top features with a relative bar width.
	feats, _ := s.data.FeatureUsage(ctx)
	maxCount := 0
	if len(feats) > 0 {
		maxCount = feats[0].Count
	}
	for i, f := range feats {
		if i >= 12 {
			break
		}
		pct := 0
		if maxCount > 0 {
			pct = f.Count * 100 / maxCount
			if pct < 4 {
				pct = 4
			}
		}
		vm.Features = append(vm.Features, featRowVM{Label: actionLabel(f.Action), Count: f.Count, Pct: pct})
	}

	// Most active users.
	top, _ := s.data.TopUsers(ctx, 10)
	for i, u := range top {
		vm.TopUsers = append(vm.TopUsers, userRowVM{
			Rank:  i + 1,
			Name:  nameOr(u.Name),
			ID:    strconv.FormatInt(u.UserID, 10),
			Count: u.Count,
		})
	}

	// Most active weekdays (Baghdad), shown Saturday→Friday.
	wd, _ := s.data.UsageByWeekday(ctx)
	peakDayIdx, peakDayN := -1, 0
	for _, idx := range weekdayDisplayOrder {
		if wd[idx] > peakDayN {
			peakDayN, peakDayIdx = wd[idx], idx
		}
	}
	for _, idx := range weekdayDisplayOrder {
		pct := 0
		if peakDayN > 0 {
			pct = wd[idx] * 100 / peakDayN
			if wd[idx] > 0 && pct < 4 {
				pct = 4
			}
		}
		vm.Weekdays = append(vm.Weekdays, barVM{Label: arabicWeekdays[idx], Count: wd[idx], Pct: pct, Peak: idx == peakDayIdx})
	}
	if peakDayIdx >= 0 {
		vm.PeakDay = arabicWeekdays[peakDayIdx]
	}

	// Most active hours (Baghdad), 0..23.
	hours, _ := s.data.UsageByHour(ctx)
	peakHour, hourMax := -1, 0
	for h, n := range hours {
		if n > hourMax {
			hourMax, peakHour = n, h
		}
	}
	for h := 0; h < 24; h++ {
		pct := 0
		if hourMax > 0 {
			pct = hours[h] * 100 / hourMax
			if hours[h] > 0 && pct < 6 {
				pct = 6
			}
		}
		vm.Hours = append(vm.Hours, barVM{Label: fmt.Sprintf("%02d", h), Count: hours[h], Pct: pct, Peak: h == peakHour && hourMax > 0})
	}
	if peakHour >= 0 && hourMax > 0 {
		vm.PeakHour = fmt.Sprintf("%02d:00", peakHour)
	}

	vm.layout = layout{
		Title:     "منهل — لوحة التحكم",
		Heading:   "📊 لوحة التحكم",
		Sub:       "نظرة عامة وتحليلات استخدام البوت",
		Active:    "dashboard",
		OpenBadge: strconv.Itoa(vm.Stats.OpenTickets),
	}
	if vm.Stats.OpenTickets == 0 {
		vm.layout.OpenBadge = ""
	}

	writeHTML(w, dashboardTemplate, vm)
}

// ---------- menu management ----------

type rowVM struct {
	ID        string
	Label     string
	Indent    string
	IsSubmenu bool
	IsLink    bool
	URL       string
}

type optionVM struct {
	Value string
	Label string
}

type menuPageVM struct {
	layout
	Rows     []rowVM
	Parents  []optionVM
	Actions  []optionVM
	Msg, Err string
}

// renderMenu renders the button-management page.
func (s *Server) renderMenu(w http.ResponseWriter, ctx context.Context, msg, errMsg string) {
	root := s.menu.Root()
	vm := menuPageVM{
		Rows:    buildRows(root, 0),
		Parents: buildParents(root),
		Actions: buildActions(),
		Msg:     msg,
		Err:     errMsg,
		layout: layout{
			Title:     "منهل — إدارة الأزرار",
			Heading:   "🔘 إدارة أزرار البوت",
			Sub:       "أضِف أو احذف الأزرار والقوائم الفرعية — تظهر فوراً في البوت",
			Active:    "menu",
			OpenBadge: s.openBadge(ctx),
		},
	}
	writeHTML(w, menuTemplate, vm)
}

// ---------- users (activity + messaging + premium) ----------

type userManageVM struct {
	Rank      int
	Name      string
	ID        string
	Count     int
	Tier      string
	TierLabel string
	IsPremium bool
	Joined    string
}

type usersVM struct {
	layout
	Users    []userManageVM
	Total    int
	Premium  int
	Msg, Err string
}

func tierLabel(t string) string {
	switch t {
	case "student":
		return "طالب"
	case "researcher":
		return "باحث"
	default:
		return "مجاني"
	}
}

// renderUsers lists every user (most active first) with messaging and tier
// controls.
func (s *Server) renderUsers(w http.ResponseWriter, ctx context.Context, msg, errMsg string) {
	vm := usersVM{Msg: msg, Err: errMsg}
	now := time.Now()

	counts := map[int64]int{}
	if top, err := s.data.TopUsers(ctx, 5000); err == nil {
		for _, u := range top {
			counts[u.UserID] = u.Count
		}
	}
	users, _ := s.data.ListUsers(ctx)
	vm.Total = len(users)
	rows := make([]userManageVM, 0, len(users))
	for _, u := range users {
		prem := u.IsPremium(now)
		if prem {
			vm.Premium++
		}
		tier := string(u.Tier)
		if tier == "" {
			tier = string(domain.TierFree)
		}
		rows = append(rows, userManageVM{
			Name:      nameOr(u.Name),
			ID:        strconv.FormatInt(u.TelegramID, 10),
			Count:     counts[u.TelegramID],
			Tier:      tier,
			TierLabel: tierLabel(tier),
			IsPremium: prem,
			Joined:    u.CreatedAt.Format("2006-01-02"),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Name < rows[j].Name
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	vm.Users = rows

	vm.layout = layout{
		Title:     "منهل — المستخدمون",
		Heading:   "👥 المستخدمون",
		Sub:       "النشاط، المراسلة المباشرة، وإدارة الاشتراك",
		Active:    "users",
		OpenBadge: s.openBadge(ctx),
	}
	writeHTML(w, usersTemplate, vm)
}

// ---------- settings (subscription gate) ----------

type settingsVM struct {
	layout
	Channel  string
	Require  bool
	Msg, Err string
}

// renderSettings renders the bot-settings page (the subscription gate).
func (s *Server) renderSettings(w http.ResponseWriter, ctx context.Context, msg, errMsg string) {
	vm := settingsVM{Msg: msg, Err: errMsg}
	if s.settings != nil {
		vm.Channel = s.settings.RequiredChannel()
		vm.Require = s.settings.RequireSubscription()
	}
	vm.layout = layout{
		Title:     "منهل — الإعدادات",
		Heading:   "⚙️ إعدادات البوت",
		Sub:       "قناة الاشتراك الإجباري قبل استخدام البوت",
		Active:    "settings",
		OpenBadge: s.openBadge(ctx),
	}
	writeHTML(w, settingsTemplate, vm)
}

// ---------- support ----------

type ticketVM struct {
	ID       string
	UserName string
	UserID   string
	Message  string
	Reply    string
	Created  string
	Answered bool
}

type supportVM struct {
	layout
	Tickets   []ticketVM
	OpenCount int
	Msg, Err  string
}

// renderSupport writes the support panel.
func (s *Server) renderSupport(w http.ResponseWriter, tickets []domain.Ticket, msg, errMsg string) {
	vm := supportVM{Msg: msg, Err: errMsg}
	for _, t := range tickets {
		answered := t.Status == domain.TicketAnswered
		if !answered {
			vm.OpenCount++
		}
		vm.Tickets = append(vm.Tickets, ticketVM{
			ID:       t.ID,
			UserName: nameOr(t.UserName),
			UserID:   strconv.FormatInt(t.UserID, 10),
			Message:  t.Message,
			Reply:    t.Reply,
			Created:  t.CreatedAt.Format("2006-01-02 15:04"),
			Answered: answered,
		})
	}
	badge := ""
	if vm.OpenCount > 0 {
		badge = strconv.Itoa(vm.OpenCount)
	}
	vm.layout = layout{
		Title:     "منهل — الدعم الفني",
		Heading:   "📨 الدعم الفني",
		Sub:       "طلبات الباحثين والردّ المباشر عليها",
		Active:    "support",
		OpenBadge: badge,
	}
	writeHTML(w, supportTemplate, vm)
}

// ---------- helpers ----------

func writeHTML(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func nameOr(name string) string {
	if strings.TrimSpace(name) == "" {
		return "مستخدم"
	}
	return name
}

// normalizeLink turns a user-entered link into a full URL Telegram accepts:
// "@handle" -> t.me link, a bare domain -> https://, schemes kept as-is.
func normalizeLink(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "tg://"):
		return s
	case strings.HasPrefix(s, "@"):
		return "https://t.me/" + strings.TrimPrefix(s, "@")
	default:
		return "https://" + s
	}
}

// validLink reports whether a normalized link has a scheme Telegram accepts.
func validLink(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "tg://")
}

// actionLabel maps a menu action key to its Arabic label for display.
func actionLabel(key string) string {
	for _, a := range actionOptions {
		if a.Key == key {
			return a.Label
		}
	}
	return key
}

// countButtons counts every button and submenu in the tree.
func countButtons(items []menu.Item) int {
	n := 0
	for _, it := range items {
		n++
		if it.IsSubmenu() {
			n += countButtons(it.Children)
		}
	}
	return n
}

func buildRows(items []menu.Item, depth int) []rowVM {
	var out []rowVM
	for _, it := range items {
		out = append(out, rowVM{
			ID:        it.ID,
			Label:     it.Label,
			Indent:    strings.Repeat("— ", depth),
			IsSubmenu: it.IsSubmenu(),
			IsLink:    it.IsLink(),
			URL:       it.URL,
		})
		if it.IsSubmenu() {
			out = append(out, buildRows(it.Children, depth+1)...)
		}
	}
	return out
}

func buildParents(items []menu.Item) []optionVM {
	parents := []optionVM{{Value: menu.RootID, Label: "🏠 القائمة الرئيسية"}}
	var walk func(its []menu.Item, prefix string)
	walk = func(its []menu.Item, prefix string) {
		for _, it := range its {
			if it.IsSubmenu() {
				parents = append(parents, optionVM{Value: it.ID, Label: prefix + "📁 " + it.Label})
				walk(it.Children, prefix+"— ")
			}
		}
	}
	walk(items, "")
	return parents
}

func buildActions() []optionVM {
	out := make([]optionVM, 0, len(actionOptions))
	for _, a := range actionOptions {
		out = append(out, optionVM{Value: a.Key, Label: a.Label})
	}
	return out
}

// ---------- templates ----------

const adminCSS = `
*{box-sizing:border-box}
body{margin:0;font-family:"Segoe UI",Tahoma,"Noto Kufi Arabic","Noto Sans Arabic",system-ui,sans-serif;background:#f5f6fa;color:#0f172a}
a{color:inherit}
.layout{display:flex;min-height:100vh}
.sidebar{width:248px;background:linear-gradient(180deg,#1e1b4b,#312e81);color:#cbd5e1;padding:22px 16px;position:sticky;top:0;height:100vh;flex-shrink:0;display:flex;flex-direction:column}
.brand{display:flex;align-items:center;gap:12px;margin-bottom:26px;padding:0 6px}
.brand-logo{width:42px;height:42px;border-radius:12px;background:rgba(255,255,255,.14);display:grid;place-items:center;font-size:22px}
.brand-name{font-weight:700;font-size:18px;color:#fff;line-height:1.2}
.brand-name span{display:block;font-weight:400;font-size:12px;color:#a5b4fc;margin-top:2px}
.nav{display:flex;flex-direction:column;gap:4px}
.nav a{display:flex;align-items:center;gap:10px;color:#c7d2fe;text-decoration:none;padding:11px 14px;border-radius:10px;font-size:15px;transition:.15s}
.nav a:hover{background:rgba(255,255,255,.08);color:#fff}
.nav a.active{background:#fff;color:#312e81;font-weight:600}
.pill{margin-inline-start:auto;background:#f59e0b;color:#fff;border-radius:999px;font-size:12px;padding:1px 9px;font-weight:600}
.nav a.active .pill{background:#312e81;color:#fff}
.side-foot{margin-top:auto;font-size:12px;color:#818cf8;opacity:.85;padding:8px 6px}
.content{flex:1;min-width:0;display:flex;flex-direction:column}
.topbar{background:#fff;border-bottom:1px solid #e9edf3;padding:18px 28px}
.topbar-title{font-size:20px;font-weight:700}
.topbar-sub{color:#64748b;font-size:13px;margin-top:3px}
.container{padding:24px 28px;max-width:1120px;width:100%}
.flash{padding:12px 16px;border-radius:10px;margin-bottom:18px;font-size:14px}
.flash.ok{background:#ecfdf5;color:#047857;border:1px solid #a7f3d0}
.flash.bad{background:#fef2f2;color:#b91c1c;border:1px solid #fecaca}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:16px;margin-bottom:24px}
.stat{background:#fff;border:1px solid #e9edf3;border-radius:14px;padding:18px;box-shadow:0 1px 2px rgba(15,23,42,.04)}
.stat .num{font-size:30px;font-weight:700;line-height:1}
.stat .lbl{color:#64748b;font-size:13px;margin-top:8px;display:flex;align-items:center;gap:6px}
.stat.accent{background:linear-gradient(135deg,#4f46e5,#6d28d9);border:none;color:#fff}
.stat.accent .lbl{color:#ddd6fe}
.stat.warn .num{color:#d97706}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:20px;align-items:start}
@media(max-width:880px){.grid2{grid-template-columns:1fr}.sidebar{width:74px}.brand-name,.side-foot,.nav a span.t{display:none}}
.card{background:#fff;border:1px solid #e9edf3;border-radius:14px;padding:20px;margin-bottom:20px;box-shadow:0 1px 2px rgba(15,23,42,.04)}
.card h3{margin:0 0 16px;font-size:16px;display:flex;align-items:center;gap:8px}
.bars{display:flex;flex-direction:column;gap:11px}
.bar-row{display:grid;grid-template-columns:130px 1fr 42px;align-items:center;gap:10px;font-size:14px}
.bar-track{background:#eef2ff;border-radius:8px;height:12px;overflow:hidden}
.bar-fill{height:100%;background:linear-gradient(90deg,#6366f1,#4338ca);border-radius:8px}
.bar-num{font-weight:700;color:#4338ca;text-align:start}
.rank-list{list-style:none;padding:0;margin:0}
.rank-list li{display:flex;align-items:center;gap:12px;padding:10px 4px;border-bottom:1px solid #f1f5f9}
.rank-list li:last-child{border-bottom:none}
.rank{width:26px;height:26px;border-radius:8px;background:#eef2ff;color:#4338ca;font-weight:700;font-size:13px;display:grid;place-items:center;flex-shrink:0}
.rank.gold{background:#fef3c7;color:#b45309}
.r-name{font-weight:600}
.r-id{color:#94a3b8;font-size:12px}
.r-count{margin-inline-start:auto;font-weight:700;color:#4338ca}
.r-tag{margin-inline-start:auto;background:#ede9fe;color:#6d28d9;border-radius:7px;font-size:12px;padding:2px 9px;font-weight:600}
.empty{color:#94a3b8;text-align:center;padding:26px}
label{display:block;font-size:13px;margin:12px 0 5px;color:#475569;font-weight:500}
input,select,textarea{font-size:15px;padding:10px 12px;border-radius:10px;border:1px solid #d4dae3;width:100%;background:#fff;font-family:inherit}
input:focus,select:focus,textarea:focus{outline:none;border-color:#4f46e5;box-shadow:0 0 0 3px rgba(79,70,229,.12)}
.btn{background:#4f46e5;color:#fff;border:none;cursor:pointer;padding:11px 18px;border-radius:10px;font-size:15px;font-weight:600;margin-top:16px;transition:.15s}
.btn:hover{background:#4338ca}
.btn.block{width:100%}
.list{list-style:none;padding:0;margin:0}
.list li{display:flex;align-items:center;justify-content:space-between;padding:11px 4px;border-bottom:1px solid #f1f5f9}
.list li:last-child{border-bottom:none}
.sub{color:#7c3aed;font-weight:600}
.id{color:#aab4c2;font-size:12px;margin-inline-start:6px}
.btn-del{background:#fee2e2;color:#dc2626;border:none;cursor:pointer;padding:6px 13px;border-radius:8px;font-size:13px;font-weight:600}
.btn-del:hover{background:#fecaca}
form.inline{margin:0;width:auto}
.ticket{border:1px solid #e9edf3;border-radius:12px;padding:16px;margin-bottom:14px}
.ticket.open{border-inline-start:4px solid #f59e0b}
.ticket.done{border-inline-start:4px solid #10b981}
.ticket .meta{color:#64748b;font-size:12px;margin-bottom:8px}
.ticket .msg{white-space:pre-wrap;background:#f8fafc;padding:11px;border-radius:9px}
.ticket .rep{white-space:pre-wrap;background:#ecfdf5;padding:11px;border-radius:9px;margin-top:9px;color:#065f46}
.tag{display:inline-block;background:#eef2ff;color:#4338ca;border-radius:7px;font-size:12px;padding:2px 9px;margin-inline-start:6px;font-weight:600}
.bar-fill.peak{background:linear-gradient(90deg,#fbbf24,#d97706)}
.hours{display:flex;align-items:flex-end;gap:3px;height:130px;margin-top:8px}
.hcol{flex:1;display:flex;flex-direction:column;justify-content:flex-end;align-items:center;height:100%}
.hcol .hbar{width:100%;background:linear-gradient(180deg,#818cf8,#4338ca);border-radius:4px 4px 0 0;min-height:2px;transition:.15s}
.hcol.peak .hbar{background:linear-gradient(180deg,#fbbf24,#d97706)}
.hcol .hlbl{font-size:9px;color:#94a3b8;margin-top:4px}
.peak-note{margin-top:12px;font-size:13px;color:#64748b}
.peak-note b{color:#4338ca}
.urow{border:1px solid #e9edf3;border-radius:12px;margin-bottom:10px;overflow:hidden}
.urow summary{display:flex;align-items:center;gap:10px;padding:12px 14px;cursor:pointer;list-style:none;user-select:none}
.urow summary::-webkit-details-marker{display:none}
.urow summary:hover{background:#f8fafc}
.urow[open] summary{background:#f8fafc;border-bottom:1px solid #eef2f7}
.usp{flex:1}
.udetail{display:grid;grid-template-columns:1fr 1fr;gap:16px;padding:16px}
@media(max-width:760px){.udetail{grid-template-columns:1fr}}
.ucard{margin:0;background:#f8fafc;border:1px solid #eef2f7;border-radius:10px;padding:14px}
`

const layoutHead = `<!doctype html>
<html lang="ar" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>` + adminCSS + `</style>
</head>
<body>
<div class="layout">
  <aside class="sidebar">
    <div class="brand">
      <div class="brand-logo">📚</div>
      <div class="brand-name">منهل<span>لوحة الإدارة</span></div>
    </div>
    <nav class="nav">
      <a href="/admin" class="{{if eq .Active "dashboard"}}active{{end}}">📊 <span class="t">لوحة التحكم</span></a>
      <a href="/admin/users" class="{{if eq .Active "users"}}active{{end}}">👥 <span class="t">المستخدمون</span></a>
      <a href="/admin/menu" class="{{if eq .Active "menu"}}active{{end}}">🔘 <span class="t">إدارة الأزرار</span></a>
      <a href="/admin/support" class="{{if eq .Active "support"}}active{{end}}">📨 <span class="t">الدعم الفني</span>{{if .OpenBadge}}<span class="pill">{{.OpenBadge}}</span>{{end}}</a>
      <a href="/admin/settings" class="{{if eq .Active "settings"}}active{{end}}">⚙️ <span class="t">الإعدادات</span></a>
    </nav>
    <div class="side-foot">منهل · مساعد الباحثين</div>
  </aside>
  <main class="content">
    <div class="topbar">
      <div class="topbar-title">{{.Heading}}</div>
      {{if .Sub}}<div class="topbar-sub">{{.Sub}}</div>{{end}}
    </div>
    <div class="container">`

const layoutFoot = `</div></main></div></body></html>`

var dashboardTemplate = template.Must(template.New("dashboard").Parse(layoutHead + `
{{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
{{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

<div class="stats">
  <div class="stat accent"><div class="num">{{.Stats.Actions}}</div><div class="lbl">⚡ إجمالي الاستخدامات</div></div>
  <div class="stat"><div class="num">{{.Stats.Users}}</div><div class="lbl">👥 المستخدمون المسجّلون</div></div>
  <div class="stat"><div class="num">{{.Stats.Active}}</div><div class="lbl">🟢 مستخدمون نشطون</div></div>
  <div class="stat"><div class="num">{{.Stats.Premium}}</div><div class="lbl">💎 مشتركو البريميم</div></div>
  <div class="stat {{if .Stats.OpenTickets}}warn{{end}}"><div class="num">{{.Stats.OpenTickets}}</div><div class="lbl">📨 طلبات دعم مفتوحة</div></div>
  <div class="stat"><div class="num">{{.Stats.Library}}</div><div class="lbl">⭐ عناصر المكتبة</div></div>
  <div class="stat"><div class="num">{{.Stats.Subscriptions}}</div><div class="lbl">🔔 اشتراكات المتابعة</div></div>
  <div class="stat"><div class="num">{{.Stats.Watches}}</div><div class="lbl">📈 مراقبة الاستشهاد</div></div>
</div>

<div class="grid2">
  <div class="card">
    <h3>🔥 أكثر الميزات استخداماً</h3>
    <div class="bars">
      {{range .Features}}
      <div class="bar-row">
        <div>{{.Label}}</div>
        <div class="bar-track"><div class="bar-fill" style="width:{{.Pct}}%"></div></div>
        <div class="bar-num">{{.Count}}</div>
      </div>
      {{else}}
      <div class="empty">لا توجد بيانات استخدام بعد — ستظهر فور تفاعل المستخدمين مع الأزرار.</div>
      {{end}}
    </div>
  </div>

  <div class="card">
    <h3>🏆 أنشط المستخدمين</h3>
    <ul class="rank-list">
      {{range .TopUsers}}
      <li>
        <span class="rank {{if le .Rank 3}}gold{{end}}">{{.Rank}}</span>
        <span><span class="r-name">{{.Name}}</span> <span class="r-id">#{{.ID}}</span></span>
        <span class="r-count">{{.Count}}</span>
      </li>
      {{else}}
      <li class="empty">لا نشاط بعد.</li>
      {{end}}
    </ul>
  </div>
</div>

<div class="card">
  <h3>📅 أكثر الأيام نشاطاً <span class="tag">توقيت بغداد</span></h3>
  <div class="bars">
    {{range .Weekdays}}
    <div class="bar-row">
      <div>{{.Label}}</div>
      <div class="bar-track"><div class="bar-fill{{if .Peak}} peak{{end}}" style="width:{{.Pct}}%"></div></div>
      <div class="bar-num">{{.Count}}</div>
    </div>
    {{end}}
  </div>
  {{if .PeakDay}}<div class="peak-note">📌 أكثر يوم نشاطاً: <b>{{.PeakDay}}</b></div>{{end}}
</div>

<div class="card">
  <h3>🕒 أكثر الساعات نشاطاً <span class="tag">توقيت بغداد</span></h3>
  <div class="hours">
    {{range .Hours}}
    <div class="hcol{{if .Peak}} peak{{end}}" title="{{.Label}}:00 — {{.Count}}">
      <div class="hbar" style="height:{{.Pct}}%"></div>
      <div class="hlbl">{{.Label}}</div>
    </div>
    {{end}}
  </div>
  {{if .PeakHour}}<div class="peak-note">📌 ذروة النشاط عند الساعة <b>{{.PeakHour}}</b></div>{{end}}
</div>

<div class="card">
  <h3>💎 مشتركو البريميم</h3>
  <ul class="rank-list">
    {{range .Premium}}
    <li>
      <span class="rank gold">{{.Rank}}</span>
      <span><span class="r-name">{{.Name}}</span> <span class="r-id">#{{.ID}}</span></span>
      <span class="r-tag">{{.Extra}}</span>
    </li>
    {{else}}
    <li class="empty">لا يوجد مشتركون في البريميم حالياً.</li>
    {{end}}
  </ul>
</div>
` + layoutFoot))

var menuTemplate = template.Must(template.New("menu").Parse(layoutHead + `
{{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
{{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

<div class="grid2">
  <div class="card">
    <h3>➕ إضافة زر</h3>
    <form method="post" action="/admin/menu/add">
      <label>المكان (القائمة الأم)</label>
      <select name="parent">{{range .Parents}}<option value="{{.Value}}">{{.Label}}</option>{{end}}</select>
      <label>نص الزر</label>
      <input name="label" placeholder="مثال: 📊 إحصائياتي" required>
      <label>الوظيفة</label>
      <select id="actionSel" name="action">{{range .Actions}}<option value="{{.Value}}">{{.Label}}</option>{{end}}</select>
      <div id="urlField" style="display:none">
        <label>الرابط 🔗</label>
        <input name="url" placeholder="@channel أو https://example.com">
      </div>
      <button class="btn block" type="submit">إضافة الزر</button>
    </form>
    <script>
      (function(){
        var sel=document.getElementById('actionSel'), uf=document.getElementById('urlField');
        function t(){ uf.style.display = (sel.value==='url') ? 'block' : 'none'; }
        sel.addEventListener('change', t); t();
      })();
    </script>
  </div>

  <div class="card">
    <h3>📋 أزرار البوت الحالية</h3>
    <ul class="list">
      {{range .Rows}}
      <li>
        <span>{{.Indent}}{{if .IsSubmenu}}<span class="sub">📁 {{.Label}}</span>{{else if .IsLink}}🔗 {{.Label}}{{else}}{{.Label}}{{end}}<span class="id">{{.ID}}</span></span>
        <form class="inline" method="post" action="/admin/menu/delete" onsubmit="return confirm('حذف هذا الزر؟');">
          <input type="hidden" name="id" value="{{.ID}}">
          <button class="btn-del" type="submit">حذف</button>
        </form>
      </li>
      {{else}}
      <li class="empty">لا توجد أزرار.</li>
      {{end}}
    </ul>
  </div>
</div>
` + layoutFoot))

var supportTemplate = template.Must(template.New("support").Parse(layoutHead + `
{{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
{{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

<div class="card">
  <h3>📨 طلبات الدعم <span class="tag">{{.OpenCount}} مفتوح</span></h3>
  {{range .Tickets}}
  <div class="ticket {{if .Answered}}done{{else}}open{{end}}">
    <div class="meta">👤 {{.UserName}} · ID {{.UserID}} · {{.Created}} {{if .Answered}}· ✅ تمت الإجابة{{end}}</div>
    <div class="msg">{{.Message}}</div>
    {{if .Answered}}
      <div class="rep">↩️ {{.Reply}}</div>
    {{else}}
      <form method="post" action="/admin/support/reply">
        <input type="hidden" name="id" value="{{.ID}}">
        <textarea name="reply" rows="3" placeholder="اكتب ردّك على الباحث..." required></textarea>
        <button class="btn" type="submit">إرسال الرد</button>
      </form>
    {{end}}
  </div>
  {{else}}
  <div class="empty">لا توجد طلبات دعم حتى الآن.</div>
  {{end}}
</div>
` + layoutFoot))

var usersTemplate = template.Must(template.New("users").Parse(layoutHead + `
{{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
{{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

<div class="stats">
  <div class="stat"><div class="num">{{.Total}}</div><div class="lbl">👥 إجمالي المستخدمين</div></div>
  <div class="stat"><div class="num">{{.Premium}}</div><div class="lbl">💎 مشتركو البريميم</div></div>
</div>

<div class="card">
  <h3>🏆 المستخدمون — الأنشط أولاً</h3>
  <p style="color:#64748b;font-size:13px;margin:0 0 14px">اضغط على أي مستخدم لمراسلته مباشرة أو تغيير اشتراكه.</p>
  {{range .Users}}
  <details class="urow">
    <summary>
      <span class="rank {{if le .Rank 3}}gold{{end}}">{{.Rank}}</span>
      <span><span class="r-name">{{.Name}}</span> <span class="r-id">#{{.ID}}</span></span>
      {{if .IsPremium}}<span class="r-tag">💎 {{.TierLabel}}</span>{{end}}
      <span class="usp"></span>
      <span class="r-count" title="عدد مرات الاستخدام">{{.Count}}</span>
    </summary>
    <div class="udetail">
      <form method="post" action="/admin/users/message" class="ucard">
        <input type="hidden" name="id" value="{{.ID}}">
        <label>💬 مراسلة المستخدم (تصله في تلكرام)</label>
        <textarea name="text" rows="2" placeholder="مثال: تم استلام مبلغ الاشتراك وتفعيل حسابك." required></textarea>
        <button class="btn" type="submit">إرسال الرسالة</button>
      </form>
      <form method="post" action="/admin/users/tier" class="ucard">
        <input type="hidden" name="id" value="{{.ID}}">
        <label>💎 الاشتراك (تفعيل/إلغاء البريميم يدوياً)</label>
        <select name="tier">
          <option value="free" {{if eq .Tier "free"}}selected{{end}}>مجاني</option>
          <option value="student" {{if eq .Tier "student"}}selected{{end}}>طالب (بريميم)</option>
          <option value="researcher" {{if eq .Tier "researcher"}}selected{{end}}>باحث (بريميم)</option>
        </select>
        <button class="btn" type="submit">تحديث الاشتراك</button>
        <div style="font-size:12px;color:#94a3b8;margin-top:8px">انضمّ: {{.Joined}}</div>
      </form>
    </div>
  </details>
  {{else}}
  <div class="empty">لا مستخدمون بعد.</div>
  {{end}}
</div>
` + layoutFoot))

var settingsTemplate = template.Must(template.New("settings").Parse(layoutHead + `
{{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
{{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

<div class="card" style="max-width:640px">
  <h3>🔒 الاشتراك الإجباري بالقناة</h3>
  <p style="color:#64748b;font-size:14px;margin:0 0 14px;line-height:1.7">
    عند التفعيل، على المستخدم الاشتراك بالقناة قبل استخدام البوت.
    <br><b>مهم:</b> اجعل البوت <b>مشرفاً (Admin)</b> في القناة حتى يستطيع التحقّق من الاشتراك.
  </p>
  <form method="post" action="/admin/settings/gate">
    <label style="display:flex;align-items:center;gap:9px;cursor:pointer;font-size:15px;color:#0f172a">
      <input type="checkbox" name="require" style="width:auto" {{if .Require}}checked{{end}}>
      <span>تفعيل الاشتراك الإجباري</span>
    </label>
    <label>معرّف القناة أو رابطها</label>
    <input name="channel" value="{{.Channel}}" placeholder="@manhal_channel أو https://t.me/manhal_channel">
    <button class="btn" type="submit">💾 حفظ الإعدادات</button>
  </form>
  <div style="margin-top:16px;font-size:13px;color:#64748b">
    الحالة الحالية:
    {{if .Require}}<span class="tag">مُفعّل ✅</span>{{else}}<span class="tag">مُعطّل</span>{{end}}
    {{if .Channel}} · القناة: <b>{{.Channel}}</b>{{end}}
  </div>
</div>
` + layoutFoot))

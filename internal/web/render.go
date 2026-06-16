package web

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/menu"
)

func urlencode(s string) string { return url.QueryEscape(s) }

type rowVM struct {
	ID        string
	Label     string
	Indent    string // visual indentation prefix
	IsSubmenu bool
}

type optionVM struct {
	Value string
	Label string
}

type pageVM struct {
	Rows    []rowVM
	Parents []optionVM
	Actions []optionVM
	Msg     string
	Err     string
}

// render writes the admin page reflecting the current menu tree.
func (s *Server) render(w http.ResponseWriter, msg, errMsg string) {
	root := s.menu.Root()
	vm := pageVM{
		Rows:    buildRows(root, 0),
		Parents: buildParents(root),
		Actions: buildActions(),
		Msg:     msg,
		Err:     errMsg,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, vm); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// buildRows flattens the tree into display rows with indentation.
func buildRows(items []menu.Item, depth int) []rowVM {
	var out []rowVM
	for _, it := range items {
		out = append(out, rowVM{
			ID:        it.ID,
			Label:     it.Label,
			Indent:    strings.Repeat("— ", depth),
			IsSubmenu: it.IsSubmenu(),
		})
		if it.IsSubmenu() {
			out = append(out, buildRows(it.Children, depth+1)...)
		}
	}
	return out
}

// buildParents lists valid add targets: the root plus every submenu.
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
	Tickets   []ticketVM
	OpenCount int
	Msg       string
	Err       string
}

// renderSupport writes the support panel.
func (s *Server) renderSupport(w http.ResponseWriter, tickets []domain.Ticket, msg, errMsg string) {
	vm := supportVM{Msg: msg, Err: errMsg}
	for _, t := range tickets {
		name := t.UserName
		if name == "" {
			name = "مستخدم"
		}
		answered := t.Status == domain.TicketAnswered
		if !answered {
			vm.OpenCount++
		}
		vm.Tickets = append(vm.Tickets, ticketVM{
			ID:       t.ID,
			UserName: name,
			UserID:   strconv.FormatInt(t.UserID, 10),
			Message:  t.Message,
			Reply:    t.Reply,
			Created:  t.CreatedAt.Format("2006-01-02 15:04"),
			Answered: answered,
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := supportTemplate.Execute(w, vm); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

var supportTemplate = template.Must(template.New("support").Parse(`<!doctype html>
<html lang="ar" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>منهل — الدعم الفني</title>
<style>
  body { font-family: -apple-system, "Segoe UI", Tahoma, sans-serif; background:#f4f6f8; color:#1c2733; margin:0; padding:24px; }
  .wrap { max-width: 820px; margin: 0 auto; }
  h1 { font-size: 20px; }
  a.nav { color:#2563eb; text-decoration:none; font-size:14px; }
  .card { background:#fff; border:1px solid #e2e8f0; border-radius:12px; padding:16px; margin-bottom:16px; }
  .open { border-inline-start:4px solid #f59e0b; }
  .done { border-inline-start:4px solid #10b981; }
  .meta { color:#64748b; font-size:12px; margin-bottom:8px; }
  .msg { white-space:pre-wrap; background:#f8fafc; padding:10px; border-radius:8px; }
  .reply { white-space:pre-wrap; background:#ecfdf5; padding:10px; border-radius:8px; margin-top:8px; }
  textarea { width:100%; box-sizing:border-box; border:1px solid #cbd5e1; border-radius:8px; padding:9px; font-size:15px; }
  button { background:#2563eb; color:#fff; border:none; cursor:pointer; padding:9px 16px; border-radius:8px; margin-top:10px; }
  button:hover { background:#1d4ed8; }
  .flash { padding:10px 14px; border-radius:8px; margin-bottom:16px; }
  .ok { background:#e6f7ed; color:#1a7f43; }
  .bad { background:#fdecec; color:#c0392b; }
  .badge { background:#fef3c7; color:#92400e; border-radius:10px; padding:2px 8px; font-size:12px; }
</style>
</head>
<body>
<div class="wrap">
  <h1>📨 الدعم الفني <span class="badge">{{.OpenCount}} مفتوح</span></h1>
  <p><a class="nav" href="/admin">→ إدارة الأزرار</a></p>
  {{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
  {{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

  {{range .Tickets}}
  <div class="card {{if .Answered}}done{{else}}open{{end}}">
    <div class="meta">{{.UserName}} (ID: {{.UserID}}) · {{.Created}} {{if .Answered}}· ✅ تمت الإجابة{{end}}</div>
    <div class="msg">{{.Message}}</div>
    {{if .Answered}}
      <div class="reply">↩️ {{.Reply}}</div>
    {{else}}
      <form method="post" action="/admin/support/reply">
        <input type="hidden" name="id" value="{{.ID}}">
        <textarea name="reply" rows="3" placeholder="اكتب الرد على المستخدم..." required></textarea>
        <button type="submit">إرسال الرد</button>
      </form>
    {{end}}
  </div>
  {{else}}
  <div class="card">لا توجد طلبات دعم.</div>
  {{end}}
</div>
</body>
</html>`))

var pageTemplate = template.Must(template.New("admin").Parse(`<!doctype html>
<html lang="ar" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>منهل — إدارة القائمة</title>
<style>
  body { font-family: -apple-system, "Segoe UI", Tahoma, sans-serif; background:#f4f6f8; color:#1c2733; margin:0; padding:24px; }
  .wrap { max-width: 760px; margin: 0 auto; }
  h1 { font-size: 20px; }
  .card { background:#fff; border:1px solid #e2e8f0; border-radius:12px; padding:20px; margin-bottom:20px; }
  .flash { padding:10px 14px; border-radius:8px; margin-bottom:16px; }
  .ok { background:#e6f7ed; color:#1a7f43; }
  .bad { background:#fdecec; color:#c0392b; }
  label { display:block; font-size:13px; margin:10px 0 4px; color:#475569; }
  input, select, button { font-size:15px; padding:9px 11px; border-radius:8px; border:1px solid #cbd5e1; width:100%; box-sizing:border-box; }
  button { background:#2563eb; color:#fff; border:none; cursor:pointer; margin-top:14px; }
  button:hover { background:#1d4ed8; }
  ul { list-style:none; padding:0; margin:0; }
  li { display:flex; align-items:center; justify-content:space-between; padding:9px 0; border-bottom:1px solid #f1f5f9; }
  .sub { color:#7c3aed; font-weight:600; }
  .del { width:auto; background:#ef4444; padding:6px 12px; margin:0; }
  .del:hover { background:#dc2626; }
  .id { color:#94a3b8; font-size:12px; margin-inline-start:6px; }
  form.inline { margin:0; width:auto; }
</style>
</head>
<body>
<div class="wrap">
  <h1>🛠️ إدارة أزرار البوت</h1>
  <p><a href="/admin/support" style="color:#2563eb;text-decoration:none;font-size:14px;">→ 📨 الدعم الفني</a></p>
  {{if .Msg}}<div class="flash ok">✅ {{.Msg}}</div>{{end}}
  {{if .Err}}<div class="flash bad">❌ {{.Err}}</div>{{end}}

  <div class="card">
    <h3>➕ إضافة زر</h3>
    <form method="post" action="/admin/menu/add">
      <label>المكان (القائمة الأم)</label>
      <select name="parent">
        {{range .Parents}}<option value="{{.Value}}">{{.Label}}</option>{{end}}
      </select>
      <label>نص الزر</label>
      <input name="label" placeholder="مثال: 📊 إحصائياتي" required>
      <label>الوظيفة</label>
      <select name="action">
        {{range .Actions}}<option value="{{.Value}}">{{.Label}}</option>{{end}}
      </select>
      <button type="submit">إضافة</button>
    </form>
  </div>

  <div class="card">
    <h3>📋 الأزرار الحالية</h3>
    <ul>
      {{range .Rows}}
      <li>
        <span>{{.Indent}}{{if .IsSubmenu}}<span class="sub">📁 {{.Label}}</span>{{else}}{{.Label}}{{end}}<span class="id">[{{.ID}}]</span></span>
        <form class="inline" method="post" action="/admin/menu/delete" onsubmit="return confirm('حذف هذا الزر؟');">
          <input type="hidden" name="id" value="{{.ID}}">
          <button class="del" type="submit">حذف</button>
        </form>
      </li>
      {{else}}
      <li>لا توجد أزرار.</li>
      {{end}}
    </ul>
  </div>
</div>
</body>
</html>`))

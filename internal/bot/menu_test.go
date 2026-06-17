package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/menu"
)

func testMenuApp(t *testing.T) *App {
	t.Helper()
	mgr := menu.NewManager(t.TempDir(), []menu.Item{
		{ID: "announcements", Label: "📢 الإعلانات", Action: "announcements"},
		{ID: "refs", Label: "المراجع", Children: []menu.Item{
			{ID: "search", Label: "🔍 بحث", Action: "search"},
			{ID: "cite", Label: "📝 اقتباس", Action: "cite"},
		}},
	})
	return &App{
		cfg:      &config.Config{AdminIDs: []int64{99}},
		settings: config.NewSettingsManager(t.TempDir(), config.DefaultBotSettings()),
		menu:     mgr,
		sessions: newSessions(),
	}
}

func TestMenuKeyboard_SubmenuVsAction(t *testing.T) {
	items := []menu.Item{
		{ID: "search", Label: "🔍", Action: "search"},
		{ID: "refs", Label: "المراجع"}, // submenu (no action)
	}
	kb := menuKeyboard(items, false)

	var sawAction, sawNav bool
	for _, row := range kb.Rows {
		for _, b := range row {
			if b.Data == "menu:search" {
				sawAction = true
			}
			if b.Data == "nav:refs" {
				sawNav = true
			}
		}
	}
	if !sawAction || !sawNav {
		t.Errorf("expected menu:search and nav:refs; got action=%v nav=%v", sawAction, sawNav)
	}
}

func TestMainMenuScreen_FromTree(t *testing.T) {
	a := testMenuApp(t)
	scr := a.mainMenuScreen()
	if !strings.Contains(scr.Text, a.settings.Get().WelcomeMessage) {
		t.Error("main menu should show the welcome message")
	}
	// Submenu shows with a folder marker, action shows directly.
	var hasRefsNav, hasAnnAction bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "nav:refs" {
				hasRefsNav = true
			}
			if b.Data == "menu:announcements" {
				hasAnnAction = true
			}
		}
	}
	if !hasRefsNav || !hasAnnAction {
		t.Errorf("main menu buttons wrong: refsNav=%v annAction=%v", hasRefsNav, hasAnnAction)
	}
}

func TestSubmenuScreen_HasBack(t *testing.T) {
	a := testMenuApp(t)
	kids, _ := a.menu.Children("refs")
	scr := a.submenuScreen("المراجع", kids)
	var hasBack bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "menu:home" {
				hasBack = true
			}
		}
	}
	if !hasBack {
		t.Error("submenu should have a back-to-home button")
	}
}

func TestRenderTree_IndentsChildren(t *testing.T) {
	a := testMenuApp(t)
	out := renderTree(a.menu.Root(), 0)
	if !strings.Contains(out, "[announcements]") || !strings.Contains(out, "[search]") {
		t.Errorf("tree should list ids:\n%s", out)
	}
	if !strings.Contains(out, "↳") {
		t.Errorf("child rows should be indented with ↳:\n%s", out)
	}
}

func TestCollectSubmenusAndFlatten(t *testing.T) {
	a := testMenuApp(t)
	subs := collectSubmenus(a.menu.Root())
	if len(subs) != 1 || subs[0].ID != "refs" {
		t.Errorf("submenus = %+v, want [refs]", subs)
	}
	flat := flatten(a.menu.Root(), 0)
	if len(flat) != 4 { // announcements, refs, search, cite
		t.Errorf("flatten count = %d, want 4", len(flat))
	}
}

func TestUniqueID(t *testing.T) {
	a := testMenuApp(t)
	if got := a.uniqueID("brandnew"); got != "brandnew" {
		t.Errorf("unused base should pass through, got %q", got)
	}
	if got := a.uniqueID("search"); got != "search-2" {
		t.Errorf("taken base should suffix, got %q", got)
	}
}

func TestAdminPanelAndPickers(t *testing.T) {
	a := testMenuApp(t)

	panel := a.adminPanelScreen()
	if !strings.Contains(panel.Text, "[refs]") {
		t.Error("panel should render the tree")
	}

	parent := a.adminParentPickerScreen()
	var hasRoot, hasRefs bool
	for _, row := range parent.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "admin:parent:"+menu.RootID {
				hasRoot = true
			}
			if b.Data == "admin:parent:refs" {
				hasRefs = true
			}
		}
	}
	if !hasRoot || !hasRefs {
		t.Errorf("parent picker missing options: root=%v refs=%v", hasRoot, hasRefs)
	}

	actions := adminActionPickerScreen()
	var actionButtons int
	for _, row := range actions.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "admin:action:") {
				actionButtons++
			}
		}
	}
	if actionButtons != len(adminActions) {
		t.Errorf("action picker buttons = %d, want %d", actionButtons, len(adminActions))
	}
}

func TestAdminAddRemove_EndToEnd(t *testing.T) {
	a := testMenuApp(t)

	// Simulate the wizard: parent=refs, label set, action=journal.
	a.sessions.startAdminLabel(99, "refs")
	a.sessions.captureAdminLabel(99, "🛡️ فحص مجلة")
	if err := a.menu.Add(func() string { p, _ := a.sessions.adminDraft(99); return p }(),
		menu.Item{ID: a.uniqueID("journal"), Label: "🛡️ فحص مجلة", Action: "journal"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	refs, _ := a.menu.Children("refs")
	if len(refs) != 3 {
		t.Errorf("refs should now have 3 children, got %d", len(refs))
	}

	if err := a.menu.Remove("journal"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := a.menu.Find("journal"); ok {
		t.Error("journal should be removed")
	}
}

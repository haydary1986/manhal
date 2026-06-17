package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/store"
)

func testApp() *App {
	deadline := time.Now().Add(5 * 24 * time.Hour)
	repo := announce.NewRepo([]announce.Announcement{
		{ID: "c1", Kind: announce.KindConference, Title: "مؤتمر الحوسبة",
			Disciplines: []string{"cs"}, Deadline: &deadline, Link: "https://x/conf",
			PostedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "g1", Kind: announce.KindGrant, Title: "منحة طبية",
			Disciplines: []string{"med"}, PostedAt: time.Now().Add(-time.Hour)},
	})
	return &App{
		store:       store.NewMemory(),
		announce:    repo,
		disciplines: config.NewDisciplinesManager("", config.DefaultDisciplines()),
		sessions:    newSessions(),
	}
}

func TestAnnouncementsScreen_AllAndContent(t *testing.T) {
	a := testApp()
	scr := a.announcementsScreen(context.Background(), 100, "all")

	if !strings.Contains(scr.Text, "مؤتمر الحوسبة") || !strings.Contains(scr.Text, "منحة طبية") {
		t.Errorf("all feed missing items:\n%s", scr.Text)
	}
	if !strings.Contains(scr.Text, "متبقّي") {
		t.Errorf("expected a deadline countdown:\n%s", scr.Text)
	}
	if !strings.Contains(scr.Text, "كل التخصصات") {
		t.Errorf("expected 'all disciplines' label when no field set")
	}
}

func TestAnnouncementsScreen_KindFilter(t *testing.T) {
	a := testApp()
	scr := a.announcementsScreen(context.Background(), 100, "grant")
	if strings.Contains(scr.Text, "مؤتمر الحوسبة") {
		t.Error("grant tab should not show the conference")
	}
	if !strings.Contains(scr.Text, "منحة طبية") {
		t.Error("grant tab should show the grant")
	}
}

func TestAnnouncementsScreen_DisciplineFilter(t *testing.T) {
	a := testApp()
	ctx := context.Background()
	a.setUserField(ctx, 100, "cs")

	scr := a.announcementsScreen(ctx, 100, "all")
	if !strings.Contains(scr.Text, "علوم الحاسوب") {
		t.Errorf("expected discipline label in header:\n%s", scr.Text)
	}
	if strings.Contains(scr.Text, "منحة طبية") {
		t.Error("cs user should not see the med-only grant")
	}
	if !strings.Contains(scr.Text, "مؤتمر الحوسبة") {
		t.Error("cs user should see the cs conference")
	}
}

func TestAnnouncementsKeyboard_MarksActiveTab(t *testing.T) {
	kb := announcementsKeyboard("grant")
	var found bool
	for _, row := range kb.Rows {
		for _, b := range row {
			if b.Data == "ann:grant" && strings.HasPrefix(b.Text, "• ") {
				found = true
			}
		}
	}
	if !found {
		t.Error("active 'grant' tab should be marked with a bullet")
	}
}

func TestDisciplinePicker_HighlightsCurrent(t *testing.T) {
	a := testApp()
	ctx := context.Background()
	a.setUserField(ctx, 7, "med")

	scr := a.disciplinePicker(ctx, 7)
	var checked bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "field:med" && strings.HasPrefix(b.Text, "✅") {
				checked = true
			}
		}
	}
	if !checked {
		t.Error("current discipline should be checkmarked in the picker")
	}
}

func TestSetUserField_RoundTrips(t *testing.T) {
	a := testApp()
	ctx := context.Background()
	a.setUserField(ctx, 55, "eng")
	if got := a.userField(ctx, 55); got != "eng" {
		t.Errorf("userField = %q, want eng", got)
	}
	a.setUserField(ctx, 55, "")
	if got := a.userField(ctx, 55); got != "" {
		t.Errorf("after clear userField = %q, want empty", got)
	}
}

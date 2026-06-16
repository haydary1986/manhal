package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/store"
)

func libApp() *App {
	return &App{store: store.NewMemory(), sessions: newSessions()}
}

func TestLibraryID_StableAndShort(t *testing.T) {
	a := cite.Work{DOI: "10.1/x"}
	if libraryID(a) != libraryID(a) {
		t.Error("id should be stable")
	}
	if len(libraryID(a)) != 10 {
		t.Errorf("id length = %d, want 10", len(libraryID(a)))
	}
	// Different works => different ids.
	if libraryID(a) == libraryID(cite.Work{DOI: "10.1/y"}) {
		t.Error("different DOIs should differ")
	}
}

func TestLibraryScreen_EmptyAndPopulated(t *testing.T) {
	a := libApp()
	ctx := context.Background()
	const uid int64 = 10

	if !strings.Contains(a.libraryScreen(ctx, uid).Text, "فارغة") {
		t.Error("empty library screen should say empty")
	}

	_ = a.store.AddLibraryItem(ctx, uid, newLibraryItem(cite.Work{Title: "Deep Learning", Year: 2015, DOI: "10.1/dl"}))

	scr := a.libraryScreen(ctx, uid)
	if !strings.Contains(scr.Text, "Deep Learning") || !strings.Contains(scr.Text, "2015") {
		t.Errorf("populated screen missing item:\n%s", scr.Text)
	}
	var hasExport, hasRemove bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "lib:export:bibtex" {
				hasExport = true
			}
			if strings.HasPrefix(b.Data, "lib:rm:") {
				hasRemove = true
			}
		}
	}
	if !hasExport || !hasRemove {
		t.Errorf("screen should offer export and remove: export=%v remove=%v", hasExport, hasRemove)
	}
}

func TestNewLibraryItem_AutoKeywords(t *testing.T) {
	it := newLibraryItem(cite.Work{Title: "Medical Image Segmentation", DOI: "10.1/mis"})
	if it.ID == "" || len(it.Tags) == 0 {
		t.Errorf("item should carry an id and auto-keywords: %+v", it)
	}
}

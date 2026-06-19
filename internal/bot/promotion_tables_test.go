package bot

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPromotionTable1Screen_Content(t *testing.T) {
	scr := promotionTable1Screen()
	for _, want := range []string{"الجدول الأول", "30 · 20 · 20", "مجلات علمية عراقية", "14 · 10.5 · 7"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("table 1 missing %q", want)
		}
	}
}

func TestPromotionTable2Screen_AllItems(t *testing.T) {
	if len(promotionTable2Items) != 34 {
		t.Fatalf("table 2 has %d items, want the full 34", len(promotionTable2Items))
	}
	scr := promotionTable2Screen()
	for _, want := range []string{"الجدول الثاني", "براءة اختراع", "مؤشر هيرتش", "معدل تقييم الأداء"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("table 2 missing %q", want)
		}
	}
	// Telegram caps a message at 4096 UTF-16 code units.
	if n := len(utf16.Encode([]rune(scr.Text))); n > 4096 {
		t.Errorf("table 2 message is %d UTF-16 units, exceeds Telegram's 4096 limit", n)
	}
}

// Every Table-2 activity must be reachable in the interactive builder exactly
// once (via a named category or the auto "بنود أخرى" bucket).
func TestPromoEffectiveCategories_CoverAllTable2Keys(t *testing.T) {
	a := promoApp()
	want := map[string]bool{}
	for _, act := range a.promotion.ActivitiesByTable(2) {
		want[act.Key] = true
	}
	seen := map[string]int{}
	for _, c := range a.promoEffectiveCategories() {
		for _, k := range c.Keys {
			seen[k]++
		}
	}
	for k := range want {
		if seen[k] == 0 {
			t.Errorf("Table-2 key %q is not reachable in any builder category", k)
		}
		if seen[k] > 1 {
			t.Errorf("Table-2 key %q appears in %d categories (should be 1)", k, seen[k])
		}
	}
}

func TestPromotionIntroScreen_HasTablesAndStart(t *testing.T) {
	scr := promoApp().promotionIntroScreen()
	got := map[string]bool{}
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			got[b.Data] = true
		}
	}
	for _, want := range []string{"promo:table1", "promo:table2", "promo:start"} {
		if !got[want] {
			t.Errorf("intro screen missing button %q", want)
		}
	}
	if !strings.Contains(scr.Text, "طريقة ملء البيانات") {
		t.Error("intro screen should explain how to fill the data")
	}
}

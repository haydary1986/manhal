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

func TestPromotionRankScreen_HasTableButtons(t *testing.T) {
	scr := promoApp().promotionRankScreen()
	got := map[string]bool{}
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			got[b.Data] = true
		}
	}
	if !got["promo:table1"] || !got["promo:table2"] {
		t.Error("rank screen should expose both table buttons")
	}
}

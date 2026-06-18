package bot

import (
	"strings"
	"testing"
)

func TestParsePromoAIJSON(t *testing.T) {
	// Tolerates prose wrapping the JSON object.
	out := "تفضل:\n{\"counts\":{\"if_first\":2,\"book_local_solo\":1},\"years\":4} انتهى"
	counts, years := parsePromoAIJSON(out)
	if years != 4 {
		t.Errorf("years = %d, want 4", years)
	}
	if counts["if_first"] != 2 || counts["book_local_solo"] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

func TestParsePromoAIJSON_Garbage(t *testing.T) {
	counts, years := parsePromoAIJSON("لا يوجد JSON هنا")
	if counts != nil || years != 0 {
		t.Errorf("garbage should yield (nil, 0), got (%v, %d)", counts, years)
	}
}

func TestPromoMethodScreen_OffersAIMethod(t *testing.T) {
	rank, _ := promoApp().promotion.FindRank("assistant_lecturer")
	scr := promoApp().promoMethodScreen(rank)
	var hasAI bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "promo:ai:") {
				hasAI = true
			}
		}
	}
	if !hasAI {
		t.Error("method chooser should offer the AI smart-text option")
	}
}

func TestPromoAIPromptScreen_NamesRank(t *testing.T) {
	rank, _ := promoApp().promotion.FindRank("assistant_lecturer")
	scr := promoAIPromptScreen(rank)
	if !strings.Contains(scr.Text, rank.Label) {
		t.Errorf("prompt should name the rank, got:\n%s", scr.Text)
	}
}

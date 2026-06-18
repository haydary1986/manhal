package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/promotion"
)

func promoApp() *App {
	return &App{
		promotion: promotion.NewManager("", promotion.DefaultRules()),
		sessions:  newSessions(),
	}
}

func TestPromotionRankScreen_ListsRanks(t *testing.T) {
	scr := promoApp().promotionRankScreen()
	var rankButtons int
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "promo:rank:") {
				rankButtons++
			}
		}
	}
	if rankButtons != len(promotion.DefaultRules().Ranks) {
		t.Errorf("rank buttons = %d, want %d", rankButtons, len(promotion.DefaultRules().Ranks))
	}
}

func TestPromotionInputScreen_ShowsRequirementsAndKeys(t *testing.T) {
	rank, _ := promotion.DefaultRules().FindRank("assistant_lecturer")
	scr := promoApp().promotionInputScreen(rank)
	for _, want := range []string{"if_first", "book_chapter", "years", "الجدول ١ ≥ 46", "الجدول ١", "الجدول ٢"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("input template missing %q", want)
		}
	}
	// if_first is worth 30 to an assistant lecturer.
	if !strings.Contains(scr.Text, "30 نقطة") {
		t.Errorf("template should show per-rank points:\n%s", scr.Text)
	}
}

func TestPromotionResultScreen_GatesAndVerdict(t *testing.T) {
	rules := promotion.DefaultRules()

	eligible, _ := rules.Compute(promotion.Input{
		RankKey:      "assistant_lecturer",
		Counts:       map[string]float64{"if_first": 2, "book_local_solo": 2, "conference_paper": 2},
		ServiceYears: 4,
	})
	ok := promotionResultScreen(eligible).Text
	if !strings.Contains(ok, "مستوفٍ") {
		t.Errorf("eligible verdict missing:\n%s", ok)
	}
	if !strings.Contains(ok, "✅ المجموع الكلي") {
		t.Errorf("total gate should pass:\n%s", ok)
	}

	short, _ := rules.Compute(promotion.Input{
		RankKey:      "assistant_lecturer",
		Counts:       map[string]float64{"if_first": 1},
		ServiceYears: 1,
	})
	bad := promotionResultScreen(short).Text
	if !strings.Contains(bad, "لم تكتمل") {
		t.Errorf("ineligible verdict missing:\n%s", bad)
	}
	if !strings.Contains(bad, "⛔") {
		t.Errorf("failed gates should be marked:\n%s", bad)
	}
}

func TestSessions_Promotion(t *testing.T) {
	s := newSessions()
	s.startPromotion(5, "lecturer")
	if s.get(5) != stateAwaitPromotion {
		t.Error("startPromotion should set await-promotion state")
	}
	if s.promoteRank(5) != "lecturer" {
		t.Errorf("promoteRank = %q, want lecturer", s.promoteRank(5))
	}
}

func TestFmtNum(t *testing.T) {
	if fmtNum(12) != "12" {
		t.Errorf("fmtNum(12) = %q, want 12", fmtNum(12))
	}
	if fmtNum(10.5) != "10.5" {
		t.Errorf("fmtNum(10.5) = %q, want 10.5", fmtNum(10.5))
	}
}

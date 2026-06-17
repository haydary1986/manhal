package bot

import (
	"strings"
	"testing"
)

func btnData(scr Screen) string {
	var b strings.Builder
	for _, row := range scr.Keyboard.Rows {
		for _, btn := range row {
			b.WriteString(btn.Data + "\n")
		}
	}
	return b.String()
}

func TestPromoBuilder_Flow(t *testing.T) {
	a := promoApp()
	rank, ok := a.promotion.FindRank("assistant_lecturer")
	if !ok {
		t.Fatal("rank not found")
	}
	const uid int64 = 1

	// The method chooser offers the interactive builder and the text method.
	ms := a.promoMethodScreen(rank)
	if !strings.Contains(btnData(ms), "promo:build:assistant_lecturer") {
		t.Error("method screen missing builder button")
	}
	if !strings.Contains(btnData(ms), "promo:text:assistant_lecturer") {
		t.Error("method screen missing text button")
	}

	// Categories cover the known indicators (incl. if_first).
	cats := a.promoEffectiveCategories()
	if len(cats) < 5 {
		t.Fatalf("expected several categories, got %d", len(cats))
	}
	foundKey := false
	for _, c := range cats {
		for _, k := range c.Keys {
			if k == "if_first" {
				foundKey = true
			}
		}
	}
	if !foundKey {
		t.Error("if_first should appear in some category")
	}

	// Fresh builder home shows zero progress.
	a.sessions.promoBegin(uid, rank.Key)
	if !strings.Contains(a.promoHomeScreen(uid, rank).Text, "0/"+fmtNum(rank.RequiredTotal)) {
		t.Errorf("fresh home should show 0 total:\n%s", a.promoHomeScreen(uid, rank).Text)
	}

	// Enter indicators: 2 first-author IF papers (2×30) + 4 years of service.
	a.sessions.promoSetCount(uid, "if_first", 2)
	a.sessions.promoSetCount(uid, promoYearsKey, 4)

	res := a.computeDraft(rank.Key, a.sessions.promoDraftCounts(uid))
	if res.Total < 60 {
		t.Errorf("total = %v, want >= 60", res.Total)
	}
	if res.ServiceYears != 4 {
		t.Errorf("service years = %d, want 4", res.ServiceYears)
	}

	// Home now reflects the entered service years.
	if !strings.Contains(a.promoHomeScreen(uid, rank).Text, "الخدمة: 4") {
		t.Errorf("home should show 4 years:\n%s", a.promoHomeScreen(uid, rank).Text)
	}
}

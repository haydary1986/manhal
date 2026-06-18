package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/plans"
)

func TestSubscribeScreen(t *testing.T) {
	bs := config.BotSettings{
		PremiumInfo:    "الباقة الشهرية 5000 دينار",
		PaymentDetails: "زين كاش: 0770",
		PaymentLink:    "https://zaincash.iq/pay",
	}
	scr := subscribeScreen(bs, nil)

	for _, want := range []string{"الباقة الشهرية 5000 دينار", "زين كاش: 0770", "طريقة الدفع"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("subscribe text missing %q", want)
		}
	}

	var hasPay, hasRequest bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.URL == "https://zaincash.iq/pay" {
				hasPay = true
			}
			if b.Data == "premium:request" {
				hasRequest = true
			}
		}
	}
	if !hasPay {
		t.Error("expected a pay-now URL button when PaymentLink is set")
	}
	if !hasRequest {
		t.Error("expected a premium:request button")
	}

	// Without a link there is no pay button, but the request button stays.
	scr2 := subscribeScreen(config.BotSettings{}, nil)
	for _, row := range scr2.Keyboard.Rows {
		for _, b := range row {
			if b.URL != "" {
				t.Errorf("no pay link expected, got URL button %q", b.URL)
			}
		}
	}
	// The benefits are always listed, even with no admin-configured info.
	if !strings.Contains(scr2.Text, premiumBenefits()[0]) {
		t.Errorf("subscribe screen should always list benefits:\n%s", scr2.Text)
	}
}

func TestSubscribeScreen_RendersPlanButtons(t *testing.T) {
	pl := []plans.Plan{
		{ID: "monthly", Name: "شهري", Months: 1, Price: 5000, Tier: domain.TierResearcher},
		{ID: "annual", Name: "سنوي", Months: 12, Price: 45000, Tier: domain.TierResearcher},
	}
	scr := subscribeScreen(config.BotSettings{}, pl)
	got := map[string]bool{}
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			got[b.Data] = true
		}
	}
	for _, want := range []string{"premium:plan:monthly", "premium:plan:annual", "premium:code"} {
		if !got[want] {
			t.Errorf("subscribe screen missing button %q", want)
		}
	}
	// With plans present, the legacy generic-request button is not shown.
	if got["premium:request"] {
		t.Error("legacy request button should be hidden when plans exist")
	}
	if !strings.Contains(scr.Text, "5000") {
		t.Error("plan prices should appear in the text")
	}
}

func TestPremiumGateScreen_ListsBenefitsAndSubscribe(t *testing.T) {
	scr := premiumGateScreen("ميزة الملفات للمشتركين")
	if !strings.Contains(scr.Text, "ميزة الملفات للمشتركين") {
		t.Error("gate should include the reason")
	}
	if !strings.Contains(scr.Text, premiumBenefits()[0]) {
		t.Error("gate should list premium benefits")
	}
	var hasSubscribe bool
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if b.Data == "menu:subscribe" {
				hasSubscribe = true
			}
		}
	}
	if !hasSubscribe {
		t.Error("gate should offer a subscribe button")
	}
}

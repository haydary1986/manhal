package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/config"
)

func TestSubscribeScreen(t *testing.T) {
	bs := config.BotSettings{
		PremiumInfo:    "الباقة الشهرية 5000 دينار",
		PaymentDetails: "زين كاش: 0770",
		PaymentLink:    "https://zaincash.iq/pay",
	}
	scr := subscribeScreen(bs)

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
	scr2 := subscribeScreen(config.BotSettings{})
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

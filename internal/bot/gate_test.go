package bot

import "testing"

func TestChannelChatID(t *testing.T) {
	cases := map[string]string{
		"@manhal":                    "@manhal",
		"manhal":                     "@manhal",
		"https://t.me/manhal":        "@manhal",
		"http://t.me/manhal":         "@manhal",
		"t.me/manhal":                "@manhal",
		"https://telegram.me/manhal": "@manhal",
		"  @manhal  ":                "@manhal",
		"-1001234567890":             "-1001234567890", // numeric private id kept as-is
		"":                           "",
	}
	for in, want := range cases {
		if got := channelChatID(in); got != want {
			t.Errorf("channelChatID(%q) = %q, want %q", in, got, want)
		}
	}
}

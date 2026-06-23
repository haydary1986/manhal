package youtube

import "testing"

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":     "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                    "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":      "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ&t=10s": "dQw4w9WgXcQ",
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
	}
	for in, want := range cases {
		got, ok := VideoID(in)
		if !ok || got != want {
			t.Errorf("VideoID(%q) = %q,%v; want %q", in, got, ok, want)
		}
	}
	if _, ok := VideoID("https://example.com/notavideo"); ok {
		t.Error("non-YouTube URL should not yield an id")
	}
}

func TestParseTimedText(t *testing.T) {
	xml := `<?xml version="1.0"?><transcript>` +
		`<text start="0" dur="2">Hello &amp;amp; welcome</text>` +
		`<text start="2" dur="3">to the&amp;#39;s talk</text>` +
		`<text start="5" dur="1"></text>` +
		`</transcript>`
	got := parseTimedText(xml)
	want := "Hello & welcome to the's talk"
	if got != want {
		t.Errorf("parseTimedText = %q, want %q", got, want)
	}
}

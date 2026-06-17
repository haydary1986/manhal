package docx

import (
	"strings"
	"testing"
)

func TestBuildRoundTrip(t *testing.T) {
	in := "السطر الأول مع رموز <و> و&.\nالسطر الثاني."
	data, err := Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty docx")
	}

	// The generated file must be readable by our own extractor.
	out, err := ExtractText(data)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"السطر الأول", "<و>", "&", "السطر الثاني"} {
		if !strings.Contains(out, want) {
			t.Errorf("round-trip lost %q; got: %q", want, out)
		}
	}
}

func TestIsRTL(t *testing.T) {
	cases := map[string]bool{
		"مرحبا بالعالم":        true,
		"Hello world":          false,
		"123 then English abc": false, // first strong char is Latin
		"123 ثم عربي":          true,  // first strong char is Arabic
		"":                     false,
	}
	for in, want := range cases {
		if got := isRTL(in); got != want {
			t.Errorf("isRTL(%q) = %v, want %v", in, got, want)
		}
	}
}

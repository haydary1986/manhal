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

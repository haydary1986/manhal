package latex

import (
	"strings"
	"testing"
)

func TestProtectRestore_RoundTrip(t *testing.T) {
	src := `The result is $E=mc^2$ and the matrix below:
\begin{equation}
a^2 + b^2 = c^2
\end{equation}
See \begin{tabular}{cc} 1 & 2 \\ 3 & 4 \end{tabular} for data.
Code: \begin{lstlisting}
x = $y$ // a dollar inside code
\end{lstlisting}
Inline display \[ \int_0^1 x\,dx \] done.`

	masked, tokens := Protect(src)

	// Protected content must be gone from the masked prose.
	for _, bad := range []string{"E=mc^2", "a^2 + b^2", "tabular", "lstlisting", "\\int_0^1"} {
		if strings.Contains(masked, bad) {
			t.Errorf("masked text still contains protected %q:\n%s", bad, masked)
		}
	}
	if len(tokens) < 5 {
		t.Errorf("expected >=5 protected spans, got %d", len(tokens))
	}

	// Restoring must reproduce the original exactly.
	if got := Restore(masked, tokens); got != src {
		t.Errorf("round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, src)
	}
}

func TestProtect_DollarInsideCodeNotTreatedAsMath(t *testing.T) {
	src := "\\begin{verbatim}\nprice = $5 and $10\n\\end{verbatim}\nReal math $x+y$."
	masked, tokens := Protect(src)
	// The verbatim block (with its dollars) is one token; the real $x+y$ another.
	if len(tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(tokens))
	}
	if Restore(masked, tokens) != src {
		t.Error("verbatim dollars should restore verbatim")
	}
}

func TestPlaceholdersIntact(t *testing.T) {
	masked, tokens := Protect("text $a$ more $b$")
	if !PlaceholdersIntact(masked, tokens) {
		t.Error("fresh placeholders should be intact")
	}
	// Simulate a model dropping a placeholder.
	broken := strings.Replace(masked, placeholder(0), "MANGLED", 1)
	if PlaceholdersIntact(broken, tokens) {
		t.Error("should detect a missing placeholder")
	}
}

func TestProtect_NoMath(t *testing.T) {
	src := "Plain prose with no math at all."
	masked, tokens := Protect(src)
	if len(tokens) != 0 || masked != src {
		t.Errorf("plain prose should be untouched: %d tokens", len(tokens))
	}
}

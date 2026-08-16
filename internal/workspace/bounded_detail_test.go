package workspace

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestBoundedDetailRuneBoundary asserts a byte-cap cut landing mid-rune yields
// valid UTF-8, and that whole-rune input under the cap is unaffected.
func TestBoundedDetailRuneBoundary(t *testing.T) {
	// "世" is 3 bytes; the cap is 200. Land the cut inside the multibyte rune.
	padded := strings.Repeat("a", 199) + "世" + strings.Repeat("a", 200)
	got := boundedDetail(padded)
	if !utf8.ValidString(got) {
		t.Fatalf("mid-rune cut produced invalid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncation marker missing: %q", got)
	}

	// A whole-rune boundary at the cap must not corrupt a valid rune.
	aligned := strings.Repeat("a", 200) + strings.Repeat("世", 10)
	got2 := boundedDetail(aligned)
	if !utf8.ValidString(got2) {
		t.Fatalf("whole-rune input produced invalid UTF-8: %q", got2)
	}
	if body := strings.TrimSuffix(got2, "…"); body != strings.Repeat("a", 200) {
		t.Fatalf("whole-rune prefix altered: %q", body)
	}

	// Short input under the cap is returned verbatim.
	if got := boundedDetail("plain"); got != "plain" {
		t.Fatalf("short input altered: %q", got)
	}
}

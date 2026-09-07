package app

import (
	"strings"
	"testing"
)

// TestIsFullObjectID pins the untrusted-receipt syntax check: exactly 40 or 64
// lowercase-hex bytes pass; everything else is rejected. Widths match the
// gitcli reader's validateObjectID ("length 40 or 64"). Cases assert observed
// behavior only — never the predicate's spelling.
func TestIsFullObjectID(t *testing.T) {
	t.Parallel()

	// 64 lowercase-hex bytes containing both digits and letters; hex40 is its
	// 40-byte prefix. Lengths are evident by construction: 4×16 and a [:40] slice.
	hex64 := strings.Repeat("0123456789abcdef", 4)
	hex40 := hex64[:40]
	if len(hex64) != 64 || len(hex40) != 40 {
		t.Fatalf("fixture self-check: len(hex64)=%d len(hex40)=%d", len(hex64), len(hex40))
	}
	// 40 bytes of non-ASCII content: 20 two-byte UTF-8 runes. Rejection of this
	// input depends on character validation, not on length.
	nonASCII40 := strings.Repeat("é", 20)
	if len(nonASCII40) != 40 {
		t.Fatalf("fixture self-check: len(nonASCII40)=%d", len(nonASCII40))
	}

	cases := []struct {
		name string
		in   string
		want bool
	}{
		// Valid full widths.
		{"sha1 40 lowercase hex", hex40, true},
		{"sha256 64 lowercase hex", hex64, true},

		// Wrong lengths (all bytes lowercase hex, so only length can reject).
		{"empty", "", false},
		{"abbreviated 7", hex64[:7], false},
		{"length 39", hex64[:39], false},
		{"length 41", hex64[:41], false},
		{"length 50", hex64[:50], false},
		{"length 63", hex64[:63], false},
		{"length 65", hex64 + "a", false},

		// Case violations at both accepted widths.
		{"uppercase 40", strings.ToUpper(hex40), false},
		{"mixed case final byte of 64", hex64[:63] + "F", false},

		// Non-hex bytes at both accepted widths, first and final positions —
		// including the final byte of a 64, so validation that stops after
		// 40 bytes is detected.
		{"non-hex first byte of 40", "g" + hex40[1:], false},
		{"non-hex final byte of 40", hex40[:39] + "g", false},
		{"non-hex first byte of 64", "g" + hex64[1:], false},
		{"non-hex final byte of 64", hex64[:63] + "g", false},

		// Whitespace, control bytes, non-ASCII — each at a nominally accepted
		// byte length, so rejection depends on character validation alone.
		{"embedded space at length 40", hex40[:39] + " ", false},
		{"trailing newline at length 40", hex40[:39] + "\n", false},
		{"control byte at length 64", hex64[:63] + "\x00", false},
		{"non-ascii bytes at length 40", nonASCII40, false},

		// Ref names and revision expressions must never pass.
		{"symbolic ref", "HEAD", false},
		{"revision expression", "HEAD~1", false},
		{"branch ref path", "refs/heads/main", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isFullObjectID(tc.in); got != tc.want {
				t.Errorf("isFullObjectID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

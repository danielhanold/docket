package app

import "testing"

// TestIsFullObjectID pins isFullObjectID to the exact acceptance of the gitcli
// layer that consumes the value (gitcli.validateObjectID, reached via IsAncestor):
// a non-empty, all-lowercase-hex id of length 40 (SHA-1) OR 64 (SHA-256 full
// representation), never truncated. The 64-hex row is the regression guard: a
// legitimate OpMigrateSeed receipt in a SHA-256 repository carries a 64-char
// Docket-Source-Revision, and rejecting it purely on width misclassifies a real
// docket branch as RootForeign at the SeedMigrate gate.
func TestIsFullObjectID(t *testing.T) {
	const hex40 = "0123456789abcdef0123456789abcdef01234567" // len 40
	const hex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // len 64

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"sha1-full-40-hex", hex40, true},
		{"sha256-full-64-hex", hex64, true},
		{"empty", "", false},
		{"too-short-39-hex", hex40[:39], false},
		{"abbreviated-12-hex", "0123456789ab", false},
		{"between-40-and-64", hex64[:50], false},
		{"too-long-65-hex", hex64 + "a", false},
		{"uppercase-hex-40", "0123456789ABCDEF0123456789abcdef01234567", false},
		{"non-hex-char-40", "0123456789abcdef0123456789abcdef0123456g", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFullObjectID(tc.in); got != tc.want {
				t.Errorf("isFullObjectID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

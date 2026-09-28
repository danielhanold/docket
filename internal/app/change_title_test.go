package app

import "testing"

// TestValidateTitle pins the one title-shape rule change.create and change.groom
// share (change 0461): non-empty after trimming, a single line, and no rune the
// frontmatter writer itself refuses — so the request layer never admits a title
// the writer would reject at Apply (learning validator-must-match-the-reader-it-feeds).
func TestValidateTitle(t *testing.T) {
	cases := []struct {
		name  string
		title string
		code  FindingCode
	}{
		{"plain", "Allow editing a title", ""},
		{"yaml-hostile punctuation is fine", "Fix: the '#1' bug | now", ""},
		{"leading quote is fine", "'quoted' start", ""},
		{"empty", "", FCEmptyTitle},
		{"whitespace only", "  \t ", FCEmptyTitle},
		{"newline", "first\nsecond", FCInvalidTitle},
		{"carriage return", "first\rsecond", FCInvalidTitle},
		{"tab", "a\tb", FCInvalidTitle},
		{"nul", "a\x00b", FCInvalidTitle},
		{"bell", "a\x07b", FCInvalidTitle},
		{"next line (C1)", "a\u0085b", FCInvalidTitle},
		{"line separator", "a\u2028b", FCInvalidTitle},
		{"paragraph separator", "a\u2029b", FCInvalidTitle},
		{"invalid utf-8", "a\xffb", FCInvalidTitle},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, msg := validateTitle(c.title)
			if code != c.code {
				t.Fatalf("validateTitle(%q) code = %q (%s), want %q", c.title, code, msg, c.code)
			}
			if code != "" && msg == "" {
				t.Errorf("validateTitle(%q) refused with an empty message", c.title)
			}
		})
	}
}

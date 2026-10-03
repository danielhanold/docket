package document

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestEmptyValueSpellings pins the one empty-value rule shared by the
// repository decoder's FieldEmpty classification and reposetup's claimed_at
// planner (change 0496): every empty spelling is empty, every non-empty value —
// including a collection — is not, and an undecoded entry (nil node) falls back
// to the located shape alone. Mutation probe: drop any clause of EmptyValue
// (the ShapeEmpty check, the !!null tag check, or the empty-string check) and
// the matching subtests redden.
func TestEmptyValueSpellings(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		want       bool
	}{
		{"bare key", "k:", true},
		{"bare key trailing space", "k: ", true},
		{"single-quoted empty", "k: ''", true},
		{"double-quoted empty", `k: ""`, true},
		{"null keyword", "k: null", true},
		{"tilde", "k: ~", true},
		{"timestamp", "k: 2026-08-01T10:00:00Z", false},
		{"plain text", "k: not-a-timestamp", false},
		{"quoted space", "k: ' '", false},
		{"quoted null word", "k: 'null'", false},
		{"flow sequence", "k: [1]", false},
		{"empty flow sequence", "k: []", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Parse([]byte("---\n" + tc.line + "\n---\nbody\n"))
			if err != nil {
				t.Fatal(err)
			}
			f, ok := d.Field("k")
			if !ok {
				t.Fatalf("field k not located in %q", tc.line)
			}
			var m map[string]yaml.Node
			if err := d.DecodeFrontmatter(&m); err != nil {
				t.Fatal(err)
			}
			n := m["k"]
			if got := EmptyValue(f, true, &n); got != tc.want {
				t.Errorf("EmptyValue(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
	t.Run("nil node falls back to the located shape", func(t *testing.T) {
		if !EmptyValue(Field{Shape: ShapeEmpty}, true, nil) {
			t.Error("a located ShapeEmpty entry with no parsed node must be empty")
		}
		if EmptyValue(Field{Shape: ShapeInline}, true, nil) {
			t.Error("a located inline entry with no parsed node must not be empty")
		}
		if EmptyValue(Field{Shape: ShapeEmpty}, false, nil) {
			t.Error("an unlocated entry's shape must not be consulted")
		}
	})
}

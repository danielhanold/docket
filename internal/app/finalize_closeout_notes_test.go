package app

import (
	"strings"
	"testing"
)

func TestNormalizeCloseoutNotes(t *testing.T) {
	t.Run("valid entries are trimmed, order preserved, empties canonicalize", func(t *testing.T) {
		n, findings := normalizeCloseoutNotes(CloseoutNotes{
			VerificationOutcomes: []string{"  a  ", "b"},
			LateFindings:         []string{},
		})
		if len(findings) != 0 {
			t.Fatalf("findings = %+v, want none", findings)
		}
		if len(n.VerificationOutcomes) != 2 || n.VerificationOutcomes[0] != "a" || n.VerificationOutcomes[1] != "b" {
			t.Fatalf("normalized = %+v", n.VerificationOutcomes)
		}
		if n.LateFindings != nil {
			t.Fatalf("empty list must canonicalize to nil, got %+v", n.LateFindings)
		}
	})

	t.Run("no input and explicitly empty input canonicalize identically", func(t *testing.T) {
		a, _ := normalizeCloseoutNotes(CloseoutNotes{})
		b, _ := normalizeCloseoutNotes(CloseoutNotes{VerificationOutcomes: []string{}, LateFindings: []string{}})
		if !a.Empty() || !b.Empty() {
			t.Fatalf("both must be Empty: %+v %+v", a, b)
		}
		if closeoutNotesDigest(a) != "" || closeoutNotesDigest(b) != "" {
			t.Fatalf("empty notes must digest to the empty string")
		}
	})

	invalid := []struct {
		name  string
		notes CloseoutNotes
		code  string
	}{
		{"entry empty after trimming is invalid not dropped",
			CloseoutNotes{VerificationOutcomes: []string{"  "}}, "empty-note-entry"},
		{"control character rejected",
			CloseoutNotes{LateFindings: []string{"bad\x07bell"}}, "invalid-note-entry"},
		{"carriage return rejected",
			CloseoutNotes{LateFindings: []string{"bad\r\nline"}}, "invalid-note-entry"},
		{"managed-marker text rejected",
			CloseoutNotes{VerificationOutcomes: []string{"x <!-- docket:artifacts:start --> y"}}, "invalid-note-entry"},
		{"oversized entry rejected",
			CloseoutNotes{VerificationOutcomes: []string{strings.Repeat("a", maxAuthoredMarkdownBytes+1)}}, "authored-input-too-large"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, findings := normalizeCloseoutNotes(tc.notes)
			found := false
			for _, f := range findings {
				if f.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings = %+v, want code %q", findings, tc.code)
			}
		})
	}
}

func TestCloseoutNotesDigestKeysTheRenderedSection(t *testing.T) {
	a := CloseoutNotes{VerificationOutcomes: []string{"x"}}
	b := CloseoutNotes{VerificationOutcomes: []string{"x"}}
	c := CloseoutNotes{VerificationOutcomes: []string{"y"}}
	d := CloseoutNotes{LateFindings: []string{"x"}} // same text, other category
	if closeoutNotesDigest(a) != closeoutNotesDigest(b) {
		t.Fatalf("identical notes must digest identically")
	}
	if closeoutNotesDigest(a) == closeoutNotesDigest(c) || closeoutNotesDigest(a) == closeoutNotesDigest(d) {
		t.Fatalf("different notes must digest differently")
	}
}

func TestSpliceCloseoutNotes(t *testing.T) {
	src := []byte("---\nid: 5\n---\n\n## Why\n\nBecause.\n")
	t.Run("empty notes leave the record byte-identical", func(t *testing.T) {
		out, err := spliceCloseoutNotes(src, CloseoutNotes{})
		if err != nil || string(out) != string(src) {
			t.Fatalf("empty splice changed bytes or errored: %v", err)
		}
	})
	t.Run("notes land as the final section", func(t *testing.T) {
		out, err := spliceCloseoutNotes(src, CloseoutNotes{VerificationOutcomes: []string{"ok"}})
		if err != nil {
			t.Fatalf("splice: %v", err)
		}
		want := "## Closeout notes\n\n### Verification\n\n- ok\n"
		if !strings.HasSuffix(string(out), want) {
			t.Fatalf("record does not end with the notes section:\n%s", out)
		}
	})
}

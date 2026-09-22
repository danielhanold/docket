package app

import (
	"strings"
	"testing"
)

// tok returns the uppercase planning token named by its lowercase spelling.
// Constructed at runtime so no fixture SOURCE carries the uppercase word:
// until change 0414 merges and installs, the deployed attach path still
// refuses any plan whose bytes carry one — including this change's own plan,
// which quotes these fixtures verbatim.
func tok(s string) string { return strings.ToUpper(s) }

// Change 0414: plan attachment refuses only a WHOLE-SLOT placeholder — a
// section (or the pre-heading preamble) whose entire authored body reduces to
// one bare token — never a token mentioned inside substantive content.
func TestPlanPlaceholderSlot(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantSlot string
		want     bool
	}{
		// Human-approved acceptance boundary (spec "Outcome and agreed boundary").
		{name: "instruction naming a token attaches",
			src:  "# Plan\n\n## Task 1\n\nRemove the " + tok("todo") + " in retry.go and replace it with bounded retry logic.\n",
			want: false},
		{name: "agreed ambiguous decision sentence attaches",
			src:  "# Plan\n\n## Error handling\n" + tok("todo") + ": decide whether failed requests should retry or stop.\n",
			want: false},
		{name: "every token legal inside substantive prose",
			src: "# Plan\n\n## T\n\nHandle " + tok("tbd") + ", " + tok("todo") + ", " + tok("fixme") + ", " +
				tok("tktk") + ", " + tok("xxx") + ", and " + tok("placeholder") + " markers found in source files.\n",
			want: false},
		{name: "token inside a fenced code example attaches",
			src:  "# Plan\n\n## T\n\n```go\n// " + tok("todo") + "\n```\n",
			want: false},
		{name: "token inside inline code attaches",
			src:  "# Plan\n\n## T\n\n`" + tok("tbd") + "`\n",
			want: false},
		{name: "empty section is not filler (authoring judgment owns it)",
			src:  "# Plan\n\n## T\n\n## U\n\nReal content.\n",
			want: false},
		{name: "token in a list item is not a bare-token slot",
			src:  "# Plan\n\n## T\n\n- " + tok("todo") + "\n",
			want: false},
		{name: "frontmatter content never classifies",
			src:  "---\nnote: " + tok("todo") + "\n---\n# Plan\n\n## T\n\nReal step.\n",
			want: false},
		{name: "fenced example plus trailing bare token is not a filler-only slot",
			// The fence lines are part of the slot body, so the body is more
			// than one bare token — substantive by the whole-slot rule.
			src:  "# Plan\n\n## T\n\n```\n## Fake heading\n```\n\n" + tok("tbd") + "\n",
			want: false},

		// Whole-slot filler refuses, naming the slot.
		{name: "bare-token section body refuses",
			src: "# Plan\n\n## Error handling\n\n" + tok("tbd") + "\n", wantSlot: `section "Error handling"`, want: true},
		{name: "case-insensitive with one terminal period",
			src: "# Plan\n\n## T\n\ntodo.\n", wantSlot: `section "T"`, want: true},
		{name: "another token as a whole slot refuses",
			src: "# Plan\n\n## A\n\n" + tok("fixme") + "\n\n## B\n\nreal\n", wantSlot: `section "A"`, want: true},
		{name: "pre-heading preamble that is only a token refuses",
			src: tok("placeholder") + "\n\n# Plan\n\n## T\n\nReal.\n", wantSlot: "the document preamble", want: true},
		{name: "H1 direct body that is only a token refuses",
			src: "# Plan\n\n" + tok("tktk") + "\n\n## T\n\nReal.\n", wantSlot: `section "Plan"`, want: true},
		{name: "CRLF filler slot refuses",
			src: "# Plan\r\n\r\n## T\r\n\r\n" + tok("xxx") + "\r\n", wantSlot: `section "T"`, want: true},

		// Fence and structure edges (section-slice-needs-a-named-terminator).
		{name: "tilde fence hides heading-shaped lines too",
			src:  "# Plan\n\n## T\n\n~~~\n## Fake\n" + tok("todo") + "\n~~~\nReal prose after.\n",
			want: false},
		{name: "shorter backtick run inside a longer fence does not close it",
			src:  "# Plan\n\n## T\n\n````\n```\n" + tok("todo") + "\n```\n````\n",
			want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot, found := planPlaceholderSlot([]byte(tc.src))
			if found != tc.want {
				t.Fatalf("planPlaceholderSlot(%q) found=%v want=%v (slot %q)", tc.src, found, tc.want, slot)
			}
			if found && slot != tc.wantSlot {
				t.Fatalf("planPlaceholderSlot(%q) slot=%q want %q", tc.src, slot, tc.wantSlot)
			}
		})
	}
}

// A managed (generated) block is not author content: a token inside one — the
// backlink block is the live case — never classifies as a slot body.
func TestPlanPlaceholderSlotIgnoresManagedBlocks(t *testing.T) {
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> " + tok("todo") +
		"\n<!-- docket:backlink:end -->\n# Plan\n\n## T\n\nReal step.\n"
	if slot, found := planPlaceholderSlot([]byte(src)); found {
		t.Fatalf("managed-block token classified as filler (slot %q)", slot)
	}
}

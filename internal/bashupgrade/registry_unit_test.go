package bashupgrade

import (
	"strings"
	"testing"
)

func TestInstallLineHarnesses(t *testing.T) {
	for line, want := range map[string][]string{
		"sh install.sh --harness claude --harness cursor":    {"claude", "cursor"},
		"sh install.sh --harness cursor":                     {"cursor"},
		"sh install.sh --harness claude":                     {"claude"},
		"sh install.sh  --harness cursor   --harness claude": {"cursor", "claude"},
	} {
		got, ok := installLineHarnesses(line)
		if !ok || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: got %v, %v; want %v", line, got, ok, want)
		}
	}
	for _, line := range []string{
		"sh install.sh",
		"sh install.sh --harness",
		"sh install.sh --harness claude extra",
		"sh install.sh --harness claude --harness claude",
		"sh install.sh --harness vim",
		"sh install.sh --harnesses claude",
		"bash install.sh --harness claude",
		"sh ./install.sh --harness claude",
	} {
		if got, ok := installLineHarnesses(line); ok {
			t.Errorf("%q was accepted as an installer line (%v)", line, got)
		}
	}
}

func TestContainsProse(t *testing.T) {
	if !containsProse("one two\n  three\tfour", "two three four") {
		t.Error("a wrapped sentence did not match its one-line form")
	}
	if !containsProse("On `v0.9.2` it is not reported as a conflict; the block\n  below deletes it either way.", "On `v0.9.2` it is not reported as a conflict; the block below deletes it either way.") {
		t.Error("a wrapped guide sentence did not match")
	}
	if containsProse("one two three", "two four") {
		t.Error("different words matched")
	}
	if containsProse("onetwo", "one two") {
		t.Error("collapsing must not delete whitespace between words")
	}
}

func TestPendingPathsFrom(t *testing.T) {
	out := "agent harnesses set to `[claude, cursor]` (needs-review); review and commit the pending paths: CLAUDE.md, .docket.yml\nnext line\n"
	if got := pendingPathsFrom(out); strings.Join(got, ",") != ".docket.yml,CLAUDE.md" {
		t.Fatalf("got %v", got)
	}
	if got := pendingPathsFrom("agent harnesses set to `[claude]` (healthy)\n"); got != nil {
		t.Fatalf("no lead must give nil, got %v", got)
	}
}

func TestSameSet(t *testing.T) {
	if !sameSet([]string{"b", "a"}, []string{"a", "b"}) {
		t.Error("order mattered")
	}
	if sameSet([]string{"a"}, []string{"a", "b"}) || sameSet([]string{"a", "a"}, []string{"a", "b"}) {
		t.Error("different sets compared equal")
	}
}

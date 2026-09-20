package reposetup

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// canonicalGitignoreBlock is the frozen expected block literal — byte-identical
// to scripts/lib/docket-gitignore-block.sh's emit_docket_gitignore_block output
// (markers inclusive, LF endings). This literal is the drift anchor for the
// native emitter: the bash lib documents this block as the "single home for ALL
// docket-owned ignores", and Task 8's TestIntegrationRepoSetupGitignoreParity
// proves cross-language byte-parity against the live bash emitter.
const canonicalGitignoreBlock = "# docket:start (managed by docket — do not hand-edit)\n" +
	".docket/\n" +
	".worktrees/\n" +
	".claude/settings.local.json\n" +
	".docket.local.yml\n" +
	".claude/agents/docket-*.md\n" +
	".codex/agents/docket-*.md\n" +
	".cursor/agents/docket-*.md\n" +
	".opencode/agents/docket-*.md\n" +
	".agents/agents/docket-*.md\n" +
	".kiro/agents/docket-*.md\n" +
	".windsurf/agents/docket-*.md\n" +
	".codex/agents/docket-*.toml\n" +
	".cursor/rules/docket-dispatch.mdc\n" +
	"# docket:end\n"

func TestGitignoreBlockCanonical(t *testing.T) {
	got := GitignoreBlock()
	if !bytes.Equal(got, []byte(canonicalGitignoreBlock)) {
		t.Fatalf("GitignoreBlock() mismatch:\n got %q\nwant %q", got, canonicalGitignoreBlock)
	}
}

func TestGitignoreBlockReturnsFreshCopy(t *testing.T) {
	a := GitignoreBlock()
	a[0] = 'X'
	b := GitignoreBlock()
	if b[0] == 'X' {
		t.Fatalf("GitignoreBlock() returned a shared backing array; mutation leaked")
	}
}

func TestValidGitignoreBlock(t *testing.T) {
	if !ValidGitignoreBlock([]byte(canonicalGitignoreBlock)) {
		t.Fatalf("canonical block not recognized as valid")
	}
	if !ValidGitignoreBlock([]byte("*.log\n\n" + canonicalGitignoreBlock)) {
		t.Fatalf("canonical block after user lines not recognized as valid")
	}
	if ValidGitignoreBlock([]byte("*.log\n")) {
		t.Fatalf("file with no block reported valid")
	}
	// A stale block (extra entry) is not the exact canonical block.
	stale := "# docket:start (managed by docket — do not hand-edit)\n.docket/\n# docket:end\n"
	if ValidGitignoreBlock([]byte(stale)) {
		t.Fatalf("stale block reported valid")
	}
}

func TestEnsureGitignoreBlockOnEmpty(t *testing.T) {
	out, changed, err := EnsureGitignoreBlock(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false on empty input, want true")
	}
	if !bytes.Equal(out, []byte(canonicalGitignoreBlock)) {
		t.Fatalf("out mismatch on empty:\n got %q\nwant %q", out, canonicalGitignoreBlock)
	}
}

func TestEnsureGitignoreBlockPreservesUserLines(t *testing.T) {
	user := "node_modules/\n*.log\n"
	out, changed, err := EnsureGitignoreBlock([]byte(user))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	want := user + "\n" + canonicalGitignoreBlock
	if !bytes.Equal(out, []byte(want)) {
		t.Fatalf("out mismatch:\n got %q\nwant %q", out, want)
	}
	// Outside bytes must be byte-preserved: the user prefix survives verbatim.
	if !bytes.HasPrefix(out, []byte(user)) {
		t.Fatalf("user lines not byte-preserved in %q", out)
	}
}

func TestEnsureGitignoreBlockIdempotent(t *testing.T) {
	user := "node_modules/\n\n"
	first, _, err := EnsureGitignoreBlock([]byte(user))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, changed, err := EnsureGitignoreBlock(first)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("changed = true on already-canonical input, want false")
	}
	if !bytes.Equal(out, first) {
		t.Fatalf("idempotent call altered bytes:\n got %q\nwant %q", out, first)
	}
}

func TestEnsureGitignoreBlockCanonicalAloneIsUnchanged(t *testing.T) {
	out, changed, err := EnsureGitignoreBlock([]byte(canonicalGitignoreBlock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("changed = true on canonical-only input, want false")
	}
	if !bytes.Equal(out, []byte(canonicalGitignoreBlock)) {
		t.Fatalf("canonical-only altered:\n got %q\nwant %q", out, canonicalGitignoreBlock)
	}
}

func TestEnsureGitignoreBlockReplacesStale(t *testing.T) {
	stale := "*.log\n" +
		"# docket:start (managed by docket — do not hand-edit)\n" +
		".docket/\n" +
		"# docket:end\n" +
		"extra-user-line\n"
	out, changed, err := EnsureGitignoreBlock([]byte(stale))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false replacing a stale block, want true")
	}
	// Outside bytes preserved, exactly one canonical block present, stale gone.
	want := "*.log\nextra-user-line\n\n" + canonicalGitignoreBlock
	if !bytes.Equal(out, []byte(want)) {
		t.Fatalf("stale-replacement mismatch:\n got %q\nwant %q", out, want)
	}
	if !ValidGitignoreBlock(out) {
		t.Fatalf("replaced output not valid")
	}
	if bytes.Count(out, []byte(GitignoreStart)) != 1 {
		t.Fatalf("block not present exactly once: %q", out)
	}
}

func TestEnsureGitignoreBlockUpgradesLegacyMarkers(t *testing.T) {
	legacy := "*.log\n" +
		"# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n" +
		".docket/\n" +
		".worktrees/\n" +
		"# docket:generated:end\n"
	out, changed, err := EnsureGitignoreBlock([]byte(legacy))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false upgrading legacy markers, want true")
	}
	want := "*.log\n\n" + canonicalGitignoreBlock
	if !bytes.Equal(out, []byte(want)) {
		t.Fatalf("legacy-upgrade mismatch:\n got %q\nwant %q", out, want)
	}
	if bytes.Contains(out, []byte("docket:generated")) {
		t.Fatalf("legacy markers survived the upgrade: %q", out)
	}
}

func TestEnsureGitignoreBlockRefusesMalformed(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"dangling-start", "# docket:start (managed by docket — do not hand-edit)\n.docket/\n"},
		{"dangling-end", "*.log\n# docket:end\n"},
		{"end-before-start", "# docket:end\n.docket/\n# docket:start (managed by docket — do not hand-edit)\n"},
		{"nested-start", "# docket:start (managed by docket — do not hand-edit)\n.docket/\n# docket:start (managed by docket — do not hand-edit)\n# docket:end\n"},
		{"legacy-dangling-start", "# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket/\n"},
		{"legacy-dangling-end", "*.log\n# docket:generated:end\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			orig := append([]byte(nil), in...)
			out, changed, err := EnsureGitignoreBlock(in)
			if err == nil {
				t.Fatalf("expected error for malformed %s, got out=%q changed=%v", tc.name, out, changed)
			}
			if out != nil {
				t.Fatalf("expected nil out on refusal, got %q", out)
			}
			if changed {
				t.Fatalf("expected changed=false on refusal")
			}
			if !bytes.Equal(in, orig) {
				t.Fatalf("caller's slice was mutated on refusal:\n got %q\nwant %q", in, orig)
			}
		})
	}
}

// --- change 0418: explanatory detail for inputs ValidGitignoreBlock rejects ---

func TestGitignoreEntriesDerivedFromCanonicalBlock(t *testing.T) {
	entries := GitignoreEntries()
	// Derived, not hand-listed: entries are exactly the canonical block's
	// interior lines, in order (AGENTS.md: never hand-list gated sites).
	lines := strings.Split(strings.TrimSuffix(string(GitignoreBlock()), "\n"), "\n")
	want := lines[1 : len(lines)-1]
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("GitignoreEntries() = %v, want interior of canonical block %v", entries, want)
	}
	if len(entries) == 0 || entries[0] != ".docket/" {
		t.Fatalf("canonical order lost: %v", entries)
	}
}

func TestExplainGitignoreBlockValidIsNone(t *testing.T) {
	for name, in := range map[string][]byte{
		"canonical alone":       GitignoreBlock(),
		"canonical with prefix": append([]byte("node_modules/\n\n"), GitignoreBlock()...),
		"canonical with suffix": append(GitignoreBlock(), []byte("\nextra/\n")...),
	} {
		if d := ExplainGitignoreBlock(in); d.Defect != IgnoreDefectNone {
			t.Fatalf("%s: Defect = %v, want None (validity predicate stays authoritative)", name, d.Defect)
		}
	}
}

// TestExplainGitignoreBlockAcceptancePinned: every input the acceptance
// predicate accepts must explain as None, so explanatory work cannot
// silently tighten validity (spec Verification item 3).
func TestExplainGitignoreBlockAcceptancePinned(t *testing.T) {
	// canonical block present + a malformed EXTRA block elsewhere: today's
	// predicate accepts it, and that stays governed by ValidGitignoreBlock.
	in := append(GitignoreBlock(), []byte("\n"+GitignoreEnd+"\n")...)
	if !ValidGitignoreBlock(in) {
		t.Fatalf("fixture drifted: predicate no longer accepts canonical+stray-end")
	}
	if d := ExplainGitignoreBlock(in); d.Defect != IgnoreDefectNone {
		t.Fatalf("accepted input explained as %v, want None", d.Defect)
	}
}

func TestExplainGitignoreBlockBlockAbsent(t *testing.T) {
	d := ExplainGitignoreBlock([]byte("node_modules/\n"))
	if d.Defect != IgnoreDefectBlockAbsent {
		t.Fatalf("Defect = %v, want BlockAbsent", d.Defect)
	}
}

func TestExplainGitignoreBlockLegacyOnly(t *testing.T) {
	in := []byte(legacyGitignoreStart + "\n.docket/\n" + legacyGitignoreEnd + "\n")
	d := ExplainGitignoreBlock(in)
	if d.Defect != IgnoreDefectLegacyOnly {
		t.Fatalf("Defect = %v, want LegacyOnly", d.Defect)
	}
}

func TestExplainGitignoreBlockMalformedMarkers(t *testing.T) {
	cases := map[string]struct {
		in         string
		generation string
	}{
		"dangling start":      {GitignoreStart + "\n.docket/\n", "docket"},
		"end before start":    {GitignoreEnd + "\n" + GitignoreStart + "\n", "docket"},
		"dangling legacy end": {legacyGitignoreEnd + "\n", "legacy"},
	}
	for name, tc := range cases {
		d := ExplainGitignoreBlock([]byte(tc.in))
		if d.Defect != IgnoreDefectMalformedMarkers || d.Generation != tc.generation {
			t.Fatalf("%s: got (%v, %q), want (MalformedMarkers, %q)", name, d.Defect, d.Generation, tc.generation)
		}
	}
}

// TestExplainGitignoreBlockMissingEntries covers the reported real-world
// regression (missing .opencode entry) plus multiple missing entries, all
// reported in canonical order. An entry elsewhere in the file does not
// satisfy membership in the managed block.
func TestExplainGitignoreBlockMissingEntries(t *testing.T) {
	strip := func(remove ...string) []byte {
		out := string(GitignoreBlock())
		for _, r := range remove {
			out = strings.Replace(out, r+"\n", "", 1)
		}
		return []byte(out)
	}
	d := ExplainGitignoreBlock(strip(".opencode/agents/docket-*.md"))
	if d.Defect != IgnoreDefectMissingEntries ||
		!reflect.DeepEqual(d.MissingEntries, []string{".opencode/agents/docket-*.md"}) {
		t.Fatalf("single missing: got (%v, %v)", d.Defect, d.MissingEntries)
	}
	d = ExplainGitignoreBlock(strip(".worktrees/", ".opencode/agents/docket-*.md"))
	if !reflect.DeepEqual(d.MissingEntries, []string{".worktrees/", ".opencode/agents/docket-*.md"}) {
		t.Fatalf("multiple missing not in canonical order: %v", d.MissingEntries)
	}
	// Entry outside the block does not count as membership.
	outside := append([]byte(".opencode/agents/docket-*.md\n\n"), strip(".opencode/agents/docket-*.md")...)
	d = ExplainGitignoreBlock(outside)
	if d.Defect != IgnoreDefectMissingEntries || len(d.MissingEntries) != 1 {
		t.Fatalf("outside-block entry satisfied membership: (%v, %v)", d.Defect, d.MissingEntries)
	}
}

// TestExplainGitignoreBlockNonCanonical: all entries present but not the
// exact canonical representation (reordered / extra interior line) explains
// the mismatch rather than inventing a missing entry.
func TestExplainGitignoreBlockNonCanonical(t *testing.T) {
	reordered := strings.Replace(string(GitignoreBlock()),
		".docket/\n.worktrees/\n", ".worktrees/\n.docket/\n", 1)
	d := ExplainGitignoreBlock([]byte(reordered))
	if d.Defect != IgnoreDefectNonCanonical || len(d.MissingEntries) != 0 {
		t.Fatalf("reordered: got (%v, %v), want (NonCanonical, none)", d.Defect, d.MissingEntries)
	}
	extra := strings.Replace(string(GitignoreBlock()),
		".docket/\n", ".docket/\nextra-line/\n", 1)
	if d := ExplainGitignoreBlock([]byte(extra)); d.Defect != IgnoreDefectNonCanonical {
		t.Fatalf("extra interior line: got %v, want NonCanonical", d.Defect)
	}
}

// TestCommittedIgnoreOutcome maps the probe's three raw results to
// (Presence, IgnoreDetail). A read error is Unknown+Unreadable, never a
// clean absence (learning probe-error-is-not-clean-absence).
func TestCommittedIgnoreOutcome(t *testing.T) {
	p, d := CommittedIgnoreOutcome(nil, false, errors.New("boom"))
	if p != PresenceUnknown || d.Defect != IgnoreDefectUnreadable {
		t.Fatalf("read error: got (%v, %v), want (Unknown, Unreadable)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome(nil, false, nil)
	if p != PresenceAbsent || d.Defect != IgnoreDefectFileAbsent {
		t.Fatalf("file not found: got (%v, %v), want (Absent, FileAbsent)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome(GitignoreBlock(), true, nil)
	if p != PresencePresent || d.Defect != IgnoreDefectNone {
		t.Fatalf("valid: got (%v, %v), want (Present, None)", p, d.Defect)
	}
	p, d = CommittedIgnoreOutcome([]byte("stuff\n"), true, nil)
	if p != PresenceAbsent || d.Defect != IgnoreDefectBlockAbsent {
		t.Fatalf("invalid: got (%v, %v), want (Absent, BlockAbsent)", p, d.Defect)
	}
}

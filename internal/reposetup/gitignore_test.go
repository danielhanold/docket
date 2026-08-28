package reposetup

import (
	"bytes"
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

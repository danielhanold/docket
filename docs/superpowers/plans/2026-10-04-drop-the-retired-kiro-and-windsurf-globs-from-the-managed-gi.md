<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0506 — Drop the retired-harness globs from the managed .gitignore block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0506-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi.md)**
<!-- docket:backlink:end -->
# Drop the retired-harness globs from the managed .gitignore block Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `.agents/agents/docket-*.md`, `.kiro/agents/docket-*.md`, and `.windsurf/agents/docket-*.md` from docket's managed `.gitignore` block (emitter, frozen test copy, and this repository's own `.gitignore`), with a roster guard that keeps every agent-wrapper glob on the accepted harness vocabulary.

**Architecture:** The managed block's bytes have one owner: `canonicalBlockBytes` in `internal/reposetup/gitignore.go`. Every consumer (`GitignoreBlock`, `ValidGitignoreBlock`, `EnsureGitignoreBlock`, `ExplainGitignoreBlock`, `GitignoreEntries`, and the `repository check` remedy via `withCanonicalBlock`) derives from it, so the code change is three deleted lines plus comment cleanup. Existing repositories upgrade through the existing `EnsureGitignoreBlock` rewrite path (`docket repository init`); no compatibility code is added.

**Tech Stack:** Go 1.26 (`internal/reposetup`, test-only import of `internal/harness`).

**Spec:** `docs/superpowers/specs/2026-10-04-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi-design.md` (on the `docket` branch).

## Global Constraints

- New canonical block, exactly (markers unchanged, order otherwise unchanged):
  ```
  # docket:start (managed by docket — do not hand-edit)
  .docket/
  .worktrees/
  .claude/settings.local.json
  .docket.local.yml
  .claude/agents/docket-*.md
  .codex/agents/docket-*.md
  .cursor/agents/docket-*.md
  .opencode/agents/docket-*.md
  .codex/agents/docket-*.toml
  .cursor/rules/docket-dispatch.mdc
  # docket:end
  ```
- Keep the four supported-harness wrapper globs and `.codex/agents/docket-*.toml` (they hide leftover wrapper files from older docket versions; removing them would make `repository init`'s clean-primary preflight refuse).
- Accepted harness vocabulary is exactly `claude`, `codex`, `cursor`, `opencode` (`harness.Order` in `internal/harness/harness.go`; `agentHarnessTokens` in `internal/config/schema.go`).
- No compatibility machinery: `repository check` keeps admitting only the byte-exact block; do not teach it to accept the previous block.
- Comment cross-references anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Accepted ADRs (0020, 0060) stay unedited. `link-skills.sh`, leftover `.cursor/agents/` files, and frozen testdata corpora are out of scope.
- Plan tasks run focused tests only: `go test -count=1 ./internal/reposetup/...`. The full suite (`go run ./cmd/docket development test`) runs once at the build gate, not inside a task.
- Results-file note (for the finishing step, not a task): the human action must tell the operator to re-run `docket repository init` in each adopting repository and commit the rewritten `.gitignore`.

## Review Focus

1. **An adopting repo whose committed `.gitignore` holds the previous 13-entry block between its own lines** — `docket repository init` must rewrite it to the new block, keep every user byte, and be a no-op on a second run. Pinned by `TestEnsureGitignoreBlockUpgradesPreviousBlock` (Task 1).
2. **`repository check` on such a repo** — must classify it `IgnoreDefectNonCanonical` (not `MissingEntries`) and its remedy must paste the new block, with no retired-harness line in it. Pinned by `TestExplainGitignoreBlockPreviousBlockIsNonCanonical` (Task 1).
3. **A user who still wants `.kiro/agents/docket-*.md` ignored and wrote that line outside the managed block** — the rewrite must leave their line alone and the result must be valid. Pinned by the `user-kiro-line-outside-block` subcase of `TestEnsureGitignoreBlockUpgradesPreviousBlock` (Task 1).
4. **Someone re-adds a retired glob, or adds a glob for an unsupported harness (for example `.gemini/agents/docket-*.md`)** — the suite must go red. Pinned by `TestGitignoreAgentGlobsNameSupportedHarnesses` plus its mutation step (Task 1).
5. **Leftover `.cursor/agents/docket-*.md` wrapper files in an adopting repo (this repo has 17 in its primary checkout)** — they must stay ignored, so a supported-harness glob must not be dropped by accident. Pinned by the non-vacuity half of `TestGitignoreAgentGlobsNameSupportedHarnesses` (every harness in `harness.Order` keeps its `.md` glob) in Task 1, and by the `git check-ignore` step in Task 2.

---

### Task 1: Drop the three globs from the emitter, guard the roster, rewrite stale comments

**Files:**
- Modify: `internal/reposetup/gitignore.go` (file header comment; `GitignoreStart`/`GitignoreEnd` const comment; `canonicalBlockBytes` and its comment; `GitignoreBlock` doc comment; `buildFile`, `gitignoreMarkersMalformed`, `stripGitignoreBlock`, `splitLines`, `joinLines` doc comments)
- Test: `internal/reposetup/gitignore_test.go` (header comment on `canonicalGitignoreBlock`; frozen literal; new `previousGitignoreBlock` snapshot; three new tests)

**Interfaces:**
- Consumes: `GitignoreBlock() []byte`, `GitignoreEntries() []string`, `EnsureGitignoreBlock([]byte) ([]byte, bool, error)`, `ValidGitignoreBlock([]byte) bool`, `ExplainGitignoreBlock([]byte) IgnoreDetail`, `committedIgnoreFinding(IgnoreDetail) *Finding` (all existing, package `reposetup`); `harness.Order []string` (existing, `github.com/danielhanold/docket/internal/harness`, test-only import — `go list -deps ./internal/harness` contains no `reposetup`, so there is no cycle).
- Produces: no new production symbols. Test-only: const `previousGitignoreBlock string`, `TestGitignoreAgentGlobsNameSupportedHarnesses`, `TestEnsureGitignoreBlockUpgradesPreviousBlock`, `TestExplainGitignoreBlockPreviousBlockIsNonCanonical`.

- [ ] **Step 1: Update the frozen literal and its header comment in `gitignore_test.go`**

Replace the comment and const `canonicalGitignoreBlock` (everything from `// canonicalGitignoreBlock is the frozen expected block literal` through the `"# docket:end\n"` that closes it) with:

```go
// canonicalGitignoreBlock is the frozen expected block literal (markers
// inclusive, LF endings). It is the drift anchor for canonicalBlockBytes in
// gitignore.go, which is the block's single owner: TestGitignoreBlockCanonical
// byte-compares the two, so any edit to the emitted block must be mirrored
// here on purpose (learning frozen-copy-needs-a-drift-assert).
const canonicalGitignoreBlock = "# docket:start (managed by docket — do not hand-edit)\n" +
	".docket/\n" +
	".worktrees/\n" +
	".claude/settings.local.json\n" +
	".docket.local.yml\n" +
	".claude/agents/docket-*.md\n" +
	".codex/agents/docket-*.md\n" +
	".cursor/agents/docket-*.md\n" +
	".opencode/agents/docket-*.md\n" +
	".codex/agents/docket-*.toml\n" +
	".cursor/rules/docket-dispatch.mdc\n" +
	"# docket:end\n"

// previousGitignoreBlock is a HISTORICAL SNAPSHOT of the managed block as
// emitted before change 0506 dropped the three retired-harness globs
// (.agents, .kiro, .windsurf). It must never be synced to the current block:
// it is the committed bytes an adopting repository still carries until it
// re-runs `docket repository init`, and the upgrade tests below rely on it
// differing from canonicalGitignoreBlock.
const previousGitignoreBlock = "# docket:start (managed by docket — do not hand-edit)\n" +
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
```

- [ ] **Step 2: Add the three new tests to `gitignore_test.go`**

Add `"regexp"` and `"github.com/danielhanold/docket/internal/harness"` to the import block (keep stdlib imports grouped, then a blank line, then the module import). Append to the end of the file:

```go
// --- change 0506: the block's agent-wrapper globs follow the harness roster ---

// agentWrapperGlob is the syntactic shape of an agent-wrapper entry:
// `.<root>/agents/docket-*.<ext>`. The guard keys on this shape, not on a
// list of forbidden spellings, so any unsupported root is caught.
var agentWrapperGlob = regexp.MustCompile(`^\.([^/]+)/agents/docket-\*\.[^/]+$`)

// TestGitignoreAgentGlobsNameSupportedHarnesses: every agent-wrapper entry in
// the managed block names a harness in the accepted vocabulary (harness.Order),
// per ADR-0020 (the block follows the harness roster). Non-vacuity: every
// supported harness keeps its `.md` wrapper glob, which also pins the spec's
// decision to keep those globs for leftover wrapper files.
func TestGitignoreAgentGlobsNameSupportedHarnesses(t *testing.T) {
	supported := map[string]bool{}
	for _, h := range harness.Order {
		supported[h] = true
	}
	if len(supported) == 0 {
		t.Fatalf("harness.Order is empty — the roster guard would pass vacuously")
	}
	entries := map[string]bool{}
	matched := 0
	for _, e := range GitignoreEntries() {
		entries[e] = true
		m := agentWrapperGlob.FindStringSubmatch(e)
		if m == nil {
			continue
		}
		matched++
		if !supported[m[1]] {
			t.Errorf("managed block entry %q names harness root %q, which is not in harness.Order %v", e, m[1], harness.Order)
		}
	}
	if matched == 0 {
		t.Fatalf("no agent-wrapper entries matched %s in %v — the shape guard is vacuous", agentWrapperGlob, GitignoreEntries())
	}
	for _, h := range harness.Order {
		want := "." + h + "/agents/docket-*.md"
		if !entries[want] {
			t.Errorf("managed block lost supported-harness wrapper glob %q", want)
		}
	}
}

// TestEnsureGitignoreBlockUpgradesPreviousBlock: an adopting repository whose
// committed .gitignore holds the previous block upgrades through the existing
// rewrite path — user bytes outside the block are preserved, the block becomes
// canonical, and a second call is a no-op.
func TestEnsureGitignoreBlockUpgradesPreviousBlock(t *testing.T) {
	cases := []struct {
		name   string
		before string // user lines above the block
		after  string // user lines below the block
	}{
		{"user-lines-around-block", "node_modules/\n*.log\n\n", "dist/\n"},
		{"user-kiro-line-outside-block", ".kiro/agents/docket-*.md\n\n", ""},
		{"block-alone", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.before + previousGitignoreBlock + tc.after
			out, changed, err := EnsureGitignoreBlock([]byte(in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !changed {
				t.Fatalf("changed = false on the previous block, want true")
			}
			rest := strings.TrimRight(tc.before+tc.after, "\n")
			want := canonicalGitignoreBlock
			if rest != "" {
				want = rest + "\n\n" + canonicalGitignoreBlock
			}
			if string(out) != want {
				t.Fatalf("upgrade mismatch:\n got %q\nwant %q", out, want)
			}
			if !ValidGitignoreBlock(out) {
				t.Fatalf("upgraded output not valid: %q", out)
			}
			if d := ExplainGitignoreBlock(out); d.Defect != IgnoreDefectNone {
				t.Fatalf("upgraded output explains as %v, want None", d.Defect)
			}
			for _, retired := range []string{".agents/agents/docket-*.md", ".windsurf/agents/docket-*.md"} {
				if strings.Contains(string(out), retired) {
					t.Fatalf("retired glob %q survived the upgrade: %q", retired, out)
				}
			}
			// The managed block itself carries no .kiro line; a user's own
			// line outside it is user content and survives verbatim.
			if strings.Contains(canonicalGitignoreBlock, ".kiro/") {
				t.Fatalf("fixture drifted: canonical block carries a .kiro line")
			}
			if strings.Contains(tc.before+tc.after, ".kiro/agents/docket-*.md") &&
				!strings.HasPrefix(string(out), ".kiro/agents/docket-*.md\n") {
				t.Fatalf("user's own .kiro line outside the block was not preserved: %q", out)
			}
			again, changed2, err := EnsureGitignoreBlock(out)
			if err != nil {
				t.Fatalf("unexpected error on second call: %v", err)
			}
			if changed2 || !bytes.Equal(again, out) {
				t.Fatalf("second call not idempotent: changed=%v\n got %q\nwant %q", changed2, again, out)
			}
		})
	}
}

// TestExplainGitignoreBlockPreviousBlockIsNonCanonical: the previous block
// holds every current entry plus the three retired ones, so repository check
// classifies it NonCanonical (not MissingEntries), and that finding's remedy
// pastes the new block — never a retired-harness line.
func TestExplainGitignoreBlockPreviousBlockIsNonCanonical(t *testing.T) {
	for name, in := range map[string]string{
		"block alone":      previousGitignoreBlock,
		"with user prefix": "node_modules/\n\n" + previousGitignoreBlock,
	} {
		if ValidGitignoreBlock([]byte(in)) {
			t.Fatalf("%s: previous block accepted as canonical", name)
		}
		d := ExplainGitignoreBlock([]byte(in))
		if d.Defect != IgnoreDefectNonCanonical || len(d.MissingEntries) != 0 {
			t.Fatalf("%s: got (%v, %v), want (NonCanonical, none)", name, d.Defect, d.MissingEntries)
		}
		fnd := committedIgnoreFinding(d)
		if !strings.Contains(fnd.Remedy, strings.TrimSuffix(canonicalGitignoreBlock, "\n")) {
			t.Fatalf("%s: remedy does not paste the canonical block:\n%s", name, fnd.Remedy)
		}
		for _, retired := range []string{".agents/agents/", ".kiro/agents/", ".windsurf/agents/"} {
			if strings.Contains(fnd.Remedy, retired) {
				t.Fatalf("%s: remedy still names retired glob %q:\n%s", name, retired, fnd.Remedy)
			}
		}
	}
}
```

- [ ] **Step 3: Run the focused tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi && go test -count=1 ./internal/reposetup/ -run 'TestGitignoreBlockCanonical|TestGitignoreAgentGlobsNameSupportedHarnesses|TestEnsureGitignoreBlockUpgradesPreviousBlock|TestExplainGitignoreBlockPreviousBlockIsNonCanonical' -v`

Expected: FAIL. `TestGitignoreBlockCanonical` reports a mismatch; `TestGitignoreAgentGlobsNameSupportedHarnesses` reports `"agents"`, `"kiro"`, `"windsurf"` not in `harness.Order`; `TestEnsureGitignoreBlockUpgradesPreviousBlock/block-alone` reports `changed = false`; `TestExplainGitignoreBlockPreviousBlockIsNonCanonical` reports `previous block accepted as canonical`. If any of the four passes here, stop — the test is vacuous; fix it before continuing (learning plan-supplied-test-code-is-unverified).

- [ ] **Step 4: Remove the three lines from `canonicalBlockBytes` and rewrite the stale comments in `gitignore.go`**

4a. Replace the file header comment (from `// gitignore.go — native emitter` through `// atomically writing the file.`) with:

```go
// gitignore.go — emitter and marker validation for docket's managed
// .gitignore block, the single home for every docket-owned ignore. This file
// is the sole owner of the block's bytes (canonicalBlockBytes) and its
// marker-balance rules; GitignoreBlock, ValidGitignoreBlock,
// EnsureGitignoreBlock, GitignoreEntries, and ExplainGitignoreBlock all derive
// from that one definition. Frozen-copy drift is caught by
// TestGitignoreBlockCanonical, which byte-compares GitignoreBlock() with the
// literal in gitignore_test.go (learning frozen-copy-needs-a-drift-assert),
// and TestGitignoreAgentGlobsNameSupportedHarnesses keeps every agent-wrapper
// glob on the accepted harness roster (ADR-0020).
//
// No I/O: these are pure functions over byte slices. Callers own reading and
// atomically writing the file.
```

4b. Replace the marker-const comment (the three lines starting `// GitignoreStart / GitignoreEnd are the exact marker lines owned by the bash lib`) with:

```go
// GitignoreStart / GitignoreEnd are the exact marker lines that delimit the
// managed block. The legacy 0051 spelling is recognized only for one-time
// upgrade detection and is never re-emitted.
```

4c. Replace the `canonicalBlockBytes` comment and value (from `// canonicalBlock is the block body` through `GitignoreEnd + "\n")`) with:

```go
// canonicalBlockBytes is the block body between (and including) the markers,
// LF endings. Order is load-bearing: core entries, .docket.local.yml, one
// agent-wrapper glob per supported harness (claude, codex, cursor, opencode —
// harness.Order), the codex toml glob, then the cursor dispatch rule. docket
// no longer writes per-repository wrappers; the wrapper globs stay so leftover
// files from older versions remain ignored.
var canonicalBlockBytes = []byte(GitignoreStart + "\n" +
	".docket/\n" +
	".worktrees/\n" +
	".claude/settings.local.json\n" +
	".docket.local.yml\n" +
	".claude/agents/docket-*.md\n" +
	".codex/agents/docket-*.md\n" +
	".cursor/agents/docket-*.md\n" +
	".opencode/agents/docket-*.md\n" +
	".codex/agents/docket-*.toml\n" +
	".cursor/rules/docket-dispatch.mdc\n" +
	GitignoreEnd + "\n")
```

4d. Replace the `GitignoreBlock` doc comment with:

```go
// GitignoreBlock returns the canonical managed block bytes (markers inclusive,
// LF line endings). A fresh copy is returned on each call so callers may
// mutate it freely.
```

4e. Replace the `buildFile` doc comment with:

```go
// buildFile assembles the outside bytes, a blank-line separator, and a single
// canonical block. Trailing newlines in the outside content are trimmed and
// replaced by exactly one blank-line separator, which is what makes a second
// call idempotent. When rest is empty the block stands alone.
```

4f. In the `gitignoreMarkersMalformed` doc comment, change `// gitignoreMarkersMalformed mirrors _docket_gi_malformed: returns true when the` to `// gitignoreMarkersMalformed returns true when the` (re-wrap the paragraph; keep the rest of its wording).

4g. In the `stripGitignoreBlock` doc comment, change `// removed and every byte outside it preserved — the port of _docket_gi_strip_block.` to `// removed and every byte outside it preserved.`

4h. In the `splitLines` doc comment, delete the sentence `This matches awk's record model, on which the bash primitives rely.` In the `joinLines` doc comment, change `(awk print semantics)` to `(each line LF-terminated)`.

- [ ] **Step 5: Verify no stale bash references remain**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi && out=$(grep -n -i -e bash -e awk -e emit_docket -e _docket_gi -e Parity -e "Task 8" internal/reposetup/gitignore.go internal/reposetup/gitignore_test.go || true); printf '%s\n' "${out:-CLEAN}"`

Expected: `CLEAN`.

Run: `cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi && grep -c -e kiro -e windsurf -e '\.agents/agents' internal/reposetup/gitignore.go`

Expected: `0`.

- [ ] **Step 6: Run the package tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi && go test -count=1 ./internal/reposetup/...`

Expected: PASS (including every pre-existing `TestExplainGitignoreBlock*`, `TestEnsureGitignoreBlock*`, and `TestCommittedIgnoreRemediesCarryPasteReadyBlock`, which derive from `GitignoreBlock()`).

- [ ] **Step 7: Mutation-test the roster guard**

Back up, mutate, run, restore — never restore with `git checkout --` (learning mutation-restore-needs-a-backup-copy).

```bash
W=/Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
BK=$(mktemp "${TMPDIR:-/tmp}/gitignore-go.XXXXXX")
cp "$W/internal/reposetup/gitignore.go" "$BK"
perl -0pi -e 's|(\t"\.opencode/agents/docket-\*\.md\\n" \+\n)|$1\t".kiro/agents/docket-*.md\\n" +\n|' "$W/internal/reposetup/gitignore.go"
grep -c 'kiro/agents' "$W/internal/reposetup/gitignore.go"   # must print 1 — proves the mutation landed
(cd "$W" && go test -count=1 ./internal/reposetup/ -run TestGitignoreAgentGlobsNameSupportedHarnesses)  # must FAIL naming "kiro"
mv -f "$BK" "$W/internal/reposetup/gitignore.go"
grep -c 'kiro/agents' "$W/internal/reposetup/gitignore.go" || true   # must print 0 — restored
```

Expected: the grep prints `1`, the test FAILS with `names harness root "kiro"`, the restore grep prints `0`.

Second mutation (non-vacuity half): repeat the same backup/restore sequence with the perl line `perl -0pi -e 's|\t"\.cursor/agents/docket-\*\.md\\n" \+\n||' …` and confirm `grep -c 'cursor/agents/docket-\*\.md' …` drops to `0`, the test FAILS with `lost supported-harness wrapper glob ".cursor/agents/docket-*.md"`, then restore and confirm the count is `1` again.

Re-run `go test -count=1 ./internal/reposetup/...` after both restores. Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
git add internal/reposetup/gitignore.go internal/reposetup/gitignore_test.go && git commit -m "fix(reposetup): drop retired-harness globs from the managed .gitignore block

Remove .agents/.kiro/.windsurf agent-wrapper globs from canonicalBlockBytes,
add a roster guard keyed on harness.Order, pin the previous-block upgrade and
check classification, and rewrite comments that named the deleted bash emitter."
```

---

### Task 2: Rewrite this repository's own `.gitignore` to the new canonical block

**Files:**
- Modify: `.gitignore` (repository root of the feature worktree)

**Interfaces:**
- Consumes: the canonical block from Task 1 (Global Constraints lists it verbatim).
- Produces: a committed `.gitignore` whose managed block is byte-exact, so this repository stays `healthy` under `repository check` once the change merges.

- [ ] **Step 1: Prove the current block is the stale one**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
grep -c -e '^\.agents/agents/docket-\*\.md$' -e '^\.kiro/agents/docket-\*\.md$' -e '^\.windsurf/agents/docket-\*\.md$' .gitignore
```
Expected: `3`.

- [ ] **Step 2: Delete exactly those three lines**

Delete the lines `.agents/agents/docket-*.md`, `.kiro/agents/docket-*.md`, and `.windsurf/agents/docket-*.md` from `.gitignore` with the Edit tool (one edit replacing the three-line run between `.opencode/agents/docket-*.md` and `.codex/agents/docket-*.toml` with nothing). Touch nothing outside the managed block (`.DS_Store`, `.superpowers/`, and the blank separator line stay byte-identical).

- [ ] **Step 3: Verify the block is byte-exact and the diff is exactly three deletions**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
got=$(sed -n '/^# docket:start /,/^# docket:end$/p' .gitignore)
want=$(printf '%s\n' '# docket:start (managed by docket — do not hand-edit)' '.docket/' '.worktrees/' '.claude/settings.local.json' '.docket.local.yml' '.claude/agents/docket-*.md' '.codex/agents/docket-*.md' '.cursor/agents/docket-*.md' '.opencode/agents/docket-*.md' '.codex/agents/docket-*.toml' '.cursor/rules/docket-dispatch.mdc' '# docket:end')
[ "$got" = "$want" ] && echo BLOCK-OK || { echo BLOCK-MISMATCH; diff <(printf '%s\n' "$want") <(printf '%s\n' "$got"); }
git diff --numstat -- .gitignore
last=$(tail -c 1 .gitignore | od -An -c | tr -d ' ')
[ "$last" = '\n' ] && echo EOF-NEWLINE || echo "EOF-BYTE=$last"
```
Expected: `BLOCK-OK`; numstat `0	3	.gitignore`; `EOF-NEWLINE`.

- [ ] **Step 4: Verify leftover-wrapper and core ignores still hold**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
for p in .cursor/agents/docket-status.md .claude/agents/docket-x.md .codex/agents/docket-x.toml .docket/x .worktrees/x .docket.local.yml; do git check-ignore -q "$p" && echo "ignored $p" || echo "NOT-IGNORED $p"; done
git check-ignore -q .kiro/agents/docket-x.md && echo "STILL-IGNORED kiro" || echo "kiro no longer ignored"
```
Expected: six `ignored …` lines and `kiro no longer ignored`.

- [ ] **Step 5: Run the package tests once more**

Run: `cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi && go test -count=1 ./internal/reposetup/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi
git add .gitignore && git commit -m "chore: rewrite this repository's managed .gitignore block to the new canonical bytes"
```

<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0500 — committed-ignore-invalid remedies print the paste-ready managed block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0500-committed-ignore-invalid-hint-for-an-absent-gitignore-names.md)**
<!-- docket:backlink:end -->
# committed-ignore-invalid Remedies Print the Paste-Ready Managed Block Implementation Plan

> **For agentic workers:** Execute with the `docket-build` skill (docket's build role): each task goes to a tier agent running the docket-build-task contract, followed by one full-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every `committed-ignore-invalid` remedy ends with the exact canonical managed `.gitignore` block on its own lines, ready to paste, and no remedy names `docket repository migrate`.

**Architecture:** One unexported helper in `internal/reposetup/health.go` appends `GitignoreBlock()` (trailing newline trimmed) to an instruction after a newline. `committedIgnoreFinding` routes every case through it with a per-defect instruction ending in a colon. A terminal unexported sentinel on the `IgnoreDefect` enum lets a variant guard derive its population instead of hand-listing it. A second guard in `internal/app` proves `repository check`'s human text prints the block at column 0.

**Tech Stack:** Go (`internal/reposetup`, `internal/app`), plain untagged Go unit tests.

**Spec:** `docs/superpowers/specs/2026-10-04-committed-ignore-invalid-hint-for-an-absent-gitignore-names-design.md` (on the `docket` branch)

## Global Constraints

- The block bytes come **only** from `GitignoreBlock()`, never from a second literal (no copy of the marker lines or entries in `health.go` or in either test).
- No `committed-ignore-invalid` remedy names `repository migrate`.
- The block starts on its own line after the instruction (instruction, `"\n"`, block). Each instruction ends in a colon that introduces the block.
- The `IgnoreDefectMissingEntries` remedy keeps its explicit list of missing entries.
- Unchanged: every finding `Message`; the `committed-ignore-unverified` finding; `migrate`, `init`, `repair`; both renderers (`appendFindingBlock` in `internal/app/config_diagnostics.go`, `status`'s `writeFinding`); the other `migrate` remedies in `health.go` (`local-metadata-missing`, `docket-worktree-missing`, the legacy and half-migrated findings).
- The sentinel is unexported and terminal; it is never a real defect and nothing outside tests may switch on it.
- Cross-references in comments anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Every Go run that observes a change in outcome uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`).
- Mutation probes restore from a `cp` backup, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`), and each probe must be observed to land (re-run shows FAIL) before it counts.
- Run the whole suite at the build gate through `build.test_command` (docket-build owns that gate; no task runs it).

## Review Focus

- **The zero-value / no-detail path (`IgnoreDefectNone`, and `IgnoreDefectUnreadable` passed straight to `committedIgnoreFinding`).** Expected: both reach `default` and still carry the block. Task 1's variant guard iterates `IgnoreDefectNone` through the sentinel, so both are in its population.
- **A defect added to the enum later.** Expected: covered automatically. Task 1 iterates up to `ignoreDefectSentinel` and asserts a population floor of 8, so a loop that silently iterates nothing (or a sentinel placed mid-enum) reddens.
- **Pasting the block from `repository check` text output.** Expected: every block line at column 0 (in `.gitignore`, leading whitespace is part of the pattern). Task 2's render guard asserts `"\n"` + trimmed block verbatim in `HumanText`, built from a real `EvaluateHealth` finding, not a hand-built `Finding`.
- **Cases collapsing to one generic instruction.** Expected: each case keeps its own instruction. Task 1 asserts at least 7 distinct instruction prefixes (None and Unreadable legitimately share `default`'s).
- **The `MissingEntries` list getting dropped while the block is appended.** Expected: the remedy still names each missing entry. Task 1 asserts the fixture's entry appears in the instruction prefix (before the block, so the block's own copy of the entry cannot satisfy it).

---

### Task 1: Remedies carry the canonical block, guarded over every `IgnoreDefect`

**Files:**
- Modify: `internal/reposetup/gitignore.go` (the `IgnoreDefect` const block: add a terminal unexported sentinel after `IgnoreDefectUnreadable`)
- Modify: `internal/reposetup/health.go` (`committedIgnoreFinding`: route every remedy through a new helper; add the helper next to it)
- Test: `internal/reposetup/health_test.go` (append `TestCommittedIgnoreRemediesCarryPasteReadyBlock`)

**Interfaces:**
- Consumes: `GitignoreBlock() []byte`, `GitignoreStart` (both in `internal/reposetup/gitignore.go`); `committedIgnoreFinding(d IgnoreDetail) *Finding` (unexported, `health.go`).
- Produces: `ignoreDefectSentinel IgnoreDefect` (unexported, terminal); `withCanonicalBlock(instruction string) string` (unexported, `health.go`). Remedy shape for every `committed-ignore-invalid` finding: `<instruction ending ":">` + `"\n"` + `strings.TrimSuffix(string(GitignoreBlock()), "\n")`. Task 2 relies on this shape.

- [ ] **Step 1: Add the sentinel to the enum**

In `internal/reposetup/gitignore.go`, extend the const block so it reads:

```go
const (
	IgnoreDefectNone             IgnoreDefect = iota
	IgnoreDefectFileAbsent                    // the committed .gitignore file does not exist
	IgnoreDefectBlockAbsent                   // file readable, no current-generation managed block
	IgnoreDefectLegacyOnly                    // only the legacy 0051 markers are present
	IgnoreDefectMalformedMarkers              // dangling / out-of-order / nested markers
	IgnoreDefectMissingEntries                // well-formed block lacking canonical entries
	IgnoreDefectNonCanonical                  // all entries present but not the exact canonical bytes
	IgnoreDefectUnreadable                    // the committed blob could not be read

	// ignoreDefectSentinel is the terminal count marker, never a real
	// defect: tests iterate IgnoreDefectNone up to it so a defect added
	// above is covered automatically (change 0500). Keep it last.
	ignoreDefectSentinel
)
```

- [ ] **Step 2: Write the failing variant guard**

Append to `internal/reposetup/health_test.go` (`strings` and `testing` are already imported):

```go
// TestCommittedIgnoreRemediesCarryPasteReadyBlock pins change 0500 over every
// IgnoreDefect committedIgnoreFinding can receive — derived by iterating up to
// the terminal ignoreDefectSentinel, never hand-listed, so a defect added later
// is covered. Each remedy must end with the canonical block (from
// GitignoreBlock(), trailing newline trimmed) on its own line after a
// colon-terminated instruction, and none may name `repository migrate`, which is
// a no-op on a migrated repository.
//
// Mutation probes (each must redden this test): restore the old
// IgnoreDefectFileAbsent remedy text; drop the withCanonicalBlock call from any
// one case; make withCanonicalBlock indent the block lines; make it join the
// instruction and block with a space instead of a newline.
func TestCommittedIgnoreRemediesCarryPasteReadyBlock(t *testing.T) {
	block := strings.TrimSuffix(string(GitignoreBlock()), "\n")
	if !strings.HasPrefix(block, GitignoreStart+"\n") {
		t.Fatalf("canonical block does not open with the start marker line: %q", block)
	}
	const missing = ".opencode/agents/docket-*.md"
	instructions := map[string]bool{}
	n := 0
	for d := IgnoreDefectNone; d < ignoreDefectSentinel; d++ {
		n++
		detail := IgnoreDetail{Defect: d}
		switch d {
		case IgnoreDefectMalformedMarkers:
			detail.Generation = "docket"
		case IgnoreDefectMissingEntries:
			detail.MissingEntries = []string{missing}
		}
		fnd := committedIgnoreFinding(detail)
		if fnd == nil || fnd.Code != "committed-ignore-invalid" {
			t.Fatalf("defect %d: want a committed-ignore-invalid finding, got %+v", d, fnd)
		}
		r := fnd.Remedy
		// (a)+(b): the block closes the remedy and starts on its own line.
		if !strings.HasSuffix(r, "\n"+block) {
			t.Errorf("defect %d: remedy must end with a newline then the canonical block verbatim:\n%s", d, r)
			continue
		}
		// (c): never send the reader to migrate.
		if strings.Contains(r, "repository migrate") {
			t.Errorf("defect %d: remedy names `repository migrate`: %q", d, r)
		}
		instr := strings.TrimSuffix(r, "\n"+block)
		if instr == "" || strings.Contains(instr, "\n") || !strings.HasSuffix(instr, ":") {
			t.Errorf("defect %d: instruction must be one non-empty line ending in a colon: %q", d, instr)
		}
		if d == IgnoreDefectMissingEntries && !strings.Contains(instr, missing) {
			t.Errorf("missing-entries instruction must still list %s: %q", missing, instr)
		}
		instructions[instr] = true
	}
	// Population floor (marker-scoped-guard-needs-a-population-floor):
	// IgnoreDefectNone through IgnoreDefectUnreadable exist today.
	if n < 8 {
		t.Fatalf("population floor: iterated %d defects, want >= 8", n)
	}
	// None and Unreadable share default's instruction; the six detailed cases
	// each keep their own.
	if len(instructions) < 7 {
		t.Errorf("want >= 7 distinct instructions, got %d: %v", len(instructions), instructions)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names && go test ./internal/reposetup -run TestCommittedIgnoreRemediesCarryPasteReadyBlock -count=1`
Expected: FAIL — every defect reports "remedy must end with a newline then the canonical block verbatim" (it compiles, because Step 1 added the sentinel).

- [ ] **Step 4: Add the helper and rewrite the remedies**

In `internal/reposetup/health.go`, add directly above `committedIgnoreFinding` (`strings` is already imported):

```go
// withCanonicalBlock appends the paste-ready canonical managed block to a
// remedy instruction: the instruction, a newline, then GitignoreBlock() with
// its trailing newline trimmed. The bytes come only from GitignoreBlock() —
// the block init writes and check validates — never a second literal. The
// block starts on its own line so a renderer that prints the remedy verbatim
// (repository check's appendFindingBlock) puts every line at column 0, which a
// .gitignore needs: leading whitespace is part of the pattern (change 0500).
func withCanonicalBlock(instruction string) string {
	return instruction + "\n" + strings.TrimSuffix(string(GitignoreBlock()), "\n")
}
```

Then replace only the `fnd.Remedy = ...` assignments inside `committedIgnoreFinding` (leave every `fnd.Message` and the `default` comment untouched):

```go
	case IgnoreDefectFileAbsent:
		fnd.Remedy = withCanonicalBlock("Add a .gitignore containing exactly these lines, then review, commit, and push it:")
	case IgnoreDefectBlockAbsent:
		fnd.Remedy = withCanonicalBlock("Append exactly these lines to the .gitignore, then review, commit, and push it:")
	case IgnoreDefectLegacyOnly:
		fnd.Remedy = withCanonicalBlock("Replace the legacy managed block with exactly these lines, then review, commit, and push the corrected .gitignore:")
	case IgnoreDefectMalformedMarkers:
		fnd.Remedy = withCanonicalBlock("Inspect and correct the reported marker structure by hand first; the finished managed block must be exactly these lines. Then review, commit, and push the corrected .gitignore:")
	case IgnoreDefectMissingEntries:
		fnd.Remedy = withCanonicalBlock("Restore the missing entries (" + strings.Join(d.MissingEntries, ", ") + ") so the managed block is exactly these lines, then review, commit, and push the corrected .gitignore:")
	case IgnoreDefectNonCanonical:
		fnd.Remedy = withCanonicalBlock("Rewrite the managed block to exactly these lines, then review, commit, and push the corrected .gitignore:")
	default:
		fnd.Remedy = withCanonicalBlock("Restore the managed block as exactly these lines, then review, commit, and push the corrected .gitignore:")
```

(Each `case` keeps its existing `fnd.Message = ...` line above the new `fnd.Remedy` line.) Also update the doc comment above `committedIgnoreFinding` if it describes the remedy wording, so it says every remedy ends with the canonical block via `withCanonicalBlock`.

- [ ] **Step 5: Run the guard and the package to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names && go test ./internal/reposetup -count=1`
Expected: PASS (including the existing `TestHealthTerminalFallbackNamesEveryCause`, whose `strings.Contains(fn.Remedy, ".opencode/agents/docket-*.md")` still holds).

- [ ] **Step 6: Mutation-probe the guard (four probes, restore from backup each time)**

```bash
cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names
cp internal/reposetup/health.go "${TMPDIR:-/tmp}/health.go.0500bak"
```

For each probe: apply the edit, confirm with `git diff --stat internal/reposetup/health.go` that it landed, run `go test ./internal/reposetup -run TestCommittedIgnoreRemediesCarryPasteReadyBlock -count=1`, observe FAIL, then `cp -f "${TMPDIR:-/tmp}/health.go.0500bak" internal/reposetup/health.go`.

1. Restore the old FileAbsent remedy: set that case's remedy line back to the original, verbatim:

   ```go
   		fnd.Remedy = "Restore the managed block (e.g. re-run `docket repository migrate`, or add it by hand from the canonical block), review, commit, and push the corrected .gitignore."
   ```
2. Drop the helper from one case: change the `IgnoreDefectNonCanonical` line to `fnd.Remedy = "Rewrite the managed block to exactly these lines, then review, commit, and push the corrected .gitignore:"`.
3. Indent the block: in `withCanonicalBlock`, return `instruction + "\n" + strings.ReplaceAll(strings.TrimSuffix(string(GitignoreBlock()), "\n"), "\n", "\n  ")`.
4. Space join: in `withCanonicalBlock`, replace `"\n"` with `" "` between instruction and block.

After the last restore, run `cmp internal/reposetup/health.go "${TMPDIR:-/tmp}/health.go.0500bak"` (expect no output) and re-run Step 5 (expect PASS). If any probe stays green, stop and fix the guard before committing.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names
git add internal/reposetup/gitignore.go internal/reposetup/health.go internal/reposetup/health_test.go && git commit -m "fix(0500): committed-ignore-invalid remedies print the paste-ready managed block"
```

---

### Task 2: `repository check` renders the block at column 0

**Files:**
- Test: `internal/app/repository_check_test.go` (append `TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft`)

**Interfaces:**
- Consumes: Task 1's remedy shape; `healthyConfigureFacts() reposetup.Facts` (unexported, `internal/app/repository_configure_tests_test.go`, same package); `reposetup.Classify`, `reposetup.EvaluateHealth`, `reposetup.GitignoreBlock`, `reposetup.IgnoreDetail`, `reposetup.IgnoreDefectFileAbsent`, `reposetup.PresenceAbsent`; `newCheckResult(cls, facts, findings) RepositoryCheckResult` and its `HumanText()`.
- Produces: nothing later tasks use.

- [ ] **Step 1: Write the render guard**

Append to `internal/app/repository_check_test.go` (`strings`, `testing`, and `reposetup` are already imported):

```go
// TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft pins that `repository
// check`'s human text prints the committed-ignore-invalid remedy's canonical
// block with every line at column 0 — paste-ready, since leading whitespace is
// part of a .gitignore pattern (change 0500). The finding comes from the real
// EvaluateHealth pipeline over facts carrying an IgnoreDefectFileAbsent detail,
// not a hand-built Finding.
//
// Mutation probes (each must redden this test): make reposetup's
// withCanonicalBlock indent the block lines, or join instruction and block with
// a space; make appendFindingBlock indent remedy continuation lines.
func TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft(t *testing.T) {
	facts := healthyConfigureFacts()
	facts.CommittedIgnoreBlock = reposetup.PresenceAbsent
	facts.CommittedIgnoreDetail = reposetup.IgnoreDetail{Defect: reposetup.IgnoreDefectFileAbsent}
	cls := reposetup.Classify(facts)
	findings := reposetup.EvaluateHealth(cls, facts, nil)
	found := false
	for _, f := range findings {
		if f.Code == "committed-ignore-invalid" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture produced no committed-ignore-invalid finding (state %q, codes %v)", cls.State, findings)
	}
	block := strings.TrimSuffix(string(reposetup.GitignoreBlock()), "\n")
	text := newCheckResult(cls, facts, findings).HumanText()
	if !strings.Contains(text, "\n"+block) {
		t.Fatalf("check text must carry the canonical block flush left (newline, then the block verbatim):\n%s", text)
	}
}
```

- [ ] **Step 2: Run it to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names && go test ./internal/app -run TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft -count=1`
Expected: PASS (Task 1 already produces the shape). If it fails with "fixture produced no committed-ignore-invalid finding", inspect `cls` and adjust only the fixture facts, not production code.

- [ ] **Step 3: Mutation-probe the render guard (restore from backup each time)**

```bash
cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names
cp internal/reposetup/health.go "${TMPDIR:-/tmp}/health.go.0500bak2"
cp internal/app/config_diagnostics.go "${TMPDIR:-/tmp}/config_diagnostics.go.0500bak"
```

For each probe: apply the edit, confirm it landed with `git diff --stat`, run the Step 2 command, observe FAIL, restore with `cp -f` from the matching backup.

1. In `withCanonicalBlock` (`internal/reposetup/health.go`), indent the block lines: `strings.ReplaceAll(strings.TrimSuffix(string(GitignoreBlock()), "\n"), "\n", "\n  ")`.
2. In `withCanonicalBlock`, join instruction and block with `" "` instead of `"\n"`.
3. In `appendFindingBlock` (`internal/app/config_diagnostics.go`), write `strings.ReplaceAll(f.Remedy, "\n", "\n    ")` in place of `f.Remedy` in the remedy `Fprintf`.

After the last restore, `cmp` both files against their backups (expect no output) and re-run Step 2 (expect PASS). Also run `go test ./internal/app -run 'TestRepositoryCheck|TestRefusalAndCheckShareFindingRenderer|TestAppendConfigFindingBlock' -count=1` (expect PASS).

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/committed-ignore-invalid-hint-for-an-absent-gitignore-names
git add internal/app/repository_check_test.go && git commit -m "test(0500): repository check prints the committed-ignore block flush left"
```

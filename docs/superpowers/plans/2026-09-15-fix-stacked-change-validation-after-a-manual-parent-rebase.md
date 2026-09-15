<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0429 — Fix stacked-change validation after a manual parent rebase](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-15-0429-fix-stacked-change-validation-after-a-manual-parent-rebase.md)**
<!-- docket:backlink:end -->
# Fix Stacked-Change Validation After a Manual Parent Rebase — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop two checks from falsely rejecting an otherwise valid stack after its parent branch is manually rebased: the ready-workspace original-base ancestry requirement (three mirrored copies) and `ProvePreserved`'s tip-only exact-content fallback.

**Architecture:** Two independent corrections. (1) In `internal/workspace`, ready-phase verification stops requiring the recorded creation base to remain an ancestor of the current head — in inspection, publication verification, and cleanup verification — while keeping every manifest-ownership, registration, ref/head-identity, clean-tree, and allocating-phase check. (2) In `internal/gitcli`, `ProvePreserved` gains one more proof path: when the child's exact delta does not reproduce at the target tip, it may reproduce completely at a single commit reachable in `base..target`; one commit matching every delta entry proves historical inclusion. All app-layer callers (rebase, publish, merge, stacked/root closeout) inherit both corrections through the existing call sites — no caller wiring changes.

**Tech Stack:** Go; real-Git integration tests via the existing `internal/gitcli` and `internal/workspace` test harnesses; whole-suite gate via the config-resolved `build.test_command` (the Go runner `internal/suiterunner` is the sole channel).

**Spec:** `docs/superpowers/specs/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-design.md` (synchronized on the `docket` metadata branch; the change file is `docs/changes/active/0429-fix-stacked-change-validation-after-a-manual-parent-rebase.md` there).

## Global Constraints

- Never combine preservation-proof matches from entries spread across different commits — the complete `B → source` delta must match at **one** commit (spec §Child preservation, rule 3).
- Compute the `B → source` delta **once**; B is the existing sole merge base of source and target. Zero or multiple bases keep their existing unproven refusals.
- Keep original-base ancestry protection for **allocating** (unfinished) workspaces; only the **ready** phase drops the requirement (spec §Workspace check).
- Keep all existing manifest ownership, recorded refs, registration, clean-tree, detached-HEAD, and local/remote/PR head checks; leave the recorded `BaseCommit` unchanged — no rebinding or synchronization command.
- Git observation errors remain errors, never verdicts; uncertainty remains unproven, never proven.
- For root closeout the preservation target remains the verified root merge-result commit (unchanged call sites), so later integration history cannot supply the proof.
- No new configuration, no persistence, no force/skip-proof overrides, no synthetic merge commits, no automatic restacking.
- Mutation-test the new same-snapshot/history restriction (repo rule: a guard is code). Defeat Go's test cache on every mutation probe: `go test -count=1`.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- The BUILD gate runs whatever `build.test_command` resolves to — read it from config, never a second copy — and read the budget report even on green.
- Update only comments/docs describing the changed checks; no unrelated refactoring.

**Learnings applied (from the learnings ledger):** `duplicated-gate-copies-the-whole-predicate` (the three workspace sites are one predicate mirrored — change all three identically, and diff them first); `test-premise-deleted-not-regated` (existing base-unreachable tests guard against out-of-band branch moves — re-target what they guard, do not just delete); `fix-reintroduces-its-own-defect-class` (the untouched twin is the allocating-phase ancestry check — verify it survives); `cached-runner-serves-a-mutated-tree` (`-count=1` on every mutation probe); `assert-detects-removal-not-replacement` (prove each mutation actually landed before believing the redden).

---

### Task 1: `gitcli` plumbing — list the candidate commits of `base..target`

**Files:**
- Modify: `internal/gitcli/preservecommit.go` (add the op label and helper beside `mergeBasesAll`)
- Test: `internal/gitcli/preservecommit_integration_test.go`

**Interfaces:**
- Consumes: `(*Client).run`, `runRequest`, `stdoutLines`, `validateObjectID`, `newFailure`, `stderrExcerpt` — all existing file-local/package plumbing.
- Produces: `func (c *Client) commitRange(ctx context.Context, repo Repository, base, target ObjectID) ([]ObjectID, *Failure)` — every commit reachable from `target` and not from `base` (`git rev-list target ^base`), newest-first, `target` itself included when it is in the range. Task 3 consumes this exact signature.

- [ ] **Step 1: Write the failing integration test**

Add to `internal/gitcli/preservecommit_integration_test.go` (reuse the file's existing helpers `newRealClient`, `historyRepo`, `commitFile`, `gitOut`):

```go
// TestIntegrationPreserveCommitRange proves commitRange lists exactly the
// commits of base..target (newest-first, target included, base excluded), that
// an empty range is empty output and no failure, and that an unresolvable
// operand is a typed command failure — never a silent empty answer.
func TestIntegrationPreserveCommitRange(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)
	base := commitFile(t, dir, "a.txt", "a\n", "c0")
	mid := commitFile(t, dir, "b.txt", "b\n", "c1")
	tip := commitFile(t, dir, "c.txt", "c\n", "c2")

	got, f := c.commitRange(ctx, repo, base, tip)
	if f != nil {
		t.Fatalf("commitRange: %v", f)
	}
	want := []ObjectID{tip, mid}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("commitRange = %v; want %v", got, want)
	}

	// Empty range: base..base is no commits and no failure.
	empty, f := c.commitRange(ctx, repo, base, base)
	if f != nil {
		t.Fatalf("commitRange(empty): %v", f)
	}
	if len(empty) != 0 {
		t.Fatalf("commitRange(empty) = %v; want none", empty)
	}

	// An absent operand is a typed failure, never an empty answer.
	absent := ObjectID(strings.Repeat("1", 40))
	if _, f := c.commitRange(ctx, repo, base, absent); f == nil {
		t.Fatalf("commitRange(absent target): want a typed failure, got nil")
	}
}
```

(`strings` is already imported by the integration test file; if not, add it.)

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -run TestIntegrationPreserveCommitRange -count=1`
Expected: FAIL to compile — `c.commitRange undefined`.

- [ ] **Step 3: Implement `commitRange`**

In `internal/gitcli/preservecommit.go`, extend the op-label const block and add the helper after `mergeBasesAll`:

```go
const (
	preserveOp     Operation = "preserve-proof"
	mergeBasesOp   Operation = "merge-bases"
	sourceDeltaOp  Operation = "source-delta"
	rangeCommitsOp Operation = "range-commits"
)
```

```go
// commitRange lists every commit reachable from target and not from base
// (`git rev-list target ^base`), newest-first. These are the candidate
// snapshots a historic-content preservation proof may inspect: history the
// target line added on top of the shared base. An empty range is an empty
// slice and no failure; a nonzero exit is a typed command failure — an
// unobservable operand is never read as "no candidates". Malformed plumbing
// output is invalid-output.
func (c *Client) commitRange(ctx context.Context, repo Repository, base, target ObjectID) ([]ObjectID, *Failure) {
	for _, id := range []ObjectID{base, target} {
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(rangeCommitsOp, KindInvalidRequest, "invalid commit id", err)
		}
	}
	res, f := c.run(ctx, runRequest{
		op:   rangeCommitsOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"rev-list", string(target), "^" + string(base)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(rangeCommitsOp, KindCommandFailed, "rev-list failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	var out []ObjectID
	for _, line := range stdoutLines(res.stdout) {
		id := ObjectID(line)
		if err := validateObjectID(id); err != nil {
			return nil, newFailure(rangeCommitsOp, KindInvalidOutput, "malformed rev-list output", err)
		}
		out = append(out, id)
	}
	return out, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -run TestIntegrationPreserveCommitRange -count=1`
Expected: PASS.

- [ ] **Step 5: Run the package to confirm nothing else moved**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/gitcli/preservecommit.go internal/gitcli/preservecommit_integration_test.go
git commit -m "feat(0429): commitRange lists base..target candidate snapshots for preservation proofs"
```

---

### Task 2: `gitcli` — extract the per-commit delta comparison (behavior-neutral refactor)

**Files:**
- Modify: `internal/gitcli/preservecommit.go` (`ProvePreserved` tail)

**Interfaces:**
- Consumes: `TreeEntryIDs`, `deltaEntry`, `maxDifferingPaths` — existing.
- Produces: `func (c *Client) deltaMatchesAt(ctx context.Context, repo Repository, commit ObjectID, delta []deltaEntry) (bool, []RepoPath, error)` — true when every entry of `delta` reproduces exactly at `commit` (oid+mode equal; a `'D'` entry absent); the `[]RepoPath` is the bounded (`maxDifferingPaths`) differing diagnostic; an observation error is an error, never a verdict. Task 3 consumes this exact signature. `ProvePreserved`'s observable behavior is unchanged by this task.

- [ ] **Step 1: Extract the helper**

In `internal/gitcli/preservecommit.go`, move the tip-comparison block of `ProvePreserved` (from the `paths := make(...)` line through the `mismatched` loop) into:

```go
// deltaMatchesAt reports whether the complete base->source delta reproduces
// exactly in the tree of one commit: every changed entry present at the same
// oid and mode, and every source deletion absent. It is the single comparison
// both the target-tip proof and the historic-snapshot fallback use, so the two
// proof paths can never diverge in what "exact" means. The returned paths are
// the bounded differing diagnostic (<= maxDifferingPaths); the cap never
// shortens the verdict — a mismatch past the cap still returns false. An
// observation error is an error, never a verdict.
func (c *Client) deltaMatchesAt(ctx context.Context, repo Repository, commit ObjectID, delta []deltaEntry) (bool, []RepoPath, error) {
	paths := make([]RepoPath, 0, len(delta))
	for _, d := range delta {
		paths = append(paths, d.Path)
	}
	entries, err := c.TreeEntryIDs(ctx, repo, commit, paths)
	if err != nil {
		return false, nil, err
	}
	// A directory-valued path (a file->directory transition) yields a single
	// `tree` entry whose mode 040000 can never equal a blob's NewMode, so the
	// comparison below stays conservative for it.
	at := make(map[RepoPath]TreeEntry, len(entries))
	for _, e := range entries {
		at[e.Path] = e
	}
	// Track "a mismatch exists" separately from the bounded diagnostic slice: the
	// path cap must never shorten the verdict.
	mismatched := false
	var differing []RepoPath
	for _, d := range delta {
		e, present := at[d.Path]
		match := false
		if d.Status == 'D' {
			match = !present // a source deletion requires absence
		} else {
			match = present && e.ObjectID == d.NewOID && e.Mode == d.NewMode
		}
		if !match {
			mismatched = true
			if len(differing) < maxDifferingPaths {
				differing = append(differing, d.Path)
			}
		}
	}
	return !mismatched, differing, nil
}
```

Rewrite the tail of `ProvePreserved` (after the `len(delta) == 0` refusal) to:

```go
	ok, differing, err := c.deltaMatchesAt(ctx, repo, target, delta)
	if err != nil {
		return PreservationCheck{}, err
	}
	if !ok {
		return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEntryDiffers, Paths: differing}, nil
	}
	return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByContent}, nil
```

Delete the now-inlined comparison code from `ProvePreserved` (the two comments about directory-valued paths and the path cap move into the helper as shown; do not leave duplicates behind).

- [ ] **Step 2: Run the package tests to prove behavior-neutrality**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -count=1`
Expected: PASS — every existing `TestIntegrationPreserveCommitProve` row (ancestral, rebase-rewrite, squash-rewrite, unrelated-destination-advance, content-dropped, partial-loss, overlapping-edit, deletion rows) is unchanged.

- [ ] **Step 3: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/gitcli/preservecommit.go
git commit -m "refactor(0429): extract deltaMatchesAt as the single exact-content comparison"
```

---

### Task 3: `gitcli` — historic-snapshot fallback in `ProvePreserved`

**Files:**
- Modify: `internal/gitcli/preservecommit.go` (`ProvePreserved`, `PreservationKind` consts)
- Test: `internal/gitcli/preservecommit_integration_test.go`

**Interfaces:**
- Consumes: `commitRange` (Task 1), `deltaMatchesAt` (Task 2).
- Produces: new proven kind `PreservationByHistoricContent PreservationKind = "exact-content-history"`. `ProvePreserved`'s signature and every unproven detail token are unchanged; callers that branch on `Outcome != PreservationProven` (both `internal/app` sites) need no change.

- [ ] **Step 1: Write the failing integration tests**

Add to `internal/gitcli/preservecommit_integration_test.go`, following the `TestIntegrationPreserveCommitProve` house style (`historyRepo`, `commitFile`, `commitAll`, `writeWorktreeFile`, `gitOut`, `assertProof`):

```go
// TestIntegrationPreserveCommitProveHistoricSnapshot covers the reported
// regression: a parent rebase rewrites a child's inclusion so its exact
// snapshot survives MID-history while later children evolve the same files.
// The proof must accept one complete historical snapshot and refuse every
// shape that lacks one.
func TestIntegrationPreserveCommitProveHistoricSnapshot(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)

	// Row 1 (regression shape 2): the child's delta reproduces exactly at a
	// mid-history commit of base..target; a LATER commit on the target line
	// edits the shared file -> proven, exact-content-history.
	t.Run("later-edit-to-shared-file", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, "Chart.yaml", "version: 1\n")
		writeWorktreeFile(t, dir, "values.yaml", "a: 1\n")
		source := commitAll(t, dir, "child adds chart+values")
		// Rebuild the stack line: the child's exact snapshot lands mid-history…
		gitOut(t, dir, "checkout", "-q", "-b", "stack", string(b0))
		writeWorktreeFile(t, dir, "Chart.yaml", "version: 1\n")
		writeWorktreeFile(t, dir, "values.yaml", "a: 1\n")
		commitAll(t, dir, "rebased child snapshot")
		// …and a later child evolves the shared files past it.
		writeWorktreeFile(t, dir, "Chart.yaml", "version: 2\n")
		target := commitAll(t, dir, "later child bumps chart")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByHistoricContent, "")
	})

	// Row 2 (same-snapshot restriction): each delta entry matches at SOME
	// commit of base..target, but never both at one commit -> unproven; a
	// proof assembled from entries spread across commits is forbidden.
	t.Run("entries-split-across-commits", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, "f1.txt", "one\n")
		writeWorktreeFile(t, dir, "f2.txt", "two\n")
		source := commitAll(t, dir, "child adds f1+f2")
		gitOut(t, dir, "checkout", "-q", "-b", "stack", string(b0))
		// C1 holds f1 exact but f2 wrong; C2 fixes f2 but rewrites f1.
		writeWorktreeFile(t, dir, "f1.txt", "one\n")
		writeWorktreeFile(t, dir, "f2.txt", "TWO-edited\n")
		commitAll(t, dir, "c1: f1 exact, f2 differs")
		writeWorktreeFile(t, dir, "f1.txt", "ONE-edited\n")
		writeWorktreeFile(t, dir, "f2.txt", "two\n")
		target := commitAll(t, dir, "c2: f2 exact, f1 differs")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "f1.txt")
	})

	// Row 3 (dropped child work still refuses): the rewrite omits the child's
	// file everywhere on the target line -> unproven, entry-differs.
	t.Run("dropped-in-rewrite", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items\n", "child adds catalog")
		gitOut(t, dir, "checkout", "-q", "-b", "stack", string(b0))
		commitFile(t, dir, "other.txt", "other\n", "rewrite without catalog")
		target := commitFile(t, dir, "more.txt", "more\n", "advance further")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "catalog.yaml")
	})

	// Row 4 (history beyond the pinned target cannot supply the proof): the
	// exact snapshot exists only AFTER target on the same line. With target
	// pinned (root closeout pins the verified root merge-result commit), the
	// candidate range base..target excludes it -> unproven.
	t.Run("snapshot-only-after-target", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items\n", "child adds catalog")
		gitOut(t, dir, "checkout", "-q", "-b", "stack", string(b0))
		target := commitFile(t, dir, "other.txt", "other\n", "merge-result without catalog")
		commitFile(t, dir, "catalog.yaml", "items\n", "later integration adds catalog")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "catalog.yaml")
	})

	// Row 5 (unrelated history is not a candidate pool): no common base keeps
	// its existing refusal — the fallback never runs without a sole base.
	t.Run("unrelated-history-unchanged", func(t *testing.T) {
		dir, repo := historyRepo(t)
		source := commitFile(t, dir, "a.txt", "a\n", "line A")
		gitOut(t, dir, "checkout", "-q", "--orphan", "island")
		gitOut(t, dir, "rm", "-rf", "--cached", ".")
		writeWorktreeFile(t, dir, "b.txt", "b\n")
		target := commitAll(t, dir, "unrelated line B")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveNoCommonBase)
	})
}
```

Note on Row 2's expected paths: the retained unproven result is the TARGET-TIP comparison's diagnostic (spec: "Otherwise retain the existing unproven result"), and at the tip only `f1.txt` differs (`f2.txt` matches there), so the diagnostic names `f1.txt`. If `assertProof`'s path assertion is exact-set rather than contains, pass exactly `"f1.txt"` — read the helper (bottom of `preserve_integration_test.go` / `preservecommit_integration_test.go`) before adjusting.

Also check the orphan-branch mechanics of Row 5 against the existing unrelated-history row in `TestIntegrationRepoRevisionConsistencyABAndUnrelatedBranch` (`internal/gitcli/preserve_integration_test.go`) and reuse its exact recipe if it differs.

- [ ] **Step 2: Run the tests to verify they fail correctly**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -run TestIntegrationPreserveCommitProveHistoricSnapshot -count=1`
Expected: compile FAIL on `PreservationByHistoricContent` undefined. After adding only the constant (next step, first hunk), Row 1 FAILS (unproven at tip, no fallback) and Rows 2–5 already PASS — that asymmetry is the point: only Row 1 needs new behavior.

- [ ] **Step 3: Implement the fallback**

In `internal/gitcli/preservecommit.go`:

Add the kind:

```go
	// PreservationByHistoricContent: source is not an ancestor and its delta
	// does not reproduce at the target tip, but the COMPLETE base->source delta
	// reproduces exactly at one commit reachable in base..target — the child was
	// included and later stacked work legitimately evolved shared files. Later
	// edits do not invalidate an already-included child.
	PreservationByHistoricContent PreservationKind = "exact-content-history"
```

Rewrite `ProvePreserved`'s tail (the Task 2 shape) to:

```go
	ok, differing, err := c.deltaMatchesAt(ctx, repo, target, delta)
	if err != nil {
		return PreservationCheck{}, err
	}
	if ok {
		return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByContent}, nil
	}
	// Historic-snapshot fallback: the tip does not reproduce the delta, but ONE
	// commit the target line added on top of the sole base may — a parent rebase
	// re-includes the child mid-history and later children evolve shared files.
	// The complete delta must match at a single commit; a proof assembled from
	// entries spread across different commits is never accepted. The candidate
	// pool is bounded to base..target so history beyond the pinned target (for
	// root closeout, the verified root merge-result commit) can never supply the
	// proof. On no match, the TIP comparison's diagnostic is returned unchanged.
	candidates, f := c.commitRange(ctx, repo, bases[0], target)
	if f != nil {
		return PreservationCheck{}, f
	}
	for _, cand := range candidates {
		if cand == target {
			continue // already compared above
		}
		histOK, _, err := c.deltaMatchesAt(ctx, repo, cand, delta)
		if err != nil {
			return PreservationCheck{}, err
		}
		if histOK {
			return PreservationCheck{Outcome: PreservationProven, Kind: PreservationByHistoricContent}, nil
		}
	}
	return PreservationCheck{Outcome: PreservationUnproven, Detail: PreserveEntryDiffers, Paths: differing}, nil
```

Update `ProvePreserved`'s doc comment to describe the third proof path in the same register, e.g. extend the existing sentence: "…must reproduce exactly in target — or, failing the tip, at ONE commit reachable in base..target (a parent rebase's historical inclusion; the complete delta at a single commit, never assembled across commits)."

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ -run 'TestIntegrationPreserveCommit|TestIntegrationRepo' -count=1`
Expected: PASS — new rows and all pre-existing preservation rows.

- [ ] **Step 5: Mutation-test the same-snapshot restriction**

Two probes; restore the file after each from your uncommitted edit (copy the file aside first — `git checkout --` would destroy the Task 3 work, per the `mutation-restore-needs-a-backup-copy` learning):

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
cp internal/gitcli/preservecommit.go "${TMPDIR:-/tmp}/preservecommit.go.keep.$$"  # plain backup copy; note the exact name for the restore
```

1. **Cross-commit assembly mutation:** in the fallback loop, accumulate per-entry matches across candidates instead of requiring one commit (e.g. track a `matchedPaths map[RepoPath]bool` filled from each candidate's per-entry comparison and prove when all paths are eventually matched — the cheapest way: change `deltaMatchesAt`'s use so a candidate marks its matching entries and the loop proves on full coverage). Run `go test ./internal/gitcli/ -run TestIntegrationPreserveCommitProveHistoricSnapshot -count=1`. Expected: Row 2 (`entries-split-across-commits`) FAILS. If it stays green the restriction is decoration — stop and fix the test.
2. **Range-bound mutation:** change `commitRange`'s args from `{"rev-list", string(target), "^" + string(base)}` to `{"rev-list", "--all", "^" + string(base)}`. Run the same command. Expected: Row 4 (`snapshot-only-after-target`) FAILS.

Restore the original file (`cp` the kept copy back), re-run the suite slice green, and record both probes (mutation → which test reddened) in the commit message body.

- [ ] **Step 6: Run the whole gitcli and app packages**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/gitcli/ ./internal/app/ -count=1`
Expected: PASS (app callers branch on `Outcome` only; the new kind changes no caller).

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/gitcli/preservecommit.go internal/gitcli/preservecommit_integration_test.go
git commit -m "feat(0429): ProvePreserved accepts a complete historic snapshot in base..target" \
  -m "Mutation probes: cross-commit assembly reddened entries-split-across-commits; --all range widening reddened snapshot-only-after-target."
```

---

### Task 4: workspace inspection — a ready workspace's classification stops requiring original-base ancestry

**Files:**
- Modify: `internal/workspace/inspect.go` (`classifyState` `PhaseReady` arm, `StateBranchGone` comment, file-header prose)
- Test: `internal/workspace/inspect_test.go`

**Interfaces:**
- Consumes: existing harness (`mainModeRepo`, `freshTarget`, `prepareOK`, `assertInspectReadOnly`, `gitOut`, `wsPathOf`, `metaDirOf`, `loadManifest`, `writeManifest`).
- Produces: for `PhaseReady`, `Inspection.BaseReached` remains a recorded FACT (the probe still runs) but no longer decides `Kind`: a registered, on-ref, tip-matching, clean workspace whose head does not reach the recorded base is `StateReady` with `BaseReached == false` (previously `StateBranchGone`). `PhaseAllocating`'s ancestry gate is untouched. Tasks 5–6 mirror the same predicate change at their sites.

- [ ] **Step 1: Write the failing test**

Add to `internal/workspace/inspect_test.go`:

```go
// TestInspectReadyAfterParentRebase proves a READY workspace whose branch was
// legitimately rebased — head no longer reaches the recorded creation base —
// still classifies ready: the recorded base is a fact (BaseReached=false), not
// an identity requirement. The allocating-phase ancestry protection and every
// registration/ref/head/clean check are unchanged (change 0429).
func TestInspectReadyAfterParentRebase(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	// Rewrite the recorded base to a commit the feature head does not reach (a
	// later origin-main commit, fetched into the object store) — the same
	// construction cleanup_test's moved-head row uses, now a legitimate state.
	c1 := r.advanceMain(t)
	gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present {
		t.Fatalf("loadManifest present=%v err=%v", present, err)
	}
	m.BaseCommit = c1
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest(rewritten base): %v", err)
	}

	insp := assertInspectReadOnly(t, svc, r, repo, tgt)
	if insp.Kind != StateReady {
		t.Errorf("Kind = %q; want ready after a parent rebase", insp.Kind)
	}
	if insp.BaseReached {
		t.Errorf("BaseReached = true; want false (fact recorded, not gated)")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -run TestInspectReadyAfterParentRebase -count=1`
Expected: FAIL — `Kind = "branch-missing"; want ready`.

- [ ] **Step 3: Implement**

In `internal/workspace/inspect.go`, `classifyState`, `case PhaseReady:` — keep the ancestry probe but stop classifying on it. Replace:

```go
		reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
		if err != nil {
			return mapGitFailure(inspectOp, "inventory", err)
		}
		insp.BaseReached = reachable
		if !reachable {
			// A ready worktree whose head no longer reaches the recorded base has a
			// moved branch.
			insp.Kind = StateBranchGone
			return nil
		}
```

with:

```go
		reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
		if err != nil {
			return mapGitFailure(inspectOp, "inventory", err)
		}
		// Recorded-base ancestry is a FACT for a ready workspace, never an
		// identity requirement: a manual parent rebase legitimately rewrites the
		// creation base out of the head's ancestry while the manifest, ref,
		// registration, and head still prove this is the owned workspace (change
		// 0429). The allocating phase above still requires it — an unfinished
		// allocation with a rewritten branch is not safely resumable.
		insp.BaseReached = reachable
```

Update the two stale comments: the `StateBranchGone` const comment becomes `// feature ref missing` (drop "or head no longer reaches base"), and adjust the file-header sentence "recorded base ancestry" phrasing only if it now misstates the gate (it still reads ancestry — as a fact — so it may stand; verify by reading it).

- [ ] **Step 4: Run the tests**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -count=1`
Expected: new test PASS; `TestInspectReady` (asserts `BaseReached=true` on a healthy ready workspace) still PASS; `TestInspectBranchGone` still PASS (its premise is a deleted ref, not a moved base — verify by reading it; if it also has a moved-base sub-case, re-target that sub-case to expect `StateReady`+`BaseReached=false`, because what it guarded — refusing out-of-band moves — is now guarded by the head==ref-tip and clean-tree checks plus the allocating-phase gate, per the `test-premise-deleted-not-regated` learning).

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/workspace/inspect.go internal/workspace/inspect_test.go
git commit -m "fix(0429): ready-workspace inspection records base ancestry as a fact, not an identity gate"
```

---

### Task 5: workspace publication verification — drop the ready-phase ancestry gate

**Files:**
- Modify: `internal/workspace/publish.go` (the verify block; also its step-list header comment naming the checks)
- Test: `internal/workspace/publish_test.go`

**Interfaces:**
- Consumes: harness helpers as in Task 4; the publish test file's existing publish-invoking helper (read the top of `publish_test.go` for its name — e.g. a `publishOK`/direct `svc.Publish` pattern — and reuse it verbatim).
- Produces: publication verification of a ready workspace no longer refuses on `recorded base is not reachable from the head`; every other verify refusal (foreign/unowned manifest, non-ready phase, missing ref, unregistered path, detached/wrong branch, `registered HEAD is not the feature ref tip`, dirty) is byte-for-byte unchanged. This is the second copy of the Task 4 predicate — mirror the whole predicate change, not a variant (per the `duplicated-gate-copies-the-whole-predicate` learning).

- [ ] **Step 1: Write the failing test**

Add to `internal/workspace/publish_test.go` (adapt the invocation to the file's existing house pattern for calling publish and asserting success — read a passing test such as `TestPublishFastForward` first and copy its request/assert shape):

```go
// TestPublishReadyAfterParentRebase proves a clean, registered, on-ref ready
// workspace publishes even when its head no longer reaches the recorded
// creation base (a manual parent rebase). The stale-head, detached, dirty, and
// identity refusals are unchanged (change 0429).
func TestPublishReadyAfterParentRebase(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	c1 := r.advanceMain(t)
	gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present {
		t.Fatalf("loadManifest present=%v err=%v", present, err)
	}
	m.BaseCommit = c1
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest(rewritten base): %v", err)
	}

	// Publish must succeed exactly as it does for an untouched ready workspace.
	// <invoke publish with the file's house helper/request shape and assert the
	// same success facts the nearest green publish test asserts>
}
```

The bracketed invocation is the one piece to lift from the neighboring green test — copy its call and success assertions verbatim; everything else above is complete.

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -run TestPublishReadyAfterParentRebase -count=1`
Expected: FAIL with the refusal Detail `recorded base is not reachable from the head`.

- [ ] **Step 3: Implement**

In `internal/workspace/publish.go`, delete the ancestry block from the ready-verify sequence:

```go
	// The recorded base must still be reachable from the head, or the branch was
	// moved out of band: refuse, never reset.
	reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
	if err != nil {
		return "", mapGitFailure(publishOp, "inventory", err)
	}
	if !reachable {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "recorded base is not reachable from the head"}
	}
```

and update the file's numbered verify-step doc comment (near the top of `publish.go`, the list that enumerates checks 1..7) to drop the base-reachability step, renumbering as needed. Do NOT touch the separate remote fast-forward ancestry check (`IsAncestor(rr.Commit, localHead)` in the push path) — that is divergence protection, not the creation-base gate.

- [ ] **Step 4: Run the tests**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -count=1`
Expected: PASS. If an existing publish test asserted the removed `recorded base is not reachable from the head` refusal, re-target it per the learning `test-premise-deleted-not-regated`: what it guarded (never publish an out-of-band-moved branch) is still held by `registered HEAD is not the feature ref tip` + clean-tree — convert the fixture to assert one of those refusals still fires for a genuinely moved head, or delete it only if that exact protection is already covered by another named test (say which in the commit message).

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/workspace/publish.go internal/workspace/publish_test.go
git commit -m "fix(0429): publication verification drops the ready-phase original-base ancestry gate"
```

---

### Task 6: workspace cleanup verification — drop the ready-phase ancestry gate

**Files:**
- Modify: `internal/workspace/cleanup.go` (`cleanupReady`; its doc comment)
- Test: `internal/workspace/cleanup_test.go`

**Interfaces:**
- Consumes: harness helpers as in Task 4; `cleanupOK`, `containsWorktreePath`, `snapshotTree`, `assertUnchanged` (existing in `cleanup_test.go`).
- Produces: `cleanupReady` no longer blocks on `recorded base is not reachable from the head`; a clean, registered, on-ref, tip-matching ready workspace with a non-ancestral recorded base is removed and tombstoned exactly like a healthy one. Detached, wrong-branch, stale-head (`registered HEAD is not the feature ref tip`), dirty, and non-forcing-removal protections unchanged. Third and final copy of the mirrored predicate.

- [ ] **Step 1: Re-target the existing moved-head test and add the success test**

In `internal/workspace/cleanup_test.go`, the sub-test `t.Run("moved-head-base-unreachable", …)` currently expects `CleanupBlocked` for a rewritten `BaseCommit`. Its premise is inverted by this change (`test-premise-deleted-not-regated`: it guarded "never remove after an out-of-band move" — that protection now lives in the head==ref-tip, detached, and clean checks, which its sibling sub-tests already pin). Rewrite it in place:

```go
	t.Run("moved-head-base-unreachable-removed", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)

		// Rewrite the recorded base to a commit the feature head does not reach —
		// after change 0429 a manual parent rebase is a legitimate ready state, so
		// an otherwise-eligible cleanup proceeds: removed, tombstoned, feature
		// branch preserved.
		c1 := r.advanceMain(t)
		gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
		m, present, err := loadManifest(metaDirOf(repo, tgt))
		if err != nil || !present {
			t.Fatalf("loadManifest present=%v err=%v", present, err)
		}
		m.BaseCommit = c1
		if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
			t.Fatalf("writeManifest(rewritten base): %v", err)
		}
		ws := wsPathOf(repo)

		res := cleanupOK(t, svc, repo, tgt)
		if res.Disposition != CleanupRemoved {
			t.Errorf("Disposition = %q; want removed after a parent rebase", res.Disposition)
		}
		if containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
			t.Errorf("registration still present; want removed")
		}
		if !branchExists(r.Primary, "feat/"+prepSlug) {
			t.Errorf("feat branch deleted; cleanup must preserve the branch")
		}
	})
```

Check the success-disposition constant name and the branch-name/`prepSlug` spelling against `TestCleanupReadyClean` at the top of the file and use exactly what it uses (e.g. if the constant is `CleanupRemoved` vs another spelling, and how it asserts the tombstone) — copy that test's success assertions.

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -run TestCleanup -count=1`
Expected: the rewritten sub-test FAILS with `Disposition = "blocked"`.

- [ ] **Step 3: Implement**

In `internal/workspace/cleanup.go`, `cleanupReady`, delete:

```go
	// The recorded base must still be reachable from the head, or the branch was
	// moved out of band: blocked, never reset.
	reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", err)
	}
	if !reachable {
		return blockedCleanup(m.Path, "recorded base is not reachable from the head"), nil
	}
```

and drop "with the recorded base still reachable" from `cleanupReady`'s doc comment, noting instead that base ancestry is not required for a ready workspace (a manual parent rebase legitimately rewrites it — change 0429) while the ref/head/clean/non-forcing checks still prove nothing is lost.

- [ ] **Step 4: Run the package**

Run: `cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase && go test ./internal/workspace/ -count=1`
Expected: PASS, including the untouched dirty/detached/stale-head/path-mismatch blocked sub-tests (they are the surviving guard for out-of-band moves).

- [ ] **Step 5: Sweep for stragglers of the removed predicate**

Run a whole-repo grep and sort hits into prose vs executable (repo rule — never hand-list gated sites):

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
grep -rn "recorded base is not reachable" internal cmd tests docs || true
grep -rn "IsAncestor(ctx, repo, m.BaseCommit" internal || true
```

Expected after Tasks 4–6: the second grep's only remaining hit is `inspect.go` (the fact probe, plus the allocating-phase `branchHead` variant); the first has no executable hits. Fix anything else found (a test asserting the removed message, a doc restating the gate). The `internal/app` layer maps inspection kinds (`branch_identity.go`'s `errBranchMissing`); confirm by reading `internal/app/workspace_ops.go`'s consumers that no app-layer text still promises the base-ancestry gate — update any comment that does.

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add internal/workspace/cleanup.go internal/workspace/cleanup_test.go
git commit -m "fix(0429): cleanup verification drops the ready-phase original-base ancestry gate"
```

(Include any Step 5 straggler fixes in this commit and name them in the body.)

---

### Task 7: whole-suite gate and evidence

**Files:**
- None created; runs the configured suite.

**Interfaces:**
- Consumes: everything above.
- Produces: a green whole-suite run as build evidence.

- [ ] **Step 1: Run the whole suite via the configured command**

Resolve the BUILD gate's command from config (`build.test_command` — read it, never restate it; the Go runner `internal/suiterunner` is the sole channel, see `tests/README.md`) and run it from the feature worktree. Under docket-build this is the suite gate the build skill drives via `docket gate drive advance` slices — do not background-and-yield.

- [ ] **Step 2: Read the budget report**

Even on green: act on any `SERIAL CONFIRMED OVER BUDGET:` line (authoritative breach); note `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines as screening findings. The new `ProvePreserved` fallback adds one `rev-list` plus one `ls-tree` per candidate commit only on the previously-refusing path (tip mismatch), so no budget movement is expected — if a preservation-touching row moved, say so in the results.

- [ ] **Step 3: Fix-forward anything red, re-run, and commit any fixes**

```bash
cd /Users/homer/dev/docket/.worktrees/fix-stacked-change-validation-after-a-manual-parent-rebase
git add <exact paths of any fix>
git commit -m "test(0429): <what the suite surfaced and why the fix is right>"
```

---

## Self-Review (performed while writing)

- **Spec coverage:** §Workspace check → Tasks 4 (inspection), 5 (publication verification), 6 (cleanup verification); "keep allocating ancestry" → Task 4 Step 3 comment + untouched `PhaseAllocating` arm; §Child preservation rules 1–4 → Tasks 1–3 (rule 1 existing rows re-run in Task 2/3; rule 2 `commitRange` over sole base; rule 3 single-commit match + mutation probe; rule 4 retained tip diagnostic and error-stays-error); "all callers inherit" → no caller edits, Task 3 Step 6 proves `internal/app` green; "root closeout target pinned" → Task 3 Row 4 + fallback comment; §Acceptance shapes → Task 4/5/6 positive tests, Task 3 Rows 1–5, negative coverage retained and re-targeted; "mutation-test the restriction" → Task 3 Step 5; "run configured whole-suite gates, inspect budget" → Task 7.
- **Placeholder scan:** two deliberate read-the-neighbor instructions remain (Task 5 Step 1 publish invocation shape; Task 6 Step 1 success-constant spelling) — each names the exact green test to copy from, which is safer than inventing a helper signature this plan has not verified; everything else is concrete code.
- **Type consistency:** `commitRange([]ObjectID, *Failure)` (Task 1) and `deltaMatchesAt(bool, []RepoPath, error)` (Task 2) are consumed with those exact shapes in Task 3; `PreservationByHistoricContent` spelled identically in Task 3 test and implementation.

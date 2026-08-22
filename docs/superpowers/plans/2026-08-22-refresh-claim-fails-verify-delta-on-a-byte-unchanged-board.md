<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0335 — refresh-claim fails verify-delta when the board is byte-unchanged** — `docs/changes/archive/2026-08-22-0335-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board.md`
<!-- docket:backlink:end -->
# Refresh-Claim Byte-Unchanged Board Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket change refresh-claim` apply cleanly when its board re-render is byte-identical to the committed `BOARD.md`, instead of tripping the engine's `verify-delta` guard.

**Architecture:** `changeClaimOp.Plan` (shared by `change.claim` and `change.refresh-claim`, in `internal/app/change_claim.go`) today appends the inline-board `FileMutation` unconditionally via `boardMutationKind`, which decides create-vs-replace but never byte-compares. Replace that append with the declare-only-when-changed switch that `change_reconcile.go` and `change_attach.go` already use verbatim: read the base-tree board blob via `st.Tree.ReadBlobs`, declare `MutationCreate` when absent, `MutationReplace` only when `!bytes.Equal`, and declare nothing when byte-identical. One plan-closure regression test pins the declaration decision; the existing git-level spike test `TestRefreshClaimFailsWhenBoardReRenderIsUnchanged` (which pins the *buggy* behavior end-to-end) is rewritten to pin the post-fix success.

**Tech Stack:** Go (`internal/app`), Go stdlib `bytes`; test gate is `scripts/run-tests.sh`.

**Spec:** `docs/superpowers/specs/2026-08-21-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board-design.md` (synchronized copy read from the metadata worktree; the spec's precedent citations and assumptions audit trail govern scope).

## Global Constraints

- Touch only: `internal/app/change_claim.go`, `internal/app/change_claim_test.go`, `internal/app/claim_workflow_git_test.go`. No metadata, board, or ADR writes.
- `boardMutationKind` (defined in `internal/app/change_create.go`) **stays** — its other call sites (create, groom, implemented, kill, lifecycle, reclaim, finalize-closeout) all flip board-visible status, so their renders always differ. Only the claim op stops calling it. Do not generalize, do not extract a shared helper (spec assumption 3).
- Every mutation probe and manual re-verification uses `go test -count=1` — Go's test cache can serve a pre-mutation verdict (repo learning: cached-runner-serves-a-mutated-tree).
- Cross-references in comments anchor on symbol names or verbatim-quoted clauses, never line numbers (AGENTS.md / ADR-0054).
- Final gate is the whole suite via `scripts/run-tests.sh`, never only the enumerated tests (AGENTS.md).
- The `bytes.Equal` comparison target is the **base-tree blob** (`st.Tree.ReadBlobs`) — the same copy `verifyActualDelta` diffs against — never the working tree (repo learning: decide-and-act-on-the-same-copy; spec assumption 5).

---

### Task 1: Declare-only-when-changed board switch in `changeClaimOp.Plan`, with mutation-tested regression coverage

**Files:**
- Modify: `internal/app/change_claim.go` — the `if o.inline { ... }` block inside `changeClaimOp.Plan` (grep anchor: `boardMutationKind(ctx, st.Tree, boardPath)` inside this file), plus the import block.
- Test: `internal/app/change_claim_test.go` — new `TestChangeRefreshClaimSkipsUnchangedBoard`, appended after `TestChangeRefreshClaimStampsOnly`.
- Test: `internal/app/claim_workflow_git_test.go` — rewrite `TestRefreshClaimFailsWhenBoardReRenderIsUnchanged` into `TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged`; delete the now-purposeless `rawRefreshClaimOutcome` helper; keep `planningDepsForClock`.

**Interfaces:**
- Consumes: `claimPlanFor`, `baseRefreshOp`, `assertPlanPaths`, `lifecycleChange`, `groomPath` (all already in `internal/app` test files); `st.Tree.ReadBlobs(ctx, []gitcli.RepoPath) ([]gitcli.BlobResult, error)`; `transaction.FileMutation`, `transaction.MutationCreate/MutationReplace`; git-level helpers `planRepoModes`, `planningDepsFor`, `planningDepsForClock`, `cloneOrigin`, `originCommitPaths`, `originFile`, `originTip`, `blobVersionAt`, `assertBoardMatchesCommitted`, `ChangeClaim`, `ChangeRefreshClaim`, `ContextImplementation`.
- Produces: nothing new for later tasks — Task 2 is only the suite gate.

Both new/rewritten tests must be RED before the fix and GREEN after; write them first, observe the red, then land the fix.

- [ ] **Step 1: Write the failing plan-closure test**

Append to `internal/app/change_claim_test.go` (after the closing braces of `TestChangeRefreshClaimStampsOnly`):

```go
// --- TestChangeRefreshClaimSkipsUnchangedBoard ------------------------------

// TestChangeRefreshClaimSkipsUnchangedBoard: a refresh re-stamps only
// claimed_at and updated — neither is board-visible — so its board re-render
// can be byte-identical to the committed BOARD.md. Declaring an unchanged
// path trips the engine's verify-delta guard ("a declared path is not an
// actual change") and fails the whole refresh (change 0335), so the plan must
// declare the board only when it truly changes the tree: absent -> create,
// differing -> replace, byte-identical -> not declared at all.
func TestChangeRefreshClaimSkipsUnchangedBoard(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	const boardPath = "docs/changes/BOARD.md"

	// Pass 1: no committed board. The refresh must still declare the board as
	// a create — and its declared bytes are the canonical render for this
	// corpus at the fixed test clock, which seeds the byte-identical case.
	plan, opRes := claimPlanFor(t, map[string]string{recPath: src}, baseRefreshOp([]string{"inline"}, 3))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:   transaction.MutationReplace,
		boardPath: transaction.MutationCreate,
	})
	var boardBytes []byte
	for _, f := range plan.Files {
		if string(f.Path) == boardPath {
			boardBytes = f.Bytes
		}
	}
	if len(boardBytes) == 0 {
		t.Fatal("absent-board refresh declared no board bytes")
	}

	t.Run("byte-identical committed board is not declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: string(boardBytes)}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		// Exactly the record — a declared-but-unchanged board is the 0335 bug.
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath: transaction.MutationReplace,
		})
	})

	t.Run("stale committed board is still declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: "# Backlog\n\nstale\n"}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath:   transaction.MutationReplace,
			boardPath: transaction.MutationReplace,
		})
	})
}
```

Notes for the implementer:
- `assertPlanPaths` asserts the plan's declared path set **exactly** (it is already used with a record-only map by `TestChangeRefreshClaimStampsOnly`); verify that by reading it in `internal/app/change_lifecycle_test.go` or wherever it is defined (`grep -rn "func assertPlanPaths" internal/app/`). If it turns out to be subset-only, add an explicit `for _, f := range plan.Files { if string(f.Path) == boardPath { t.Errorf(...) } }` loop to the byte-identical subtest — the absence assert is the load-bearing one.
- The two passes are deterministic: `baseRefreshOp` uses the fixed `testClock()`, and the loader ignores `BOARD.md` in the files map (the reconcile tests already seed one).

- [ ] **Step 2: Run the plan-closure test to verify it fails**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && go test -count=1 -run 'TestChangeRefreshClaimSkipsUnchangedBoard' ./internal/app/
```
Expected: FAIL — the "byte-identical committed board is not declared" subtest reports the board path declared as `MutationReplace` (today's unconditional `boardMutationKind` append). The absent-board and stale-board subtests should already PASS. If the byte-identical subtest passes here, stop: the premise is wrong — re-read `changeClaimOp.Plan` before touching anything.

- [ ] **Step 3: Rewrite the git-level spike test to pin the post-fix success**

In `internal/app/claim_workflow_git_test.go`, three edits:

1. Delete the whole `rawRefreshClaimOutcome` function and its doc comment (it exists solely to observe the failure mechanism "before the fix lands" — its premise ends with this change). Keep `planningDepsForClock`: the advanced clock is still what makes the record genuinely change while the board re-renders identically.
2. Update the section banner comment `// --- 0329 spike: refresh-claim fails when the re-rendered board is unchanged --` to `// --- 0335: refresh-claim succeeds when the re-rendered board is unchanged ---`.
3. Replace `TestRefreshClaimFailsWhenBoardReRenderIsUnchanged` (doc comment and body, from its comment block through its closing brace) with:

```go
// TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged: a refresh re-stamps a
// claimed record's claimed_at and updated fields (a real change to the record)
// and re-renders the inline board. The board's in-progress row shows only
// id/title/priority/type/spec/branch — none of which a refresh touches — so
// the re-render is byte-identical to the board the claim already committed.
// Pre-0335 the plan nonetheless DECLARED the board, and the engine's two-way
// actual-delta guard rejected it as "a declared path is not an actual change"
// (a *Failure at stage verify-delta), silently failing every such lease
// refresh. Post-fix the plan declares only the record, so the refresh applies:
// one metadata commit touching exactly the record, the lease re-stamped, the
// committed board untouched and still matching a fresh render.
//
// The refresh runs a day after the claim (advanced clock) so the record
// genuinely changes; that isolates the board as the sole would-be
// declared-but-unchanged path.
func TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	advanced := fixedClock{t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: buildReadyChange(id, slug),
			})
			ctx := context.Background()

			// Claim at the base clock: stamps the record in-progress and
			// commits the inline board reflecting that in-progress row.
			claimNode := planningDepsFor(t, cloneOrigin(t, repo.origin))
			claimCtx := ContextImplementation(ctx, claimNode.deps, claimNode.dir, ImplementationContextRequest{ID: id})
			if claimCtx.Result != ResultApplied || claimCtx.Context == nil || !claimCtx.Context.ClaimEligible {
				t.Fatalf("claim context read = %q (reason %q)", claimCtx.Result, claimCtx.Reason)
			}
			claim := ChangeClaim(ctx, claimNode.deps, claimNode.dir, ChangeClaimRequest{ID: id, Version: claimCtx.Context.Change.Version})
			if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
				t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
			}
			assertBoardMatchesCommitted(t, repo.origin, m.branch, claimNode.dir)

			claimedVersion := blobVersionAt(t, repo.origin, m.branch, recPath)

			// Refresh a day later: claimed_at/updated genuinely change on the
			// record while the board re-renders byte-identical.
			refreshNode := planningDepsForClock(t, cloneOrigin(t, repo.origin), advanced)
			res := ChangeRefreshClaim(ctx, refreshNode.deps, refreshNode.dir, ChangeClaimRequest{ID: id, Version: claimedVersion})
			if res.Result != ResultApplied || res.Disposition != ClaimDispositionApplied {
				t.Fatalf("refresh = (%q, %q), want applied/applied (reason %q, failure %+v, findings %v)",
					res.Result, res.Disposition, res.Reason, res.Failure, res.Findings)
			}
			if res.Revision == "" {
				t.Fatal("applied refresh carried no committed revision")
			}

			// The refresh commit touches exactly the record — the
			// byte-identical board was not declared, so it is not in the
			// commit's changed-path set.
			got := originCommitPaths(t, repo.origin, res.Revision)
			if strings.Join(got, ",") != recPath {
				t.Errorf("refresh commit changed %v, want exactly [%s] (unchanged board must not be declared)", got, recPath)
			}

			// The lease actually moved to the advanced clock.
			final, ok := originFile(t, repo.origin, m.branch, recPath)
			if !ok {
				t.Fatalf("record %s missing from origin after refresh", recPath)
			}
			if !strings.Contains(final, "claimed_at: '2026-08-17T12:00:00Z'") {
				t.Errorf("refresh did not re-stamp claimed_at:\n%s", final)
			}
			if !strings.Contains(final, "updated: '2026-08-17'") {
				t.Errorf("refresh did not re-stamp updated:\n%s", final)
			}

			// The committed board still matches a fresh render of the corpus.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, refreshNode.dir)
		})
	}
}
```

Coverage note (verified while planning, no action needed): rewriting this test does not orphan change 0337's diagnosability payoff — the typed-cause mapping on a failed disposition stays pinned by the unit test `TestClaimResultFromOutcomeFailedCarriesCause` in `internal/app/change_claim_test.go`, which fabricates the `*transaction.Failure` directly and does not depend on this reproduction.

- [ ] **Step 4: Run the rewritten git-level test to verify it fails**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && go test -count=1 -run 'TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged' ./internal/app/
```
Expected: FAIL — `refresh = ("invalid-state", "failed"), want applied/applied` with the failure carrying stage `verify-delta`. That red reproduces the bug end-to-end with the mechanism named. (If real git is unavailable the test skips via `requireRealGit`; the plan-closure red from Step 2 then remains the primary red, but do not skip this run silently — record the skip.)

- [ ] **Step 5: Implement the fix**

In `internal/app/change_claim.go`:

1. Add `"bytes"` to the import block (first stdlib import, before `"context"`).
2. Replace the entire `if o.inline { ... }` block in `changeClaimOp.Plan` — currently the block that calls `boardMutationKind(ctx, st.Tree, boardPath)` — with:

```go
	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		results, err := st.Tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(boardPath)})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: probing board path: %w", err)
		}
		existing := len(results) == 1 && results[0].Found
		// A refresh re-stamps only claimed_at and the updated date — neither is
		// board-visible — so the board re-render can be byte-identical to the
		// committed board. The engine's verify-delta refuses a declared path
		// that is not an actual change ("a declared path is not an actual
		// change"), so the board mutation is declared only when it truly
		// changes the base tree — the same declare-only-when-changed switch
		// change_attach.go and change_reconcile.go use.
		switch {
		case !existing:
			files = append(files, transaction.FileMutation{
				Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationCreate, Bytes: boardBytes,
			})
		case !bytes.Equal(results[0].Blob.Bytes, boardBytes):
			files = append(files, transaction.FileMutation{
				Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationReplace, Bytes: boardBytes,
			})
		}
	}
```

This mirrors `change_reconcile.go`'s board block (grep anchor: `probing board path` in that file) with `change claim:` error prefixes. Do not touch `boardMutationKind` in `change_create.go` — its other call sites keep it.

- [ ] **Step 6: Run both tests to verify they pass**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && go test -count=1 -run 'TestChangeRefreshClaimSkipsUnchangedBoard|TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged' ./internal/app/
```
Expected: PASS, both tests, all subtests. Then run the whole package to catch collateral:
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && go test -count=1 ./internal/app/
```
Expected: PASS. Pay attention to `TestChangeClaimApplies` (fresh claim with inline: its board must still be declared — the fixture has no committed board, so the `!existing` arm fires) and `TestChangeRefreshClaimStampsOnly` (non-inline refresh, unaffected).

- [ ] **Step 7: Mutation-test the guard**

Temporarily revert the switch to the unconditional append (keep a copy of the fixed block first — do NOT use `git checkout -- internal/app/change_claim.go` to restore, that restores to HEAD and destroys the fix; use the editor's undo or re-paste the saved block — repo learning: mutation-restore-needs-a-backup-copy). The mutant:

```go
	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		kind, err := boardMutationKind(ctx, st.Tree, boardPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(boardPath), Kind: kind, Bytes: boardBytes,
		})
	}
```

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && go test -count=1 -run 'TestChangeRefreshClaimSkipsUnchangedBoard|TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged' ./internal/app/
```
Expected: FAIL on both — the plan-closure byte-identical subtest (board declared) and the git-level test (verify-delta failure). `-count=1` is mandatory here; a `(cached)` pass is absence of evidence. Restore the fixed block, re-run the same command, expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && git add internal/app/change_claim.go internal/app/change_claim_test.go internal/app/claim_workflow_git_test.go && git commit -m "fix(0335): declare the claim op's board only when it actually changes

A refresh-claim re-stamps only claimed_at/updated — not board-visible — so
its board re-render is normally byte-identical to the committed BOARD.md.
changeClaimOp.Plan still declared the board via boardMutationKind, so the
engine's verify-delta refused the transaction (\"a declared path is not an
actual change\") and every such lease refresh silently failed. Adopt the
declare-only-when-changed switch change_attach/change_reconcile already
use: read the base-tree blob, create when absent, replace when unequal,
declare nothing when byte-identical."
```

### Task 2: Whole-suite gate

**Files:**
- None modified. This is the verification gate: the change touches a shared op (`changeClaimOp.Plan` serves both `change.claim` and `change.refresh-claim`), so the enumerated tests are not sufficient evidence.

**Interfaces:**
- Consumes: the committed Task 1 tree.
- Produces: the green-suite evidence the build record cites.

- [ ] **Step 1: Run the full suite**

Run (this is what `finalize.test_command` resolves to; it runs files in parallel with per-job isolation and wall-clock budgets):
```bash
cd /Users/homer/dev/docket/.worktrees/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board && ./scripts/run-tests.sh
```
Expected: PASS, zero failures. A trailing `OVER BUDGET:` line does not fail the run but is a finding to act on and record — do not discard it as noise.

- [ ] **Step 2: If red, fix forward within scope**

Any failure outside the three files this plan touches is a stop-and-diagnose, not a patch-to-green: read the failing test's premise first (a red in an untouched file can be a genuine collateral of the declaration change — for example a test asserting a claim/refresh commit's changed-path set that included the board). Fixes must stay within this change's thesis; anything broader is a follow-up finding, not a commit on this branch.

---

## Self-Review

- **Spec coverage:** the switch replacement, the `"bytes"` import, the header comment quoting the verify-delta refuse string, `boardMutationKind` retained for its other callers, the mutation-tested regression test in `change_claim_test.go`, and the whole-suite gate each map to Task 1 steps 1–8 / Task 2. The spec's out-of-scope items (verify-delta redesign, `change_groom`, shared helper extraction) are constrained out in Global Constraints. One addition beyond the spec's letter: the existing git-level spike test pins the buggy behavior and would go permanently red under the fix — rewriting it into the success-shaped end-to-end regression is required for a green suite and is the strongest possible regression pin; the 0337 diagnosability coverage it carried remains held by `TestClaimResultFromOutcomeFailedCarriesCause`.
- **Placeholder scan:** all code steps carry complete code; the one conditional instruction (subset-vs-exact `assertPlanPaths`) names the exact fallback assert to add.
- **Type consistency:** `claimPlanFor(t, map[string]string, changeClaimOp)`, `baseRefreshOp([]string{"inline"}, 3)`, `assertPlanPaths(t, plan, map[string]transaction.MutationKind{...})`, and the git-helper signatures all match their in-repo definitions as read during planning; test names are used identically across Steps 2, 4, 6, and 7.

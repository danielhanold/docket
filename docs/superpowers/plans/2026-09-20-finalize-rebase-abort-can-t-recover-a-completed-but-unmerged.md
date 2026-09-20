<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0438 — finalize.rebase-abort can't recover a completed-but-unmerged rebase whose base later moved](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0438-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged.md)**
<!-- docket:backlink:end -->
# Finalize Forward-Rebase Recovery (change 0438) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `finalize.rebase` recovers a completed-but-unpublished owned rebase whose effective base has since advanced by forward-rebasing the current head onto the new base — preserving prior conflict resolutions, the remote publication lease, and the consumed resolver budget — and always retesting on the new base; separately, repeated `finalize.block` / absent-marker `finalize.clear-block` become real transaction no-ops instead of `invalid-input`.

**Architecture:** All policy changes live in `internal/app/finalize_rebase.go` (restructured `recoverFromReceipt` plus a new refresh path) and `internal/app/finalize_block.go` (no-op plan producers). No new gitcli primitive, workspace receipt field, CLI operation, config key, or lifecycle state is added: the refresh reuses `workspace.RebaseReceipt`'s existing fields, `gitcli.Client.BeginRebase`, the operation lock, and the existing gate/checkpoint/publication machinery. Receipt writers gain an attempt-identity guard so a late write for a superseded rewrite can never clobber a refreshed receipt.

**Tech Stack:** Go; real-Git integration fixtures in `internal/app/finalize_rebase_integration_test.go` (build tag `integration`); suite via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-20-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged-design.md` (on the `docket` metadata branch; the change file is `docs/changes/active/0438-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged.md`). Read the spec before each task — every task below cites the spec section it implements.

## Global Constraints

- No new CLI operation, argument, configuration key, receipt field, lifecycle status, or independent store (spec §6). Existing receipt schemas remain readable; legacy (no-budget) receipts are never silently given a budget.
- Base movement never replenishes resolver-dispatch capacity: preserve `ResolverBudgetVersion`, `ResolverLimit`, `ResolverUsed` verbatim across a refresh (spec §2).
- The remote publication lease is untouchable: `OrigRemoteHead` never changes on a refresh, and a moved/missing remote, changed PR destination, merged/closed PR, foreign workspace, failed probe, or malformed receipt stays a refusal with local work retained (spec §1).
- A probe error is never a clean absence: unknown retains and blocks, it never authorizes the destructive/forward branch (learnings: `probe-error-is-not-clean-absence`).
- Resolver report bodies are never echoed into results (existing file-header rule in `finalize_rebase.go`).
- A base refresh always runs the full configured finalize suite on the resulting head — even when Git reports no textual conflicts or an already-contained target; superseded-base checkpoints and stale PR-body evidence never waive it (spec §4).
- Never `git add -A`; each commit stages exactly the files the task names.
- The build gate runs the whole suite through `build.test_command` resolved from config (currently `go run ./cmd/docket development test`), entered from source. Mutation-test every new guard with `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`).
- Do not renumber or reword the change-0411 reconciliation-write remedy (`reconcileRecoveryRemedy`) or the change-0439 admission diagnostics (`GateReport.Reason/Message/Stage/Locator`); both must survive verbatim (spec §6).

**Repository layout note:** you are in the feature worktree on branch `fix/finalize-rebase-abort-can-t-recover-a-completed-but-unmerged`. Key files: `internal/app/finalize_rebase.go` (~1890 lines), `internal/app/finalize_block.go`, `internal/workspace/rebasereceipt.go`, `internal/gitcli/rebase.go`, `internal/app/finalize_rebase_integration_test.go` (fixtures: `setupRebaseFixture`, `f.advanceBase`, `f.finalizeDeps(gh, gate)`, `fakeRebaseGitHub`, `fakeGate`, `greenEvidenceFor`, `beginSuccessiveConflicts`, `reserveResolveContinue`, `f.repo.writerAdvance`, `f.localHead()`, `f.svc.ReadRebaseReceipt`, `f.metaDir`), `internal/app/finalize_state_integration_test.go` (block/publish engine tests), `skills/docket-finalize-change/SKILL.md`. Integration test shards select by prefix: `TestIntegrationFinalizeRebaseRecovery*` runs in `tests/test_go_integration_app_rebaserecovery.sh`, `TestIntegrationFinalizeRebaseGate*` in `tests/test_go_integration_app_rebase.sh`, `TestIntegrationFinalizeState*` in `tests/test_go_integration_app_state.sh` — name new tests with the matching existing prefixes so no shard or contract edit is needed.

---

### Task 1: Attempt-guarded receipt writers

Spec §3: "A late gate or resolver write for the prior token must not overwrite the refreshed receipt; affected receipt writers must compare the rewrite identity they observed before modifying it."

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`recordGatePassedReceipt`, `clearGateContinuation`, the `FinalizeGateWaiting` persist inside `composeLocalGate`)
- Test: `internal/app/finalize_rebase_test.go`

**Interfaces:**
- Produces: `mutateReceiptForAttempt(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, attempt string, mutate func(*workspace.RebaseReceipt)) (written bool, err error)` — reloads the on-disk receipt under no lock (these writers already run lock-free after the gate), refuses to write when the disk receipt is absent or `disk.Attempt != attempt` (returns `false, nil`: the rewrite was superseded, the late write must not land), otherwise applies `mutate` to the **disk copy** and writes it. Because the mutation applies to the freshly reloaded copy, the resolver-budget group is preserved by construction — the explicit `copyResolverBudget`/`resolverBudgetForWrite` overlay in these three writers becomes redundant and is removed there (both helpers stay: `finalize_reserve.go` and other callers still use them).
- Consumes: `deps.Workspace.ReadRebaseReceipt` / `WriteRebaseReceipt`, `workspace.RebaseReceipt`.

- [ ] **Step 1: Write the failing unit test**

In `internal/app/finalize_rebase_test.go`, add a test using the existing unit-test workspace seam pattern in that file (find the existing fake/stub `Workspace` service the unit tests inject; if the unit file only exercises pure helpers, write this as an integration test in `finalize_rebase_integration_test.go` named `TestIntegrationFinalizeRebaseRecoverySupersededWriterSkipped` instead, using `setupRebaseFixture` + `f.svc.WriteRebaseReceipt` directly):

```go
// A receipt writer observing attempt A must not modify a receipt that now
// records attempt B (a refresh superseded the rewrite mid-flight).
func TestMutateReceiptForAttemptSkipsSuperseded(t *testing.T) {
	// Arrange: a real temp metaDir via workspace.NewService (mirror how
	// rebasereceipt_test.go constructs one), write a receipt with Attempt "B".
	// Act: mutateReceiptForAttempt(ctx, deps, rc, "A", func(r *workspace.RebaseReceipt) {
	//     r.GateDriveID, r.GateOwnerGeneration = "", ""
	// })
	// Assert: written == false, err == nil, and the on-disk receipt still has
	// Attempt "B" with every field byte-identical.
	// Then: mutateReceiptForAttempt(..., "B", mutate) => written == true and
	// only the mutated fields changed.
}
```

- [ ] **Step 2: Run it to verify it fails** — `go test ./internal/app/ -run MutateReceiptForAttempt -count=1` (or the integration name with `-tags integration`). Expected: compile failure, `mutateReceiptForAttempt` undefined.

- [ ] **Step 3: Implement**

```go
// mutateReceiptForAttempt applies mutate to the CURRENT on-disk receipt only
// when it still records the attempt the caller observed. A refresh (change
// 0438) can supersede a rewrite between a caller's read and its terminal
// write; comparing the attempt identity here keeps a late gate or resolver
// write for the prior token from overwriting the refreshed receipt. A missing
// receipt or a foreign attempt is a silent skip (false, nil) — the superseded
// write simply must not land; a read/write error is returned for the caller's
// best-effort message handling.
func mutateReceiptForAttempt(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, attempt string, mutate func(*workspace.RebaseReceipt)) (bool, error) {
	disk, present, err := deps.Workspace.ReadRebaseReceipt(ctx, rc.metaDir)
	if err != nil {
		return false, err
	}
	if !present || disk.Attempt != attempt {
		return false, nil
	}
	before := disk
	mutate(&disk)
	if disk == before {
		return true, nil
	}
	return true, deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, disk)
}
```

Rewrite the three existing writers over it, preserving their exact result-message behavior:

- `recordGatePassedReceipt`: keep the signature; body becomes a `mutateReceiptForAttempt(ctx, deps, rc, rec.Attempt, func(u *workspace.RebaseReceipt) { ...same field assignments as today, minus copyResolverBudget... })`; on `err != nil` append the same `" (persisting the gate terminal to the rebase receipt failed: ...)"` message.
- `clearGateContinuation`: same treatment with its existing message.
- The `FinalizeGateWaiting` branch in `composeLocalGate`: replace the manual `updated := rec` + `copyResolverBudget` + write with the guarded helper; a `written == false` (superseded mid-drive) takes the existing write-failure branch shape but with `ResultContended`, reason `ReasonRebaseVersionDrift`, message `"the owned rewrite was superseded while the gate was running; re-read context finalize"`.

- [ ] **Step 4: Run the test and the package tests** — `go test ./internal/app/ -run 'MutateReceiptForAttempt|FinalizeRebase' -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** — `git add internal/app/finalize_rebase.go internal/app/finalize_rebase_test.go && git commit -m "feat(app): guard rebase-receipt writers on the observed attempt token (change 0438)"`

---

### Task 2: Recovery proofs against the recorded base, pre-start resume, and no evidence-skip on recovery

Spec §3 (durable-before-effect window, in-progress agreement) and §4 ("Receipt-based completion recovery with no valid checkpoint runs the gate"). This task keeps the moved-base refusal in place but relocates it so a still-conflicted or mid-drive owned attempt settles against its **recorded** base first — the precondition Task 3's forward refresh builds on.

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`recoverFromReceipt`, `mapBegunRebase`, `composeLocalGate` signature, `mapContinuedRebase` call site)
- Test: `internal/app/finalize_rebase_integration_test.go`, `internal/app/finalize_rebase_test.go`

**Interfaces:**
- Produces:
  - `composeLocalGate(ctx, deps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, head gitcli.ObjectID, noop, allowEvidenceSkip bool) FinalizeRebaseResult` — new final param; the `gateDecision` PR-body skip is consulted only when `cont.DriveID == "" && allowEvidenceSkip`.
  - `type rebaseMapOptions struct { evidenceSkip bool; forceRetest bool }` and `mapBegunRebase(ctx, deps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, status gitcli.RebaseStatus, opts rebaseMapOptions) FinalizeRebaseResult` — `noop := status.Disposition == gitcli.RebaseUnchanged && !opts.forceRetest`; passes `noop, opts.evidenceSkip` to `composeLocalGate`.
- Consumes: `deps.Planning.Client.ResolveRef(ctx, rc.repo, gitcli.RefName(...))` (exists in `internal/gitcli/refs.go`), `deps.Planning.Client.IsAncestor`, Task 1's writers (unchanged call shape).

Caller matrix for the new params (type-checked exhaustively — the compiler forces every call site):

| Caller | evidenceSkip | forceRetest / noop |
|---|---|---|
| `FinalizeRebase` fresh path | `true` | computed |
| `recoverFromReceipt` conflicted-recovery generated-only completion | `false` | computed (`false` today — keep) |
| `recoverFromReceipt` completed-state compose | `false` | `noop := localHead == rec.OrigHead` as today |
| `recoverFromReceipt` pre-start resume (new) | `false` | computed |
| `mapContinuedRebase` completion compose | `false` | `noop=false` as today |
| Task 3 refresh | `false` | `forceRetest: true` |

- [ ] **Step 1: Write the failing integration tests**

```go
// Durable-before-effect: the receipt persisted but Git never started. Re-entry
// resumes BeginRebase against the receipt's recorded target instead of
// refusing "head does not descend the base" (spec §3).
func TestIntegrationFinalizeRebaseRecoveryPreStartResume(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]
	f := setupRebaseFixture(t, main)
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	// Simulate the crash window: hand-write the receipt FinalizeRebase would
	// have written (OrigHead = current head, BaseHead = the advanced base's
	// head), with Git untouched. Derive the base head the same way the fixture
	// does elsewhere (rev-parse the origin main head after advanceBase).
	baseHead := f.repo.branchHead(t, "main") // use the fixture's existing head accessor; if none exists, runGit(t, f.repo.writerDir, "rev-parse", "refs/heads/main")
	rec := workspace.RebaseReceipt{
		RepoIdentity: f.commonDir(t), ChangeID: strconv.Itoa(f.id),
		OrigHead: f.head, OrigRemoteHead: f.head,
		BaseRef: "refs/heads/main", BaseHead: baseHead,
		Attempt: "20260920T000000Z-pre-start", CreatedUTC: "2026-09-20T00:00:00Z",
		ResolverBudgetVersion: "1", ResolverLimit: "10", ResolverUsed: "0",
	}
	if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
		t.Fatal(err)
	}
	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if out.Disposition != RebaseDispRebased {
		t.Fatalf("pre-start resume = %q (reason %q msg %q), want rebased", out.Disposition, out.Reason, out.Message)
	}
	if out.Attempt != rec.Attempt {
		t.Errorf("resume minted a new attempt %q; want the recorded %q", out.Attempt, rec.Attempt)
	}
	// The gate ran (no PR-evidence skip on recovery) and the head now descends the base.
	if out.Gate == nil || out.Gate.Compose != gateComposeRan {
		t.Fatalf("gate = %+v, want compose ran", out.Gate)
	}
}

// Recovery of a completed rewrite never skips the suite on PR-body evidence:
// receipt-based completion recovery with no valid checkpoint runs the gate
// (spec §4).
func TestIntegrationFinalizeRebaseRecoveryNoEvidenceSkip(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	// Base NOT advanced: a fresh call is a mechanical no-op whose PR body
	// carries green evidence for the exact current head and command — the
	// fresh invocation may skip (existing behavior), but a REPLAY after
	// response loss must run the gate because a receipt now exists and no
	// checkpoint was recorded.
	// Build the PR with a body carrying greenEvidenceFor(t, f.head) the way
	// the existing gateDecision-skip integration test in this file does
	// (find the test that asserts gateComposeSkipped and copy its PR setup).
	// First call => skipped (unchanged). Second call => recovery; assert
	// Gate.Compose == gateComposeRan and the fakeGate was invoked.
}
```

Adjust the second test to this file's actual PR-body helper; if an existing test pins the opposite (a recovery that skips via PR evidence), rewrite that test to assert the new contract and cite spec §4 in its comment — ask what the old test guarded (fresh-path skip still works; only recovery loses the shortcut).

- [ ] **Step 2: Run to verify failure** — `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebaseRecoveryPreStartResume|TestIntegrationFinalizeRebaseRecoveryNoEvidenceSkip' -count=1`. Expected: FAIL — today the pre-start receipt hits `"an owned attempt exists but the workspace head does not descend the base"` (actually first the moved-base refusal never fires here because the receipt's BaseHead equals the live base; the failure is the `!descends` refusal), and the replay skips.

- [ ] **Step 3: Restructure**

In `recoverFromReceipt`, after the identity check and the `RebaseState` probe (keep both), make these changes:

1. **Conflicted-recovery agreement check** (spec §3 "In-progress Git state must agree with the recorded rewrite before it is adopted"): before adopting a `RebaseConflicted` state, resolve the owned base anchor and compare:

```go
anchored, aerr := deps.Planning.Client.ResolveRef(ctx, rc.repo, gitcli.RefName(ownedPrefix+"/base"))
if aerr != nil {
	return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe,
		"the owned base anchor could not be resolved: "+aerr.Error(), id)
}
if string(anchored) != rec.BaseHead {
	return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
		"the in-progress rebase does not match the recorded rewrite's base anchor; retained, not adopted", id)
}
```

2. **Pre-start resume**: in the no-rebase-in-progress region, before the `!descends` refusal:

```go
localHead := rc.insp.HeadCommit
descends, err := deps.Planning.Client.IsAncestor(ctx, rc.repo, gitcli.ObjectID(rec.BaseHead), localHead)
if err != nil { /* existing external refusal */ }
if !descends {
	// The durable-before-effect window (and any crash during the two anchor
	// writes): the receipt is the validated intent, Git never completed. When
	// the clean local head still equals the receipt's OrigHead, resume
	// BeginRebase against the recorded target — anchors are recreated from
	// this proven pre-start state (spec §3). Anything else is retained.
	if rc.insp.Kind == workspace.StateReady && string(localHead) == rec.OrigHead {
		status, berr := deps.Planning.Client.BeginRebase(ctx, rc.wsDir, localHead, gitcli.ObjectID(rec.BaseHead), ownedPrefix)
		if berr != nil {
			return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed, berr.Error(), id)
		}
		return mapBegunRebase(ctx, deps, repoDir, op, rc, pr, rec, status, rebaseMapOptions{})
	}
	return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
		"an owned attempt exists but the workspace head does not descend the base; retained for abort", id)
}
```

Note the descends check and the conflicted/completed branches in this function now key on `rec.BaseHead` (the **recorded** target), not the freshly probed `baseHead` — today they are interchangeable because the moved-base refusal fires first; Task 3 removes that guarantee, so make the recovery proofs recorded-base-relative now (also switch the conflicted-result `BaseHead: string(baseHead)` field to `rec.BaseHead`, matching what the attempt actually targets). Keep the moved-base refusal itself for this task, but move it to fire only in the quiescent-completed region (after the descends proof, before checkpoint/compose) so conflicted and foreign states settle first.

3. **Signatures**: apply the `composeLocalGate` `allowEvidenceSkip` parameter and the `mapBegunRebase` `rebaseMapOptions` refactor per the caller matrix above.

- [ ] **Step 4: Run the new tests plus both rebase unit+integration prefixes** — `go test ./internal/app/ -run FinalizeRebase -count=1 && go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebase' -count=1`. Expected: PASS (fix any test the compose-signature change touches mechanically).

- [ ] **Step 5: Commit** — `git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go internal/app/finalize_rebase_test.go && git commit -m "feat(app): recovery proofs against the recorded base, pre-start resume, no evidence-skip on recovery (change 0438)"`

---

### Task 3: Forward refresh of a completed, quiescent, unpublished rewrite

Spec §1 (classification), §2 (receipt + Git reuse), §4 (forced retest). The core of the change.

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`recoverFromReceipt` quiescent-completed region; new `refreshOwnedRewrite`; one new reason constant)
- Test: `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Produces: `refreshOwnedRewrite(ctx context.Context, deps FinalizeDeps, repoDir string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, newBaseHead, remoteHead gitcli.ObjectID, ownedPrefix string) FinalizeRebaseResult`; new reason constant `ReasonRebaseRefreshContended = "refresh-contended"` in the continue/abort-refusals block.
- Consumes: Task 2's `mapBegunRebase(..., rebaseMapOptions{forceRetest: true})`, `deps.Workspace.AcquireOperationLock`, `newRebaseAttempt`, `requireCarriedPreserved`, `deps.Planning.Clock`.

- [ ] **Step 1: Write the failing integration tests** (acceptance §1–§2; all `TestIntegrationFinalizeRebaseRecovery*` so they land in the rebaserecovery shard):

```go
// Acceptance 1+2: a completed rebase with a resolver-authored resolution,
// interrupted before publication; the base advances; re-invoking
// finalize.rebase forward-rebases from the current head, preserving the
// resolution, running the suite on the new head, refreshing OrigHead/BaseHead/
// Attempt while preserving OrigRemoteHead and the consumed budget.
func TestIntegrationFinalizeRebaseRecoveryForwardRefresh(t *testing.T) {
	requireRealGit(t)
	f, deps, head, begin := beginSuccessiveConflicts(t, 0, nil,
		map[string]string{"feature.txt": "conflicting base content\n"})
	attempt := begin.Attempt
	_, cont := reserveResolveContinue(t, f, deps, attempt, begin.UnmergedPaths, 1)
	if cont.Disposition != RebaseDispRebased {
		t.Fatalf("continue = %q, want rebased (completed rewrite)", cont.Disposition)
	}
	rewritten := f.localHead()
	recBefore, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

	// The base advances AGAIN (non-conflicting this time) before publication.
	f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved again\n"})

	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if out.Disposition != RebaseDispRebased || out.Result != ResultApplied {
		t.Fatalf("forward refresh = %q/%q (reason %q msg %q), want applied/rebased", out.Result, out.Disposition, out.Reason, out.Message)
	}
	if out.Gate == nil || out.Gate.Compose != gateComposeRan {
		t.Fatalf("gate = %+v, want compose ran (a base refresh always retests)", out.Gate)
	}
	// The resolution content survived and the branch was never reset.
	if got := readRepoFile(t, f.wp, "feature.txt"); got != "reconciled content for cycle 1\n" {
		t.Fatalf("resolution lost: feature.txt = %q", got)
	}
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if rec.OrigHead != rewritten {
		t.Errorf("refreshed OrigHead = %q, want the completed rewrite head %q (never the pre-first-rebase head)", rec.OrigHead, rewritten)
	}
	if rec.OrigRemoteHead != recBefore.OrigRemoteHead {
		t.Errorf("the publication lease moved: %q -> %q", recBefore.OrigRemoteHead, rec.OrigRemoteHead)
	}
	if rec.Attempt == attempt {
		t.Errorf("refresh kept the superseded attempt token %q", attempt)
	}
	if rec.ResolverLimit != recBefore.ResolverLimit || rec.ResolverUsed != recBefore.ResolverUsed {
		t.Errorf("budget changed: %s/%s -> %s/%s (base movement never replenishes)",
			recBefore.ResolverUsed, recBefore.ResolverLimit, rec.ResolverUsed, rec.ResolverLimit)
	}
	// The old attempt token can no longer continue the refreshed rewrite.
	stale := FinalizeRebaseContinue(context.Background(), deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"x"}})
	if stale.Reason != ReasonRebaseAttemptMismatch {
		t.Errorf("stale-token continue reason = %q, want attempt-token-mismatch", stale.Reason)
	}
}

// Refusals (acceptance 4): each admission conjunct refuses with head, files,
// receipt, and remote unchanged.
func TestIntegrationFinalizeRebaseRecoveryForwardRefreshRefusals(t *testing.T) {
	requireRealGit(t)
	// Table-drive over sabotage closures applied after the completed rewrite
	// and base advance, asserting the disposition/reason and that
	// f.localHead(), the on-disk receipt, and the remote feature head are
	// byte-identical before/after:
	//  - "remote-lease-moved": push a new commit to origin feat/<slug>
	//     => ReasonRebaseRemoteHeadMismatch, blocked.
	//  - "divergent-base": rewrite origin main via writer-side
	//     commit --amend + push --force (the new base must NOT descend the
	//     recorded base) => ReasonRebaseMovedBase, blocked.
	//  - "dirty-workspace": writeRepoFile an uncommitted edit
	//     => ReasonRebaseWorkspaceDirty, blocked.
	//  - "reservation-outstanding": a second conflicting base advance whose
	//     rebase stops, reserve (FinalizeResolverReserve) but do not continue,
	//     then... (this one stays conflicted, so it exercises the
	//     conflicted-settles-first path instead: assert disposition
	//     conflicted against the RECORDED base, not a refresh).
}
```

Use the file's existing helpers for reading files/remote heads (`readRepoFile` exists in the app test helpers — if named differently, use the fixture's actual reader; `f.repo.writerAdvance` advances main). Add a `checkpoint-cleared` assertion to the first test: after the pre-refresh gate passed, `recBefore` carries a publish checkpoint (change 0408); assert the refreshed `rec` has all six `PublishCheckpoint*` fields empty **before** its own gate ran — since the refresh's gate also passes in this test, instead assert `out.Gate.Compose == gateComposeRan` (the old checkpoint was not reused) and that the fake gate was invoked for the refresh call (give `fakeGate` a call counter if it lacks one).

- [ ] **Step 2: Run to verify failure** — `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebaseRecoveryForwardRefresh -count=1`. Expected: FAIL with today's `base-moved-under-receipt` blocked refusal.

- [ ] **Step 3: Implement**

In `recoverFromReceipt`'s quiescent-completed region (after the `descends rec.BaseHead` proof), replace the relocated moved-base refusal with classification:

```go
baseMoved := rec.BaseHead != string(baseHead)
if !baseMoved {
	// ... existing checkpoint-reuse + compose flow, unchanged ...
}
// The base advanced under a completed, quiescent, owned, unpublished rewrite:
// classify for forward refresh (spec §1). A live gate continuation settles
// FIRST through the existing machinery — the moved base does not stop that
// work, and its result is never permission to publish against the new base
// (the refresh below clears the old checkpoint and mints a new attempt).
if rec.GateDriveID != "" {
	return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, localHead, string(localHead) == rec.OrigHead, false)
}
return refreshOwnedRewrite(ctx, deps, repoDir, rc, pr, rec, baseHead, remoteHead, ownedPrefix)
```

Ordering note: today the checkpoint-reuse block is gated on `!noop && rec.GateDriveID == ""` — Task 4 adjusts the `!noop` half; this task only wraps the existing block in the `!baseMoved` arm (a superseded-base checkpoint is structurally unreachable for reuse because `checkpointDecision` compares `cp.BaseHead == liveBaseHead`, but the refresh still clears it durably below — belt and suspenders the spec demands).

```go
// refreshOwnedRewrite advances a completed, clean, quiescent, unpublished
// owned rewrite onto the newly observed effective base head (change 0438). It
// preserves prior resolutions (the rebase starts from the CURRENT head), the
// exact remote publication lease, and the consumed resolver budget; it mints a
// fresh attempt token and clears the gate pair and the superseded publish
// checkpoint, so nothing recorded against the old base can authorize
// publication of the refreshed rewrite. Every refusal retains local work.
func refreshOwnedRewrite(ctx context.Context, deps FinalizeDeps, repoDir string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, newBaseHead, remoteHead gitcli.ObjectID, ownedPrefix string) FinalizeRebaseResult {
	op := OperationFinalizeRebase
	id := int(rc.change.ID())
	localHead := rc.insp.HeadCommit

	// Unsettled resolver work blocks a refresh: settle it against the recorded
	// base first (spec §1). Quiescent Git with an outstanding reservation or a
	// started continuation is ambiguous — retained, never refreshed over.
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		return withResolverCounts(rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseMovedBase,
			"the effective base moved but resolver work on the recorded attempt is unsettled; settle or abort it before the rewrite can be refreshed", id), rec)
	}
	// The clean, registered feature workspace — a dirty tree refuses exactly
	// like a fresh begin.
	switch rc.insp.Kind {
	case workspace.StateReady:
	case workspace.StateDirty:
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseWorkspaceDirty,
			"the feature workspace has uncommitted changes; a base refresh requires a clean tree", id)
	default:
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseWorkspaceNotReady,
			fmt.Sprintf("the feature workspace is %q, not the clean registered feature state", rc.insp.Kind), id)
	}
	// The publication lease must be intact: this recovery is for an
	// UNPUBLISHED rewrite, so the remote feature head must still be the
	// receipt's recorded lease value (spec §1).
	if string(remoteHead) != rec.OrigRemoteHead {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseRemoteHeadMismatch,
			"the remote feature head is not the receipt's recorded publication lease; the rewrite may have been published or the remote moved — retained, not refreshed", id)
	}
	// Only forward base movement is in scope: the new base must descend the
	// recorded one. A rewritten/divergent base keeps the moved-base refusal.
	fastForward, err := deps.Planning.Client.IsAncestor(ctx, rc.repo, gitcli.ObjectID(rec.BaseHead), newBaseHead)
	if err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
	}
	if !fastForward {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseMovedBase,
			"the effective base was rewritten (it does not descend the recorded base); retained, not adopted", id)
	}
	// Carried descendants must be preserved at the refresh's starting head
	// BEFORE any receipt or Git mutation; composeLocalGate re-proves the
	// resulting head.
	if r := requireCarriedPreserved(ctx, deps, repoDir, op, rc, localHead); r != nil {
		return *r
	}

	release, lerr := deps.Workspace.AcquireOperationLock(rc.metaDir)
	if lerr != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe,
			"could not acquire the workspace operation lock: "+lerr.Error(), id)
	}
	released := false
	releaseLock := func() {
		if !released {
			released = true
			release()
		}
	}
	defer releaseLock()

	// Decide and act on the same copy: reload under the lock and require the
	// receipt this classification observed. A concurrent refresh/continue wins;
	// the loser re-reads (spec §3: one owned rewrite; the loser reloads the
	// winner's receipt).
	disk, present, rerr := deps.Workspace.ReadRebaseReceipt(ctx, rc.metaDir)
	if rerr != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptRead, rerr.Error(), id)
	}
	if !present || disk != rec {
		return rebaseRefusal(op, ResultContended, RebaseDispContended, ReasonRebaseRefreshContended,
			"the owned rebase receipt changed while the refresh was being admitted; re-read context finalize", id)
	}

	refreshed := disk
	refreshed.OrigHead = string(localHead)
	refreshed.BaseHead = string(newBaseHead)
	refreshed.Attempt = newRebaseAttempt(deps, newBaseHead)
	refreshed.GateDriveID, refreshed.GateOwnerGeneration = "", ""
	refreshed.PublishCheckpointHead, refreshed.PublishCheckpointBaseHead = "", ""
	refreshed.PublishCheckpointCommand, refreshed.PublishCheckpointGate = "", ""
	refreshed.PublishCheckpointPRNumber, refreshed.PublishCheckpointEvidence = "", ""
	refreshed.CreatedUTC = deps.Planning.Clock.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, refreshed); werr != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite, werr.Error(), id)
	}

	// The refreshed intent is durable; the Git rewrite may run. An
	// interruption from here is recovered by the pre-start resume (Task 2) or
	// the ordinary conflicted/completed recovery against the refreshed
	// receipt.
	status, berr := deps.Planning.Client.BeginRebase(ctx, rc.wsDir, localHead, newBaseHead, ownedPrefix)
	if berr != nil {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed, berr.Error(), id)
	}
	releaseLock()
	return mapBegunRebase(ctx, deps, repoDir, op, rc, pr, rec /* see below */, status, rebaseMapOptions{forceRetest: true})
}
```

Pass `refreshed` (not `rec`) into `mapBegunRebase` — the result's `OrigHead`/`BaseHead`/`Attempt` and resolver counts must describe the refreshed rewrite. Add the constant beside the other continue/abort reasons:

```go
ReasonRebaseRefreshContended = "refresh-contended" // the receipt changed while a base refresh was being admitted
```

Attempt-token uniqueness: `newRebaseAttempt` derives from clock + base-head prefix; the refresh targets a **different** base head than the recorded attempt, so the token always differs even within one clock second — assert this reasoning in a short comment there.

- [ ] **Step 4: Run** — `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebaseRecovery' -count=1 && go test ./internal/app/ -run FinalizeRebase -count=1`. Expected: PASS, including all pre-existing recovery tests (the response-loss, checkpoint, carry, and budget tests must be untouched by behavior: their bases don't move after the receipt is written, except `TestIntegrationFinalizeRebaseRecoveryCheckpointInvalidation` — read it; if it advances the base to invalidate a checkpoint and expects a moved-base block, it now gets a forward refresh instead: update its expectations to the refreshed contract and say so in its comment, citing spec §4).

- [ ] **Step 5: Add the interruption/concurrency test** (acceptance §3):

```go
// Interruption matrix around the refresh: after receipt persistence but before
// Git (pre-start resume against the NEW target), after Git completion but
// before the gate (recovery runs the gate; no stale evidence), and another
// base advance after a settled refresh (a second refresh).
func TestIntegrationFinalizeRebaseRecoveryForwardRefreshInterruptions(t *testing.T) {
	requireRealGit(t)
	// (a) receipt-persisted/Git-not-started: run the ForwardRefresh setup, but
	//     before re-invoking, hand-write the refreshed receipt the refresh
	//     would write (OrigHead = completed rewrite head, BaseHead = new base
	//     head, fresh Attempt, budget copied, checkpoint/gate cleared) via
	//     f.svc.WriteRebaseReceipt. Invoke FinalizeRebase: it must resume
	//     BeginRebase to the recorded target (Task 2 path), keep the recorded
	//     attempt, run the gate.
	// (b) response-loss after a completed refresh: invoke FinalizeRebase twice
	//     after the base advance; the second call must not rebase again
	//     (localHead unchanged between calls), must keep the refreshed
	//     attempt, and — with the gate PASSED and checkpoint recorded on the
	//     first call — may reuse the NEW checkpoint (Gate.Compose skipped with
	//     Permit == current head).
	// (c) base advances once more after (b): a second refresh fires; assert a
	//     second new attempt token and a second gate run.
}
```

- [ ] **Step 6: Run it, then commit** — `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeRebaseRecoveryForwardRefresh -count=1`. `git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go && git commit -m "feat(app): forward-rebase a completed unpublished rewrite onto an advanced base (change 0438)"`

---

### Task 4: Checkpoint reuse for the mechanically-unchanged retest

Spec §4: "Apply that checkpoint path also when the required gate ran after a mechanically unchanged rebase, so interruption does not require a new persistent flag" — and acceptance §5.

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`recoverFromReceipt` checkpoint gate; `recordGatePassedReceipt` noop condition)
- Test: `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: Task 2/3 structure. No new symbols.

- [ ] **Step 1: Write the failing test**

```go
// A base advance the feature head already contains (mechanically unchanged
// refresh) still runs the full suite, records a checkpoint for the NEW base,
// and a response-lost replay reuses that checkpoint instead of re-running.
func TestIntegrationFinalizeRebaseRecoveryForwardRefreshUnchangedRetests(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	// Complete the first rebase (real rewrite onto the advanced base).
	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Disposition != RebaseDispRebased { t.Fatal(first.Disposition) }
	rewritten := f.localHead()
	// Advance main to EXACTLY the rewritten feature head's ancestor... simplest
	// mechanically-unchanged advance: fast-forward origin main to the rewritten
	// head itself is wrong (that's the merge); instead advance main by merging
	// nothing new the feature lacks — push the feature's FIRST parent chain:
	// advance origin main to the OLD base + a commit that is already an
	// ancestor of the rewritten head is not constructible generically, so use
	// the direct construction: writerAdvance main with a file identical to a
	// commit already in the feature history is still a NEW commit (different
	// sha) and rebases non-trivially. The reliable mechanically-unchanged case
	// is: base advances, feature ALREADY descends it — i.e. push the rewritten
	// feature head's parent (the new base commit from f.advanceBase) plus one
	// commit that the rewrite also carried. In practice: fast-forward origin
	// main to `rewritten`'s parent chain via
	// runGit(t, f.repo.writerDir, "fetch", "origin") ;
	// runGit(t, f.repo.writerDir, "merge", "--ff-only", <base-ancestor-of-rewritten>).
	// Concretely: git -C writer rev-parse of rewritten~N where N = number of
	// feature commits (1 in this fixture) gives the base tip the rewrite sits
	// on; any FURTHER advance must come from the feature side. So: advance
	// origin main to `rewritten~1` first (a no-move), then to a NEW commit and
	// verify gate-run; the already-contained arm is covered by advancing main
	// to a commit the feature will pick up as an empty rebase (git reports
	// unchanged only when head==expectedHead — with a moved base BeginRebase
	// reports rebased/unchanged per classify). Keep whichever arm is
	// constructible: the LOAD-BEARING asserts are (1) second FinalizeRebase
	// call after the advance has Gate.Compose == gateComposeRan even though
	// PR-body-style evidence for the current head exists, and (2) a third,
	// response-lost call reuses the checkpoint: Gate.Compose == gateComposeSkipped,
	// Permit == current head, and the receipt's PublishCheckpointBaseHead is
	// the NEW base head.
}
```

(The worker should simplify this fixture to the smallest constructible mechanically-relevant case — the asserts, not the construction, are the contract. If a genuinely already-contained advance is not constructible with the fixture's writer helpers, cover the "no textual conflicts" refresh (a clean non-conflicting rebase) plus checkpoint reuse, and assert the `forceRetest` unit behavior separately: a unit test on `mapBegunRebase` with `status.Disposition == gitcli.RebaseUnchanged` and `rebaseMapOptions{forceRetest: true}` must compose the gate, not skip.)

- [ ] **Step 2: Run to verify the checkpoint-reuse half fails** — the replay currently cannot reuse a checkpoint recorded for a noop-classified pass, and `recordGatePassedReceipt` refuses to record one (`!noop` guard).

- [ ] **Step 3: Implement**

- In `recoverFromReceipt`, change the checkpoint-reuse gate from `if !noop && rec.GateDriveID == ""` to `if rec.GateDriveID == ""` — checkpoint presence itself proves a required gate ran (a fresh no-op never records one), and `checkpointDecision` still requires full identity including the live base head.
- In `recordGatePassedReceipt`, the checkpoint-record condition `!noop && pr.Number > 0 && ...`: keep `!noop` as the caller-computed value — Task 2/3 already force `noop=false` on every refresh/recovery compose, so a required gate after a mechanically unchanged refresh records its checkpoint through the existing condition. Verify this by reading the call chain; if the pre-start-resume path can reach here with `noop=true` for a genuinely-unchanged fresh target, that is the fresh no-op case and correctly records nothing.

- [ ] **Step 4: Run** — `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebaseRecovery' -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** — `git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go && git commit -m "feat(app): refreshed-base retest with checkpoint reuse for the mechanically unchanged rebase (change 0438)"`

---

### Task 5: Real no-op plans for finalize.block / finalize.clear-block

Spec §5 and acceptance §6. The zero-valued `transaction.MutationPlan{}` both producers return for their nothing-to-do branches fails `validatePlan` ("empty commit subject" / "empty receipt") at engine step 7 before the empty-`Files` no-op at step 9 — so a repeated block and an absent-marker clear-block surface as `invalid-input` instead of a no-op.

**Files:**
- Modify: `internal/app/finalize_block.go` (`finalizeBlockOp.Plan`, `finalizeClearBlockOp.Plan`, `blockResultFromOutcome`, `clearBlockResultFromOutcome` and their call sites)
- Test: `internal/app/finalize_state_integration_test.go`, `internal/app/finalize_block_test.go`

**Interfaces:**
- Produces: `blockResultFromOutcome(op string, res transaction.Result, execErr error, url string, id int) BlockResult` and `clearBlockResultFromOutcome(res transaction.Result, execErr error, id int) BlockResult` — new trailing `id`; `out.ID` starts as `id` and the decoded receipt (when the engine persisted one) overrides it. Update the two call sites (`FinalizeBlock` near `return blockResultFromOutcome(OperationFinalizeBlock, res, execErr, url)`, and `FinalizeClearBlock`'s `return clearBlockResultFromOutcome(res, execErr)`) to pass `req.ID`.
- Consumes: existing `blockReceipt`, `transaction.MutationPlan`, `BlockDispAlready`, `BlockDispNothingToClear`.

- [ ] **Step 1: Write the failing integration test** (must go through the real engine — planning-only tests are insufficient per acceptance §6):

```go
// Repeated block and absent-marker clear-block are REAL transaction no-ops:
// no-op disposition, the requested change id, byte-stable metadata and board,
// an unmoved remote metadata revision, and no duplicate comment or marker.
func TestIntegrationFinalizeStateBlockAndClearNoOps(t *testing.T) {
	// Mirror TestIntegrationFinalizeStateBlockCommentThenMarker's fixture and
	// fake GitHub (it counts EnsureComment calls or can be extended to).
	// 1. FinalizeBlock once => BlockDispRecorded, a marker commit.
	//    Capture: remote docket-branch head, the change file bytes, comment count.
	// 2. FinalizeBlock again, SAME attempt token, fresh version (re-read status
	//    for the new record version) => want:
	//       Result == ResultNoOp, Disposition == BlockDispAlready,
	//       out.ID == the requested id (not 0),
	//       remote docket-branch head UNMOVED,
	//       change file bytes identical, comment count unchanged,
	//       exactly one attempt marker in the "## Finalize blocked" section.
	// 3. FinalizeClearBlock on a change with NO marker (use a second change or
	//    clear first then clear again) => want:
	//       Result == ResultNoOp, Disposition == BlockDispNothingToClear,
	//       out.ID == the requested id, remote head unmoved.
	// 4. Stale-version refusal unchanged: FinalizeBlock with the pre-step-1
	//    version => ResultContended/BlockDispContended (regression pin that the
	//    no-op repair did not weaken expectation checking).
}
```

Note on step 2's version: the engine's expectation pins the record's blob version; after step 1's commit the version moved, so re-read it (the existing tests in this file show the re-read idiom). The no-op must be reached through a **valid current** version — that is the repeated-request scenario the spec names.

- [ ] **Step 2: Run to verify it fails** — `go test -tags integration ./internal/app/ -run TestIntegrationFinalizeStateBlockAndClearNoOps -count=1`. Expected: FAIL — step 2 today returns `ResultInvalidInput`/`DispositionFailed` ("operation produced an invalid plan").

- [ ] **Step 3: Implement**

In `finalizeBlockOp.Plan`, replace the bare no-op return:

```go
// Idempotency keyed on the promised state: this attempt already recorded.
// A REAL no-op plan (zero file mutations, valid subject/receipt metadata)
// so validatePlan admits it and the engine's empty-Files path reports
// no-op — never invalid-input (change 0438). The engine commits nothing
// and persists no receipt for an empty plan; the subject/receipt exist
// only to satisfy plan validation.
if present && strings.Contains(oldBody, finalizeBlockedAttemptMarker(o.req.Attempt)) {
	receipt, err := json.Marshal(blockReceipt{ID: o.req.ID, Op: OperationFinalizeBlock})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("finalize block: encoding no-op receipt: %w", err)
	}
	return transaction.MutationPlan{
		CommitSubject: fmt.Sprintf("change %04d finalize blocked (no-op)", o.req.ID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
```

Same shape in `finalizeClearBlockOp.Plan`'s absent-marker branch (`Op: OperationFinalizeClearBlock`, subject `"change %04d finalize block cleared (no-op)"`). Then add the `id` parameter to both outcome mappers (`out := BlockResult{ID: id, ...}`; keep the receipt-decode override) and thread `req.ID` from both call sites. Do **not** touch `internal/repository/transaction` — the engine keeps validating every plan and keeps its no-commit/no-receipt no-op behavior (the comment above `if len(plan.Files) == 0` explains why no receipt is persisted; do not "fix" that).

- [ ] **Step 4: Run** — `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeState' -count=1 && go test ./internal/app/ -run Block -count=1`. Expected: PASS (update `finalize_block_test.go` planning-only expectations that pinned the zero-valued plan, if any — they now expect the populated no-op plan).

- [ ] **Step 5: Commit** — `git add internal/app/finalize_block.go internal/app/finalize_block_test.go internal/app/finalize_state_integration_test.go && git commit -m "fix(app): finalize block/clear-block no-op plans pass engine validation (change 0438)"`

---

### Task 6: Finalize workflow guidance and embedded assets

Spec §6: describe forward recovery, normal conflict handling, and retesting in the moved-base guidance; retained ambiguity still halts; generic `blocked` is never permission to reset; preserve the 0411 remedy and 0439 diagnostics; regenerate embedded assets through the established process.

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets` (runs `cmd/genassets`)

**Interfaces:** none (prose + generated bundle).

- [ ] **Step 1: Edit the guidance**

In `skills/docket-finalize-change/SKILL.md`, find the disposition bullets (the `blocked` bullet currently reads "retained foreign rebase state, a moved base, an unresolved effective base, a dirty workspace, or a precondition failure the receipt was **not** written for"). Update the moved-base story:

- Remove "a moved base" from the flat `blocked` enumeration and add a short dedicated paragraph after the disposition list, in this skill's voice, saying: when the effective base advances under a **completed, clean, unpublished** owned rebase, re-running the `finalize.rebase` operation with the same inputs forward-rebases the current head onto the new base — prior conflict resolutions and committed work are preserved, never reset; conflicts route through the ordinary reserve/resolve/continue flow with the attempt's remaining resolver budget (base movement replenishes nothing); the full local suite always re-runs on the refreshed head, and a fresh attempt token supersedes the old one. A `blocked` with reason `base-moved-under-receipt` now means the base **diverged** (was rewritten) or resolver work on the recorded attempt is unsettled — retained state, `halted`, a human is needed; a generic `blocked` is never permission to reset or abort a completed rewrite.
- Leave the abort-flow paragraph, the reconciliation-write exception paragraph (change 0411), and every publish/clear-block sentence byte-identical except where the moved-base wording itself appears.
- Check `skills/docket-finalize-change/references/gate-failure.md` (referenced at the reconciliation-exception paragraph) for moved-base wording with `grep -rn "moved" skills/docket-finalize-change/references/` and update consistently if present.

- [ ] **Step 2: Regenerate the embedded bundle** — `go generate ./internal/assets` from the worktree root, then `git status` to confirm only `internal/assets/embedded/` (manifest + the changed skill payload) moved.

- [ ] **Step 3: Verify** — `go test ./internal/assets/ -count=1` (the bundle-matches-source guard must be green) and `grep -c "retry the finalize.rebase-continue operation" internal/app/finalize_rebase.go` still reports 1 (0411 remedy untouched).

- [ ] **Step 4: Commit** — `git add skills/docket-finalize-change/SKILL.md internal/assets/embedded && git commit -m "docs(skills): finalize moved-base guidance describes forward recovery (change 0438)"` (add the references file too if edited).

---

### Task 7: Mutation-test the new guards, then the full suite gate

Global constraints: a guard is code — strip what it guards and watch the assert redden; run the whole suite from source and read the budget report.

**Files:** none new (temporary mutations only; every mutation reverted).

- [ ] **Step 1: Mutation-test each new safety guard** — for each mutation: apply it, run the named test with `-count=1` (defeat the Go test cache), confirm RED, then `git checkout -- <file>` **only if the working tree has no other uncommitted work** (all prior tasks are committed, so the tree is clean — verify with `git status` first; learnings: `mutation-restore-needs-a-backup-copy`).

| # | Mutation (in `finalize_rebase.go` / `finalize_block.go`) | Must redden |
|---|---|---|
| 1 | In `refreshOwnedRewrite`, delete the `remoteHead != rec.OrigRemoteHead` refusal | `ForwardRefreshRefusals` remote-lease-moved case |
| 2 | Delete the `!fastForward` divergent-base refusal | `ForwardRefreshRefusals` divergent-base case |
| 3 | Make the refresh keep the old token (`refreshed.Attempt = disk.Attempt`) | `ForwardRefresh` stale-token-continue assert |
| 4 | Reset the budget on refresh (`refreshed.ResolverUsed = "0"`) | `ForwardRefresh` budget-preservation assert |
| 5 | Skip clearing the checkpoint on refresh | `ForwardRefresh` gate-ran assert (or `UnchangedRetests` reuse assert) |
| 6 | In `mutateReceiptForAttempt`, drop the `disk.Attempt != attempt` skip | Task 1's superseded-writer test |
| 7 | In `composeLocalGate`, ignore `allowEvidenceSkip` (always consult `gateDecision`) | `RecoveryNoEvidenceSkip` |
| 8 | Restore `finalizeBlockOp.Plan`'s zero-valued no-op plan | `BlockAndClearNoOps` step 2 |
| 9 | In the pre-start resume, drop the `localHead == rec.OrigHead` conjunct (resume from any head) | `ForwardRefreshRefusals` dirty/foreign retention, or add the direct assert to `PreStartResume` (hand-write a receipt whose OrigHead ≠ current head → must refuse retained) |

Record each mutation and its red test name in the eventual results file notes (the build role's evidence).

- [ ] **Step 2: Run the whole suite from source** — `go run ./cmd/docket development test`. Expected: green. Read the budget report even on green: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line on `test_go_integration_app_rebaserecovery.sh` or `test_go_integration_app_state.sh` is a screening finding (the new real-Git tests add wall clock); a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on per `tests/README.md` (serial confirm, then adjust `tests/runtime-budgets.tsv` in a dedicated commit only if the breach is real and justified).

- [ ] **Step 3: Commit any budget-file adjustment** (only if step 2 required one) — `git add tests/runtime-budgets.tsv && git commit -m "test: budget adjustment for change 0438 integration coverage"`

---

## Self-Review (performed while writing)

- **Spec coverage:** §1 classification → Tasks 2–3; §2 receipt/Git reuse → Task 3; §3 interruption/one-rewrite/late-writer → Tasks 1–3 (pre-start resume, refresh-contended reload, attempt-guarded writers); §4 retest + checkpoint scoping → Tasks 2 (no recovery evidence-skip), 3 (`forceRetest`), 4 (checkpoint reuse); §5 no-op producers → Task 5; §6 docs/compat → Task 6; acceptance 1–6 → Tasks 3 (1, 2, 4), 3 step 5 (3), 4 (5), 5 (6); mutation/budget discipline → Task 7.
- **Known judgment calls, recorded for the builder:** (a) a live gate continuation with a moved base settles via `composeLocalGate` against the recorded receipt — its terminal is old-base-scoped and cannot publish the refreshed rewrite because the refresh mints a new attempt and clears the checkpoint; (b) recovery composes with `allowEvidenceSkip=false` uniformly (spec §4 sentence "Receipt-based completion recovery with no valid checkpoint runs the gate"), which intentionally retires any recovery-side PR-evidence skip; (c) `ReasonRebaseRefreshContended` is a new reason token — reason vocabulary is app-level and not among the spec's prohibited additions (no CLI op/config/receipt field/lifecycle state).
- **Type consistency:** `mutateReceiptForAttempt` (Task 1) is used by name in Task 7 mutation 6; `rebaseMapOptions{forceRetest: true}` (Task 2) is consumed by Task 3; `refreshOwnedRewrite`'s signature matches its Task 3 call site; `blockResultFromOutcome`/`clearBlockResultFromOutcome` id parameters match Task 5's call-site edits.

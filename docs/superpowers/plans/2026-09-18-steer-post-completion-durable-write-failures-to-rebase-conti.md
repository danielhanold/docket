<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0411 — Steer post-completion durable-write failures to rebase-continue, not abort](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0411-steer-post-completion-durable-write-failures-to-rebase-conti.md)**
<!-- docket:backlink:end -->
# Steer Post-Completion Durable-Write Failures to Rebase-Continue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing `finalize.rebase-continue` recovery path discoverable when Git advanced or completed an owned resolver continuation but Docket could not persist the reservation reconciliation — via reworded diagnostics and a harness-neutral skill-doc exception — instead of operators defaulting to `finalize.rebase-abort` and discarding the completed rewrite.

**Architecture:** Three `message` strings in `internal/app/finalize_rebase.go` gain a shared recovery-remedy tail naming `finalize.rebase-continue` with the same change id, owned attempt, and original resolved report; result/disposition/reason (`external-failed`/`blocked`/`receipt-write-failed`) and the receipt schema are untouched. The finalize skill and its `gate-failure.md` reference gain the matching operator exception, guarded by whitespace-collapsed section-bound prose contracts in `internal/repoguard`. Embedded skill assets are regenerated from maintained source.

**Tech Stack:** Go (stdlib testing), existing `internal/app` rebase fixtures (`reserveOnConflict`, `completedBudgetedReceipt`, `seedReserveReceipt`, `stageSeam`, `reloadReceipt`, `fakeGate`), `internal/repoguard` prose/budget guards, `cmd/genassets` bundle generator.

**Spec:** `docs/superpowers/specs/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-design.md` (on the metadata branch: `.docket/docs/superpowers/specs/...`). The change file is `.docket/docs/changes/active/0411-steer-post-completion-durable-write-failures-to-rebase-conti.md`.

## Global Constraints

- **No behavior change** to `finalize.rebase-continue` / `finalize.rebase-abort` control flow, reservation admission or limits, refunds, gate/evidence policy, merge authorization, or the generated-bundle fast path. Only `message` strings change in Go.
- Keep protocol version, operation, result, disposition, reason (`receipt-write-failed`), identity/count fields, and receipt schema unchanged. No new receipt fields, probes, retry machinery, or recovery classification.
- `internal/app/finalize_reserve.go` is a **negative control**: read it, change nothing. Pre-dispatch reserve-write failures and the pre-continue started-marker write failure must NOT acquire the post-completion recovery claim.
- No general `HumanText` message renderer; `HumanText` continues to omit `Message`. The message travels in the existing JSON result.
- Skill prose is harness-neutral: owned attempt, resolver report, operation, native child-completion evidence. No vendor tool names, approval syntax, session ids, model names, or timing assumptions (learning: distributed-body-has-no-local-repo).
- Never hand-edit files under `internal/assets/embedded/` — regenerate via the existing generator.
- The full build gate at the end is `go run ./cmd/docket development test`, run from the feature worktree root.
- Always run Go tests with `-count=1` when verifying a mutation or a just-edited tree (learning: cached-runner-serves-a-mutated-tree).
- Commit per task on the current feature branch `docs/steer-post-completion-durable-write-failures-to-rebase-conti`; stage only the files you touched (never `git add -A` — the worktree is shared with docket metadata tooling).

---

### Task 1: Reword the three reservation-reconciliation write-failure diagnostics (TDD)

**Files:**
- Modify: `internal/app/finalize_rebase.go` (the reconcile write in `finalizeRebaseContinueBudgeted`, ~line 1067; the advanced-conflict branch of `finalizeRebaseReconcileStarted`, ~line 1108; its completed-rebase branch, ~line 1124-1128)
- Test: `internal/app/finalize_rebase_test.go` (new tests + one fault-injection wrapper; existing helpers reused)

**Interfaces:**
- Consumes: `FinalizeWorkspace` seam (`internal/app/finalize_context.go:74`, method `WriteRebaseReceipt(ctx, dir, r) error`); test helpers `reserveOnConflict(t, limit)`, `completedBudgetedReceipt(t, seed)`, `seedReserveReceipt`, `stageSeam` (with its `script *gitcli.RebaseStatus` field), `reloadReceipt(t, f)`, `fakeGate` (has a `calls` counter), `writeRepoFile`; error sentinel pattern from `writeFailWorkspace` in `internal/app/finalize_reserve_test.go:86-101` (same package — directly visible).
- Produces: unexported const `reconcileRecoveryRemedy` (string) in `finalize_rebase.go`; test wrapper `reconcileFailWorkspace` in `finalize_rebase_test.go`. Task 2's doc contracts name the same operation string `finalize.rebase-continue` (producer/consumer pairing; learning: specified-but-unreachable).

The three current sites (all keep `ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite` and their `withResolverCounts` receipt argument — `started` at the first site, `rec` at the other two):

1. `finalizeRebaseContinueBudgeted`, after `StageAndContinueRebase` succeeded: message is currently `"the continue completed but the reservation could not be reconciled on the receipt: "+werr.Error()`. Per the spec this wording is wrong twice over: it may not claim the whole rebase finished (another conflict may remain), and it names no remedy.
2. `finalizeRebaseReconcileStarted`, `RebaseConflicted` branch with a different stopped commit (advanced): message is currently `werr.Error()` alone.
3. `finalizeRebaseReconcileStarted`, `RebaseUnchanged` branch with head descending the base (completed): message is currently `werr.Error()` alone.

- [ ] **Step 1: Add the fault-injection wrapper to `finalize_rebase_test.go`**

Place it near `stageSeam` (~line 784). It must let the started-marker write (the first write in a continue) succeed while the reconcile write (the second) fails, and be disarmable so a retry can succeed:

```go
// errReconcileWrite is the injected durable-write failure for the
// reservation-reconciliation sites (change 0411).
var errReconcileWrite = errors.New("reconcile write boom")

// reconcileFailWorkspace wraps the real FinalizeWorkspace and faults
// WriteRebaseReceipt after `allow` successful writes while `fail` is set, so a
// test can let the continuation-started marker land durably and then fail only
// the reservation reconciliation. Clearing `fail` restores writes for the
// recovery retry. Every other seam method delegates to the embedded service.
type reconcileFailWorkspace struct {
	FinalizeWorkspace
	allow  int  // writes that pass through before faulting
	fail   bool // fault writes past allow while set
	writes int  // observed write count
}

func (w *reconcileFailWorkspace) WriteRebaseReceipt(ctx context.Context, dir string, r workspace.RebaseReceipt) error {
	w.writes++
	if w.fail && w.writes > w.allow {
		return errReconcileWrite
	}
	return w.FinalizeWorkspace.WriteRebaseReceipt(ctx, dir, r)
}
```

- [ ] **Step 2: Write the failing tests (all five below), run them, verify they fail on the message asserts**

Append after `TestFinalizeRebaseContinueStartedCompletedRecovers` (~line 1140). Every message assert pins the mechanism, not just "it failed" (learning: assert-pins-outcome-not-mechanism): it checks the remedy operation name, the same-attempt/report reuse phrase, the underlying error detail, and — where the spec forbids it — the absence of a whole-rebase-completed claim.

**Test A — AC1, immediate post-continue failure, rebase actually completed** (real Git: `reserveOnConflict` has one conflicted commit, so resolving `feature.txt` and continuing completes the rebase):

```go
// TestFinalizeRebaseContinueReconcileWriteFailurePreserves (change 0411, AC1)
// proves a receipt-write failure injected ONLY at post-continue reservation
// reconciliation (the started marker landed durably first) keeps the unchanged
// error result/disposition/reason, emits a message naming the same-attempt
// finalize.rebase-continue remedy WITHOUT claiming the whole rebase finished,
// preserves the outstanding reservation + started marker + used count, and never
// runs the gate before reconciliation succeeds.
func TestFinalizeRebaseContinueReconcileWriteFailurePreserves(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2) // used == 1
	ctx := context.Background()
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n") // resolve so the continue completes
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 1, fail: true} // marker write passes, reconcile write faults
	deps.Workspace = ws
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed}}
	deps.Gate = gate

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Disposition != RebaseDispBlocked || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("continue = (%q, %q, %q), want external-failed/blocked/%q unchanged", res.Result, res.Disposition, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "finalize.rebase-continue") ||
		!strings.Contains(res.Message, "same change id, owned attempt, and original resolved report") ||
		!strings.Contains(res.Message, errReconcileWrite.Error()) {
		t.Errorf("message %q must name the same-attempt finalize.rebase-continue remedy and keep the write error", res.Message)
	}
	if strings.Contains(res.Message, "rebase completed") {
		t.Errorf("message %q may not assert the whole rebase finished at the post-continue site", res.Message)
	}
	if gate.calls != 0 {
		t.Errorf("gate ran %d time(s) before reconciliation succeeded; want 0", gate.calls)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != token || rec.ResolverContinuationStarted != "1" || rec.ResolverUsed != "1" {
		t.Errorf("receipt after failed reconcile: token %q cont %q used %q, want reservation + started marker + used preserved", rec.ResolverReservationToken, rec.ResolverContinuationStarted, rec.ResolverUsed)
	}
}
```

Note on `deps.Gate`: check the field name `FinalizeRebase`'s gate seam actually uses in `FinalizeDeps` (`internal/app/finalize_context.go`) — `f.finalizeDeps(gh, gate)` wires it in other tests; here `reserveOnConflict` built `deps` already, so set the same field it populates (read `finalizeDeps` at `finalize_rebase_test.go:231` and mirror it). Also note: the rewritten HEAD preservation is implicit — the operation performs no Git mutation after the continue; assert Git is no longer in a rebase by probing with the fixture's client only if a ready-made helper exists (`liveStoppedCommit` exists for the stopped case; do not add new Git plumbing just for this).

**Test B — AC3, immediate post-continue failure with another conflict remaining** (scripted continue, mirroring `TestFinalizeRebaseContinueNextConflictUnderBudget`):

```go
// TestFinalizeRebaseContinueReconcileWriteFailureNextConflict (change 0411, AC3)
// proves the post-continue reconcile-write failure message stays honest when the
// continue surfaced ANOTHER conflict: same remedy, no completion claim, receipt
// retained with the reservation outstanding.
func TestFinalizeRebaseContinueReconcileWriteFailureNextConflict(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f,
		script: &gitcli.RebaseStatus{Disposition: gitcli.RebaseConflicted, HeadOID: gitcli.ObjectID(strings.Repeat("c", 40)), UnmergedPaths: []string{"feature.txt"}}}
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 1, fail: true}
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("continue = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "finalize.rebase-continue") || strings.Contains(res.Message, "rebase completed") {
		t.Errorf("message %q must name the remedy and never claim completion while a conflict remains", res.Message)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != token || rec.ResolverContinuationStarted != "1" {
		t.Errorf("receipt lost the outstanding reservation: token %q cont %q", rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
}
```

**Test C — AC2, completed-rebase recovery branch: failed write, then successful retry** (extends the `TestFinalizeRebaseContinueStartedCompletedRecovers` fixture):

```go
// TestFinalizeRebaseContinueStartedCompletedRecoveryWriteFails (change 0411, AC2)
// proves the completed-rebase recovery branch's failed reconciliation write emits
// the completed-specific remedy message, repeats no staging, preserves the receipt
// — and that restoring writes and retrying the SAME report recovers: reservation
// cleared only by the existing recovery, used preserved, gate composed.
func TestFinalizeRebaseContinueStartedCompletedRecoveryWriteFails(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		r.ResolverContinuationStarted = "1"
	})
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps := f.finalizeDeps(gh, &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}})
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true} // the recovery's one write faults
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: real.Attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: real.ResolverReservationToken}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, real.Attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("recovery write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "owned rebase completed") || !strings.Contains(res.Message, "finalize.rebase-continue") ||
		!strings.Contains(res.Message, errReconcileWrite.Error()) {
		t.Errorf("message %q must say the owned rebase completed, name the remedy, and keep the write error", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("recovery staged %d time(s); want 0", seam.calls)
	}
	if rec := reloadReceipt(t, f); rec.ResolverReservationToken == "" || rec.ResolverContinuationStarted != "1" || rec.ResolverUsed != real.ResolverUsed {
		t.Errorf("failed recovery mutated the receipt: %+v", rec)
	}

	ws.fail = false // durable writes restored — retry the SAME report
	res2 := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, real.Attempt, report)
	if res2.Result != ResultApplied || res2.Gate == nil {
		t.Fatalf("retry = %q gate %+v (reason %q), want applied with gate", res2.Result, res2.Gate, res2.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("retry staged %d time(s); want 0 (no repeated Git continuation)", seam.calls)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" || rec.ResolverUsed != real.ResolverUsed {
		t.Errorf("retry left receipt %+v; want reservation cleared, used %q preserved (no charge, no refund)", rec, real.ResolverUsed)
	}
}
```

**Test D — AC3, advanced-conflict recovery branch: failed write, then retry surfaces the conflict** (mirrors `TestFinalizeRebaseContinueStartedAmbiguousRetains` but with a stopped commit DIFFERENT from the reserved one, so recovery proves advancement):

```go
// TestFinalizeRebaseContinueStartedAdvancedRecoveryWriteFails (change 0411, AC3)
// proves the advanced-conflict recovery branch's failed reconciliation write says
// the continuation advanced — never that the rebase completed — and a retry after
// restoring writes surfaces the next conflict without replaying the continuation.
func TestFinalizeRebaseContinueStartedAdvancedRecoveryWriteFails(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	// The receipt records a DIFFERENT stopped commit than the live rebase, so the
	// started continuation provably advanced.
	seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverReservationStopped = strings.Repeat("d", 40)
		r.ResolverContinuationStarted = "1"
	})
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true}
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("advanced recovery write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "advanced to another conflict") || strings.Contains(res.Message, "rebase completed") ||
		!strings.Contains(res.Message, "finalize.rebase-continue") {
		t.Errorf("message %q must say advanced-to-another-conflict, name the remedy, and never claim completion", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("recovery staged %d time(s); want 0", seam.calls)
	}

	ws.fail = false
	res2 := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res2.Result != ResultApplied || res2.Disposition != RebaseDispConflicted {
		t.Fatalf("retry = (%q, %q, reason %q), want applied/conflicted surfacing the live conflict", res2.Result, res2.Disposition, res2.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("retry replayed the continuation %d time(s); want 0", seam.calls)
	}
}
```

(Adjust the exact seed helper spelling to what `TestFinalizeRebaseContinueStartedAmbiguousRetains` at ~line 1056 actually does — it calls `seedReserveReceipt(t, f, ...)` returning `before`; reuse identically. If the retry's conflicted result differs in shape — e.g. exhaustion at `used == limit` — assert the existing exhaustion result instead; the spec accepts "surfaces the next conflict or existing exhaustion result".)

**Test E — AC4 negative boundary: the pre-continue started-marker write failure carries NO recovery remedy:**

```go
// TestFinalizeRebaseContinueMarkerWriteFailureNoRecoveryClaim (change 0411, AC4)
// proves a write failure BEFORE Git ran (the continuation-started marker) does not
// acquire the post-completion recovery remedy: same reason, no remedy phrase.
func TestFinalizeRebaseContinueMarkerWriteFailureNoRecoveryClaim(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	deps.Workspace = &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true} // the FIRST write (marker) faults

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("marker write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if strings.Contains(res.Message, "finalize.rebase-continue") {
		t.Errorf("pre-continue marker write failure %q must not carry the post-completion recovery remedy", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("a failed marker write staged %d time(s); want 0", seam.calls)
	}
}
```

Run: `go test ./internal/app/ -run 'TestFinalizeRebaseContinue(ReconcileWriteFailure|Started.*WriteFails|MarkerWriteFailure)' -count=1`
Expected: FAIL — Tests A–D fail on the missing remedy phrases (the current messages are `werr.Error()` alone or the old "continue completed" wording); Test E should PASS already (write it anyway; it is the boundary pin that must stay green through the change — if it fails, stop and re-read the site). Fix any compile errors from helper-name drift first (the helpers named here were verified against the current tree, but confirm signatures as you go).

- [ ] **Step 3: Implement the message changes in `finalize_rebase.go`**

Add one shared remedy const near `rebaseRefusal` (~line 258):

```go
// reconcileRecoveryRemedy is the shared recovery tail for the three
// reservation-reconciliation write-failure diagnostics (change 0411): Git already
// advanced or completed the owned continuation, so the remedy is the SAME
// operation with the SAME inputs — never rebase-abort, which would discard the
// completed local rewrite. Recovery (finalizeRebaseReconcileStarted) rechecks
// live state and reconciles without replaying Git or charging the budget.
const reconcileRecoveryRemedy = "; retry the finalize.rebase-continue operation with the same change id, owned attempt, and original resolved report (including its resolver_reservation token) — recovery rechecks live state and reconciles the outstanding continuation; it does not authorize another resolver dispatch"
```

Site 1 — in `finalizeRebaseContinueBudgeted`, replace the reconcile-write refusal message (keep everything else identical, including `withResolverCounts(..., started)`):

```go
	reconciled := clearResolverReservation(started)
	if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, reconciled); werr != nil {
		return withResolverCounts(rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite,
			"the Git continuation returned, but its reservation reconciliation could not be persisted on the receipt (another conflict may remain): "+werr.Error()+reconcileRecoveryRemedy, id), started)
	}
```

Site 2 — in `finalizeRebaseReconcileStarted`, `RebaseConflicted` branch (advanced; keep `withResolverCounts(..., rec)`):

```go
		reconciled := clearResolverReservation(rec)
		if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, reconciled); werr != nil {
			return withResolverCounts(rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite,
				"the prior continuation advanced to another conflict, but its reservation reconciliation could not be persisted on the receipt: "+werr.Error()+reconcileRecoveryRemedy, id), rec)
		}
```

Site 3 — in `finalizeRebaseReconcileStarted`, `RebaseUnchanged` branch (completed; keep `withResolverCounts(..., rec)`):

```go
		reconciled := clearResolverReservation(rec)
		if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, reconciled); werr != nil {
			return withResolverCounts(rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite,
				"the owned rebase completed, but its reservation reconciliation could not be persisted on the receipt: "+werr.Error()+reconcileRecoveryRemedy, id), rec)
		}
```

Touch nothing else: no change to the started-marker write refusal (~line 1053), the ambiguous/unresolvable branches, `finalize_reserve.go`, `mapContinuedRebase`, or any `HumanText`.

- [ ] **Step 4: Run the new tests and the full rebase/reserve test files**

Run: `go test ./internal/app/ -run 'TestFinalizeRebase|TestFinalizeResolver' -count=1`
Expected: PASS — the five new tests green, and every existing continuation/reserve boundary test (ReservationMissing/StaleToken/StaleCommit, MarksStartedBeforeStaging, NextConflictUnderBudget/Exhausted, LegacyRefuses, RepeatedConsumedReservation, StartedAmbiguousRetains, StartedCompletedRecovers, and the reserve write-failure test in `finalize_reserve_test.go`) still green, proving AC4's retained refusals and mutation protections.

- [ ] **Step 5: Mutation-check the message guards** (learning: assert-detects-removal-not-replacement)

Temporarily replace `reconcileRecoveryRemedy`'s operation name with `finalize.rebase-abort` in the const; run the Step 4 command with `-count=1`; expect Tests A–D to redden on the missing `finalize.rebase-continue` phrase. Revert the mutation (undo the edit by hand — do NOT `git checkout --` over uncommitted work; learning: mutation-restore-needs-a-backup-copy). Re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_test.go
git commit -m "fix(finalize): steer post-completion reconcile-write failures to rebase-continue (change 0411)"
```

---

### Task 2: The skill-doc recovery exception + repoguard prose contracts and budget ceilings (TDD)

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (inside section `### 3. Rebase onto the effective base (resolver loop)`, within the existing `<!-- docket:feature-dispatch:start ... -->` / `<!-- docket:feature-dispatch:end -->` markers — keep both markers exactly as they are and balanced)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (new subsection + one line in `## abort-and-report points (the full set)`)
- Modify: `internal/repoguard/budgets_test.go` (~lines 173-174: the two ceiling rows)
- Test: `internal/repoguard/prose_contracts_test.go` (new section-bound contract table + test)

**Interfaces:**
- Consumes: `docSectionContract` type and `scanDocSection(content string, c docSectionContract) []string` from `prose_contracts_test.go` (~lines 372-415) — clauses matched whitespace-collapsed via `collapseWS`, bound to a `[section, terminator)` heading pair (reflow-tolerant, per AC5 and learnings phrase-grep-over-wrapped-prose / prose-guard-binds-phrase-to-claim). `guardRoot(t)` resolves the repo root.
- Produces: `rebaseRecoveryDocContracts []docSectionContract` and `TestRebaseRecoveryDocContracts` — nothing downstream consumes them; Task 3's regenerated assets embed the two edited docs.

- [ ] **Step 1: Write the failing repoguard test**

Append to `prose_contracts_test.go`, after `TestUninstallCollectionDocContracts`:

```go
// change 0411 — the reconciliation-write recovery exception: a post-completion
// durable-write failure re-enters finalize.rebase-continue with the same inputs
// and is never routed to rebase-abort. Each clause is bound to its owning
// section (prose-guard-binds-phrase-to-claim) and matched whitespace-collapsed
// (phrase-grep-over-wrapped-prose), so a pure re-flow stays green while removing
// the exception, or substituting abort as the remedy, goes red.
var rebaseRecoveryDocContracts = []docSectionContract{
	{change: "change_0411_recovery_exception_skill", file: "skills/docket-finalize-change/SKILL.md",
		section: "### 3. Rebase onto the effective base (resolver loop)", terminator: "### 4. The local gate",
		present: []string{
			"re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"never route this persistence failure to `finalize.rebase-abort`",
		}},
	{change: "change_0411_recovery_exception_reference", file: "skills/docket-finalize-change/references/gate-failure.md",
		section: "## The reconciliation-write exception (recover, not abort)", terminator: "## The finalize gate shares the worktree's one execution slot",
		present: []string{
			"Preserve the workspace, the receipt, and the original resolver report",
			"Re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"an operator remedy, not an autonomous retry loop",
			"resumes via the original identical `finalize.rebase` invocation",
		}},
	{change: "change_0411_recovery_not_in_abort_set", file: "skills/docket-finalize-change/references/gate-failure.md",
		section: "## abort-and-report points (the full set)", terminator: "## The reconciliation-write exception (recover, not abort)",
		present: []string{
			"a reservation-reconciliation write failure after Git advanced or completed the owned continuation is **not** in this set",
		}},
}

// TestRebaseRecoveryDocContracts binds the change 0411 recovery-exception
// clauses to their sections in both finalize documents; scanDocSection's
// missing-section / missing-terminator / missing-clause branches are exercised
// by TestUninstallCollectionDocContracts' non_vacuity subtest.
func TestRebaseRecoveryDocContracts(t *testing.T) {
	root := guardRoot(t)
	cache := map[string]string{}
	var violations []string
	for _, c := range rebaseRecoveryDocContracts {
		content, ok := cache[c.file]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatalf("read contract file %s (%s): %v (fail closed)", c.file, c.change, err)
			}
			content = string(b)
			cache[c.file] = content
		}
		for _, msg := range scanDocSection(content, c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	if len(violations) != 0 {
		t.Errorf("recovery-exception doc contracts (%d violations):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
```

Mirror the exact existing-test spelling for reading files and reporting (copy the loop shape from `TestUninstallCollectionDocContracts` in the same file). Note the third contract's terminator IS the new section heading — that also pins the exception section's position after the abort set, so deleting the new section reddens two contracts.

Run: `go test ./internal/repoguard/ -run TestRebaseRecoveryDocContracts -count=1`
Expected: FAIL — every clause missing (the docs are unedited).

- [ ] **Step 2: Edit `skills/docket-finalize-change/SKILL.md`**

In section 3, insert a new paragraph immediately AFTER the existing **Abort flow** paragraph (the one beginning "A resolver that returns `disposition: stuck`…") and BEFORE `<!-- docket:feature-dispatch:end -->`. Verify marker balance before and after the edit (both `start`/`end` markers present, in order, once). Insert exactly:

```markdown
**Reconciliation-write exception (not an abort case).** A `finalize.rebase-continue` refusal (`receipt-write-failed`) whose message says Git advanced to another conflict or completed the owned rebase but the reservation reconciliation could not be persisted means the work is done and only the durable bookkeeping is behind: preserve the workspace, receipt, and original resolver report, and re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>` — it rechecks live state and reconciles the outstanding continuation without authorizing another resolver dispatch. Never route this persistence failure to `finalize.rebase-abort`, restart the rebase, reserve or dispatch another resolver, fabricate a replacement report, or edit the receipt; a generic `blocked` or the `receipt-write-failed` token alone does not diagnose this window — read the message. If persistence still fails or the original report is unavailable, this is `halted` with the workspace retained (record the block per step 8 where possible; a failed block recording never authorizes abort). Follow the recovery's actual result as usual — a new conflict re-enters the reserve step, `waiting` re-enters through `finalize.rebase`, and a successful reconciliation consumes the reservation, so stop replaying the old report. Full flow: *The reconciliation-write exception* in `references/gate-failure.md`.
```

Wording notes (bind them, the doc contracts check the load-bearing spans): keep the two guarded clauses verbatim — "re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`" and "never route this persistence failure to `finalize.rebase-abort`" (case-fold the leading N as written above: the contract phrase starts mid-sentence, so spell the guarded text exactly as the contract's lowercase-n "never route this persistence failure to `finalize.rebase-abort`" — the paragraph above satisfies it inside "Never route this…" ONLY if the case matches; `scanDocSection` is case-sensitive, so either lower-case the contract phrase's first word context or rephrase the doc sentence to contain the exact lowercase span, e.g. "…and never route this persistence failure to `finalize.rebase-abort`…". Pick one and keep test and doc consistent.)

- [ ] **Step 3: Edit `skills/docket-finalize-change/references/gate-failure.md`**

(a) In `## abort-and-report points (the full set)`, after the intro line "Each maps to the **`halted`** disposition…", add one line:

```markdown
Two outcomes look abort-shaped and are not: a `waiting` (`reason: gate-waiting`) re-enters `finalize.rebase` (below), and a reservation-reconciliation write failure after Git advanced or completed the owned continuation is **not** in this set — see *The reconciliation-write exception* below.
```

(Do not disturb the existing sentence "A `waiting` (`reason: gate-waiting`) is not in this set" if it lives in this section — it is guarded by `test_finalize_gate_waiting`. Read the section first; if that sentence already covers waiting, add only the reconciliation clause to it or as its own sentence beside it, keeping the guarded span byte-intact.)

(b) Insert a new section between `## abort-and-report points (the full set)` and `## The finalize gate shares the worktree's one execution slot`:

```markdown
## The reconciliation-write exception (recover, not abort)

An owned resolver continuation can succeed in Git — advancing to another conflict, or completing the rebase — and then fail only the durable write that reconciles the reservation on the receipt (`receipt-write-failed`, with a message naming this exception and the failed write). Aborting here restores the recorded original head and discards the completed local rewrite, so this window is recovered, never aborted:

1. Preserve the workspace, the receipt, and the original resolver report. Re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>` (the report still carries its `resolver_reservation` token). Resolve the invocation from capabilities and the report shape from schema as usual. Recovery rechecks live state and reconciles the outstanding continuation; it charges and refunds nothing and authorizes no new resolver dispatch.
2. Do not route this persistence failure to `finalize.rebase-abort`, restart the rebase, reserve or dispatch another resolver, fabricate a replacement report, or edit or delete the receipt. A generic `blocked` disposition or the `receipt-write-failed` token alone is insufficient to diagnose this window — the operation's message says which write failed and what it proved; its ownership and live-state checks stay authoritative, so never reproduce them with handwritten Git probes.
3. This is an operator remedy, not an autonomous retry loop. If persistence still fails, or the original report is unavailable, halt with the work retained: report the actual diagnostic and the missing input, and record the block through the existing finalize-block path where possible — a failed block recording is reported honestly and never authorizes abort.
4. Follow the recovery's actual result. A new conflict requires normal reserve-before-dispatch admission; an exhausted budget keeps its existing abort/halt route; a completed rebase still passes the normal gate and publication checks; a `waiting` (`gate-waiting`) resumes via the original identical `finalize.rebase` invocation, never another `rebase-continue` or a direct gate-drive call. A successful reconciliation consumes the reservation — do not replay the old report afterward.
5. Everything else keeps its verified abort route: stuck or unavailable resolvers, a continuation still stopped on the same commit, foreign or unprovable state, legacy receipts, and exhausted budgets. Establish resolver-child completion before any abort, as ever; this exception never authorizes abort on an unproven state or bypasses an existing refusal. Write failures before Git ran (reserve admission, the continuation-started marker) prove nothing about completion and carry no recovery claim.
```

Keep every guarded clause from Step 1's contracts verbatim inside this section (adjust the section heading in BOTH the doc and the test table if you reword it — they must match exactly, including the terminator pinning).

- [ ] **Step 4: Bump the two budget ceilings in `internal/repoguard/budgets_test.go`**

Measure after editing: `grep -c "" <file>` for lines, `wc -w <file>` for words, run on both docs. Current actuals/ceilings: SKILL.md 237/5230 against 238/5232; gate-failure.md 135/1471 against 135/1472 — both will breach. Update the two rows, setting each ceiling to the NEW measured actuals (house style: ceiling ~= actual, dated change note prepended to the comment), e.g.:

```go
	{"docket-finalize-change/SKILL.md", <new-lines>, <new-words>},                   // 0411: +reconciliation-write recovery exception paragraph in the resolver loop (ceilings 238/5232 -> <new>); 0413: ... (keep the existing comment tail)
	{"docket-finalize-change/references/gate-failure.md", <new-lines>, <new-words>}, // 0411: +reconciliation-write exception section and abort-set carve-out (ceilings 135/1472 -> <new>); 0413: ... (keep the existing comment tail)
```

The exception prose is a real budget spend — keep both additions tight; if the SKILL.md paragraph can be tightened without losing a guarded clause, tighten it (the reference carries the full flow).

- [ ] **Step 5: Run the repoguard suite**

Run: `go test ./internal/repoguard/ -count=1`
Expected: PASS — `TestRebaseRecoveryDocContracts` green, `TestProseContracts` (including `test_finalize_gate_waiting`'s guarded spans) green, budget rows green.

- [ ] **Step 6: Mutation-test the doc guards** (AC5; learning: guards-are-code)

Three mutations, each followed by `go test ./internal/repoguard/ -run 'TestRebaseRecoveryDocContracts|TestProseContracts' -count=1`, each reverted by hand afterward:
1. Delete the new gate-failure.md section → expect red (missing section AND the abort-set contract's missing terminator).
2. In the SKILL.md paragraph, replace the remedy operation `finalize.rebase-continue` with `finalize.rebase-abort` → expect red (missing required clause).
3. Re-wrap one guarded gate-failure.md clause across a line break (insert a newline mid-phrase, keeping the words) → expect GREEN (reflow tolerance; `collapseWS` matching). Revert.
If any mutation fails to behave as expected, fix the contract before proceeding — a green mutation on 1 or 2 is a vacuous guard.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md internal/repoguard/budgets_test.go internal/repoguard/prose_contracts_test.go
git commit -m "docs(finalize): reconciliation-write recovery exception in skill + reference, with section-bound guards (change 0411)"
```

---

### Task 3: Regenerate embedded assets and run the full gate

**Files:**
- Modify (generated): `internal/assets/embedded/skills/docket-finalize-change/SKILL.md`, `internal/assets/embedded/skills/docket-finalize-change/references/gate-failure.md`, and the bundle manifest the generator rewrites (whatever `cmd/genassets` touches — commit exactly what it changes, nothing hand-edited)

**Interfaces:**
- Consumes: the maintained docs from Task 2; the generator entry `//go:generate go run ../../cmd/genassets -repo ../..` in `internal/assets/generate.go`.
- Produces: an embedded bundle in sync with maintained source, which the assets drift tests (`internal/assets`, `internal/app` generated tests) verify.

- [ ] **Step 1: Regenerate**

Run from the worktree root: `go generate ./internal/assets`
Expected: exit 0; `git status --porcelain` shows only files under `internal/assets/embedded/` (and any generator-owned manifest). If anything OUTSIDE generated paths changed, stop and investigate — never hand-adjust embedded copies.

- [ ] **Step 2: Focused verification**

Run: `go test ./internal/assets/ ./internal/app/ ./internal/repoguard/ -count=1`
Expected: PASS (drift guards accept the regenerated bundle; Task 1 and Task 2 tests still green).

- [ ] **Step 3: Full build gate**

Run: `go run ./cmd/docket development test`
Expected: green suite. Read the budget report even on green: any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding and a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach — record either in the run's notes for the results artifact; a machine-dependent parallel wall-clock number alone is not a breach (`tests/README.md`).

- [ ] **Step 4: Commit**

```bash
git add internal/assets/embedded
git commit -m "chore(assets): regenerate embedded finalize skill bundle (change 0411)"
```

---

## Acceptance-criteria map (spec §Acceptance criteria → tasks)

1. Failure after Git completion → Task 1 Test A (plus Test B for the message-honesty half).
2. Recovery, including another failed write, then successful retry → Task 1 Test C.
3. Advanced conflict, both failure points, no completion claim → Task 1 Tests B and D.
4. Negative boundaries → Task 1 Test E + the existing reserve/continuation boundary tests re-run in Task 1 Step 4 (reused, not duplicated, per the spec).
5. Reachable, consistent guidance; reflow-tolerant, mutation-proven prose guards; WAITING distinction; unchanged abort routes → Task 1 message asserts (producer) + Task 2 contracts and mutations (consumer).
6. Repository validation: regenerate assets, focused tests, full configured gate → Task 3. (The results artifact recording actual outcomes is the build/run loop's own required artifact, written at implementation time — not a plan task.)

## Known risks

- **Helper-signature drift:** the fixture helpers (`reserveOnConflict`, `seedReserveReceipt`, `finalizeDeps` field names) were verified against HEAD `3ccf9fac`; if the tree moved, mirror the neighboring tests rather than this plan's exact spellings — the asserts, not the plumbing, are the contract.
- **Guarded-span collisions:** `test_finalize_gate_waiting` (repoguard `prose_contracts_test.go` ~line 111) pins exact phrases in both docs; Task 2 edits must leave those spans byte-intact.
- **Budget pressure:** both doc ceilings sit at their actuals; Task 2 Step 4 bumps them with the dated 0411 note — a red budget row after editing means you forgot the bump or the prose grew more than measured (learning: budget-headroom-is-spent-before-it-is-breached).

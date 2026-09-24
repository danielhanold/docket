# Same-Scope First-Start Loser Must Not Rotate the Winner's Executing Slot — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In `admitScopedWorktree`'s `admissionExecuting` case, only a start carrying a predecessor receipt may rotate the slot; a receipt-less first start refuses typed `ErrScopeSecondDrive` without touching the winner's executing reservation.

**Architecture:** One receipt guard at the top of the `admissionExecuting` switch arm in `internal/gatedrive/driver.go`, a deterministic regression test pinning the late-loser interleaving that today only `TestBarrierSameScopeFirstStartContention` catches (~48% of runs under `GOMAXPROCS=1 -race`), and a doc-comment update saying rotation is successor-only. The refusal path is already leak-free: `admitScoped` returns an admission error before any drive record or worktree reservation is minted for the loser.

**Tech Stack:** Go, `go test -race`, existing gatedrive test helpers (`scopedTestDriver`, `prepareScopedStart`, `fakeProc`, `fakeClock`, `stableGit`, `isOwnershipKind`, `testsupport.TempDir`).

**Spec:** Trivial change — no separate spec. The change body is the scope: `docs/changes/active/0452-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex.md` (metadata worktree). Its "What changes" section: (1) receipt guard in the `admissionExecuting` case, (2) deterministic regression test for the late-loser interleaving that fails without the guard, (3) doc comment updated to say rotation is successor-only.

## Global Constraints

- All work happens in the feature worktree `/Users/homer/dev/docket/.worktrees/same-scope-first-start-loser-must-not-rotate-the-winner-s-ex` on branch `fix/same-scope-first-start-loser-must-not-rotate-the-winner-s-ex`.
- Every `go test` invocation passes `-count=1` (repo learning: the Go test cache can serve a green PASS against a tree you just mutated).
- The write-test-first step is the mutation check: the new test must FAIL against the unguarded code before the guard lands (repo rule: a guard is code; strip the thing it guards, watch it redden).
- Cross-references in comments anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- The build gate at the end of the build runs the whole suite via the configured `build.test_command`; this plan's per-task runs are focused, not a substitute.

## Background for the implementer (read before Task 1)

The race, as traced in the change body: two receipt-less (`req.PredecessorDriveID == ""`) same-scope first starts both pass `precheckScopedStart` while the scope slot is still empty. The winner reserves the worktree slot, launches, and confirms the slot to `executing`. The loser then enters `admitScopedWorktree` (`internal/gatedrive/driver.go`), finds `ErrWorktreeBusy`, sees the slot held by its own scope in state `admissionExecuting`, and — because that arm currently rotates for ANY same-scope start — calls `rotateWorktreeExecutionForSuccessor`, replacing the winner's `ReservationToken` and bumping `ExecutionGen` while the winner's run is live. The loser then loses `reserveScopeDrive` with `ErrScopeSecondDrive`; `isSameScopeRaceLoss` treats that as "a peer adopted my reservation," so nothing is released and the winner's live run is stranded under a `reserved` slot with a foreign token. The observable symptom is `TestBarrierSameScopeFirstStartContention` failing with `the winner's worktree slot must be executing, got "reserved"`.

The fix: rotation in the `admissionExecuting` arm is for successors only — a start presenting a predecessor receipt. A receipt-less start reaching an executing same-scope slot has, by definition, raced an already-launched same-scope drive; that is exactly the condition `precheckScopedStart` names `ErrScopeSecondDrive` (see its comment: "ErrScopeSecondDrive (a launched drive with no successor receipt)"), so the admission arm refuses with the same typed error, constructed the way the package constructs all ownership errors: `ownershipErr(ErrScopeSecondDrive, "start")` (`ownershipErr` lives in `internal/gatedrive/ownership.go`; it returns a `*OwnershipError` with the given `Kind` and op). The refusal is consistent with `isSameScopeRaceLoss`, which already classifies `ErrScopeSecondDrive` as a same-scope race loss — but that classifier only matters on the `reserveScopeDrive` path; the admission refusal returns before any reservation exists, so there is nothing to release (see `admitScoped`: `if aerr != nil { return nil, aerr }` precedes `NewReservedDrive`).

---

### Task 1: Deterministic late-loser regression test + receipt guard + doc comment

**Files:**
- Modify: `internal/gatedrive/driver.go` (the `admitScopedWorktree` function: its doc comment and its `admissionExecuting` switch arm)
- Test: `internal/gatedrive/driver_concurrency_test.go` (append the new test after `TestBarrierSameScopeFirstStartContention`)

**Interfaces:**
- Consumes: `d.admitScopedWorktree(req StartRequest) (token string, reservedFresh, ownsSlot, rotated bool, legacy *LegacyHistorySummary, err error)` — package-private, called directly by the test to pin the exact interleaving; `store.LoadWorktreeExecution(worktree)` returning a slot with `State admissionState`, `ExecutionGen int`, `ReservationToken string`; helpers `scopedTestDriver(store, clk, proc, git)`, `prepareScopedStart(t, store)` (returns `(ScopeGrant, StartRequest)`), `stableGit()`, `isOwnershipKind(err, kind)`, `ownershipErr(kind, op)`.
- Produces: the guarded `admissionExecuting` arm — a receipt-less request returns `("", false, false, false, nil, ownershipErr(ErrScopeSecondDrive, "start"))` and leaves the slot record byte-identical. Task 2 relies on this making `TestBarrierSameScopeFirstStartContention` deterministic-green.

- [ ] **Step 1: Write the failing regression test**

Append to `internal/gatedrive/driver_concurrency_test.go`, directly after `TestBarrierSameScopeFirstStartContention` (whose flaky interleaving this test pins deterministically):

```go
// TestSameScopeFirstStartLateLoserDoesNotRotate deterministically pins the losing
// interleaving of TestBarrierSameScopeFirstStartContention: a receipt-less first
// start that passed precheckScopedStart before the winner published the scope, and
// whose worktree admission runs only AFTER the winner confirmed the slot to
// executing. Rotation is successor-only, so the late loser must be refused typed
// ErrScopeSecondDrive with the winner's executing reservation untouched — same
// state, same token, same ExecutionGen.
func TestSameScopeFirstStartLateLoserDoesNotRotate(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)

	// The winner: a full first start that launches and confirms its slot.
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("winner Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("winner must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	before, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if before.State != admissionExecuting {
		t.Fatalf("precondition: the winner's slot must be executing, got %q", before.State)
	}

	// The late loser: the SAME receipt-less request reaches worktree admission only
	// now. Calling admitScopedWorktree directly models the loser that already passed
	// its precheck against the then-empty scope; admission must refuse typed and
	// must not rotate the winner's live reservation.
	_, _, _, _, _, aerr := d.admitScopedWorktree(req)
	if !isOwnershipKind(aerr, ErrScopeSecondDrive) {
		t.Fatalf("a late receipt-less first start must refuse ErrScopeSecondDrive, got %v", aerr)
	}
	after, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	if after.State != admissionExecuting || after.ReservationToken != before.ReservationToken || after.ExecutionGen != before.ExecutionGen {
		t.Fatalf("the winner's executing reservation must be untouched: state %q->%q, token changed=%v, gen %d->%d",
			before.State, after.State, after.ReservationToken != before.ReservationToken, before.ExecutionGen, after.ExecutionGen)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails against the unguarded code (this is the mutation check)**

Run (from the worktree root):
```bash
go test ./internal/gatedrive/ -run TestSameScopeFirstStartLateLoserDoesNotRotate -race -count=1 -v
```
Expected: FAIL. The current `admissionExecuting` arm rotates for any same-scope start, so `admitScopedWorktree` returns `aerr == nil` and the test fails at "must refuse ErrScopeSecondDrive, got <nil>". If it fails anywhere else (compile error, precondition), fix the test, not the assertion's target. Do not proceed until the failure is exactly the refusal assert.

- [ ] **Step 3: Add the receipt guard**

In `internal/gatedrive/driver.go`, in `admitScopedWorktree`, change the `admissionExecuting` case. Current shape:

```go
		case admissionExecuting:
			// A same-scope successor continues over the executing slot: rotate it to
			// this start's OWN fresh reservation rather than reusing the predecessor's
			// token. ...
			newToken, rotErr := d.store.rotateWorktreeExecutionForSuccessor(req.Worktree, slot.ReservationToken)
```

New shape — guard first, rotation unchanged below it:

```go
		case admissionExecuting:
			// Rotation is successor-only. A receipt-less first start that reaches an
			// executing same-scope slot has raced an already-launched same-scope drive
			// past its precheck (the winner confirmed while this loser was in flight):
			// refuse with the same typed rejection precheckScopedStart gives that
			// condition, without touching the winner's live reservation. Rotating here
			// would replace the winner's token under the loser's hands and strand the
			// winner's run under a reserved slot with a foreign token.
			if req.PredecessorDriveID == "" {
				return "", false, false, false, nil, ownershipErr(ErrScopeSecondDrive, "start")
			}
			// A same-scope successor continues over the executing slot: rotate it to
			// this start's OWN fresh reservation rather than reusing the predecessor's
			// token. The successor then confirms and owns its slot (ownsSlot=true), and
			// its stale predecessor cannot free or poison it. A rotation failure
			// (a token race, a state change under the lock, an unreadable record) fails
			// closed with the typed rotation error.
			newToken, rotErr := d.store.rotateWorktreeExecutionForSuccessor(req.Worktree, slot.ReservationToken)
			if rotErr != nil {
				return "", false, false, false, nil, rotErr
			}
			return newToken, false, true, true, nil, nil // rotated; this successor confirms its own slot
```

(Only the guard and its comment are new; the successor comment, rotation call, and returns stay exactly as they are today.)

- [ ] **Step 4: Update the `admitScopedWorktree` doc comment to say rotation is successor-only**

In the same file, the function's doc comment currently says:

```
// worktree slot) arbitrates same-scope races. A start that finds a same-scope EXECUTING
// slot (a successor continuing the sequence in the terminal-before-release window)
// ROTATES it to its OWN fresh reservation — a new ReservationToken and bumped
```

Replace that sentence's opening so the successor-only condition is the stated rule, and name the receipt-less refusal. New text for the passage (the surrounding sentences are unchanged):

```
// worktree slot) arbitrates same-scope races. Rotation is successor-only: a start
// carrying a predecessor receipt that finds a same-scope EXECUTING slot (a successor
// continuing the sequence in the terminal-before-release window) ROTATES it to its
// OWN fresh reservation — a new ReservationToken and bumped
// ExecutionGen — so the predecessor's stale token can never free or poison the
// successor's slot, and the successor confirms and owns its own post-launch failure
// legs (ownsSlot=true). A RECEIPT-LESS first start that finds a same-scope executing
// slot has raced an already-launched drive and is refused typed ErrScopeSecondDrive
// without touching the slot. A slot held by a DIFFERENT scope, or in a
// stopping/unresolved state, is a genuine cross-scope refusal returned verbatim.
```

(The trailing "ErrUnresolvedExecution and every other error … fail closed unchanged." sentence stays as is.)

- [ ] **Step 5: Run the new test to verify it passes**

```bash
go test ./internal/gatedrive/ -run TestSameScopeFirstStartLateLoserDoesNotRotate -race -count=1 -v
```
Expected: PASS.

- [ ] **Step 6: Run the neighboring admission/successor/concurrency tests**

The guard sits on the successor path's switch arm, so prove the successor and barrier behaviors still hold:
```bash
go test ./internal/gatedrive/ -run 'TestBarrier|Successor|Admission|Admit' -race -count=1
```
Expected: PASS (all matched tests).

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/driver_concurrency_test.go
git commit -m "fix(gatedrive): same-scope first-start loser must not rotate the winner's executing slot (change 0452)"
```

---

### Task 2: Prove the flake is gone and the package is whole

**Files:**
- No source changes. Verification only; findings (if any) are fixed here, in the files Task 1 touched.

**Interfaces:**
- Consumes: Task 1's guarded `admissionExecuting` arm and the two tests `TestBarrierSameScopeFirstStartContention` and `TestSameScopeFirstStartLateLoserDoesNotRotate`.
- Produces: recorded evidence (command + result lines, for the build's evidence record) that the ~48% `GOMAXPROCS=1` reproduction is now 500/500 green and the whole `gatedrive` package passes under `-race`.

- [ ] **Step 1: Stress the original flaky reproduction under the change body's exact conditions**

```bash
GOMAXPROCS=1 go test ./internal/gatedrive/ -run TestBarrierSameScopeFirstStartContention -race -count=500
```
Expected: `ok` — 500/500 green (this reproduction failed ~48% of the time per run-batch before the guard; the change body's scratch run of this same guard went 500/500). Any single failure here is a defect in the guard or the test, not noise: stop and debug (superpowers:systematic-debugging), do not shrink `-count` or drop `GOMAXPROCS=1`.

- [ ] **Step 2: Run the full gatedrive package under -race**

```bash
go test ./internal/gatedrive/ -race -count=1
```
Expected: `ok` — the whole package passes. This catches any test elsewhere in the package that asserted the old rotate-for-any-same-scope-start behavior.

- [ ] **Step 3: Record the evidence**

No commit (nothing changed). Carry both command lines and their `ok` results into the task report so they land in the build's evidence record — this is the change's acceptance evidence for "no longer depends on scheduler timing."

---

## Self-Review

- **Scope coverage:** change body item (1) receipt guard → Task 1 Steps 3; item (2) deterministic regression test, mutation-checked → Task 1 Steps 1–2 (Step 2 is the mutation check: the test reds against the unguarded arm); item (3) successor-only doc comment → Task 1 Step 4. Out-of-scope items (admission rework, CI GOMAXPROCS change) touched by no task.
- **Placeholder scan:** none — every code step carries the exact code or comment text.
- **Type consistency:** `admitScopedWorktree`'s six-value return, the slot fields (`State`, `ReservationToken`, `ExecutionGen`), and the helper signatures in Task 1 match the current source; `ownershipErr(ErrScopeSecondDrive, "start")` matches the package's constructor and the precheck's existing use of the same kind/op pair.

<!-- docket:backlink:start -->
> ↩ **[Change 0452 — Same-scope first-start loser must not rotate the winner's executing worktree slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0452-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex.md)**
<!-- docket:backlink:end -->

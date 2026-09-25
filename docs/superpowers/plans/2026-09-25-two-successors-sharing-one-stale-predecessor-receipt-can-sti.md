<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0453 — Two successors sharing one stale predecessor receipt can still free a live worktree slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0453-two-successors-sharing-one-stale-predecessor-receipt-can-sti.md)**
<!-- docket:backlink:end -->
# Successor stale-receipt must not rotate a live worktree slot — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. For this change execution is by the docket-build skill.

**Goal:** In `admitScopedWorktree`, refuse a successor whose predecessor receipt no longer names the scope's CURRENT drive with `ErrStalePredecessor` BEFORE it rotates an executing same-scope worktree slot, so a second successor sharing a now-stale receipt can never rotate — and later release — the first successor's live slot.

**Architecture:** One guard inserted in the `admissionExecuting` arm of `admitScopedWorktree` (`internal/gatedrive/driver.go`), between the existing receipt-less `ErrScopeSecondDrive` refusal and `rotateWorktreeExecutionForSuccessor`. It loads the scope record and applies the exact predicate `reserveScopeDrive` already owns (`receipt drive id != scope current drive id` → `ErrStalePredecessor`), evaluated earlier to protect the one mutating step that precedes it. No new field, lock, error kind, or store function; `reserveScopeDrive` stays the authority for the scope slot, and nothing about the admission order (ADR-0118), `releasable`, or `isSameScopeRaceLoss` changes.

**Tech Stack:** Go; deterministic in-package tests in `internal/gatedrive` using the existing fakes (`fakeClock`, `fakeProc`, `stableGit`, `scopedTestDriver`, `prepareScopedStart`).

**Spec:** `docs/superpowers/specs/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-design.md` (on the `docket` metadata branch).

## Global Constraints

- The guard duplicates `reserveScopeDrive`'s predicate by value; per the spec that duplication carries the WHOLE predicate for this case (drive-id equality producing `ErrStalePredecessor`), and the code comment at the new site must name `reserveScopeDrive` as the authority so the twin is findable.
- Fail closed: a `LoadScope` error is returned as-is without touching the slot — never swallowed into a default that would let the comparison run against a zero-valued record.
- Every verification run defeats Go's test cache: `go test -count=1 …` always (a bare `go test` can serve a cached pass against a tree you just mutated).
- Mutation discipline: the red-first run of the new regression test against the unguarded code IS the spec's required mutation check; record its failure output before implementing the guard. Any later mutation probe restores `driver.go` only from committed state (`git checkout -- internal/gatedrive/driver.go`), never the uncommitted test file.
- The whole suite runs at the BUILD gate via whatever `build.test_command` resolves to (docket-build owns that); the per-task commands below are focused convenience runs, not the gate.
- Out of scope (spec): any admission/arbitration path not involving receipt staleness on rotation; change 0452's receipt-less fix; the ADR-0118 admission order.

## Review Focus

1. A stale-receipt successor reaching an executing same-scope slot must be refused `ErrStalePredecessor` with the slot's state, token, and `ExecutionGen` untouched — Task 1's test pins it.
2. A scope record that cannot be read (corrupt/IO) at the new guard must fail closed with the load error itself, not rotate and not degrade into `ErrStalePredecessor` against a zero record — Task 2's test pins it.
3. A FRESH successor (receipt naming the scope's current drive) must still rotate and launch exactly as before — Task 1 re-runs `TestBarrierSuccessorUnderCancel` and the successor admission tests.
4. A receipt-less first start reaching the executing slot must still refuse `ErrScopeSecondDrive` (change 0452's behavior) — Task 1 re-runs `TestSameScopeFirstStartLateLoserDoesNotRotate`.
5. An end-to-end `Start` by the stale second successor must launch nothing and leave the winner's slot intact (whatever typed refusal its precheck produces) — extra asserts at the end of Task 1's test.

---

### Task 1: Stale-receipt rotation guard, TDD

**Files:**
- Modify: `internal/gatedrive/driver.go` (function `admitScopedWorktree`, the `case admissionExecuting:` arm, and the function's doc comment)
- Test: `internal/gatedrive/driver_concurrency_test.go` (append the new test directly after `TestSameScopeFirstStartLateLoserDoesNotRotate`)

**Interfaces:**
- Consumes: `d.store.LoadScope(id string) (scopeRecord, error)`; `scopeRecord.CurrentDriveID string`; `ownershipErr(kind OwnershipErrorKind, op string) error`; `ErrStalePredecessor`; existing test helpers `prepareScopedStart(t, store) (ScopeGrant, StartRequest)`, `scopedTestDriver(store, clk, proc, git) *Driver`, `store.ownerCAS(driveID string, mutate func(*driveRecord) error) error`, `store.LoadWorktreeExecution(worktree string)`, `isOwnershipKind(err, kind) bool`, `fakeProc.launchN`.
- Produces: `TestSameScopeSuccessorStaleReceiptDoesNotRotate` (Task 2 sits beside it and reuses its fixture shape); the guarded `admitScopedWorktree` behavior Task 2's fail-closed test depends on.

- [ ] **Step 1: Write the failing regression test**

Append to `internal/gatedrive/driver_concurrency_test.go`, immediately after `TestSameScopeFirstStartLateLoserDoesNotRotate` (keep its style — same fixtures, same untouched-slot assert shape):

```go
// TestSameScopeSuccessorStaleReceiptDoesNotRotate deterministically pins the
// two-successor sibling of TestSameScopeFirstStartLateLoserDoesNotRotate
// (change 0453): successors S1 and S2 both present predecessor P's receipt and
// both passed precheckScopedStart before S1 retired P. S1 wins — launches and
// leaves the worktree slot executing under its own token. S2's worktree
// admission runs only now, with a receipt that no longer names the scope's
// CURRENT drive: it must refuse typed ErrStalePredecessor WITHOUT rotating,
// so its failure cleanup can never release S1's live reservation — same
// state, same token, same ExecutionGen.
func TestSameScopeSuccessorStaleReceiptDoesNotRotate(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)

	// Predecessor P: a full first start that launches, then settles to a
	// durable PASSED in the terminal-before-release window (the
	// TestBarrierSuccessorUnderCancel pattern), so successors may present it.
	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("predecessor Start: %v", err)
	}
	if first.Outcome != WAITING {
		t.Fatalf("predecessor must WAIT, got %s (%s)", first.Outcome, first.Cause)
	}
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	// Successor S1 with P's receipt: rotates P's slot, launches, and leaves the
	// worktree slot executing under S1's OWN token.
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	s1, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor S1 Start: %v", err)
	}
	if s1.Outcome != WAITING {
		t.Fatalf("S1 must WAIT, got %s (%s)", s1.Outcome, s1.Cause)
	}
	before, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if before.State != admissionExecuting {
		t.Fatalf("precondition: S1's slot must be executing, got %q", before.State)
	}

	// S2: the SAME (now-stale) P receipt reaches worktree admission only now.
	// Calling admitScopedWorktree directly models the successor that already
	// passed its precheck before P was retired; admission must refuse typed
	// and must not rotate S1's live reservation.
	_, _, _, _, _, aerr := d.admitScopedWorktree(succ)
	if !isOwnershipKind(aerr, ErrStalePredecessor) {
		t.Fatalf("a stale-receipt successor must refuse ErrStalePredecessor, got %v", aerr)
	}
	after, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	if after.State != admissionExecuting || after.ReservationToken != before.ReservationToken || after.ExecutionGen != before.ExecutionGen {
		t.Fatalf("S1's executing reservation must be untouched: state %q->%q, token changed=%v, gen %d->%d",
			before.State, after.State, after.ReservationToken != before.ReservationToken, before.ExecutionGen, after.ExecutionGen)
	}

	// Belt and suspenders: the FULL Start path for S2 must also launch nothing
	// and leave S1's slot intact, whatever typed refusal its precheck produces.
	launchesBefore := proc.launchN
	if _, serr := d.Start(succ); serr == nil {
		t.Fatalf("a stale-receipt successor Start must refuse")
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a stale-receipt successor must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}
	final, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after full Start: %v", err)
	}
	if final.State != admissionExecuting || final.ReservationToken != before.ReservationToken || final.ExecutionGen != before.ExecutionGen {
		t.Fatalf("S1's executing reservation must survive S2's full Start: state %q, token changed=%v, gen %d->%d",
			final.State, final.ReservationToken != before.ReservationToken, before.ExecutionGen, final.ExecutionGen)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails against the unguarded code (this is the spec's mutation check)**

Run: `go test -count=1 ./internal/gatedrive/ -run TestSameScopeSuccessorStaleReceiptDoesNotRotate -v`
Expected: FAIL at the direct `admitScopedWorktree` assert — the unguarded code rotates and returns a nil error, so the message is `a stale-receipt successor must refuse ErrStalePredecessor, got <nil>`. If it instead fails earlier (in fixture setup), fix the fixture — the red must come from the assert that pins the defect. Record this output; it is the evidence that the test discriminates.

- [ ] **Step 3: Implement the guard**

In `internal/gatedrive/driver.go`, inside `admitScopedWorktree`'s `case admissionExecuting:` arm, insert between the receipt-less refusal (`if req.PredecessorDriveID == "" { … ErrScopeSecondDrive … }`) and the comment block above `rotateWorktreeExecutionForSuccessor`:

```go
			// The receipt must still name the scope's CURRENT drive before the one
			// mutating admission step (the rotation) runs. reserveScopeDrive stays
			// the authority for the scope slot — this is its own staleness predicate
			// (receipt drive id vs the scope's CurrentDriveID) evaluated earlier, so
			// a second successor holding a retired predecessor's receipt never
			// rotates a live slot that its inevitable ErrStalePredecessor cleanup
			// would then release (change 0453). Reading the scope AFTER the slot
			// read is sufficient: a slot executing under a successor's token was
			// confirmed only after that successor's reserveScopeDrive advanced the
			// scope, and the scope never moves back to an earlier drive; a racer
			// holding an older slot token is refused by the rotation's own token
			// check. A scope load failure fails closed unchanged, like the
			// unreadable-slot leg above.
			scope, serr := d.store.LoadScope(req.ScopeID)
			if serr != nil {
				return "", false, false, false, nil, serr
			}
			if scope.CurrentDriveID != req.PredecessorDriveID {
				return "", false, false, false, nil, ownershipErr(ErrStalePredecessor, "start")
			}
```

Also extend the function's doc comment: after the sentence ending "the successor confirms and owns its own post-launch failure legs (ownsSlot=true).", add:

```
// Rotation additionally requires the presented receipt to still name the
// scope's CURRENT drive (reserveScopeDrive's own staleness predicate, applied
// before the mutating step): a successor whose predecessor was already
// superseded is refused typed ErrStalePredecessor without touching the slot.
```

- [ ] **Step 4: Run the new test and the neighboring coverage**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestSameScopeSuccessorStaleReceiptDoesNotRotate|TestSameScopeFirstStartLateLoserDoesNotRotate|TestBarrierSuccessorUnderCancel|TestBarrierSameScopeFirstStartContention' -v`
Expected: all PASS — the new refusal, 0452's receipt-less refusal, and the fresh-successor rotation path all intact.

- [ ] **Step 5: Run the whole package**

Run: `go test -count=1 ./internal/gatedrive/`
Expected: PASS (this includes `admission_successor_test.go`'s successor coverage the spec names).

- [ ] **Step 6: Commit**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/driver_concurrency_test.go
git commit -m "fix(gatedrive): refuse a stale predecessor receipt before rotating an executing slot (change 0453)"
```

### Task 2: Fail-closed scope-read leg

**Files:**
- Test: `internal/gatedrive/driver_concurrency_test.go` (append directly after `TestSameScopeSuccessorStaleReceiptDoesNotRotate`)

**Interfaces:**
- Consumes: the Task 1 guard (its `LoadScope` call and error return); `store.scopeDir(id string) (string, error)` and the package constant `recordFileName` (both in-package, used to corrupt the stored scope record); everything Task 1's test consumes.
- Produces: `TestSameScopeSuccessorScopeReadFailureFailsClosed`.

- [ ] **Step 1: Write the fail-closed test**

```go
// TestSameScopeSuccessorScopeReadFailureFailsClosed pins the guard's error leg
// (change 0453): when the scope record cannot be read at the pre-rotation
// staleness check, admission must fail closed with the load error itself —
// never rotate, and never degrade into an ErrStalePredecessor verdict computed
// against a zero-valued record.
func TestSameScopeSuccessorScopeReadFailureFailsClosed(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)

	// Predecessor P launches and settles terminal; successor S1 rotates,
	// launches, and leaves the slot executing under its own token (the
	// TestSameScopeSuccessorStaleReceiptDoesNotRotate fixture).
	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("predecessor Start: %v", err)
	}
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	if _, err := d.Start(succ); err != nil {
		t.Fatalf("successor S1 Start: %v", err)
	}
	before, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if before.State != admissionExecuting {
		t.Fatalf("precondition: S1's slot must be executing, got %q", before.State)
	}

	// Corrupt the stored scope record so the guard's LoadScope fails.
	dir, err := store.scopeDir(req.ScopeID)
	if err != nil {
		t.Fatalf("scopeDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), []byte("{corrupt"), 0o644); err != nil {
		t.Fatalf("corrupt scope record: %v", err)
	}

	_, _, _, _, _, aerr := d.admitScopedWorktree(succ)
	if aerr == nil {
		t.Fatalf("a failed scope read must refuse admission")
	}
	if isOwnershipKind(aerr, ErrStalePredecessor) {
		t.Fatalf("a failed scope read must surface the load error, not a staleness verdict: %v", aerr)
	}
	after, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	if after.State != admissionExecuting || after.ReservationToken != before.ReservationToken || after.ExecutionGen != before.ExecutionGen {
		t.Fatalf("a failed scope read must leave S1's reservation untouched: state %q->%q, token changed=%v, gen %d->%d",
			before.State, after.State, after.ReservationToken != before.ReservationToken, before.ExecutionGen, after.ExecutionGen)
	}
}
```

If `os`/`path/filepath` are not already imported by `driver_concurrency_test.go`, add them to its import block.

- [ ] **Step 2: Run it**

Run: `go test -count=1 ./internal/gatedrive/ -run TestSameScopeSuccessorScopeReadFailureFailsClosed -v`
Expected: PASS (the guard from Task 1 is already in place). If `LoadScope` on the corrupt record somehow returns nil, that is a finding about the store, not a reason to weaken the asserts — stop and investigate (`readStoredScope` documents fail-closed on a corrupt document).

- [ ] **Step 3: Mutation-check the error leg**

In `internal/gatedrive/driver.go`, temporarily make the guard ignore the load error — replace `scope, serr := d.store.LoadScope(req.ScopeID)` and its `if serr != nil { … }` return with `scope, _ := d.store.LoadScope(req.ScopeID)` — then run:

`go test -count=1 ./internal/gatedrive/ -run TestSameScopeSuccessorScopeReadFailureFailsClosed -v`

Expected: FAIL with `a failed scope read must surface the load error, not a staleness verdict` (the zero record's empty `CurrentDriveID` mismatches the receipt). Then restore ONLY the committed guard file — the Task 1 commit contains it, so this is safe:

```bash
git checkout -- internal/gatedrive/driver.go
```

Do not `git checkout` the test file — it is still uncommitted. Re-run the test once more after the restore and confirm PASS (with `-count=1`; never trust a cached verdict for either direction of a mutation probe).

- [ ] **Step 4: Run the whole package**

Run: `go test -count=1 ./internal/gatedrive/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/driver_concurrency_test.go
git commit -m "test(gatedrive): pin fail-closed scope read at the pre-rotation staleness guard (change 0453)"
```

---

## Self-Review (performed at plan time)

- Spec coverage: the guard (spec "Design", all three bullets — LoadScope, fail-closed error, staleness comparison, otherwise rotate unchanged) is Task 1 Step 3; the regression test with the spec's four numbered steps and untouched-slot asserts is Task 1 Step 1; the mutation check against unguarded code is Task 1 Step 2 (red-first); existing successor coverage staying green is Task 1 Steps 4–5; the whole suite runs at the docket-build gate. The spec's "why an unlocked read is sufficient" argument is preserved in the new code comment; the rejected alternative (threading run identity into the locked rotate CAS) is correctly not built.
- Placeholders: none — every step carries the exact code, command, and expected output.
- Type consistency: `LoadScope` returns `(scopeRecord, error)`; `scopeRecord.CurrentDriveID` and `req.PredecessorDriveID` are both `string`; `admitScopedWorktree` returns six values and every new return spells all six; helper names (`ownerCAS`, `scopeDir`, `recordFileName`, `isOwnershipKind`, `launchN`) verified against the current tree.
- Review Focus: all five lines have owning tests — 1, 3, 4, 5 in Task 1; 2 in Task 2.

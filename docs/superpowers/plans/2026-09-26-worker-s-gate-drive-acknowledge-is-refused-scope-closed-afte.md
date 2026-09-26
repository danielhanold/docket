<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0459 — Worker's gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0459-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md)**
<!-- docket:backlink:end -->
# Worker scope-transferred refusal (change 0459) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A handed-off worker whose scope the parent claimed gets a distinct typed refusal, `scope-transferred`, on `gate.drive.acknowledge` and scoped `gate.drive.start` — and the worker/parent skill contracts stop steering a completed task into a false `BLOCKED`.

**Architecture:** Keep the gate-drive authority model strict: `Claim` still closes the worker's scope, and the parent-side takeover path keeps `scope-closed`. Split the existing closed-scope refusals on the two child-capability operations by `FinalAcked`: a scope finished by its own terminal acknowledgement keeps `ErrScopeClosed`; a scope closed by a claim or takeover (`Closed && !FinalAcked`) returns a new `ErrScopeTransferred` whose app-layer message names the real state and never says "return BLOCKED". The worker contract (docket-build-task) learns that a post-handoff continuation reports on the verdict the continuation supplies and never touches the original scope; the parent contract (docket-build) makes the continuation carry that verdict plus a fresh scope bundle. Contract guards land in `internal/repoguard`.

**Tech Stack:** Go (no new dependencies); the repo's own test suite via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-design.md` (on the `docket` metadata branch; the synchronized copy is at `.docket/docs/superpowers/specs/…` from the repo root).

## Global Constraints

- Test suite: run through `go run ./cmd/docket development test` from the feature worktree — never a hand-rolled runner. Focused runs during TDD use `go test -count=1 ./internal/<pkg>/ -run <Name>`; **every** mutation probe and manual re-verification passes `-count=1` (Go's test cache otherwise serves a pre-mutation verdict).
- **Unchanged by this change** (spec "Unchanged"): `Claim` still closes the scope; the parent-side takeover path (`takeover.go`, `takeoverClose`, the "second detached-crash takeover halts scope-closed" rule referenced in `internal/app/rungate_verdict.go`) keeps `ErrScopeClosed`; `bindScopeChange` keeps `ErrScopeClosed`; the `closeScopeFinal` race branch (`ownershipErr(ErrScopeClosed, "acknowledge-close")` in `acknowledge.go`) keeps `ErrScopeClosed` — the spec scopes the new kind to Acknowledge's closed-scope branch and the scoped-start admission checks only.
- Both new refusals write nothing (assert with the existing `assertUnchanged` byte-compare helpers).
- The result-vocabulary mapping for `scope-transferred` is `invalid-input` — this falls out of the existing generic `AsOwnershipError` branch in `mapDriveFailure` (`internal/app/gate_drive.go`); pin it with a test, do not add a special case.
- Prose guards: `internal/repoguard/prose_contracts_test.go`'s `scanProse` is raw `strings.Contains` over file bytes — every guarded phrase must sit on a single unwrapped line in the skill file. When editing SKILL.md prose, keep each phrase listed in Task 5's table intact on one line.
- Cross-reference comments anchor on symbol names or verbatim-quoted clauses, never line numbers (`TestCommentAnchorStyle`).
- Guards are code: each new repoguard row and each flipped/new assert gets mutation evidence (strip the guarded clause → red; restore → green). Take before/after counts through a whitespace-flattened copy when a phrase could wrap, and back up the working tree state before a mutation (`git stash` is forbidden mid-task — copy the file aside instead; `git checkout --` restores to HEAD, destroying uncommitted work).
- Point-in-time records (`docs/superpowers/plans/*`, `docs/results/*`, archived changes, Accepted ADRs) are history — never edit them, even where they mention `scope-closed`.

## Review Focus

1. **Byte-identical repeat of a successful acknowledgement** (FinalAcked scope, matching drive, owner-cleared terminal record) must still return the recorded document, not `scope-transferred` — pinned in Task 1's non-regression subtest.
2. **Wrong child capability on a claim-closed scope** must still refuse `scope-capability-mismatch` (capability is checked before the closed check in `Acknowledge` and in `scopeReserveRefusal`) — a transferred scope must not leak its state to an unauthenticated caller. Pinned in Task 2 Step 1's ordering subtest.
3. **Takeover racing a completed ack / second detached-crash takeover** must keep HALT cause `scope-closed` — proven by the existing `takeover_test.go` / `integration_takeover_test.go` / rungate second-takeover tests staying green unchanged (Task 3 Step 6 runs them explicitly).
4. **`bind-scope-change` on a claim-closed scope** must keep `ErrScopeClosed` — pinned in Task 2's Step 1 subtest (today no test pins this kind on a closed bind; deleting the `rec.Closed` branch or retyping it would otherwise go unnoticed).
5. **Message hygiene both ways**: the `scope-transferred` message must not contain "BLOCKED" and must name the fresh-scope next action; the reworded `scope-closed` message must no longer say "transferred". Pinned in Task 3's app-layer test.

---

### Task 1: `ErrScopeTransferred` kind + Acknowledge closed-branch split

**Files:**
- Modify: `internal/gatedrive/ownership.go` (the `OwnershipErrorKind` const block, after `ErrScopeClosed`)
- Modify: `internal/gatedrive/acknowledge.go` (the `if scope.Closed { … }` branch of `Driver.Acknowledge`)
- Test: `internal/gatedrive/acknowledge_test.go` (extend `TestAcknowledgeRefusals`), `internal/gatedrive/ownership_test.go` (extend `TestOwnershipKindSpellings`)

**Interfaces:**
- Consumes: existing fixtures `startedScope`, `passObserveProc`, `readScopeBytes`, `readDriveBytes`, `assertUnchanged`, `isOwnershipKind`, `store.closeScope`, `overwriteDriveRecord`, `seedRecord`, `seedDrive` (all already in `internal/gatedrive` test files).
- Produces: `ErrScopeTransferred OwnershipErrorKind = "scope-transferred"` — Tasks 2 and 3 reference this exact identifier and wire spelling.

- [ ] **Step 1: Write the failing tests**

In `internal/gatedrive/acknowledge_test.go`, inside `TestAcknowledgeRefusals`, **replace** the existing subtest `t.Run("scope closed by claim or takeover", …)` (it currently expects `ErrScopeClosed`) with this pair:

```go
	t.Run("scope closed by claim or takeover is transferred", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		if err := store.closeScope(grant.ScopeID); err != nil {
			t.Fatalf("closeScope: %v", err)
		}
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeTransferred) {
			t.Fatalf("a claim/takeover-closed scope (not final-acked) must reject ack ErrScopeTransferred, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("final-acked scope with a non-matching drive stays scope-closed", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		if started.Outcome != PASSED {
			t.Fatalf("want a PASSED current drive, got %s", started.Outcome)
		}
		// A NORMAL terminal acknowledgement closes the scope with FinalAcked.
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); err != nil {
			t.Fatalf("terminal Acknowledge: %v", err)
		}
		// A separate durable drive: acknowledging it against the finished scope is
		// the finished-scope refusal, never the transferred one.
		other := seedRecord(t)
		other.LastOutcome = PASSED
		otherID, otherGen := seedDrive(t, store, other)
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		otherBytes := readDriveBytes(t, store, otherID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, otherID, otherGen); !isOwnershipKind(err, ErrScopeClosed) {
			t.Fatalf("a FinalAcked scope must keep the scope-closed refusal, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, otherID, otherBytes)
	})
```

Do **not** touch `TestAcknowledgeIdempotentRepeat` — it already pins Review Focus 1 (the byte-identical repeat still returns the recorded document); re-run it in Step 4 as the non-regression witness.

In `internal/gatedrive/ownership_test.go`, extend the `cases` map in `TestOwnershipKindSpellings` with the two scope-terminal spellings (this is the reason-vocabulary enumeration surface — the app layer surfaces these tokens verbatim):

```go
		ErrScopeClosed:                "scope-closed",
		ErrScopeTransferred:           "scope-transferred",
```

Also update that test's doc comment first line to say it pins the wire spellings of the sequential scope kinds **including the two closed-scope terminals** (keep the existing protocol-break rationale sentence).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestAcknowledgeRefusals|TestOwnershipKindSpellings'`
Expected: FAIL — `ErrScopeTransferred` undefined (compile error). That is the red for both.

- [ ] **Step 3: Implement the kind and the branch split**

In `internal/gatedrive/ownership.go`, immediately after the `ErrScopeClosed` const (keep its comment, but tighten it as shown so the two kinds partition the closed states):

```go
	// ErrScopeClosed: a transition was attempted on a scope already finished by
	// its own terminal acknowledgement (Closed && FinalAcked). A closed scope is
	// terminal.
	ErrScopeClosed OwnershipErrorKind = "scope-closed"
	// ErrScopeTransferred: a child-capability transition (an acknowledgement, or
	// a scoped start) was attempted on a scope closed by a claim or takeover
	// (Closed && !FinalAcked) — authority over the scope's drive moved to the
	// parent, so the scope is no longer the worker's to acknowledge or reuse.
	// Distinct from ErrScopeClosed so the refusal names the real state instead of
	// directing a finished worker to report BLOCKED (change 0459). Parent-side
	// paths (takeoverClose, bindScopeChange, closeScopeFinal's race branch) keep
	// ErrScopeClosed.
	ErrScopeTransferred OwnershipErrorKind = "scope-transferred"
```

(Replace the current two-line `ErrScopeClosed` comment "closed by a normal claim or an event-authorized takeover" — that sentence describes exactly the state that is now `ErrScopeTransferred`, so leaving it would make the comment lie.)

In `internal/gatedrive/acknowledge.go`, rework the tail of the `if scope.Closed { … }` branch in `Driver.Acknowledge`. Today it ends with one `return DriveDoc{}, ownershipErr(ErrScopeClosed, "acknowledge")` covering every closed case. Replace that single return with:

```go
		if !scope.FinalAcked {
			// Closed by a claim or takeover, not by a terminal acknowledgement:
			// scope authority transferred to the parent (change 0459).
			return DriveDoc{}, ownershipErr(ErrScopeTransferred, "acknowledge")
		}
		return DriveDoc{}, ownershipErr(ErrScopeClosed, "acknowledge")
```

so the branch reads: FinalAcked + matching drive + owner-cleared terminal → recorded document (unchanged); `!FinalAcked` → `ErrScopeTransferred`; every other closed case (FinalAcked with a non-matching drive or a non-terminal record) → `ErrScopeClosed`. Update the branch's leading comment: the sentence "A scope closed by a claim or takeover (FinalAcked false), or a non-matching drive, is a fail-closed ErrScopeClosed." becomes "A scope closed by a claim or takeover (FinalAcked false) is ErrScopeTransferred — authority moved to the parent; a FinalAcked scope with a non-matching drive is a fail-closed ErrScopeClosed."

- [ ] **Step 4: Run the package tests**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestAcknowledge'`
Expected: PASS — including `TestAcknowledgeIdempotentRepeat`, `TestAcknowledgeHappyPathClosesScope`, `TestAcknowledgeFailedFinalResult`, `TestAcknowledgePostRetirementOwnerGenAsymmetry` unchanged.
Then: `go test -count=1 ./internal/gatedrive/ -run 'TestOwnershipKindSpellings'` — PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/ownership.go internal/gatedrive/acknowledge.go internal/gatedrive/acknowledge_test.go internal/gatedrive/ownership_test.go
git commit -m "fix(gatedrive): claim-closed scope acknowledgement refuses scope-transferred (change 0459)"
```

---

### Task 2: Scoped start on a transferred scope + end-to-end regression

**Files:**
- Modify: `internal/gatedrive/driver.go` (`precheckScopedStart`, its `if scope.Closed` check)
- Modify: `internal/gatedrive/scope.go` (`scopeReserveRefusal`, its `if rec.Closed` clause, plus the function's ordered-refusal doc comment)
- Modify: `internal/gatedrive/handoff_test.go` (`TestScopedWaitingHandoffClaimClosesScope`)
- Test: `internal/gatedrive/handoff_test.go` (new `TestClaimedScopeAcknowledgeAndStartAreTransferred`), `internal/gatedrive/scope_test.go` or `acknowledge_test.go` (bind/capability-order subtests)

**Interfaces:**
- Consumes: `ErrScopeTransferred` from Task 1; existing fixtures in `handoff_test.go` (`fakeClock`, `startEpoch`, `OpenStore`, `testsupport.TempDir`, `fakeProc`, `obs`, `scopedTestDriver`, `stableGit`, `sampleStart`, `scopeReqFor`, `isOwnershipKind`) — copy the setup shape of `TestScopedWaitingHandoffClaimClosesScope`, which is in the same file.
- Produces: nothing new for later tasks; both scoped-start admission sites (`precheckScopedStart` and `scopeReserveRefusal`, the latter serving both `reserveScopeDrive` and `admitScopedWorktree`) return `ErrScopeTransferred` for `Closed && !FinalAcked`.

- [ ] **Step 1: Write the failing tests**

In `internal/gatedrive/handoff_test.go`, add the spec's end-to-end regression (spec Testing 1 + 2): handoff → claim → advance to `PASSED` → the worker's acknowledge is `ErrScopeTransferred` and writes nothing; a scoped start under the same scope is `ErrScopeTransferred` and launches nothing. Model the setup on `TestScopedWaitingHandoffClaimClosesScope` directly above it:

```go
// TestClaimedScopeAcknowledgeAndStartAreTransferred is the change-0459
// regression: a worker hands off its WAITING drive, the parent claims it and
// advances it to PASSED, and the returning worker's child-capability operations
// on the original scope — acknowledge, and a scoped start — are refused with the
// distinct ErrScopeTransferred (never the finished-scope ErrScopeClosed), with
// nothing written and nothing launched. Observed on change 0458 Task 2, where
// the old ErrScopeClosed refusal steered a finished worker into a false BLOCKED.
func TestClaimedScopeAcknowledgeAndStartAreTransferred(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	running := true
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability

	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("drive must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}

	// Worker hands off; parent claims and advances to the terminal PASSED.
	handoff, err := d.Handoff(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	claimed, err := d.Claim(started.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	running = false
	final, err := d.Advance(started.DriveID, claimed.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if final.Outcome != PASSED {
		t.Fatalf("parent-driven drive must PASS, got %s (%s)", final.Outcome, final.Cause)
	}

	// The returning worker's acknowledge on its original scope: transferred, no write.
	scopeBytes := readScopeBytes(t, store, grant.ScopeID)
	driveBytes := readDriveBytes(t, store, started.DriveID)
	if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeTransferred) {
		t.Fatalf("acknowledge after a parent claim must be ErrScopeTransferred, got %v", err)
	}
	assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)

	// A scoped start under the same scope: transferred, nothing launched.
	further := req
	further.PredecessorDriveID = started.DriveID
	further.PredecessorOwnerGen = claimed.Generation
	launchesBefore := proc.launchN
	if _, err := d.Start(further); !isOwnershipKind(err, ErrScopeTransferred) {
		t.Fatalf("a scoped start under a claim-closed scope must be ErrScopeTransferred, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a transferred-scope start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}
}
```

(If `assertUnchanged`/`readScopeBytes`/`readDriveBytes` are not visible from `handoff_test.go` — they live in `acknowledge_test.go`, same package, so they are — use them directly.)

In `TestScopedWaitingHandoffClaimClosesScope` (same file), flip the further-successor expectation: change `!isOwnershipKind(err, ErrScopeClosed)` to `!isOwnershipKind(err, ErrScopeTransferred)` and reword its failure message to `"a successor start under a claimed (transferred) scope must fail ErrScopeTransferred, got %v"`. Update the test's doc-comment clause "is refused ErrScopeClosed" to "is refused ErrScopeTransferred (change 0459)". Leave `TestAcknowledgePostAckSuccessorRefused` untouched — it pins that a **FinalAcked** scope's successor start stays `ErrScopeClosed`.

Add two ordering/non-regression subtests (put them in `acknowledge_test.go` beside `TestAcknowledgeRefusals`, as one new test function):

```go
// TestTransferredScopeRefusalOrdering pins two boundaries of the change-0459
// split: a wrong child capability on a claim-closed scope is still refused
// scope-capability-mismatch (a transferred scope leaks nothing to an
// unauthenticated caller), and bindScopeChange on a claim-closed scope keeps
// the parent-side ErrScopeClosed (the spec's "Unchanged" list).
func TestTransferredScopeRefusalOrdering(t *testing.T) {
	t.Run("wrong capability outranks transferred", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		if err := store.closeScope(grant.ScopeID); err != nil {
			t.Fatalf("closeScope: %v", err)
		}
		if _, err := d.Acknowledge(grant.ScopeID, "wrong-capability", started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
			t.Fatalf("wrong capability on a transferred scope must stay ErrScopeCapabilityMismatch, got %v", err)
		}
	})
	t.Run("bind-scope-change keeps scope-closed", func(t *testing.T) {
		_, store, grant, _, _ := startedScope(t, passObserveProc())
		if err := store.closeScope(grant.ScopeID); err != nil {
			t.Fatalf("closeScope: %v", err)
		}
		if err := store.bindScopeChange(grant.ScopeID, "0459"); !isOwnershipKind(err, ErrScopeClosed) {
			t.Fatalf("bindScopeChange on a closed scope must keep ErrScopeClosed, got %v", err)
		}
	})
}
```

(Check `startedScope`'s actual return signature at the top of `acknowledge_test.go` before writing — it returns five values in the existing subtests; if the scope request already binds a change id, bind the **same** id first or pick the idempotent path deliberately; the assert must exercise the `rec.Closed` clause, which is checked before the change-id clauses in `bindScopeChange`, so any id works.)

- [ ] **Step 2: Run the tests to verify the red**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestClaimedScopeAcknowledgeAndStartAreTransferred|TestScopedWaitingHandoffClaimClosesScope|TestTransferredScopeRefusalOrdering'`
Expected: `TestClaimedScopeAcknowledgeAndStartAreTransferred` FAILs on the scoped-start assertion (start still returns `ErrScopeClosed`; the acknowledge leg already passes from Task 1), `TestScopedWaitingHandoffClaimClosesScope` FAILs the same way, `TestTransferredScopeRefusalOrdering` PASSes (it pins current behavior — that is deliberate: it is a mutation tripwire, and Step 5 proves it can redden).

- [ ] **Step 3: Implement the start-side split**

In `internal/gatedrive/driver.go`, `precheckScopedStart`, replace:

```go
	if scope.Closed {
		return ownershipErr(ErrScopeClosed, "start")
	}
```

with:

```go
	if scope.Closed {
		if !scope.FinalAcked {
			// Closed by a claim or takeover: scope authority transferred to the
			// parent, not finished by its own terminal acknowledgement (change 0459).
			return ownershipErr(ErrScopeTransferred, "start")
		}
		return ownershipErr(ErrScopeClosed, "start")
	}
```

In `internal/gatedrive/scope.go`, `scopeReserveRefusal`, replace:

```go
	if rec.Closed {
		return ownershipErr(ErrScopeClosed, op)
	}
```

with:

```go
	if rec.Closed {
		if !rec.FinalAcked {
			return ownershipErr(ErrScopeTransferred, op)
		}
		return ownershipErr(ErrScopeClosed, op)
	}
```

and update the function's ordered-refusal doc comment bullet from "a closed scope is ErrScopeClosed;" to "a scope closed by its terminal acknowledgement is ErrScopeClosed, and one closed by a claim or takeover is ErrScopeTransferred;". `scopeReserveRefusal` serves both `reserveScopeDrive` (the locked authority) and `admitScopedWorktree` (driver.go's snapshot pre-check), so both agree by construction. Also update the stale comment in `scope.go` near the `Store.PrepareScope`/scope-lifecycle prose if it enumerates "a closed scope is ErrScopeClosed" (search the file for `ErrScopeClosed` mentions in comments and reword the ones describing the claim/takeover closure — `git grep -n "ErrScopeClosed" internal/gatedrive/scope.go`).

- [ ] **Step 4: Run the package**

Run: `go test -count=1 ./internal/gatedrive/`
Expected: PASS — including all of `takeover_test.go` and `integration_takeover_test.go` untouched and green (the parent-side takeover path still refuses/halts `scope-closed`; those tests double as the spec's Testing 4 proof).

- [ ] **Step 5: Mutation-test the ordering tripwires**

`TestTransferredScopeRefusalOrdering` was born green, so prove it can redden (assert-detects-removal, cached-runner rules):

1. Copy `internal/gatedrive/acknowledge.go` and `internal/gatedrive/scope.go` aside (e.g. `cp internal/gatedrive/scope.go "${TMPDIR:-/tmp}/scope.go.bak.XXXX"` — actual `cp`, not stash).
2. Mutation A: in `Acknowledge`, move the `scope.Closed` check above the capability check → `wrong capability outranks transferred` must FAIL. Restore.
3. Mutation B: in `bindScopeChange`, change `ErrScopeClosed` to `ErrScopeTransferred` → `bind-scope-change keeps scope-closed` must FAIL. Restore.
4. Mutation C: in `scopeReserveRefusal`, delete the `!rec.FinalAcked` inner branch (always return `ErrScopeClosed`) → `TestScopedWaitingHandoffClaimClosesScope` and `TestClaimedScopeAcknowledgeAndStartAreTransferred` must FAIL. Restore.
Every probe runs with `-count=1`. Record the three red observations in the task notes. After restoring, re-run Step 4's command green.

- [ ] **Step 6: Commit**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/scope.go internal/gatedrive/handoff_test.go internal/gatedrive/acknowledge_test.go
git commit -m "fix(gatedrive): scoped start on a claim-closed scope refuses scope-transferred (change 0459)"
```

---

### Task 3: App-layer messages and the JSON envelope

**Files:**
- Modify: `internal/app/gate_drive.go` (`ownershipNextAction`)
- Test: `internal/app/gate_drive_test.go` (new test beside `TestAcknowledgeForwardsArgsAndMapsDoc`)

**Interfaces:**
- Consumes: `gatedrive.ErrScopeTransferred` (Task 1); `newGateDriveService`, `fakeDriveEngine`, `OperationGateDriveAcknowledge` (existing test seam in `gate_drive_test.go` — see `TestAcknowledgeForwardsArgsAndMapsDoc` for the exact shape).
- Produces: the two message strings below, verbatim — Task 5's repoguard work does **not** guard them (they are pinned here, in Go tests).

- [ ] **Step 1: Write the failing test**

Add to `internal/app/gate_drive_test.go` (import `strings` if not already imported):

```go
// TestAcknowledgeScopeTransferredEnvelope is the change-0459 app-layer pin: a
// scope-transferred ownership rejection surfaces reason "scope-transferred"
// under invalid-input, with a next-action message that names the real state
// (parent claimed/took over; report on the continuation's verdict; fresh scope
// for further tests) and never says BLOCKED — while the reworded scope-closed
// message keeps BLOCKED and drops the old "transferred or" wording.
func TestAcknowledgeScopeTransferredEnvelope(t *testing.T) {
	bad := &fakeDriveEngine{err: &gatedrive.OwnershipError{Kind: gatedrive.ErrScopeTransferred, Op: "acknowledge"}}
	svc := newGateDriveService(bad, 0, "", "")
	got := svc.Acknowledge("sc-x", "childcap", "dx", "genx")
	if got.Result != ResultInvalidInput || got.Drive != nil {
		t.Fatalf("scope-transferred must map to invalid-input with no drive, got result=%s", got.Result)
	}
	if got.Reason != string(gatedrive.ErrScopeTransferred) {
		t.Fatalf("reason = %q, want %q", got.Reason, string(gatedrive.ErrScopeTransferred))
	}
	if strings.Contains(got.Message, "BLOCKED") {
		t.Fatalf("the scope-transferred message must never direct the worker to BLOCKED, got %q", got.Message)
	}
	for _, want := range []string{"parent claimed or took over", "verdict your continuation supplied", "fresh scope"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("scope-transferred message must contain %q, got %q", want, got.Message)
		}
	}

	// The finished-scope message: still directs BLOCKED, no longer claims a transfer.
	closedMsg := ownershipNextAction(gatedrive.ErrScopeClosed)
	if !strings.Contains(closedMsg, "BLOCKED") {
		t.Fatalf("the scope-closed message must keep directing BLOCKED, got %q", closedMsg)
	}
	if strings.Contains(closedMsg, "transferred") {
		t.Fatalf("the scope-closed message must no longer say transferred, got %q", closedMsg)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/app/ -run 'TestAcknowledgeScopeTransferredEnvelope'`
Expected: FAIL — `ownershipNextAction` has no `ErrScopeTransferred` case (empty Message), and the current `ErrScopeClosed` message contains "transferred".

- [ ] **Step 3: Implement the two messages**

In `internal/app/gate_drive.go`, `ownershipNextAction`, replace the `ErrScopeClosed` case and add the new one:

```go
	case gatedrive.ErrScopeClosed:
		return "this scope was already finished by its terminal acknowledgement; stop and return BLOCKED"
	case gatedrive.ErrScopeTransferred:
		return "the parent claimed or took over this scope's drive; this scope is no longer yours — report on the verdict your continuation supplied, and run further tests only under a fresh scope"
```

(No change to `mapDriveFailure`: the generic `AsOwnershipError` branch already surfaces `scope-transferred` as `ResultInvalidInput` + the kind token, which is the spec's required result-vocabulary mapping.)

- [ ] **Step 4: Run the app tests**

Run: `go test -count=1 ./internal/app/ -run 'TestAcknowledge|TestStartForwards|TestTakeoverMapsDoc'`
Expected: PASS, including the pre-existing `TestAcknowledgeForwardsArgsAndMapsDoc` (it asserts only a non-empty message for `ErrScopeClosed`, so the reword keeps it green — verify, don't assume).

- [ ] **Step 5: Sweep every `scope-closed` enumeration site (derive, never hand-list)**

Run: `git grep -n "scope-closed" -- ':!docs/superpowers' ':!docs/results' ':!docs/changes'` and `git grep -rn "ErrScopeClosed"` from the worktree root. Sort the hits into executable vs prose:
- Executable sites already handled: `ownership.go`, `acknowledge.go`, `driver.go`, `scope.go`, `gate_drive.go` (this task), plus the tests updated in Tasks 1–2.
- Parent-side sites that must stay untouched: `takeover.go` (`haltDoc(… string(ErrScopeClosed))`, `takeoverClose`), `internal/app/rungate_verdict.go` (the second-takeover halt comment).
- Comment-only sites: reword any comment whose "closed by a claim or takeover → scope-closed" description is now false (Task 2 Step 3 covered `scope.go`; check `takeover.go`'s comment above `takeoverClose` — its scope-closed wording describes the parent path and stays TRUE, so leave it).
Confirm no schema/JSON/docs file outside point-in-time records enumerates the reason tokens (at authoring time the whole-repo grep found none — re-derive rather than trusting this sentence). If the sweep turns up a reason-member enumeration this plan missed (for example a docs/reference table or a JSON schema), add `scope-transferred` beside `scope-closed` there in this task.

- [ ] **Step 6: Non-regression witnesses for the untouched parent path**

Run: `go test -count=1 ./internal/gatedrive/ -run 'Takeover' && go test -count=1 ./internal/app/ -run 'RunGate'`
Expected: PASS with zero diffs to those test files in `git status` (spec Testing 4).

- [ ] **Step 7: Commit**

```bash
git add internal/app/gate_drive.go internal/app/gate_drive_test.go
git commit -m "fix(app): scope-transferred names the real state and drops BLOCKED from its message (change 0459)"
```

---

### Task 4: Worker and parent skill contracts

**Files:**
- Modify: `skills/docket-build-task/SKILL.md` (the "**Sequential drives within your scope.**" paragraph's neighborhood)
- Modify: `skills/docket-build/SKILL.md` (the "## Task-level WAITING and the continuation" section)

**Interfaces:**
- Consumes: the `scope-transferred` wire spelling (Task 1).
- Produces: the exact contract sentences Task 5's repoguard rows grep for — every phrase in Task 5's table must appear **verbatim and unwrapped on a single line** in these edits. Write these paragraphs with the guarded phrases on their own lines (do not let a re-wrap split them; the guard matcher is raw `strings.Contains`).

- [ ] **Step 1: Worker contract (docket-build-task)**

In `skills/docket-build-task/SKILL.md`, insert a new paragraph immediately **after** the "**Sequential drives within your scope.** …" paragraph (which ends "…keep the final drive id and verdict in `VERIFICATION`/`NOTES`."):

```markdown
**Continued after a `WAITING` handoff — the original scope is no longer yours.** A worker that
performed `gate.drive.handoff` and returned `WAITING` surrendered its drive; the parent's `claim`
closed the scope, so when you are resumed or re-dispatched to continue that task you
never `acknowledge` the original scope and never start a drive on it.
A `scope-transferred` refusal means you misapplied this rule
— it is not an acknowledgement failure of an owned scope and it never means the work failed.
Report on the terminal verdict your continuation supplies
(the handed-off drive id and its `PASSED`/`FAILED` disposition) plus your own work:
`PASSED` with exactly one task commit → `COMPLETE`; `FAILED` → the existing repair discretion, and
never `COMPLETE` on that verdict. Any further test drive runs only under the fresh scope bundle the
continuation provides, under the normal sequential-drive rules above; with no fresh bundle you
cannot run tests — return `BLOCKED` naming
"continuation needs a fresh scope".
The rule that a failed acknowledgement returns `BLOCKED`, never `COMPLETE`, continues to bind the
scopes you still own, and the final drive id and verdict stay in `VERIFICATION`/`NOTES` as always.
```

Keep the five guarded phrases exactly as spelled in Task 5 Step 1's table, each intact on one physical line as shown.

- [ ] **Step 2: Parent contract (docket-build)**

In `skills/docket-build/SKILL.md`, in "## Task-level WAITING and the continuation", extend the paragraph that today reads "…When agent judgment is needed again, dispatch a fresh worker for the **same** task and worktree with an explicit continuation; a trusted `PASSED` is not re-driven for a changed transcript. …". After the sentence ending "…is not re-driven for a changed transcript.", insert:

```markdown
The continuation — a same-agent resume or a fresh dispatch alike — must carry
the claimed drive's id, its terminal verdict, and an explicit statement that the original scope is closed
by your claim and must never be acknowledged or reused. When the continued task may still need test
drives, run `gate.drive.prepare-scope` again
for the same change, task, phase, branch, and worktree (and dispatch context) and include the new
start-ready scope bundle — child capability only; the parent capability stays in your notes, as for
any dispatch. Reading the continuation's return is unchanged: a `COMPLETE` is settled against git
state exactly as *Reading a worker's return* requires.
```

Again: the guarded phrases from Task 5 Step 1's table stay verbatim on single lines as shown.

- [ ] **Step 3: Sanity-read both sections end to end**

Re-read each edited section as a worker in an unknown consuming repo (learnings: distributed-body-has-no-local-repo): no sentence may be true only of the docket repo, and no aside may contradict a numbered rule in its own section. Specifically check the new worker paragraph does not contradict "After a first `WAITING` never `advance` or restart — the controller owns the drive" (it must read as its continuation) and that the closed-outcome vocabulary is respected — every prohibition names the enumerated return it maps to (learnings: prohibition-needs-a-return-value; here: `COMPLETE`, repair discretion, or `BLOCKED "continuation needs a fresh scope"`).

- [ ] **Step 4: Commit**

```bash
git add skills/docket-build-task/SKILL.md skills/docket-build/SKILL.md
git commit -m "docs(skills): post-handoff continuation contract — transferred scope is never acknowledged or reused (change 0459)"
```

---

### Task 5: Repoguard contract guards + mutation evidence

**Files:**
- Modify: `internal/repoguard/prose_contracts_test.go` (append rows to the `proseContracts` table)

**Interfaces:**
- Consumes: the exact sentences committed in Task 4. If Task 4's final wording drifted from this plan, repoint these phrases at the committed wording **in this task** — the row must match the file as committed, and each phrase must still be a distinctive, load-bearing clause bound to its claim (learnings: prose-guard-binds-phrase-to-claim), not a floating word.
- Produces: nothing further.

- [ ] **Step 1: Add the guard rows**

Append to the `proseContracts` table in `internal/repoguard/prose_contracts_test.go`:

```go
	// change 0459 — a handed-off worker's scope authority ends at the parent's
	// claim: the worker contract forbids acknowledging or reusing the claim-closed
	// scope, keys the outcome to the continuation's verdict, names the honest
	// BLOCKED for a missing fresh bundle, and classifies scope-transferred as a
	// misapplied-rule signal; the parent contract makes the continuation carry the
	// verdict, the closed-scope statement, and a fresh prepare-scope bundle.
	{sentinel: "change_0459_scope_transferred", file: "skills/docket-build-task/SKILL.md",
		present: []string{
			"never `acknowledge` the original scope and never start a drive on it",
			"A `scope-transferred` refusal means you misapplied this rule",
			"Report on the terminal verdict your continuation supplies",
			"never `COMPLETE` on that verdict",
			"\"continuation needs a fresh scope\"",
		}},
	{sentinel: "change_0459_scope_transferred", file: "skills/docket-build/SKILL.md",
		present: []string{
			"the claimed drive's id, its terminal verdict, and an explicit statement that the original scope is closed",
			"run `gate.drive.prepare-scope` again",
		}},
```

- [ ] **Step 2: Run the guard green**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestProseContracts'`
Expected: PASS (the population floor rises by 7; it is a `>=` floor, so no constant needs editing — verify the assertion still reads `checks < 40` and leave it).

- [ ] **Step 3: Mutation-test each guard row (both files)**

For each of the two edited skill files: copy the file aside (`cp <file> "${TMPDIR:-/tmp}/skill.md.bak.XXXX"`), delete the entire new paragraph from Task 4, run `go test -count=1 ./internal/repoguard/ -run 'TestProseContracts'`, and confirm it FAILS naming the `change_0459_scope_transferred` sentinel and the missing phrases. Restore the file (`mv -f` the backup back), re-run green. Then one finer probe per file: reword a single guarded clause (e.g. change "never be acknowledged or reused" to "should not generally be reused") and confirm the guard reddens — proving the phrase is bound to the claim, not satisfied by leftover vocabulary. Restore, re-run green, and confirm `git status` shows only `prose_contracts_test.go` modified. Record all four red observations in the task notes.

- [ ] **Step 4: Commit**

```bash
git add internal/repoguard/prose_contracts_test.go
git commit -m "test(repoguard): guard the post-handoff continuation contract sentences (change 0459)"
```

---

### Task 6: Whole-suite gate

**Files:** none (verification only)

- [ ] **Step 1: Run the full suite from the feature worktree**

Run: `go run ./cmd/docket development test`
Expected: SUITE PASS. Read the budget report even on green: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (serial-confirm per `tests/README.md`) before calling the gate met.

- [ ] **Step 2: Verify the branch state**

`git log --oneline` shows the five commits above on `fix/worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte`; `git status` is clean. No file under `docs/superpowers/plans/` (other than this plan), `docs/results/`, or `docs/changes/` was modified.

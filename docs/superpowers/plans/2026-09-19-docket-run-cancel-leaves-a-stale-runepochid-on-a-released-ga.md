<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0435 — docket run cancel leaves a stale RunEpochID on a released gate-admission slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md)**
<!-- docket:backlink:end -->
# Retire Cancelled Run Ownership From Released Gate-Admission Slots — Implementation Plan (change 0435)

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the docket-build skill (subagent-driven, one worker per task). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make authorized `run cancel` completion (and the matching terminal-repair and resume paths) retire a cancelled epoch's stale `RunEpochID` from a released worktree admission slot, so a legitimate replacement build gate or epoch-less finalize gate can admit again — without weakening any 437 launch fence or ordinary-release semantics.

**Architecture:** One new ownership-checked store operation in `internal/gatedrive` (an `admissionCAS` mutate closure that clears ONLY `RunEpochID` on a released slot the expected epoch owns), invoked from exactly three app-layer sites: the authorized-cancel completion decision in `runCancel`, the new bounded terminal-repair path that replaces the `EpochCancelled/EpochSuperseded` early return, and the `run.gate-before --resume` quiescence check. Slot teardown in `reconcileEpochTeardown` becomes ownership-checked (foreign and unlinked epoch-less slots are never touched). The death guardian keeps its teardown but NEVER retires ownership.

**Tech Stack:** Go; existing `gatedrive` store primitives (`admissionCAS`, flock + atomic JSON writes, typed `StoreError`/`OwnershipError`); existing app-layer epoch record CAS (`epochCAS`); standard `go test` package tests; the source-resolved full suite `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md` (on the `docket` metadata branch — locally readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md`). Change file: `docs/changes/active/0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md`. ADR-0118 remains the governing contract.

## Global Constraints

- No new daemon, background loop, store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or retry layer (spec "Complexity limit and exclusions"). New typed errors reuse EXISTING kinds (`ErrStaleRunEpoch`, `ErrNotOwner`, `ErrWorktreeBusy`, `ErrNotFound`, `ErrInvalidID`); new report findings are bounded, credential-free tokens.
- Change 437 exclusively owns launch fencing and pending-launch accounting (`EpochLaunchGate`/`epochGated`, `Driver.ReconcileEpochLaunches`, relaunch reservation/recovery). Reuse them; never re-implement or weaken them. All existing 437 tests must stay untouched and green.
- Ordinary `ReleaseWorktreeExecution` KEEPS retaining `RunEpochID` (between-drive ownership); the state-independent epoch-mismatch fence in `reserveWorktreeExecution` and `rawStaleEpochRefusal` are not weakened.
- Retirement preserves `DriveID`, `RawRunID`, `RawRunDir`, `ExecutionGen`, `ScopeID`, `Kind`, `ReservationToken`, legacy-inventory fields, and `ReservedAt`; only `RunEpochID` and `UpdatedAt` change.
- The guardian (`guardianFenceAndReap`) performs teardown but never retires epoch ownership — it leaves the epoch `cancelling`; only authorized `run.cancel` completion (and the bounded terminal repair / resume check) retires.
- Lock order stands: never acquire the epoch lock while holding an inner (admission/scope/drive) lock; never hold epoch or admission locks over process stops. Retirement and the final epoch write are two separate, individually CAS-guarded writes — no cross-store transaction, no cleanup journal.
- Cancellation, repair, and denied resume charge NO suite attempt and reset NO budget/deadline/retry state (existing `TestCancelNeverChargesOrResets` discipline, extended in Task 6).
- Every guard added is mutation-tested (strip the guarded thing, watch it redden — repo rule "Guards and tests"); Go test re-runs during mutation probes use `-count=1` (learning `cached-runner-serves-a-mutated-tree`).
- The single full-suite gate at the end runs whatever `build.test_command` resolves to (`go run ./cmd/docket development test`), from source, and its budget report is read even on green.
- Comment cross-references anchor on symbol names or quoted clauses, never line numbers (ADR-0054).

## File Structure

| File | Role |
|---|---|
| `internal/gatedrive/admission_retire.go` (create) | The one new store op: `RetireWorktreeExecutionEpoch` + its sentinel. |
| `internal/gatedrive/admission_retire_test.go` (create) | Store-level retirement tests (ownership, preservation, refusals, admission-after-retire). |
| `internal/gatedrive/admission.go` (modify) | Add the exported epoch-carrying reserve entry `ReserveWorktreeExecutionForEpoch`; extend the file-header lifecycle prose with the retirement transition. |
| `internal/app/rungate_cancel.go` (modify) | Ownership-checked slot teardown (`classifySlotOwnership`, epoch-aware `markWorktreeSlotStopping`/`reconcileWorktreeSlot`, checked release errors), completion retirement (`retireWorktreeSlotOwnership`), finalize-failure → pending, terminal repair (`repairTerminalEpoch`, `verifyTerminalEpochQuiescence`), `retire` seam on `cancelSeams`. |
| `internal/app/rungate_cancel_test.go` (modify) | Fixture records a real owning `RunEpochID`; existing tests updated; new ownership/retirement/interruption/repair tests. |
| `internal/app/rungate_before.go` (modify) | Resume quiescence validation (`validateResumeQuiescence`) in the `EpochCancelled` and `EpochSuperseded` branches; `CancelSeams` seam on `GateScopeDeps`. |
| `internal/app/rungate_before_resume_test.go` (modify) | Inject the seam into existing tests; new denial/retirement/one-replacement tests. |
| `internal/app/agent_guardian_test.go` or `rungate_cancel_test.go` (modify) | Guardian-never-retires regression. |

Naming used consistently across all tasks (type-consistency contract):

```go
// gatedrive
func (s *Store) RetireWorktreeExecutionEpoch(worktreeRoot, expectEpoch, expectToken string) error
func (s *Store) ReserveWorktreeExecutionForEpoch(repoIdentity, worktreeRoot, runEpochID string, proc recoverySeam) (token string, err error)

// app
type slotOwnershipClass int // slotOwned | slotLinkedLegacy | slotForeign | slotUnowned
func classifySlotOwnership(slotEpochID, slotRawRunDir string, ep EpochRecord) slotOwnershipClass
func markWorktreeSlotStopping(seams cancelSeams, ep EpochRecord)          // was (seams, worktree string)
func reconcileWorktreeSlot(seams cancelSeams, ep EpochRecord) (bool, string) // was (seams, worktree string)
func (s cancelSeams) retireSlot(worktree, epoch, token string) error      // seam-backed
func retireWorktreeSlotOwnership(seams cancelSeams, ep EpochRecord) (accounted bool, finding string)
func verifyTerminalEpochQuiescence(seams cancelSeams, ep EpochRecord) (quiescent bool, findings []string)
func repairTerminalEpoch(seams cancelSeams, ep EpochRecord) RunCancelResult
func validateResumeQuiescence(seams cancelSeams, ep EpochRecord) (ok bool, detail string)
```

---

### Task 1: `RetireWorktreeExecutionEpoch` store operation

**Files:**
- Create: `internal/gatedrive/admission_retire.go`
- Create: `internal/gatedrive/admission_retire_test.go`
- Modify: `internal/gatedrive/admission.go` (file-header "State lifecycle" paragraph only)

**Interfaces:**
- Consumes: `admissionCAS`, `verifyAdmissionToken`, `ownershipErr`, `storeErr`, existing error kinds, `admissionRecord` (in-package).
- Produces: `(*Store).RetireWorktreeExecutionEpoch(worktreeRoot, expectEpoch, expectToken string) error` — nil on success AND on the already-detached no-op; typed errors otherwise. Tasks 4/5/7 call it through the app seam.

- [ ] **Step 1: Write the failing tests**

In `internal/gatedrive/admission_retire_test.go` (in-package `gatedrive`, so `admissionRecord` is constructible; reuse the existing admission-test repo/store helpers from `admission_test.go` — the pattern there is `newStore(t)`-style construction with a real temp dir; mirror whatever helper `admission_test.go` actually uses):

```go
package gatedrive

// Helper: reserve+confirm+release a slot owned by epoch "ep-1", returning its token.
// Reuse the store fixture admission_test.go uses (a Store rooted at a t.TempDir git
// common dir with a real worktree dir); do not invent a second fixture style.
func retireFixtureSlot(t *testing.T, s *Store, worktree string) (token string) {
	t.Helper()
	token, _, err := s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: "repo-1", WorktreeRoot: worktree, Kind: "scopeless", RunEpochID: "ep-1",
	}, nil)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(worktree, token, "run-1", filepath.Join(worktree, "rd")); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(worktree, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	return token
}

// TestRetireClearsOnlyRunEpochID: retiring the owning epoch on a released slot
// clears RunEpochID and updates UpdatedAt, preserving every historical field.
func TestRetireClearsOnlyRunEpochID(t *testing.T) { /* build fixture */
	// before := LoadWorktreeExecution(...)
	// err := s.RetireWorktreeExecutionEpoch(worktree, "ep-1", token) — want nil
	// after := LoadWorktreeExecution(...)
	// assert after.RunEpochID == "" && after.State == "released"
	// assert after.DriveID==before.DriveID, RawRunID, RawRunDir, ExecutionGen,
	//        ScopeID, Kind, ReservationToken, ReservedAt, LegacyInventoried all equal
}

// TestRetireIdempotentWhenAlreadyDetached: a second retire (RunEpochID already "")
// returns nil and writes nothing (compare the stored Generation before/after via
// LoadWorktreeExecution's second return).
func TestRetireIdempotentWhenAlreadyDetached(t *testing.T) { … }

// TestRetireRefusesForeignEpoch: expectEpoch "ep-2" against a slot owned by "ep-1"
// is ErrStaleRunEpoch; the record is byte-identical after (same Generation).
func TestRetireRefusesForeignEpoch(t *testing.T) { … }

// TestRetireRefusesTokenMismatch: the right epoch with a stale/changed token is
// ErrNotOwner (a raced replacement's reservation must never be cleared blindly).
func TestRetireRefusesTokenMismatch(t *testing.T) { … }

// TestRetireRefusesNonReleasedState: an executing (confirm, no release) owned slot
// refuses ErrWorktreeBusy — retirement requires released state.
func TestRetireRefusesNonReleasedState(t *testing.T) { … }

// TestRetireEmptyExpectEpochRefused: expectEpoch "" is ErrInvalidID — an empty
// epoch is never ownership proof (spec "An empty epoch is not ownership proof").
func TestRetireEmptyExpectEpochRefused(t *testing.T) { … }

// TestRetireAbsentSlotIsNotFound: no slot record → typed ErrNotFound (the caller
// maps absence to an idempotent no-op only after independent accounting).
func TestRetireAbsentSlotIsNotFound(t *testing.T) { … }

// TestOrdinaryReleaseStillRetainsEpoch (AC2 regression): after
// ReleaseWorktreeExecution the slot still carries RunEpochID "ep-1", and a reserve
// carrying a DIFFERENT epoch — and an epoch-less one — are both ErrStaleRunEpoch
// (the between-drives fence is untouched).
func TestOrdinaryReleaseStillRetainsEpoch(t *testing.T) { … }

// TestAdmissionAfterRetirement (AC1, store half): after retirement, (a) a reserve
// carrying a NEW epoch "ep-2" succeeds, and (b) on a second retired fixture an
// epoch-less reserve (RunEpochID "") succeeds — the released slot is genuinely
// reusable, with ExecutionGen continuing monotonically.
func TestAdmissionAfterRetirement(t *testing.T) { … }
```

Use the real error-kind assertions the package already uses (`AsStoreError` / the ownership-error unwrap pattern in `ownership_test.go` — match the house idiom, do not invent a new one).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gatedrive/ -run 'TestRetire|TestOrdinaryReleaseStillRetainsEpoch|TestAdmissionAfterRetirement' -count=1 -v`
Expected: compile FAIL — `RetireWorktreeExecutionEpoch` undefined.

- [ ] **Step 3: Implement `internal/gatedrive/admission_retire.go`**

```go
// Cancellation-specific epoch retirement (change 0435). Ordinary execution release
// (ReleaseWorktreeExecution) deliberately RETAINS RunEpochID so a live epoch owns
// its worktree between drives; a COMPLETED cancellation must be able to detach that
// ownership so a replacement build gate or an epoch-less finalize gate can admit
// again. RetireWorktreeExecutionEpoch is that one store operation: an admissionCAS
// mutate closure that clears ONLY RunEpochID on a RELEASED slot the expected epoch
// owns, preserving every historical field (DriveID/RawRunID/RawRunDir/ExecutionGen/
// ScopeID/Kind/ReservationToken and the legacy-inventory facts). Only authorized
// cancellation completion — and the matching bounded terminal-repair / resume
// quiescence check — invokes it, after complete launch/participant/mutation
// accounting; the death guardian never does (it fences and reaps but leaves the
// epoch cancelling — see the app layer's guardianFenceAndReap contract).
package gatedrive

import (
	"errors"
	"time"
)

// errEpochAlreadyDetached aborts the CAS with no write when the slot carries no
// RunEpochID: retirement is idempotent, so an already-detached slot is success,
// not a refusal. Internal to RetireWorktreeExecutionEpoch.
var errEpochAlreadyDetached = errors.New("worktree slot epoch already detached")

// RetireWorktreeExecutionEpoch clears the run-epoch ownership of a RELEASED
// worktree slot, atomically checking expected epoch, reservation token, and state
// under the slot's flock + physical generation (admissionCAS):
//
//   - RunEpochID already ""            → idempotent success, no write.
//   - RunEpochID != expectEpoch        → ErrStaleRunEpoch (a foreign owner or a
//     successor: never touched — spec "A different nonempty RunEpochID is a
//     foreign owner").
//   - reservation token mismatch       → ErrNotOwner (the reservation changed
//     under the caller: a raced replacement is never cleared blindly).
//   - State != released                → ErrWorktreeBusy (retirement additionally
//     requires released state; a live or unresolved slot is never detached).
//   - absent slot                      → ErrNotFound; unreadable/corrupt/unknown
//     schema fail closed with their typed StoreError, exactly as every other
//     admission transition.
//
// expectEpoch must be non-empty: an empty epoch is not ownership proof
// (ErrInvalidID). On success only RunEpochID and UpdatedAt change.
func (s *Store) RetireWorktreeExecutionEpoch(worktreeRoot, expectEpoch, expectToken string) error {
	const op = "retire-worktree-execution-epoch"
	if expectEpoch == "" {
		return storeErr(ErrInvalidID, op, nil)
	}
	err := s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if rec.RunEpochID == "" {
			return errEpochAlreadyDetached
		}
		if rec.RunEpochID != expectEpoch {
			return ownershipErr(ErrStaleRunEpoch, op)
		}
		if verr := verifyAdmissionToken(rec, expectToken, op); verr != nil {
			return verr
		}
		if rec.State != admissionReleased {
			return ownershipErr(ErrWorktreeBusy, op)
		}
		rec.RunEpochID = ""
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
	if errors.Is(err, errEpochAlreadyDetached) {
		return nil
	}
	return err
}
```

Also extend the "State lifecycle" paragraph of `internal/gatedrive/admission.go`'s file header with one sentence after "releasing preserves the historical DriveID/RawRunID…": `Completed cancellation may additionally retire a released slot's RunEpochID through RetireWorktreeExecutionEpoch (change 0435), leaving the historical evidence intact while detaching the cancelled epoch's ownership.`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gatedrive/ -run 'TestRetire|TestOrdinaryReleaseStillRetainsEpoch|TestAdmissionAfterRetirement' -count=1 -v`
Expected: PASS. Then run the whole package: `go test ./internal/gatedrive/ -count=1` — PASS (no 437 regression).

- [ ] **Step 5: Mutation-probe the ownership guard**

Temporarily (a) delete the `rec.RunEpochID != expectEpoch` refusal, (b) delete the `verifyAdmissionToken` call, (c) delete the `State != admissionReleased` refusal — one at a time, re-running Step 4's command with `-count=1` after each: each mutation must redden at least one test (Foreign/TokenMismatch/NonReleased respectively). Restore the code exactly (keep a copy before mutating — never `git checkout --` over uncommitted work, learning `mutation-restore-needs-a-backup-copy`).

- [ ] **Step 6: Commit**

```bash
git add internal/gatedrive/admission_retire.go internal/gatedrive/admission_retire_test.go internal/gatedrive/admission.go
git commit -m "feat(gatedrive): ownership-checked RunEpochID retirement for released slots (change 0435)"
```

---

### Task 2: Exported epoch-carrying reserve entry

**Files:**
- Modify: `internal/gatedrive/admission.go` (add one function beside `ReserveRawWorktreeExecution`)
- Test: `internal/gatedrive/admission_retire_test.go` (append one test)

**Interfaces:**
- Produces: `(*Store).ReserveWorktreeExecutionForEpoch(repoIdentity, worktreeRoot, runEpochID string, proc recoverySeam) (token string, err error)` — Task 3's app fixture consumes it.

Why: `ReserveWorktreeExecution` takes the unexported `admissionRecord`, so package `app` (where the cancel fixture lives) cannot create an epoch-owned slot; `ReserveRawWorktreeExecution` is by contract epoch-less. The spec requires the fixture to "record a real owning RunEpochID" instead of preserving the epoch-less assumption, so the store needs one exported epoch-carrying entry at the app boundary, mirroring `ReserveRawWorktreeExecution`'s shape and doc discipline. It reserves only — it launches nothing, so the 437 launch-sites guard (which binds `.Launch(` call sites) is unaffected.

- [ ] **Step 1: Write the failing test** (append to `admission_retire_test.go`)

```go
// TestReserveWorktreeExecutionForEpochRecordsOwnership: the exported epoch-carrying
// reserve records the owning RunEpochID (so the app boundary can create an
// epoch-owned slot without the unexported record type), and refuses an empty epoch.
func TestReserveWorktreeExecutionForEpochRecordsOwnership(t *testing.T) {
	// s, worktree := fixture as in Task 1
	// token, err := s.ReserveWorktreeExecutionForEpoch("repo-1", worktree, "ep-1", nil)
	// want err == nil, non-empty token; LoadWorktreeExecution → RunEpochID "ep-1",
	//   State "reserved", Kind "scopeless".
	// _, err = s.ReserveWorktreeExecutionForEpoch("repo-1", worktree2, "", nil)
	// want typed ErrInvalidID.
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/gatedrive/ -run TestReserveWorktreeExecutionForEpoch -count=1` → compile FAIL.

- [ ] **Step 3: Implement** (in `admission.go`, directly below `ReserveRawWorktreeExecution`)

```go
// ReserveWorktreeExecutionForEpoch reserves the worktree execution slot for a
// top-level execution a workflow RUN EPOCH owns (change 0435). It is the exported
// epoch-carrying sibling of ReserveRawWorktreeExecution for callers outside this
// package, where the unexported admissionRecord literal is unreachable — today the
// app layer's cancellation fixtures, which must exercise the owning-epoch
// retirement case against a slot that genuinely records its epoch. It composes a
// Kind "scopeless" record carrying runEpochID (no drive id, no scope id) and
// delegates to the same reserveWorktreeExecution every scoped, scopeless, and raw
// start admits through — one authority, one lock/CAS discipline. An empty
// runEpochID is refused ErrInvalidID: the raw (epoch-less) entry is
// ReserveRawWorktreeExecution, and the two must not blur.
func (s *Store) ReserveWorktreeExecutionForEpoch(repoIdentity, worktreeRoot, runEpochID string, proc recoverySeam) (token string, err error) {
	if runEpochID == "" {
		return "", storeErr(ErrInvalidID, "reserve-worktree-execution-epoch", nil)
	}
	token, _, err = s.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: repoIdentity,
		WorktreeRoot: worktreeRoot,
		Kind:         "scopeless",
		RunEpochID:   runEpochID,
	}, proc)
	return token, err
}
```

- [ ] **Step 4: Run to verify pass** — same command, then `go test ./internal/gatedrive/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/admission.go internal/gatedrive/admission_retire_test.go
git commit -m "feat(gatedrive): exported epoch-carrying reserve entry for the app boundary (change 0435)"
```

---

### Task 3: Ownership-checked slot teardown + epoch-owning cancel fixture

**Files:**
- Modify: `internal/app/rungate_cancel.go` (`markWorktreeSlotStopping`, `reconcileWorktreeSlot`, new `classifySlotOwnership`; call sites in `reconcileEpochTeardown`)
- Modify: `internal/app/rungate_cancel_test.go` (fixture + existing-test updates + new tests)

**Interfaces:**
- Consumes: Task 2's `ReserveWorktreeExecutionForEpoch` (fixture); existing `EpochRecord` (fields `EpochID`, `Worktree`, `Participants []EpochParticipant{Kind, NativeHandle}`), `isExecutionParticipant`.
- Produces: `classifySlotOwnership(slotEpochID, slotRawRunDir string, ep EpochRecord) slotOwnershipClass` with constants `slotOwned`, `slotLinkedLegacy`, `slotForeign`, `slotUnowned`; `markWorktreeSlotStopping(seams cancelSeams, ep EpochRecord)`; `reconcileWorktreeSlot(seams cancelSeams, ep EpochRecord) (accounted bool, finding string)`. Findings vocabulary added: `slot-foreign-owner`, `slot-ownership-unresolved`, `slot-release-failed`. Tasks 4/5/7 reuse `classifySlotOwnership`.

- [ ] **Step 1: Update the fixture and existing tests (they must go red first for the right reason)**

In `newCancelFixture`, replace the raw reservation with the epoch-owning one and register the slot's execution as an epoch participant is NOT needed (ownership flows from `RunEpochID`); keep the shape minimal:

```go
	if slot {
		runDir := filepath.Join(worktree, "run-1")
		token, terr := fx.store.ReserveWorktreeExecutionForEpoch(common, worktree, ep.EpochID, nil)
		if terr != nil {
			t.Fatalf("ReserveWorktreeExecutionForEpoch: %v", terr)
		}
		if cerr := fx.store.ConfirmWorktreeExecution(worktree, token, "run-1", runDir); cerr != nil {
			t.Fatalf("ConfirmWorktreeExecution: %v", cerr)
		}
		fx.runDir = runDir
	}
```

Add a slot-epoch reader helper beside `loadSlotState`:

```go
// loadSlotEpoch reads the worktree slot's current RunEpochID.
func loadSlotEpoch(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return slot.RunEpochID
}
```

- [ ] **Step 2: Write the new failing ownership tests**

```go
// TestCancelNeverTouchesForeignSlot (AC4): a released-then-re-owned slot carrying a
// DIFFERENT epoch is never marked, stopped, or released by this epoch's cancel.
func TestCancelNeverTouchesForeignSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	// Occupy the worktree with a FOREIGN epoch's executing slot.
	ftoken, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "foreign-epoch", nil)
	if err != nil { t.Fatalf("reserve foreign: %v", err) }
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, ftoken, "run-F", filepath.Join(fx.worktree, "run-F")); err != nil {
		t.Fatalf("confirm foreign: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (a foreign slot is not this epoch's obligation; findings=%v)", res.Disposition, res.Findings)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("a foreign slot's process must never be stopped: calls=%v", stopper.calls)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "executing" {
		t.Fatalf("foreign slot state = %q, want executing (untouched)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "foreign-epoch" {
		t.Fatalf("foreign slot epoch = %q, want foreign-epoch (untouched)", epo)
	}
	if !hasFinding(res.Findings, "slot-foreign-owner") {
		t.Fatalf("findings = %v, want the informational slot-foreign-owner", res.Findings)
	}
}

// TestCancelLeavesUnlinkedEpochlessSlot (AC4): an epoch-less slot whose execution
// is NOT independently linked to this epoch's registered participants is left
// untouched, with an unresolved-ownership finding; cancellation still completes.
func TestCancelLeavesUnlinkedEpochlessSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	rtoken, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil { t.Fatalf("reserve raw: %v", err) }
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, rtoken, "run-X", filepath.Join(fx.worktree, "run-X")); err != nil {
		t.Fatalf("confirm raw: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("an unlinked epoch-less slot must not be stopped: calls=%v", stopper.calls)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "executing" {
		t.Fatalf("epoch-less slot state = %q, want executing (untouched)", st)
	}
	if !hasFinding(res.Findings, "slot-ownership-unresolved") {
		t.Fatalf("findings = %v, want slot-ownership-unresolved", res.Findings)
	}
}

// TestCancelStopsLinkedEpochlessSlot (AC4): an epoch-less slot IS torn down when its
// exact execution (RawRunDir) is independently linked to a registered execution
// participant of this epoch.
func TestCancelStopsLinkedEpochlessSlot(t *testing.T) {
	fx := newCancelFixture(t, false)
	runDir := filepath.Join(fx.worktree, "run-L")
	rtoken, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil { t.Fatalf("reserve raw: %v", err) }
	if err := fx.store.ConfirmWorktreeExecution(fx.worktree, rtoken, "run-L", runDir); err != nil {
		t.Fatalf("confirm raw: %v", err)
	}
	if err := RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "raw-run", NativeHandle: runDir}); err != nil {
		t.Fatalf("RegisterEpochParticipant: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("linked epoch-less slot state = %q, want released", st)
	}
}
```

- [ ] **Step 3: Run to verify the new tests fail**

Run: `go test ./internal/app/ -run 'TestCancelNeverTouchesForeignSlot|TestCancelLeavesUnlinkedEpochlessSlot|TestCancelStopsLinkedEpochlessSlot' -count=1 -v`
Expected: FAIL — current code stops/releases any slot at the worktree (the foreign test reddens on `stopper.calls`/state).

- [ ] **Step 4: Implement ownership-checked teardown in `rungate_cancel.go`**

```go
// slotOwnershipClass classifies a loaded worktree slot against the fenced epoch —
// the ownership predicate every slot-touching cancel path shares (spec "Establish
// ownership before marking, stopping, releasing, or retiring; the current token
// alone is not proof of epoch ownership").
type slotOwnershipClass int

const (
	// slotOwned: the slot records this epoch's id — the epoch's own top-level execution.
	slotOwned slotOwnershipClass = iota
	// slotLinkedLegacy: an epoch-less slot whose exact execution (RawRunDir) is
	// independently linked to one of this epoch's REGISTERED execution participants.
	slotLinkedLegacy
	// slotForeign: a different nonempty RunEpochID — a foreign owner or successor.
	// Never marked, stopped, released, or cleared.
	slotForeign
	// slotUnowned: epoch-less with no independent linkage — not provably this
	// epoch's; left untouched with an unresolved-ownership finding.
	slotUnowned
)

func classifySlotOwnership(slotEpochID, slotRawRunDir string, ep EpochRecord) slotOwnershipClass {
	if slotEpochID != "" {
		if slotEpochID == ep.EpochID {
			return slotOwned
		}
		return slotForeign
	}
	if slotRawRunDir != "" {
		for _, p := range ep.Participants {
			if isExecutionParticipant(p.Kind) && p.NativeHandle == slotRawRunDir {
				return slotLinkedLegacy
			}
		}
	}
	return slotUnowned
}
```

Rewrite `markWorktreeSlotStopping` to take `ep EpochRecord` (replace the `worktree string` parameter; keep its best-effort character but gate on ownership):

```go
func markWorktreeSlotStopping(seams cancelSeams, ep EpochRecord) {
	if seams.store == nil || ep.Worktree == "" {
		return
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		return
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotOwned, slotLinkedLegacy:
		// proceed
	default:
		return // foreign or unowned: never marked
	}
	switch string(slot.State) {
	case "reserved", "executing":
		_ = seams.store.MarkWorktreeExecutionStopping(ep.Worktree, slot.ReservationToken)
	}
}
```

Rewrite `reconcileWorktreeSlot` to take `ep EpochRecord`, apply the same classification, and CHECK the release write (spec "Check all slot-write errors … A successful process stop does not prove release … was durably recorded"):

```go
func reconcileWorktreeSlot(seams cancelSeams, ep EpochRecord) (accounted bool, finding string) {
	if seams.store == nil || ep.Worktree == "" {
		return true, ""
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return true, "" // no slot to reconcile
		}
		return false, "slot-unreadable"
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotForeign:
		// A foreign owner is not this epoch's obligation: never marked, stopped,
		// released, or cleared. Launch obligations independently linked to THIS epoch
		// are still accounted by the launch reconciler (spec "Continue accounting only
		// for execution and launch obligations independently linked to the old epoch").
		return true, "slot-foreign-owner"
	case slotUnowned:
		// Epoch-less with no independent participant linkage: not provably ours —
		// left untouched, ownership surfaced (spec "Stop an epoch-less legacy slot
		// only when its exact execution is independently linked …").
		return true, "slot-ownership-unresolved"
	}
	switch string(slot.State) {
	case "released":
		return true, ""
	case "reserved", "executing", "stopping":
		_ = seams.store.MarkWorktreeExecutionStopping(ep.Worktree, slot.ReservationToken)
		if slot.RawRunDir == "" {
			return false, "slot-stop-unproven"
		}
		if stopParticipantProcess(seams, slot.RawRunDir) {
			if rerr := seams.store.ReleaseWorktreeExecution(ep.Worktree, slot.ReservationToken); rerr != nil {
				// A stopped process with an unrecorded release is NOT accounted: the
				// durable slot still claims a live execution (fail closed).
				return false, "slot-release-failed"
			}
			return true, ""
		}
		return false, "slot-stop-unproven"
	default:
		return false, "slot-state-unknown"
	}
}
```

Update both call sites in `reconcileEpochTeardown`: the per-participant loop calls `markWorktreeSlotStopping(seams, ep)` and step (5b) calls `reconcileWorktreeSlot(seams, ep)`. Update the two functions' doc comments to name the ownership rule (quote "a different nonempty RunEpochID is a foreign owner", not line numbers).

- [ ] **Step 5: Add the release-failure test (AC5, release leg)**

Simplest deterministic injection: release fails when the token has been rotated out from under the cancel between load and release — but that path is a race. Instead assert the guard directly at the unit seam: temporarily NOT practical to fake `*gatedrive.Store`. Use the racing hook the fixture already has: `stopper.onStop` replaces the slot's reservation (successor rotation is not available for a stopping slot), so use `MarkWorktreeExecutionUnresolved`? That changes state, not token. The deterministic injection that works with the real store: delete the slot's record file between load and release via `onStop`:

```go
// TestCancelReleaseWriteFailureFailsClosed (AC5): a release whose durable write
// fails (the record vanishes between the proven stop and the release) keeps the
// cancellation pending — a successful process stop never proves the release was
// recorded.
func TestCancelReleaseWriteFailureFailsClosed(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	stopper.onStop = func(runDir string) {
		if runDir != fx.runDir {
			return
		}
		// Remove the slot record so the ReleaseWorktreeExecution CAS fails typed.
		removeAdmissionRecord(t, fx.common, fx.worktree)
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (release write failed)", res.Disposition)
	}
	if !hasFinding(res.Findings, "slot-release-failed") {
		t.Fatalf("findings = %v, want slot-release-failed", res.Findings)
	}
}
```

with the helper (test file):

```go
// removeAdmissionRecord deletes the worktree slot's record file so the next slot
// write fails typed (ErrNotFound) — a deterministic durable-write failure.
func removeAdmissionRecord(t *testing.T, common, worktree string) {
	t.Helper()
	canon, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	sum := sha256.Sum256([]byte(canon))
	rec := filepath.Join(common, "docket", "gate-admission", "v1", hex.EncodeToString(sum[:]), "record.json")
	if err := os.Remove(rec); err != nil {
		t.Fatalf("remove admission record: %v", err)
	}
}
```

(The path shape is the documented storage layout in `admission.go`'s header: `<git-common-dir>/docket/gate-admission/v1/<admission-key>/record.json`, with the key being the sha256 of the canonical root. If the constant differs, read `gatedrive`'s `store.go` for `recordFileName` and adjust — assert by failing loudly, not by skipping.)

- [ ] **Step 6: Run the package**

Run: `go test ./internal/app/ -run 'TestRunCancel|TestCancel' -count=1 -v`
Expected: the three ownership tests and the release-failure test PASS; existing tests still PASS (the fixture's slot is now epoch-owned, which is `slotOwned` — same teardown behavior). Note: `TestRunCancelHappyPath` and `TestCancelRepeatResumesCleanup` will assert the retained epoch until Task 4 lands retirement — do NOT add `RunEpochID == ""` asserts yet; that is Task 4's red.

- [ ] **Step 7: Commit**

```bash
git add internal/app/rungate_cancel.go internal/app/rungate_cancel_test.go
git commit -m "feat(app): ownership-checked cancel slot teardown; epoch-owning cancel fixture (change 0435)"
```

---

### Task 4: Completion retirement in `runCancel` + interruption convergence

**Files:**
- Modify: `internal/app/rungate_cancel.go` (`cancelSeams` gains `retire`; `retireWorktreeSlotOwnership`; completion block of `runCancel`; `productionCancelSeams`)
- Modify: `internal/app/rungate_cancel_test.go`

**Interfaces:**
- Consumes: Task 1's `RetireWorktreeExecutionEpoch`, Task 3's `classifySlotOwnership`.
- Produces: `cancelSeams.retire func(worktree, epoch, token string) error` (nil → production store op), `(cancelSeams).retireSlot`, `retireWorktreeSlotOwnership(seams, ep) (bool, string)`. Findings added: `slot-retire-failed`, `slot-replaced-by-successor`, `slot-not-released`, `finalize-unpersisted`. Behavior change: a failed final epoch CAS now returns `cancellation-pending` (was `refused "finalize-failed"`).

- [ ] **Step 1: Write the failing tests**

```go
// TestCancelRetiresOwnedReleasedSlot (AC1/AC2 app half): completed cancellation
// releases AND detaches the slot — RunEpochID cleared, historical fields preserved.
func TestCancelRetiresOwnedReleasedSlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	before, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil { t.Fatalf("load before: %v", err) }
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	after, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil { t.Fatalf("load after: %v", err) }
	if after.RunEpochID != "" {
		t.Fatalf("slot RunEpochID = %q, want cleared", after.RunEpochID)
	}
	if string(after.State) != "released" {
		t.Fatalf("slot state = %q, want released", after.State)
	}
	if after.RawRunID != before.RawRunID || after.RawRunDir != before.RawRunDir ||
		after.ExecutionGen != before.ExecutionGen || after.DriveID != before.DriveID ||
		after.Kind != before.Kind {
		t.Fatalf("retirement must preserve history: before=%+v after=%+v", before, after)
	}
}

// TestCancelPendingWhenRetirementFails (AC5): a retirement write failure keeps the
// epoch cancelling and the disposition pending — never a false cancelled.
func TestCancelPendingWhenRetirementFails(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler(),
		retire: func(worktree, epoch, token string) error { return fmt.Errorf("injected retire fault") }}
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "slot-retire-failed") {
		t.Fatalf("findings = %v, want slot-retire-failed", res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (fence held, ownership intact)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != fx.epochID {
		t.Fatalf("slot epoch = %q, want retained %q", epo, fx.epochID)
	}
}

// TestCancelInterruptedBetweenRetireAndFinalizeConverges (AC5): retirement landed
// but cancelled was never persisted (simulated crash between the two writes); the
// retry revalidates, accepts the already-detached slot, and finishes the epoch
// transition — without touching a successor.
func TestCancelInterruptedBetweenRetireAndFinalizeConverges(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	// First pass: real retirement, then injected finalize failure via a seam that
	// retires for real but poisons the SECOND write by removing... — the epoch CAS
	// has no seam, so simulate the crash state directly instead:
	//  (a) run teardown to released via a real cancel whose retire seam records the
	//      call, retires for real, then the test STOPS the flow by asserting the
	//      pending path — simplest deterministic construction:
	// Fence + teardown + retire manually, leaving the epoch cancelling:
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelling; return nil }); err != nil {
		t.Fatalf("fence: %v", err)
	}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil { t.Fatalf("LoadEpochRecord: %v", err) }
	if ok, f, terr := reconcileEpochTeardown(seams, fx.repo, fx.key, ep); terr != nil || !ok {
		t.Fatalf("teardown = (%v,%v,%v), want accounted", ok, f, terr)
	}
	if ok, f := retireWorktreeSlotOwnership(seams, ep); !ok {
		t.Fatalf("retire = (false,%q), want retired", f)
	}
	// Crash happened here: slot detached, epoch still cancelling. The retry:
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("retry disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want still cleared", epo)
	}
}

// TestCancelRetireRaceWithSuccessorLeavesSuccessor (AC4): the slot is replaced by a
// successor between the cancel's load and its retire CAS — the retire refuses on
// the changed reservation, the re-read classifies the successor as foreign, and
// cancellation completes WITHOUT touching it (never retried with the successor's
// token).
func TestCancelRetireRaceWithSuccessorLeavesSuccessor(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	var raced bool
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	seams.retire = func(worktree, epoch, token string) error {
		if !raced {
			raced = true
			// The successor replaces the slot NOW: retire the old epoch out-of-band and
			// admit a new epoch's reservation (what a real winner would have produced).
			if err := fx.store.RetireWorktreeExecutionEpoch(worktree, epoch, token); err != nil {
				t.Fatalf("out-of-band retire: %v", err)
			}
			if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, worktree, "successor-epoch", nil); err != nil {
				t.Fatalf("successor reserve: %v", err)
			}
		}
		return fx.store.RetireWorktreeExecutionEpoch(worktree, epoch, token) // now refuses ErrStaleRunEpoch
	}
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (successor is not our obligation; findings=%v)", res.Disposition, res.Findings)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "successor-epoch" {
		t.Fatalf("slot epoch = %q, want the untouched successor-epoch", epo)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "reserved" {
		t.Fatalf("successor slot state = %q, want reserved (untouched)", st)
	}
}

// TestCancelConcurrentReplayIsIdempotent (AC4): two sequential replays of a
// completed cancellation are no-ops (already-cancelled) leaving slot and epoch
// byte-stable.
func TestCancelConcurrentReplayIsIdempotent(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	if res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("first = %q, want cancelled", res.Disposition)
	}
	for i := 0; i < 2; i++ {
		res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
		if res.Disposition != CancelDispositionAlreadyCancelled {
			t.Fatalf("replay %d = %q, want already-cancelled (findings=%v)", i, res.Disposition, res.Findings)
		}
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared and stable", epo)
	}
}
```

Also extend `TestRunCancelHappyPath` and `TestCancelRepeatResumesCleanup` with a final `loadSlotEpoch(...) == ""` assert (the disposition-only assert would stay green if retirement were dropped — assert fields, not only dispositions, AC3 discipline).

Note: `TestCancelConcurrentReplayIsIdempotent` depends on Task 5's terminal path returning `already-cancelled` for a quiescent, already-detached history. If Task 5 has not landed yet, the replay hits the OLD unconditional early return which also returns `already-cancelled` — the test is valid in both orderings.

- [ ] **Step 2: Run to verify the new tests fail**

Run: `go test ./internal/app/ -run 'TestCancelRetires|TestCancelPendingWhenRetirementFails|TestCancelInterrupted|TestCancelRetireRace|TestCancelConcurrentReplay|TestRunCancelHappyPath' -count=1 -v`
Expected: FAIL — `retire` field, `retireWorktreeSlotOwnership` undefined; happy path's new epoch-cleared assert red.

- [ ] **Step 3: Implement**

`cancelSeams` gains the seam (document nil = production):

```go
type cancelSeams struct {
	store    *gatedrive.Store
	stopper  cancelStopper
	native   nativeTaskCanceller
	launches epochLaunchReconciler
	// retire overrides the slot epoch-retirement write (unit tests inject faults and
	// races); nil delegates to store.RetireWorktreeExecutionEpoch. A nil store with a
	// nil retire proves nothing (fail closed).
	retire func(worktree, epoch, token string) error
}

// retireSlot performs the ownership-checked epoch retirement write through the
// seam, defaulting to the production store operation.
func (s cancelSeams) retireSlot(worktree, epoch, token string) error {
	if s.retire != nil {
		return s.retire(worktree, epoch, token)
	}
	if s.store == nil {
		return fmt.Errorf("gate store unavailable")
	}
	return s.store.RetireWorktreeExecutionEpoch(worktree, epoch, token)
}
```

`productionCancelSeams` needs no change (nil `retire` delegates to the store it already carries).

```go
// retireWorktreeSlotOwnership retires the fenced epoch's ownership of its RELEASED
// worktree slot — the cancellation-specific detachment ordinary release never
// performs (spec "Separate ordinary execution release from cancellation-specific
// epoch retirement"). It runs ONLY after complete accounting, inside the
// authorized completion decision. It returns whether ownership is accounted
// detached (retired now, already detached, absent, foreign, or not provably ours)
// and a bounded finding when it is not. On a raced CAS it re-reads ONCE and
// distinguishes a successor to leave alone from unresolved old work — never
// retrying with the successor's token.
func retireWorktreeSlotOwnership(seams cancelSeams, ep EpochRecord) (bool, string) {
	if seams.store == nil || ep.Worktree == "" {
		return true, "" // keyless/standalone: no slot ownership to retire
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return true, "" // absent after full accounting: idempotently detached
		}
		return false, "slot-unreadable"
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotForeign, slotUnowned, slotLinkedLegacy:
		// Foreign/successor: never cleared. Epoch-less (linked or not): carries no
		// ownership field to retire.
		return true, ""
	}
	// slotOwned:
	if string(slot.State) != "released" {
		return false, "slot-not-released"
	}
	if rerr := seams.retireSlot(ep.Worktree, ep.EpochID, slot.ReservationToken); rerr != nil {
		// Re-read once: a successor may have replaced the slot between the load and
		// the CAS. A foreign owner now — or an already-cleared field (a concurrent
		// replay's retirement) — is accounted; anything else stays pending.
		cur, _, lerr := seams.store.LoadWorktreeExecution(ep.Worktree)
		if lerr == nil && cur.RunEpochID == "" {
			return true, ""
		}
		if lerr == nil && cur.RunEpochID != ep.EpochID {
			return true, "slot-replaced-by-successor"
		}
		return false, "slot-retire-failed"
	}
	return true, ""
}
```

Replace `runCancel`'s completion block (step 8):

```go
	// (8) Verdict. Not fully accounted → cancellation-pending (durable fence held,
	// repeatable). Fully accounted → retire the epoch's released-slot ownership
	// UNDER THE ADMISSION LOCK, and only then CAS cancelling→cancelled (spec "In the
	// same protected completion decision, retire the matching released slot under
	// the admission lock and only then persist cancelled"). These are two existing
	// records, not a transaction: a failure before retirement leaves ownership
	// intact and cancellation pending; a failure AFTER safe detachment (all launch,
	// execution, and mutation obligations settled first) leaves the epoch fenced
	// cancelling — also pending, never refused and never a rollback — and a retry
	// revalidates the proof, accepts the already-detached slot, and finishes the
	// transition.
	if !accounted {
		return cancelResult(CancelDispositionPending, findings)
	}
	retired, rfinding := retireWorktreeSlotOwnership(seams, ep)
	if rfinding != "" {
		findings = append(findings, rfinding)
	}
	if !retired {
		return cancelResult(CancelDispositionPending, findings)
	}
	if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
		if r.State == EpochCancelling {
			r.State = EpochCancelled
		}
		return nil
	}); ferr != nil {
		findings = append(findings, "finalize-unpersisted")
		return cancelResult(CancelDispositionPending, findings)
	}
	return cancelResult(CancelDispositionCancelled, findings)
```

(Note the `ep` used for retirement is the record reloaded after the fence — its `EpochID` and `Worktree` are immutable for the run's lifetime; the teardown's own re-enumeration read `reEp` for participants/mutations.)

Update the file-header ORDER paragraph: after "all accounted → " insert "retire the epoch's released-slot ownership (RetireWorktreeExecutionEpoch), then ".

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestRunCancel|TestCancel' -count=1 -v`
Expected: all PASS, including all pre-existing cancel tests.

- [ ] **Step 5: Mutation-probe the completion order**

Mutations, one at a time (with backup copies, restore after each; `-count=1`):
1. Swap the order — persist cancelled BEFORE `retireWorktreeSlotOwnership` and ignore its result: `TestCancelPendingWhenRetirementFails` must redden (it asserts state cancelling + retained epoch).
2. Drop the `retireWorktreeSlotOwnership` call entirely: `TestCancelRetiresOwnedReleasedSlot` and the happy-path epoch-cleared assert must redden.
3. In `retireWorktreeSlotOwnership`, make the raced re-read retry with `cur.ReservationToken`: `TestCancelRetireRaceWithSuccessorLeavesSuccessor` must redden (successor epoch cleared).

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_cancel.go internal/app/rungate_cancel_test.go
git commit -m "feat(app): authorized cancel completion retires released-slot epoch ownership (change 0435)"
```

---

### Task 5: Bounded terminal repair + guardian-never-retires

**Files:**
- Modify: `internal/app/rungate_cancel.go` (replace the `EpochCancelled, EpochSuperseded` early return; add `verifyTerminalEpochQuiescence`, `repairTerminalEpoch`)
- Modify: `internal/app/rungate_cancel_test.go`

**Interfaces:**
- Consumes: Tasks 3–4 helpers; `LoadEpochRecord`; the `launches` seam.
- Produces: `verifyTerminalEpochQuiescence(seams cancelSeams, ep EpochRecord) (bool, []string)` (Task 7 reuses it); `repairTerminalEpoch(seams cancelSeams, ep EpochRecord) RunCancelResult`. Behavior change: a terminal cancel is no longer an unconditional `already-cancelled` — it is the spec's bounded repair. **Existing-test consequence:** `TestRunCancelAlreadyCancelled` must now inject `launches: okLaunchReconciler()` (a nil reconciler on the terminal path is unverifiable → refused, by design).

- [ ] **Step 1: Write the failing tests**

```go
// TestTerminalRepairRetiresHistoricalStaleSlot (AC6): a durably CANCELLED epoch
// whose released slot still carries its RunEpochID (the recorded incident shape:
// changes 434/368) is repaired by an authorized repeat cancel — disposition
// cancelled/applied, slot detached, epoch state untouched-terminal.
func TestTerminalRepairRetiresHistoricalStaleSlot(t *testing.T) {
	fx := newCancelFixture(t, true)
	// Manufacture the historical defect: release WITHOUT retirement, then force the
	// epoch terminal (what the pre-0435 cancel produced).
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil { t.Fatalf("load: %v", err) }
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (historical retirement applied; findings=%v)", res.Disposition, res.Findings)
	}
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared", epo)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled (never regressed)", st)
	}
	// Repeated repair is a no-op.
	res2 := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human repair")
	if res2.Disposition != CancelDispositionAlreadyCancelled || res2.Result != ResultNoOp {
		t.Fatalf("repeat = (%q,%q), want (already-cancelled,no-op)", res2.Disposition, res2.Result)
	}
}

// TestTerminalRepairSupersededSlot (AC6): the same repair works for a SUPERSEDED
// epoch's stale released slot, and never regresses the superseded state.
func TestTerminalRepairSupersededSlot(t *testing.T) { /* as above with r.State = EpochSuperseded; same asserts with EpochSuperseded */ }

// TestTerminalRepairRefusesUnsafeHistories (AC6): each unsafe terminal history is
// refused with a specific finding, with NO slot or epoch mutation, and never
// cancellation-pending over durable terminal state.
func TestTerminalRepairRefusesUnsafeHistories(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, fx cancelFixture) cancelSeams
		finding string
	}{
		{"busy-claim-unresolved-relaunch", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{report: gatedrive.EpochLaunchReport{Accounted: false, Findings: []string{"claim-busy:d1"}}}}
		}, "claim-busy:d1"},
		{"contradictory-mutation", func(t *testing.T, fx cancelFixture) cancelSeams {
			if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
				r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
				return nil
			}); err != nil { t.Fatalf("seed mutation: %v", err) }
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
		}, "mutation-pending:pr.publish"},
		{"unreadable-launch-evidence", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{err: fmt.Errorf("injected")}}
		}, "launch-reconcile-failed"},
		{"nonreleased-owned-slot", func(t *testing.T, fx cancelFixture) cancelSeams {
			// slot left executing (fixture default) — owned but not released
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
		}, "slot-not-released"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t, true)
			if tc.name != "nonreleased-owned-slot" {
				slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
				if err != nil { t.Fatalf("load: %v", err) }
				if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
					t.Fatalf("release: %v", err)
				}
			}
			seams := tc.arrange(t, fx)
			if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error { r.State = EpochCancelled; return nil }); err != nil {
				t.Fatalf("force cancelled: %v", err)
			}
			epochBefore := loadSlotEpoch(t, fx.store, fx.worktree)
			res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human repair")
			if res.Disposition != CancelDispositionRefused {
				t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
			}
			if !hasFinding(res.Findings, tc.finding) {
				t.Fatalf("findings = %v, want %q", res.Findings, tc.finding)
			}
			if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelled {
				t.Fatalf("epoch state = %q, want cancelled (no regression, no revival)", st)
			}
			if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != epochBefore {
				t.Fatalf("slot epoch changed %q→%q under a refused repair", epochBefore, epo)
			}
		})
	}
}

// TestGuardianReapsButNeverRetires: the death guardian's fence+reap releases the
// proven-stopped slot but RETAINS RunEpochID and leaves the epoch CANCELLING —
// only authorized run.cancel completion retires (spec "The death guardian may
// perform teardown but never retires epoch ownership").
func TestGuardianReapsButNeverRetires(t *testing.T) {
	fx := newCancelFixture(t, true)
	// guardianFenceAndReap composes productionCancelSeams, whose stopper/reconciler
	// reach the real process service — unavailable here. Drive its exact sequence
	// with injected seams instead: fence, then the SAME teardown accounting, and
	// assert what the guardian contract asserts — no retirement, no finalize.
	guardianFenceAndReapWithSeams := func() {
		ferr := epochCAS(fx.repo, fx.key, func(rec *EpochRecord) error {
			if rec.State == EpochActive { rec.State = EpochCancelling }
			return nil
		})
		if ferr != nil { t.Fatalf("fence: %v", ferr) }
		ep, _, err := LoadEpochRecord(fx.repo, fx.key)
		if err != nil { t.Fatalf("LoadEpochRecord: %v", err) }
		_, _, _ = reconcileEpochTeardown(cancelSeams{store: fx.store,
			stopper: &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}},
			launches: okLaunchReconciler()}, fx.repo, fx.key, ep)
	}
	guardianFenceAndReapWithSeams()
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (guardian never finalizes)", st)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("slot state = %q, want released (guardian reaps)", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != fx.epochID {
		t.Fatalf("slot epoch = %q, want retained %q (guardian never retires ownership)", epo, fx.epochID)
	}
	// The authorized completion then retires and finalizes.
	res := runCancel(cancelSeams{store: fx.store,
		stopper: &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}},
		launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("authorized completion = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want cleared by the authorized path", epo)
	}
}
```

The guardian test proves the CONTRACT holds for the shared teardown (`reconcileEpochTeardown` performs no retirement); the syntactic complement — that `guardianFenceAndReap` itself never calls a retire — is covered by keeping retirement exclusively inside `runCancel`'s post-`accounted` block and `repairTerminalEpoch`, which the guardian never reaches (it returns before the state switch's terminal branch and never finalizes). Add one grep-shaped guard ONLY if the reviewer requests; the behavioral assert above is the load-bearing one.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/ -run 'TestTerminalRepair|TestGuardianReapsButNeverRetires|TestRunCancelAlreadyCancelled' -count=1 -v`
Expected: FAIL — terminal path still returns unconditional already-cancelled; repair tests redden.

- [ ] **Step 3: Implement**

Replace the terminal early return in `runCancel`:

```go
	switch ep.State {
	case EpochCancelled, EpochSuperseded:
		// Terminal state by itself is not proof of quiescence: run the bounded
		// historical repair (spec "Terminal repair and resume use the same proof").
		// Authority was already validated above; the repair never revives the epoch,
		// replays mutations, resets budgets, regresses terminal state, or stops a
		// replacement's process.
		return repairTerminalEpoch(seams, ep)
	case EpochActive:
		…unchanged…
```

```go
// verifyTerminalEpochQuiescence revalidates a terminal (cancelled/superseded)
// epoch's EXISTING launch and mutation evidence using the same bounded accounting
// cancellation uses — 437's launch reconciler plus the admitted-mutation journal.
// It fails closed: an unavailable or erroring reconciler, an unaccounted launch
// obligation (a busy claim, an unresolved relaunch), or an admitted-not-completed
// mutation is non-quiescence with a bounded finding. It performs no epoch write.
// Task 7's resume validation consumes it unchanged.
func verifyTerminalEpochQuiescence(seams cancelSeams, ep EpochRecord) (bool, []string) {
	var findings []string
	quiescent := true
	if seams.launches == nil {
		findings = append(findings, "launch-reconciler-unavailable")
		quiescent = false
	} else if report, err := seams.launches.reconcile(ep.Worktree, ep.EpochID); err != nil {
		findings = append(findings, "launch-reconcile-failed")
		quiescent = false
	} else {
		findings = append(findings, report.Findings...)
		if !report.Accounted {
			quiescent = false
		}
	}
	for _, m := range ep.AdmittedMutations {
		if m.Status != mutationStatusCompleted {
			findings = append(findings, "mutation-pending:"+m.OpKey)
			quiescent = false
		}
	}
	return quiescent, findings
}

// repairTerminalEpoch is the bounded repair a repeat run.cancel performs against a
// DURABLY terminal epoch: after re-proving quiescence it retires a released slot
// that still carries this epoch, and otherwise no-ops idempotently. An unsafe or
// unverifiable history is refused with its specific finding — never
// cancellation-pending over durable terminal state, never a regression to
// cancelling, never a revived epoch, and never a touched successor. It is not a
// general recovery engine: it uses only the records already present, and missing
// evidence fails closed to refused.
func repairTerminalEpoch(seams cancelSeams, ep EpochRecord) RunCancelResult {
	quiescent, findings := verifyTerminalEpochQuiescence(seams, ep)
	if !quiescent {
		return cancelResult(CancelDispositionRefused, findings)
	}
	if seams.store == nil || ep.Worktree == "" {
		return cancelResult(CancelDispositionAlreadyCancelled, findings)
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			// A missing slot is an idempotent no-op ONLY after the accounting above.
			return cancelResult(CancelDispositionAlreadyCancelled, findings)
		}
		return cancelResult(CancelDispositionRefused, append(findings, "slot-unreadable"))
	}
	if classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) != slotOwned {
		// An empty epoch field or a foreign successor: nothing of this epoch's to
		// repair (the foreign slot is neither touched nor read as disproof).
		return cancelResult(CancelDispositionAlreadyCancelled, findings)
	}
	if string(slot.State) != "released" {
		// A nonreleased owned slot under a terminal epoch is contradictory history —
		// not safely repairable by this bounded path. Leave it fenced and untouched.
		return cancelResult(CancelDispositionRefused, append(findings, "slot-not-released"))
	}
	if rerr := seams.retireSlot(ep.Worktree, ep.EpochID, slot.ReservationToken); rerr != nil {
		cur, _, lerr := seams.store.LoadWorktreeExecution(ep.Worktree)
		if lerr == nil && (cur.RunEpochID == "" || cur.RunEpochID != ep.EpochID) {
			return cancelResult(CancelDispositionAlreadyCancelled, findings)
		}
		return cancelResult(CancelDispositionRefused, append(findings, "slot-retire-failed"))
	}
	return cancelResult(CancelDispositionCancelled, findings)
}
```

Refactor note (DRY): rewrite `reconcileEpochTeardown`'s step (5c) and (7) to CALL nothing new — they keep their own inline logic (their accounting also covers participants and re-enumeration, which terminal repair deliberately does not re-prove). Do NOT try to force the two paths through one function; the shared parts are exactly `verifyTerminalEpochQuiescence`'s two legs, and the cancelling path's copies stay where they are (they interleave with stops/re-enumeration). Update `TestRunCancelAlreadyCancelled` to inject `launches: okLaunchReconciler()` and keep its `stopper.calls == 0` assert (repair stops nothing through the cancel stopper).

Update the file-header ORDER paragraph's last sentence: "A repeat against a cancelling epoch RESUMES cleanup…; against a cancelled/superseded epoch it performs the bounded historical repair (repairTerminalEpoch): quiescence-checked retirement of a stale released slot, an idempotent already-cancelled when there is nothing to repair, and a refused finding for an unsafe history."

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestTerminalRepair|TestGuardianReaps|TestRunCancel|TestCancel' -count=1 -v` → PASS.

- [ ] **Step 5: Mutation-probe the repair guards**

One at a time (backup, restore, `-count=1`):
1. Make `repairTerminalEpoch` skip `verifyTerminalEpochQuiescence` (treat as quiescent): `TestTerminalRepairRefusesUnsafeHistories` (busy-claim + contradictory-mutation + unreadable legs) must redden.
2. Make the nonreleased-owned branch retire anyway: the `nonreleased-owned-slot` leg must redden.
3. Make refusal return `CancelDispositionPending`: the refused asserts must redden (spec: never pending over durable terminal state).

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_cancel.go internal/app/rungate_cancel_test.go
git commit -m "feat(app): bounded terminal repair for historical stale slots; guardian never retires (change 0435)"
```

---

### Task 6: Post-retirement admission proofs + old-epoch launch foreclosure + accounting neutrality

**Files:**
- Modify: `internal/app/rungate_cancel_test.go` (three tests; no production code expected — this task PROVES cross-cutting acceptance criteria against the code Tasks 1–5 landed)

**Interfaces:**
- Consumes: `rawStaleEpochRefusal(store, worktreeRoot)` (`internal/app/gate.go`), `epochLaunchGate(gitCommonDir)` (`internal/app/rungate_gate.go`, signature `func(epochID, worktree string, reserve func() error) error`), Task 2's reserve, the completed-cancel fixture flow.

- [ ] **Step 1: Write the tests (red only if Tasks 1–5 missed something — a green first run is acceptable HERE only because each test is then mutation-probed in Step 3)**

```go
// TestFinalizeGateAdmitsAfterRetirement (AC1): before retirement the epoch-owned
// released slot blocks an epoch-less raw/finalize launch (stale-run-epoch) and a
// different-epoch reservation; after authorized cancellation both admit.
func TestFinalizeGateAdmitsAfterRetirement(t *testing.T) {
	fx := newCancelFixture(t, true)
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil { t.Fatalf("load: %v", err) }
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release: %v", err)
	}
	// BEFORE: the released slot still owns the worktree.
	if _, refused := rawStaleEpochRefusal(fx.store, fx.worktree); !refused {
		t.Fatal("pre-retirement: an epoch-less raw launch must be refused stale-run-epoch")
	}
	if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "replacement-epoch", nil); err == nil {
		t.Fatal("pre-retirement: a different epoch's reservation must be refused")
	}
	// Authorized cancellation retires (slot already released; teardown is vacuous).
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	// AFTER: the epoch-less finalize gate no longer refuses…
	if _, refused := rawStaleEpochRefusal(fx.store, fx.worktree); refused {
		t.Fatal("post-retirement: rawStaleEpochRefusal must not refuse an epoch-less launch")
	}
	// …and a replacement epoch's build-gate reservation admits.
	if _, err := fx.store.ReserveWorktreeExecutionForEpoch(fx.common, fx.worktree, "replacement-epoch", nil); err != nil {
		t.Fatalf("post-retirement replacement reserve: %v", err)
	}
}

// TestRetirementDoesNotUnfenceOldEpochLaunches (AC8): after retirement the OLD
// epoch's fresh start is still refused by the 437 launch gate — clearing slot
// ownership never revives the cancelled epoch's launch authority.
func TestRetirementDoesNotUnfenceOldEpochLaunches(t *testing.T) {
	fx := newCancelFixture(t, true)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	if res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel = %q, want cancelled", res.Disposition)
	}
	gate := epochLaunchGate(fx.common)
	reserveRan := false
	err := gate(fx.epochID, fx.worktree, func() error { reserveRan = true; return nil })
	if err == nil {
		t.Fatal("the cancelled epoch's launch authorization must be refused after retirement")
	}
	if reserveRan {
		t.Fatal("the refused gate must never run the reservation body")
	}
}

// TestRepairChargesNothing (AC8): terminal repair — like cancellation — touches
// neither the suite budget nor the gate retry markers. Mirrors
// TestCancelNeverChargesOrResets for the repair path.
func TestRepairChargesNothing(t *testing.T) {
	// Same seeding as TestCancelNeverChargesOrResets (ConsumeGateRetry + a reserved
	// suite attempt), then: force the historical stale state (release without
	// retirement + epoch forced cancelled, as in TestTerminalRepairRetiresHistoricalStaleSlot),
	// run the repair cancel, and assert retry usage, budget usage, Retry marker, and
	// AttemptLimit are all unchanged while the repair reports cancelled.
}
```

(Write `TestRepairChargesNothing` in full by copying the seeding/assert body of `TestCancelNeverChargesOrResets` — it is in the same file — with the repair arrangement between seeding and asserts. Repeat the code; do not reference it by name only.)

- [ ] **Step 2: Run**

Run: `go test ./internal/app/ -run 'TestFinalizeGateAdmitsAfterRetirement|TestRetirementDoesNotUnfence|TestRepairChargesNothing' -count=1 -v`
Expected: PASS (these criteria should already hold). Any FAIL is a real Task 1–5 defect — fix it there, never weaken the test.

- [ ] **Step 3: Mutation-probe (this is what makes green-first acceptable)**

1. In `RetireWorktreeExecutionEpoch`, clear `State` to a fresh value or skip the `released` requirement → `TestFinalizeGateAdmitsAfterRetirement`'s pre-retirement refusal asserts and Task 1 tests must redden.
2. In `epochLaunchGate` usage nothing is modified by this change — mutate the TEST instead to prove it can fail: point the gate at a still-ACTIVE epoch fixture and confirm the reservation runs (sanity that the assert discriminates), then restore.
3. Re-run with `-count=1`; restore everything.

- [ ] **Step 4: Commit**

```bash
git add internal/app/rungate_cancel_test.go
git commit -m "test(app): post-retirement admission, old-epoch launch foreclosure, repair accounting neutrality (change 0435)"
```

---

### Task 7: Resume quiescence validation in `run.gate-before --resume`

**Files:**
- Modify: `internal/app/rungate_before.go` (`GateScopeDeps` + the `EpochCancelled`/`EpochSuperseded` branches + `validateResumeQuiescence`)
- Modify: `internal/app/rungate_before_resume_test.go`

**Interfaces:**
- Consumes: Task 5's `verifyTerminalEpochQuiescence`, Task 4's slot-retirement pieces (`classifySlotOwnership`, `cancelSeams.retireSlot`), `productionCancelSeams`.
- Produces: `GateScopeDeps.CancelSeams func(repoDir string) cancelSeams` (nil → production); `validateResumeQuiescence(seams cancelSeams, ep EpochRecord) (ok bool, detail string)`. Refusals ride the EXISTING gate-unarmed channel with `ReasonGateResumeCancellationPending` — no new lifecycle state, no new reason token.

- [ ] **Step 1: Update existing resume tests to inject the seam (they must stay green)**

Every existing call in `rungate_before_resume_test.go` that reaches the `EpochCancelled`/`EpochSuperseded` branches (`TestResumeAfterCancelledSupersedesOnce`, `TestRepeatArmObservesReservation`, `TestResumeDoesNotResetSuiteBudget`) gets a permissive injected seam on its `GateScopeDeps` (adapt to the file's existing `sp.deps()` helper — extend that helper so ALL its callers inherit the seam):

```go
	deps.CancelSeams = func(repoDir string) cancelSeams {
		return cancelSeams{launches: okLaunchReconciler()}
	}
```

(A nil store in the seam makes the slot leg vacuous — correct for fixtures that bind no worktree slot. `okLaunchReconciler` and `fakeLaunchReconciler` already live in `rungate_cancel_test.go`, same package.)

- [ ] **Step 2: Write the failing tests**

```go
// TestResumeDeniedWhileOldEpochNotQuiescent (AC6/AC7): a durably cancelled epoch
// whose launch evidence is still unsettled cannot authorize a replacement — the
// arm refuses on the existing gate-unarmed channel, mints no record, reserves no
// replacement, and touches no slot.
func TestResumeDeniedWhileOldEpochNotQuiescent(t *testing.T) {
	// Arrange the file's standard cancelled-epoch resume fixture (as
	// TestResumeAfterCancelledSupersedesOnce arranges it), but inject:
	//   deps.CancelSeams = func(string) cancelSeams {
	//       return cancelSeams{launches: &fakeLaunchReconciler{report: gatedrive.EpochLaunchReport{
	//           Accounted: false, Findings: []string{"claim-busy:d1"}}}}
	//   }
	// res := RunGateBefore(..., resumeID)
	// want: res.Armed == false, res.Reason == ReasonGateResumeCancellationPending,
	//       res.Message contains "claim-busy:d1",
	//       the old epoch still EpochCancelled with ReplacementReserved == "".
}

// TestResumeRetiresStaleSlotThenReservesOnce (AC1/AC7): a cancelled epoch whose
// released slot still carries its RunEpochID is retired by the resume validation,
// then EXACTLY ONE replacement is reserved; a repeat arm observes the same key.
func TestResumeRetiresStaleSlotThenReservesOnce(t *testing.T) {
	// Arrange the cancelled-epoch resume fixture WITH a worktree-bound epoch and a
	// released, epoch-owning slot (ReserveWorktreeExecutionForEpoch + Confirm +
	// Release with the epoch's id, at the epoch's bound Worktree), permissive
	// launches seam AND a real store in the seam:
	//   deps.CancelSeams = func(string) cancelSeams {
	//       return cancelSeams{store: store, launches: okLaunchReconciler()}
	//   }
	// first := RunGateBefore(...): want Armed == true.
	// assert the slot's RunEpochID is now "" (retired) and State still "released"
	//   until the replacement's own drive reserves it.
	// second := RunGateBefore(...) (repeat arm): want Armed == false,
	//   Reason == ReasonGateResumeReplacementReserved, Key == the reserved key —
	//   the same single reservation, never a second (exactly-one preserved).
}

// TestResumeSupersededValidatesBeforeObserve (AC6): re-authorizing a previously
// reserved replacement from a SUPERSEDED epoch also requires quiescence; unsettled
// evidence refuses without touching the reservation.
func TestResumeSupersededValidatesBeforeObserve(t *testing.T) {
	// Arrange a superseded old epoch with ReplacementReserved set (run the winner arm
	// first with permissive seams, as TestRepeatArmObservesReservation does), then
	// re-arm with the non-accounted fakeLaunchReconciler seam:
	// want Armed == false, Reason == ReasonGateResumeCancellationPending,
	// and the old epoch's ReplacementReserved UNCHANGED (successor never altered).
}

// TestResumeForeignSlotIsNeutral (AC7): a slot owned by a DIFFERENT epoch neither
// blocks nor is touched by resume — quiescent old-epoch evidence still admits the
// replacement, and the foreign slot is byte-identical after.
func TestResumeForeignSlotIsNeutral(t *testing.T) {
	// Cancelled-epoch resume fixture; seed the worktree slot with epoch
	// "someone-else" (ReserveWorktreeExecutionForEpoch), seams with real store +
	// okLaunchReconciler. want Armed == true and the slot still records
	// "someone-else" with its state unchanged.
}
```

(Each test adapts the file's own fixture helpers — read the top of `rungate_before_resume_test.go` first and reuse its repo/claim/epoch arrangement verbatim rather than inventing a parallel fixture. The slot-bearing tests must bind the epoch's `Worktree` to the same directory they reserve the slot for, via the same `epochCAS` worktree bind the cancel fixture uses.)

- [ ] **Step 3: Run to verify the new tests fail**

Run: `go test ./internal/app/ -run 'TestResumeDenied|TestResumeRetires|TestResumeSuperseded|TestResumeForeign' -count=1 -v`
Expected: FAIL — `CancelSeams` field and `validateResumeQuiescence` undefined; today's resume reserves without validation.

- [ ] **Step 4: Implement in `rungate_before.go`**

`GateScopeDeps` gains the seam:

```go
type GateScopeDeps struct {
	Prepare func(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)
	// CancelSeams overrides the cancellation seams the resume path's old-epoch
	// quiescence validation composes (change 0435); nil composes
	// productionCancelSeams(repoDir). Unit tests inject permissive or adversarial
	// seams; production callers leave it nil.
	CancelSeams func(repoDir string) cancelSeams
}
```

```go
// validateResumeQuiescence re-proves the OLD epoch's quiescence before resume may
// reserve a replacement (EpochCancelled) or re-authorize a previously reserved one
// (EpochSuperseded) — the same bounded proof terminal repair uses (spec "Terminal
// repair and resume use the same proof"; the cancellation command's last reported
// disposition is not durable authority). When the evidence is accounted and a
// RELEASED slot still carries the old epoch, it performs the same ownership-checked
// retirement so the replacement's own reservation is not refused stale-run-epoch.
// It never cancels or alters an already-reserved successor, and a foreign slot
// alone neither proves nor disproves quiescence. Incomplete or unreadable proof
// returns ok=false with a bounded, credential-free detail for the gate-unarmed
// message; it creates no replacement and yields no dispatch authorization.
func validateResumeQuiescence(seams cancelSeams, ep EpochRecord) (ok bool, detail string) {
	quiescent, findings := verifyTerminalEpochQuiescence(seams, ep)
	if !quiescent {
		return false, strings.Join(findings, "; ")
	}
	if seams.store == nil || ep.Worktree == "" {
		return true, ""
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, aok := gatedrive.AsStoreError(err); aok && se.Kind == gatedrive.ErrNotFound {
			return true, ""
		}
		return false, "slot-unreadable"
	}
	if classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) != slotOwned {
		return true, "" // absent ownership, or a foreign successor: neutral
	}
	if string(slot.State) != "released" {
		return false, "slot-not-released"
	}
	if rerr := seams.retireSlot(ep.Worktree, ep.EpochID, slot.ReservationToken); rerr != nil {
		cur, _, lerr := seams.store.LoadWorktreeExecution(ep.Worktree)
		if lerr == nil && (cur.RunEpochID == "" || cur.RunEpochID != ep.EpochID) {
			return true, "" // concurrently retired, or replaced by a successor: neutral
		}
		return false, "slot-retire-failed"
	}
	return true, ""
}
```

In `RunGateBefore`, resolve the seams once inside the `if foundEp` block (before the switch):

```go
			seams := productionCancelSeams(repoDir)
			if sdeps.CancelSeams != nil {
				seams = sdeps.CancelSeams(repoDir)
			}
```

Extend the two branches (the validation is integrated into this already-serialized resume decision — the epoch-state read plus the supersede CAS's one-winner point — never a separate unlocked preflight; every mutation it performs is individually CAS-guarded, so a raced state change refuses rather than blindly retrying):

```go
			case EpochSuperseded:
				if oldEp.ReplacementReserved == "" {
					return gateUnarmedMsg(ReasonGateResumeEpochUnreadable,
						"change "+scopeChangeID+" was superseded without a recorded replacement")
				}
				if qok, detail := validateResumeQuiescence(seams, oldEp); !qok {
					return gateUnarmedMsg(ReasonGateResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); the reserved replacement cannot be re-authorized until it is resolved")
				}
				return gateResumeObserve(oldEp.ReplacementReserved)
			case EpochCancelled:
				if qok, detail := validateResumeQuiescence(seams, oldEp); !qok {
					return gateUnarmedMsg(ReasonGateResumeCancellationPending,
						"change "+scopeChangeID+" has unresolved cancellation evidence ("+detail+
							"); resume cannot reserve a replacement — resolve it with 'docket run cancel'")
				}
				return armResumeReplacement(…unchanged…)
```

Add `"strings"` to imports if absent.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestResume|TestRepeatArm' -count=1 -v` → PASS (new + existing).

- [ ] **Step 6: Mutation-probe**

1. Delete the `EpochCancelled` branch's validation call → `TestResumeDeniedWhileOldEpochNotQuiescent` reddens.
2. Delete the `EpochSuperseded` branch's validation call → `TestResumeSupersededValidatesBeforeObserve` reddens.
3. In `validateResumeQuiescence`, retire a `slotForeign` slot too → `TestResumeForeignSlotIsNeutral` reddens.
Restore each with backups; `-count=1` throughout.

- [ ] **Step 7: Commit**

```bash
git add internal/app/rungate_before.go internal/app/rungate_before_resume_test.go
git commit -m "feat(app): resume validates old-epoch quiescence and retires stale slots before replacement (change 0435)"
```

---

### Task 8: Full-suite gate, budget report, and doc-comment coherence sweep

**Files:**
- Modify (verify only, edit if the sweep finds drift): `internal/app/rungate_cancel.go`, `internal/gatedrive/admission.go` file headers; CLI help/docs are untouched (no new command).

- [ ] **Step 1: Thesis-over-the-diff self-audit** (learning `fix-reintroduces-its-own-defect-class`)

The change's thesis is "no slot mutation without proven epoch ownership, and no completion claim over an unchecked write." Grep the branch's own additions for the class:
- every new `seams.store.` / `seams.retireSlot` / `ReleaseWorktreeExecution` call site: is its error checked or explicitly documented best-effort with a downstream authoritative check?
- every new probe (`LoadWorktreeExecution`) that branches on error: does "unknown" share a branch with "absent" anywhere a destructive/authorizing action follows? (learning `probe-error-is-not-clean-absence` — only the `ErrNotFound`-mapped legs may treat absence as benign, and each must sit behind completed accounting).
- the twin check: `markWorktreeSlotStopping` (best-effort) vs `reconcileWorktreeSlot` (authoritative) — confirm the ownership classification is applied in BOTH (the untouched twin is where the class reappears).

- [ ] **Step 2: Run the whole suite from source**

Run (from the feature worktree root): `go run ./cmd/docket development test`
Expected: SUITE green. This is the build gate's own command (`build.test_command`) — never a subset.

- [ ] **Step 3: Read the budget report**

Even on green: any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note in the results evidence; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (serial confirmation per `tests/README.md`) before the task completes.

- [ ] **Step 4: Commit any sweep fixes**

```bash
git add -u internal/app internal/gatedrive
git commit -m "docs(app,gatedrive): retirement contract coherence sweep (change 0435)"
```

(Skip the commit if the sweep changed nothing.)

---

## Acceptance-criteria coverage map (self-review record)

| Spec AC | Where proven |
|---|---|
| 1 — stale slot rejects other/epoch-less reserve; cancel detaches; replacement + finalize admit | Task 1 `TestOrdinaryReleaseStillRetainsEpoch`/`TestAdmissionAfterRetirement`; Task 6 `TestFinalizeGateAdmitsAfterRetirement`; Task 7 `TestResumeRetiresStaleSlotThenReservesOnce` |
| 2 — already-released and executing-then-released; field preservation; ordinary release unchanged | Task 1 `TestRetireClearsOnlyRunEpochID` (already-released), Task 4 `TestCancelRetiresOwnedReleasedSlot` (executing→released), Task 1 `TestOrdinaryReleaseStillRetainsEpoch` |
| 3 — 437 barriers carried forward; unsettled obligations block retirement; assert fields not dispositions | untouched 437 suites (`internal/gatedrive` epoch-fence/reconcile tests) + Task 4 `TestCancelPendingWhenRetirementFails` (field asserts) + existing `TestCancelPendingWhileLaunchObligationUnresolved` (epoch retained implied by pending; slot-epoch assert added in Task 4's happy-path extension) + Task 5 unsafe-history legs |
| 4 — never stop/mutate foreign; replacement-between-load-and-mutation; concurrent replay; stale predecessor vs successor; epoch-less linkage | Task 3 three ownership tests; Task 4 `TestCancelRetireRaceWithSuccessorLeavesSuccessor`, `TestCancelConcurrentReplayIsIdempotent`; Task 1 `TestRetireRefusesTokenMismatch` |
| 5 — release/retirement/final-write failure injection; interrupt + convergent retry; no false completion | Task 3 `TestCancelReleaseWriteFailureFailsClosed`; Task 4 `TestCancelPendingWhenRetirementFails`, `TestCancelInterruptedBetweenRetireAndFinalizeConverges` |
| 6 — historical repair (cancelled + superseded), idempotent repeat, unsafe → refused, cannot authorize resume | Task 5 repair tests; Task 7 `TestResumeDeniedWhileOldEpochNotQuiescent`, `TestResumeSupersededValidatesBeforeObserve` |
| 7 — proven-complete resumable; interrupted-final-write repaired then resumable; one replacement/same key; foreign slot neutral | Task 4 convergence + Task 7 `TestResumeRetiresStaleSlotThenReservesOnce`, `TestResumeForeignSlotIsNeutral` |
| 8 — retirement never revives old-epoch launches; no charge/reset | Task 6 `TestRetirementDoesNotUnfenceOldEpochLaunches`, `TestRepairChargesNothing`; existing `TestCancelNeverChargesOrResets` |
| 9 — mutation-test the guards; full source-resolved suite; budget report | mutation steps in Tasks 1, 4, 5, 6, 7; Task 8 |

Deliberate scope guards honored: no change to `ReleaseWorktreeExecution`, `reserveWorktreeExecution`'s fence, `rawStaleEpochRefusal`, takeover, raw-gate recover, or mutation-owner lookup; no new store/schema/state/CLI; 437's accounting consumed through the existing `epochLaunchReconciler` seam only.

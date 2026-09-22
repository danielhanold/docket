<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0441 — Release successful implementation ownership before standalone finalize](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-22-0441-release-successful-implementation-ownership-before-standalon.md)**
<!-- docket:backlink:end -->
# Release Successful Implementation Ownership Before Standalone Finalize — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a keyed implementation run is verified complete and its registered work is proven settled, close out the run's epoch ownership (a durable `completing`→`completed` lifecycle) so a standalone finalize gate can admit on the same worktree without a stale-run-epoch refusal or a human cancellation — while implementation with outstanding work keeps full exclusion.

**Architecture:** Extend the existing run-epoch lifecycle (`internal/app/rungate_epoch.go`) with two states, `completing` (durable success fence, accounting unfinished) and `completed` (retirement finished), driven ONLY from the attributed keyed `RunGateVerdict` path on a verified `run-complete`. The closeout reuses cancellation's accounting shapes through narrowly factored observation-only helpers (it stops nothing, signals nothing, settles nothing), persists exact native-task terminal observation into the existing `EpochParticipant` entries at the codexentry adapter boundary, retires the released worktree slot through the existing `RetireWorktreeExecutionEpoch`, and excludes fully completed epochs from ambient worktree-owner lookup so finalize and later mutations proceed.

**Tech Stack:** Go (stdlib only, matching the package); flock + atomic-rename durable records (existing `epochCAS` / `admissionCAS`); table-driven Go tests with injected seams (`cancelSeams` pattern).

**Spec:** `docs/superpowers/specs/2026-09-21-release-successful-implementation-ownership-before-standalon-design.md` (on the `docket` metadata branch; change 0441). The spec is authority for scope and the 8 acceptance criteria.

## Global Constraints

- Only the attributed, keyed `RunGateVerdict` path may drive completion. `RunVerify` stays read-only; unattributed verdicts (`gate-observe`) stay observe-only and never change ownership.
- The success path must NOT invoke native cancellation, stop or signal any process, or settle a never-launched reservation terminal — observation only. A live, busy, pending, uncertain, or unreadable obligation blocks completion (fail closed).
- Never encode success as cancellation: `completing`/`completed` are new states; a cancelling/cancelled/superseded/mismatched run is never relabelled successful, and explicit human cancellation may win from `completing` (completion then loses without reporting success).
- Missing historical terminal evidence means unproven, never implicitly complete. Preserve readable cancellation history; keep `epochSchemaVersion = 1` with additive `omitempty` fields only.
- Keep existing lock ordering: never hold the epoch or admission lock across process observation, transport work, or a per-drive claimant probe.
- No daemon, sweeper, new CLI command, config policy, retry layer, or timeout expiry. No suite-attempt or outer-retry budget is consumed or reset by closeout.
- Report-line TOKEN vocabulary is preserved; new information rides additive JSON fields and new bounded reason tokens on the existing `gate-stop <key> gate-unavailable <reason>` channel.
- Comments cross-reference symbols or quoted clauses, never line numbers (ADR-0054 / `TestCommentAnchorStyle`).
- Tests: `go test ./internal/<pkg>/ -run '<Name>' -count=1` per task (`-count=1` defeats the cache — every mutation probe must use it); the finish gate runs the full source-resolved suite via the configured runner (`go run ./cmd/docket development test`) and reads the budget report.

---

### Task 1: Epoch lifecycle states `completing` and `completed`

**Files:**
- Modify: `internal/app/rungate_epoch.go`
- Test: `internal/app/rungate_epoch_test.go`

**Interfaces:**
- Consumes: existing `epochCAS`, `readStoredEpoch`, `epochErr`, `EpochRecord`, `MintEpochRecord`.
- Produces:
  - `const EpochCompleting epochState = "completing"`, `const EpochCompleted epochState = "completed"`
  - `func FenceEpochCompleting(repoDir, gateKey, expectEpoch string) (epochState, error)` — CAS `active`→`completing`; returns the state OBSERVED under the lock. `active`→fenced (returns `EpochCompleting, nil`); already `completing`→idempotent replay (`EpochCompleting, nil`); `completed`→(`EpochCompleted, nil`) (replay of a finished closeout); `cancelling`/`cancelled`/`superseded`/unknown → that state plus a typed `ErrEpochNotActive` error (never relabelled). Non-empty `expectEpoch` mismatching `EpochID` → `ErrEpochMismatch`.
  - `func CompleteEpoch(repoDir, gateKey string) error` — CAS `completing`→`completed`; already `completed`→idempotent nil; ANY other state → `ErrEpochNotActive` (a concurrent cancellation won from `completing`; completion loses).

- [ ] **Step 1: Write the failing tests**

In `internal/app/rungate_epoch_test.go` (follow the file's existing fixture helpers for minting a gate key dir + epoch):

```go
func TestFenceEpochCompletingFromActive(t *testing.T) {
	repo, key := mintEpochFixture(t) // reuse/extract the file's existing mint helper; changeID "441"
	st, err := FenceEpochCompleting(repo, key, "")
	if err != nil || st != EpochCompleting {
		t.Fatalf("fence: state %q err %v", st, err)
	}
	rec, _, _ := LoadEpochRecord(repo, key)
	if rec.State != EpochCompleting {
		t.Fatalf("persisted state %q", rec.State)
	}
	// Idempotent replay resumes the same closeout.
	if st, err = FenceEpochCompleting(repo, key, ""); err != nil || st != EpochCompleting {
		t.Fatalf("replay: state %q err %v", st, err)
	}
}

func TestFenceEpochCompletingNeverRelabelsTerminalStates(t *testing.T) {
	for _, s := range []epochState{EpochCancelling, EpochCancelled, EpochSuperseded, epochState("garbage")} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s) // helper: epochCAS setting rec.State = s
		st, err := FenceEpochCompleting(repo, key, "")
		ee, ok := AsEpochError(err)
		if !ok || ee.Kind != ErrEpochNotActive || st != s {
			t.Fatalf("state %q: got st %q err %v", s, st, err)
		}
		rec, _, _ := LoadEpochRecord(repo, key)
		if rec.State != s {
			t.Fatalf("state %q was rewritten to %q", s, rec.State)
		}
	}
}

func TestFenceEpochCompletingRejectsStaleLocator(t *testing.T) {
	repo, key := mintEpochFixture(t)
	_, err := FenceEpochCompleting(repo, key, "not-the-epoch-id")
	if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochMismatch {
		t.Fatalf("err %v", err)
	}
}

func TestCompleteEpochOnlyFromCompleting(t *testing.T) {
	repo, key := mintEpochFixture(t)
	if err := CompleteEpoch(repo, key); err == nil {
		t.Fatal("completed from active") // never a shortcut past the fence
	}
	_, _ = FenceEpochCompleting(repo, key, "")
	if err := CompleteEpoch(repo, key); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := CompleteEpoch(repo, key); err != nil {
		t.Fatalf("idempotent replay: %v", err) // completed receipt replay is safe
	}
	// Cancellation that won from completing makes completion lose.
	repo2, key2 := mintEpochFixture(t)
	_, _ = FenceEpochCompleting(repo2, key2, "")
	forceEpochState(t, repo2, key2, EpochCancelling)
	if err := CompleteEpoch(repo2, key2); err == nil {
		t.Fatal("completion must lose to a cancellation that won")
	}
}

func TestRegisterEpochParticipantRejectedOnCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s)
		err := RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "task", NativeHandle: "h"})
		if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochNotActive {
			t.Fatalf("state %q admitted a registration: %v", s, err)
		}
	}
}

func TestSupersedeRefusesCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s)
		err := SupersedeCancelledEpoch(repo, key, "replacement-key")
		if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochNotCancelled {
			t.Fatalf("state %q superseded: %v", s, err)
		}
	}
}
```

If the file has no reusable mint helper, add `mintEpochFixture(t)` (temp git-common-dir repo + `MintGateRecord` + `MintEpochRecord`) and `forceEpochState` in the test file, modeled on the file's existing setup.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestFenceEpochCompleting|TestCompleteEpoch|TestRegisterEpochParticipantRejectedOnCompleting|TestSupersedeRefusesCompleting' -count=1`
Expected: FAIL — `EpochCompleting`, `FenceEpochCompleting`, `CompleteEpoch` undefined.

- [ ] **Step 3: Implement**

In `rungate_epoch.go`, beside the existing state constants (document each per the file's style — `completing` is "a keyed verdict verified success and durably fenced the epoch; accounting/retirement unfinished; still the worktree owner", `completed` is "successful closeout finished; terminal; excluded from ambient worktree-owner lookup"):

```go
// EpochCompleting / EpochCompleted: the successful-run closeout lifecycle
// (change 0441). Completing is the durable success fence — RunGateVerdict
// verified run-complete but ownership accounting/retirement is unfinished, so
// the epoch still owns its worktree and admits no NEW registration, start,
// mutation, takeover, or relaunch. Completed means retirement finished:
// terminal, excluded from ambient worktree-owner lookup, revoked for explicit
// references. Success is never encoded as cancellation.
const (
	EpochCompleting epochState = "completing"
	EpochCompleted  epochState = "completed"
)

func FenceEpochCompleting(repoDir, gateKey, expectEpoch string) (epochState, error) {
	var observed epochState
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if expectEpoch != "" && rec.EpochID != expectEpoch {
			return epochErr(ErrEpochMismatch, "fence-completing", nil)
		}
		observed = rec.State
		switch rec.State {
		case EpochActive:
			rec.State = EpochCompleting
			observed = EpochCompleting
			return nil
		case EpochCompleting, EpochCompleted:
			return errEpochFenceNoWrite // idempotent observation, no write
		default:
			return epochErr(ErrEpochNotActive, "fence-completing", nil)
		}
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return observed, nil
	}
	return observed, err
}

func CompleteEpoch(repoDir, gateKey string) error {
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		switch rec.State {
		case EpochCompleting:
			rec.State = EpochCompleted
			return nil
		case EpochCompleted:
			return errEpochFenceNoWrite
		default:
			return epochErr(ErrEpochNotActive, "complete-epoch", nil)
		}
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return nil
	}
	return err
}
```

Add the sentinel beside `errEpochAlreadySuperseded`:

```go
// errEpochFenceNoWrite aborts a completion-lifecycle CAS with no write when the
// observed state already satisfies the transition (idempotent replay).
var errEpochFenceNoWrite = errors.New("run epoch completion state already satisfied")
```

`RegisterEpochParticipant` and `SupersedeCancelledEpoch` need no code change (non-active already rejects; non-cancelled already refuses) — the tests pin that the new states inherit those refusals.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestFenceEpochCompleting|TestCompleteEpoch|TestRegisterEpochParticipantRejectedOnCompleting|TestSupersedeRefusesCompleting' -count=1`
Expected: PASS. Then `go test ./internal/app/ -count=1` for the package.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_epoch.go internal/app/rungate_epoch_test.go
git commit -m "feat(app): add completing/completed run-epoch states (change 0441)"
```

---

### Task 2: Persist native-participant terminal observation

**Files:**
- Modify: `internal/app/rungate_epoch.go` (`EpochParticipant` + new CAS op)
- Test: `internal/app/rungate_epoch_test.go`

**Interfaces:**
- Produces:
  - `EpochParticipant` gains additive fields: `TerminalStatus string` (json `terminal_status,omitempty`; `"completed"` or `"failed"`), `TerminalTurn string` (json `terminal_turn,omitempty`), `TerminalObservedAt string` (json `terminal_observed_at,omitempty`, RFC3339 UTC).
  - `const participantTerminalCompleted = "completed"`, `const participantTerminalFailed = "failed"`.
  - `const ErrEpochParticipantUnknown EpochErrorKind = "epoch-participant-unknown"`.
  - `func RecordEpochParticipantTerminal(repoDir, gateKey, expectEpoch, handle, turn, status string) error` — stamps terminal evidence on the EXACT participant whose `NativeHandle == handle`. Allowed in ANY epoch state (completing an existing participant's record is observation of fact, mirroring the mutation journal's `done` callback, which has no state gate; registering NEW work stays active-only). Idempotent on identical evidence; a DIFFERENT already-recorded status or turn fails closed (`ErrEpochMismatch`); unknown handle → `ErrEpochParticipantUnknown`; empty handle/turn/status or a status outside the two-value set → `ErrEpochMismatch` (malformed evidence is never stored).

- [ ] **Step 1: Write the failing tests**

```go
func TestRecordEpochParticipantTerminal(t *testing.T) {
	repo, key := mintEpochFixture(t)
	must(t, RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "coordinator", NativeHandle: "thread-1"}))
	must(t, RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", participantTerminalCompleted))
	rec, _, _ := LoadEpochRecord(repo, key)
	p := rec.Participants[0]
	if p.TerminalStatus != participantTerminalCompleted || p.TerminalTurn != "turn-9" || p.TerminalObservedAt == "" {
		t.Fatalf("evidence not persisted: %+v", p)
	}
	// Idempotent identical replay; conflicting evidence fails closed.
	must(t, RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", participantTerminalCompleted))
	if err := RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", participantTerminalFailed); err == nil {
		t.Fatal("conflicting terminal status accepted")
	}
	if err := RecordEpochParticipantTerminal(repo, key, "", "thread-1", "other-turn", participantTerminalCompleted); err == nil {
		t.Fatal("mismatched turn accepted") // AC4: mismatched turn cannot satisfy
	}
}

func TestRecordEpochParticipantTerminalUnknownHandleAndBadInput(t *testing.T) {
	repo, key := mintEpochFixture(t)
	err := RecordEpochParticipantTerminal(repo, key, "", "ghost", "t", participantTerminalCompleted)
	if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochParticipantUnknown {
		t.Fatalf("err %v", err)
	}
	for _, bad := range [][3]string{{"", "t", "completed"}, {"h", "", "completed"}, {"h", "t", ""}, {"h", "t", "yielded"}} {
		if RecordEpochParticipantTerminal(repo, key, "", bad[0], bad[1], bad[2]) == nil {
			t.Fatalf("malformed evidence %v accepted", bad)
		}
	}
}

func TestRecordEpochParticipantTerminalAllowedAfterFence(t *testing.T) {
	// "Completion of an existing participant is allowed after the completing
	// fence; registering or reopening work is not."
	for _, s := range []epochState{EpochCompleting, EpochCancelling} {
		repo, key := mintEpochFixture(t)
		must(t, RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "task", NativeHandle: "h1"}))
		forceEpochState(t, repo, key, s)
		must(t, RecordEpochParticipantTerminal(repo, key, "", "h1", "turn-1", participantTerminalFailed))
	}
}
```

(`must` is a tiny `t.Helper()` fatal-on-error helper; add it if the file lacks one.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestRecordEpochParticipantTerminal' -count=1`
Expected: FAIL — symbols undefined.

- [ ] **Step 3: Implement**

Add the three fields to `EpochParticipant` (with a doc comment: "Terminal* persist the adapter's exact terminal observation of this native task — change 0441. Absent evidence means UNPROVEN, never implicitly complete; a terminal failure is termination evidence too (RunVerify independently decides implementation success)."), the two status constants, the new `EpochErrorKind`, and:

```go
func RecordEpochParticipantTerminal(repoDir, gateKey, expectEpoch, handle, turn, status string) error {
	if handle == "" || turn == "" ||
		(status != participantTerminalCompleted && status != participantTerminalFailed) {
		return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
	}
	return epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if expectEpoch != "" && rec.EpochID != expectEpoch {
			return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
		}
		for i := range rec.Participants {
			p := &rec.Participants[i]
			if p.NativeHandle != handle {
				continue
			}
			if p.TerminalStatus != "" {
				if p.TerminalStatus == status && p.TerminalTurn == turn {
					return errEpochFenceNoWrite // idempotent
				}
				return epochErr(ErrEpochMismatch, "record-participant-terminal", nil)
			}
			p.TerminalStatus = status
			p.TerminalTurn = turn
			p.TerminalObservedAt = time.Now().UTC().Format(time.RFC3339)
			return nil
		}
		return epochErr(ErrEpochParticipantUnknown, "record-participant-terminal", nil)
	})
}
```

Map `errEpochFenceNoWrite` to nil at the return, as Task 1 does.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestRecordEpochParticipantTerminal' -count=1` then `go test ./internal/app/ -count=1`.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_epoch.go internal/app/rungate_epoch_test.go
git commit -m "feat(app): persist native-participant terminal observation (change 0441)"
```

---

### Task 3: Fence the new states at every epoch consumer; exclude completed from ambient lookup

**Files:**
- Modify: `internal/app/rungate_fence.go` (`admitWorkflowMutation`, `findEpochByWorktree`, new sentinel)
- Modify: `internal/app/rungate_gate.go` (`epochLaunchGate` state switch)
- Modify: `internal/app/rungate_epoch.go` (`epochRevokedResolver`)
- Test: `internal/app/rungate_fence_test.go`, `internal/app/rungate_epoch_test.go` (or the file holding `epochLaunchGate` tests — locate with `grep -rln epochLaunchGate internal/app/*_test.go`)

**Interfaces:**
- Produces: `var ErrRunCompleted = &MutationFenceError{Reason: "run-completed"}` in `rungate_fence.go`, beside `ErrRunCancelled`/`ErrStaleRunEpoch`.
- Behavior:
  - `admitWorkflowMutation` state switch: `EpochCompleting, EpochCompleted` → `ErrRunCompleted` (defense in depth for completed — see next line).
  - `findEpochByWorktree` SKIPS `EpochCompleted` records (spec: "Exclude fully completed epochs from ambient worktree-owner lookup"). `EpochCompleting` still matches (still an owner).
  - `epochLaunchGate` switch: `EpochCompleting, EpochCompleted` → `ErrRunCompleted` (no launch, delayed ticket, or relaunch admits — its refusal is what later settles a pre-fence never-launched ticket terminal).
  - `epochRevokedResolver` reports revoked for `EpochCompleted` and `EpochCompleting` too (takeover of a completing/completed run refuses; "Explicit references to completed epochs remain revoked").

- [ ] **Step 1: Audit every epoch-state consumer by repo-wide grep (never a remembered list)**

Run and record the output in the task's commit message body:

```bash
grep -rn -e 'EpochActive' -e 'EpochCancelling' -e 'EpochCancelled' -e 'EpochSuperseded' \
  --include='*.go' internal/ | grep -v _test
grep -rn 'AsMutationFenceError\|ErrRunCancelled\|ErrStaleRunEpoch' --include='*.go' internal/ | grep -v _test
```

Expected consumer set (verify against the grep, do not trust this list): `rungate_fence.go` (mutation admission + worktree lookup), `rungate_gate.go` (launch gate), `rungate_epoch.go` (revocation resolver, `FindEpochByChange` state preference), `rungate_before.go` (resume — Task 6), `rungate_cancel.go` (cancel — Task 5), `agent_guardian.go` (death guardian — pinned in Task 5). Any consumer the grep finds beyond these gets an explicit decision in this task. Also enumerate every `AsMutationFenceError` / fence-reason consumer and confirm each handles the new `run-completed` reason sanely (most treat any `MutationFenceError` uniformly; note any that key on the two old sentinels by identity).

- [ ] **Step 2: Write the failing tests**

In `rungate_fence_test.go` (reuse its existing fixtures that mint an epoch bound to a worktree):

```go
func TestAdmitWorkflowMutationRefusesCompletingEpoch(t *testing.T) {
	repo, key, worktree := mintWorktreeBoundEpochFixture(t) // reuse the file's existing helper shape
	forceEpochState(t, repo, key, EpochCompleting)
	_, err := admitWorkflowMutation(worktree, "test.op")
	fe, ok := AsMutationFenceError(err)
	if !ok || fe.Reason != "run-completed" {
		t.Fatalf("err %v", err)
	}
}

func TestCompletedEpochExcludedFromAmbientOwnerLookup(t *testing.T) {
	repo, key, worktree := mintWorktreeBoundEpochFixture(t)
	forceEpochState(t, repo, key, EpochCompleted)
	done, err := admitWorkflowMutation(worktree, "test.op")
	if err != nil || done == nil {
		t.Fatalf("completed epoch trapped a standalone mutation: %v", err)
	}
	// findEpochByWorktree itself no longer names the completed epoch.
	canon, _ := canonicalWorktree(worktree)
	if _, found, _ := findEpochByWorktree(worktree, canon); found {
		t.Fatal("completed epoch still owns the worktree lookup")
	}
	// A COMPLETING epoch, in contrast, is still the owner.
	forceEpochState(t, repo, key, EpochCompleting)
	if _, found, _ := findEpochByWorktree(worktree, canon); !found {
		t.Fatal("completing epoch lost worktree ownership before closeout finished")
	}
}
```

Launch-gate + revocation tests beside the existing `epochLaunchGate` / `epochRevokedResolver` tests, same fixture shape:

```go
func TestEpochLaunchGateRefusesCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		// arrange epoch in state s bound to worktree; call the gate func
		// exactly as the neighboring EpochCancelling test does; expect the
		// reserve callback NOT invoked and the returned error to be
		// ErrRunCompleted.
	}
}

func TestEpochRevokedResolverRevokesCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		// arrange; expect revoked=true, mirroring the cancelled/superseded cases.
	}
}
```

(Write these as full tests copied from the shape of the adjacent cancelled-state tests in the same files — same helpers, new state, new expected sentinel.)

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestAdmitWorkflowMutationRefusesCompleting|TestCompletedEpochExcluded|TestEpochLaunchGateRefusesCompleting|TestEpochRevokedResolverRevokes' -count=1`
Expected: FAIL (completing currently falls into `default:` → `ErrRunCancelled`; completed is currently still found by lookup; resolver reports not revoked).

- [ ] **Step 4: Implement**

`rungate_fence.go`:

```go
// ErrRunCompleted: the owning run epoch finished (or is finishing) a SUCCESSFUL
// closeout (change 0441) — no new mutation, launch, or registration admits, and
// the refusal is distinguishable from cancellation.
var ErrRunCompleted = &MutationFenceError{Reason: "run-completed"}
```

In `admitWorkflowMutation`'s switch add, before the default:

```go
case EpochCompleting, EpochCompleted:
	return nil, ErrRunCompleted
```

In `findEpochByWorktree`'s loop, after the `r.Worktree == ""` skip:

```go
if r.State == EpochCompleted {
	continue // a fully completed epoch no longer owns any worktree (change 0441)
}
```

`rungate_gate.go` `epochLaunchGate` switch, before the default:

```go
case EpochCompleting, EpochCompleted:
	return ErrRunCompleted
```

`rungate_epoch.go` `epochRevokedResolver` return:

```go
return rec.State == EpochCancelled || rec.State == EpochSuperseded ||
	rec.State == EpochCompleting || rec.State == EpochCompleted, nil
```

- [ ] **Step 5: Run tests to verify they pass; run the package**

Run: `go test ./internal/app/ -count=1` and `go test ./internal/repoguard/ -count=1` (the gatelaunch-admission correspondence guard must stay green).
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_fence.go internal/app/rungate_gate.go internal/app/rungate_epoch.go internal/app/*_test.go
git commit -m "feat(app): fence completing/completed epochs at every consumer; exclude completed from ambient owner lookup (change 0441)"
```

---

### Task 4: Observation-only launch accounting in gatedrive

**Files:**
- Modify: `internal/gatedrive/reconcile.go`
- Test: `internal/gatedrive/reconcile_test.go`

**Interfaces:**
- Consumes: existing `Driver`, `resolveDriveEpoch`, `tryRelaunchClaim`, `isTerminalOutcome`, `stopProvesTeardown`, `ProcessSeam.Observe`.
- Produces: `func (d *Driver) ObserveEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error)` — identical walk, linkage, and claim-probe semantics to `ReconcileEpochLaunches`, but it NEVER stops a process, never settles a never-launched reservation terminal, and never mutates any record. Shared implementation, one inventory (spec: "provide observation-only behavior for success rather than copying a second inventory").

Per-drive observe-mode dispositions:
- terminal drive → settled (as today).
- busy claimant flock → pending, `claim-busy:<id>`.
- never-launched / no relaunch reserved → pending, `launch-pending:<id>` (the completing launch-gate refusal is the "established fenced settlement" that later settles it; a replay then accounts — never settle it here, and never invent timeout expiry).
- reserved relaunch → `ResolveReservation`; disposition `never-launched` → pending `launch-pending:<id>` (observe mode must not foreclose it terminal); `identified` → observe that run; else pending `resolution-unresolved:<id>`.
- identified/attached run → `d.proc.Observe(runDir)`; `stopProvesTeardown(obs.State)` true → settled with informational `run-terminal:<id>`; observation error or live/signalled state → pending `run-live:<id>` (or `resolution-unresolved:<id>` on error).

- [ ] **Step 1: Write the failing tests**

In `reconcile_test.go`, reuse the file's existing fake `ProcessSeam` and drive-record fixtures (read the neighboring `TestReconcileEpochLaunches*` tests first and mirror their arrangement):

```go
func TestObserveEpochLaunchesNeverStopsOrSettles(t *testing.T) {
	// Arrange one epoch-linked NONTERMINAL drive with an attached RawRunDir
	// whose fake proc reports a RUNNING state, exactly as the neighboring
	// reconcile test arranges its stop case.
	// Act: ObserveEpochLaunches.
	// Assert: Accounted == false; findings contain "run-live:<id>";
	// the fake proc records ZERO Stop calls; the drive record on disk is
	// byte-identical (re-Load and compare LastOutcome/RelaunchReserved).
}

func TestObserveEpochLaunchesAccountsProvenTerminalRun(t *testing.T) {
	// Same arrangement, fake proc Observe returns StateStopped.
	// Assert: Accounted == true, finding "run-terminal:<id>", zero Stop calls.
}

func TestObserveEpochLaunchesKeepsNeverLaunchedPending(t *testing.T) {
	// Arrange a reserved-never-launched drive (RelaunchReserved with a
	// resolution of "never-launched", and separately a bare reservation with
	// RawRunDir == "" / !RelaunchReserved).
	// Assert both: Accounted == false, finding "launch-pending:<id>", and the
	// record is NOT settled terminal (LastOutcome unchanged) — the observe
	// path never runs settleNeverLaunchedCancelled.
}

func TestObserveEpochLaunchesBusyClaimIsPending(t *testing.T) {
	// Hold the drive's relaunch claim flock (as the reconcile busy test does);
	// assert Accounted == false with "claim-busy:<id>".
}

func TestReconcileEpochLaunchesBehaviorUnchanged(t *testing.T) {
	// Regression pin: after the factoring, the reconcile (cancel) mode still
	// stops an identified run and still settles a proven never-launched
	// reservation terminal HALTED "run-cancelled" — re-run the two key
	// scenarios through ReconcileEpochLaunches and assert the OLD outcomes.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gatedrive/ -run 'TestObserveEpochLaunches|TestReconcileEpochLaunchesBehaviorUnchanged' -count=1`
Expected: FAIL — `ObserveEpochLaunches` undefined (the regression pin should pass immediately; keep it anyway).

- [ ] **Step 3: Implement by factoring, not copying**

Rename the body of `ReconcileEpochLaunches` to `accountEpochLaunches(worktreeRoot, epochID string, observeOnly bool)`; both exported methods delegate:

```go
func (d *Driver) ReconcileEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error) {
	return d.accountEpochLaunches(worktreeRoot, epochID, false)
}

// ObserveEpochLaunches is the SUCCESS-closeout view of one epoch's launch
// obligations (change 0441): the same walk, epoch linkage, and claimant probe
// as ReconcileEpochLaunches, but observation-only — it never stops a process,
// never settles a reservation terminal, and never mutates a record. A pending
// never-launched ticket stays pending until the completing launch-gate refusal
// settles it terminal; a replay then accounts.
func (d *Driver) ObserveEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error) {
	return d.accountEpochLaunches(worktreeRoot, epochID, true)
}
```

Thread `observeOnly` into `reconcileEpochDrive(id, rec, observeOnly)`:
- in the never-launched branches (`RawRunDir == "" && !RelaunchReserved`, and `ResolveReservation` → `never-launched`): when `observeOnly`, return `false, "launch-pending:"+id` (never call `settleNeverLaunchedCancelled`).
- in the identified-run branches: when `observeOnly`, call a new `observeIdentifiedRun`:

```go
func (d *Driver) observeIdentifiedRun(id, runDir string) (bool, string) {
	if runDir == "" {
		return false, "resolution-unresolved:" + id
	}
	obs, err := d.proc.Observe(runDir)
	if err != nil || obs == nil {
		return false, "resolution-unresolved:" + id
	}
	if stopProvesTeardown(obs.State) {
		return true, "run-terminal:" + id
	}
	return false, "run-live:" + id
}
```

(Check the field name on `process.Observation` — `Observe`'s snapshot state — and use exactly what `internal/process/observe.go` exports.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gatedrive/ -count=1`
Expected: PASS, including every pre-existing reconcile/cancellation test.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/reconcile.go internal/gatedrive/reconcile_test.go
git commit -m "feat(gatedrive): observation-only epoch launch accounting for successful closeout (change 0441)"
```

---

### Task 5: Cancellation interplay — cancel wins from completing, refuses completed; guardian leaves completing fenced

**Files:**
- Modify: `internal/app/rungate_cancel.go` (`runCancel` state switch)
- Test: `internal/app/rungate_cancel_test.go`, `internal/app/agent_guardian_test.go`

**Interfaces:**
- Behavior:
  - `runCancel` on `EpochCompleting`: explicit human cancellation WINS — fence `completing`→`cancelling` and run the existing teardown/accounting flow unchanged (its authority conjunction was already validated above the switch).
  - `runCancel` on `EpochCompleted`: refusal with the completed-run explanation — `cancelRefused("run-completed")` — never a state regression, never `cancellation-pending` over durable terminal state.
  - Death guardian: NO code change — pin by test that an owner death over a `completing` epoch leaves it `completing` (the guardian fences only `active` and then declines to reap a non-cancelling epoch), so a keyed-verdict replay resumes the closeout.

- [ ] **Step 1: Write the failing tests**

In `rungate_cancel_test.go`, reusing the file's existing seam fakes and authority fixtures (read a passing `runCancel` test first and copy its arrangement):

```go
func TestRunCancelWinsFromCompletingEpoch(t *testing.T) {
	// Arrange a fully authorized cancel fixture (record + confirmed binding +
	// epoch) with the epoch forced to EpochCompleting and permissive seams.
	// Act: runCancel.
	// Assert: disposition is cancelled (or pending per the seam shape used),
	// and the persisted epoch state is EpochCancelling or EpochCancelled —
	// never completing, never completed.
}

func TestRunCancelRefusesCompletedEpoch(t *testing.T) {
	// Same fixture, epoch forced to EpochCompleted.
	// Assert: disposition refused; findings contain "run-completed"; the
	// persisted epoch state is STILL EpochCompleted (no regression).
}

func TestGuardianLeavesCompletingEpochForReplay(t *testing.T) { // in agent_guardian_test.go
	// Arrange the guardian fence path (mirror the existing guardian test
	// fixture) over an epoch in EpochCompleting.
	// Assert: after the guardian's fence-and-reap runs, the epoch state is
	// still EpochCompleting and no teardown was performed.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestRunCancelWinsFromCompleting|TestRunCancelRefusesCompleted|TestGuardianLeavesCompletingEpoch' -count=1`
Expected: `TestRunCancelWinsFromCompleting` and `TestRunCancelRefusesCompleted` FAIL (both currently hit `default:` → `cancelRefused("epoch-state-unknown")`). The guardian pin should pass immediately; verify it reddens if you temporarily make the guardian CAS also flip `EpochCompleting` (mutation-check it now, then restore).

- [ ] **Step 3: Implement**

In `runCancel`'s state switch:

```go
case EpochActive, EpochCompleting:
	// An explicit human cancellation wins even from a completing (successful,
	// mid-closeout) epoch — completion then loses without reporting success
	// (its completing→completed CAS refuses once this fence lands).
	if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
		if r.State == EpochActive || r.State == EpochCompleting {
			r.State = EpochCancelling
		}
		return nil
	}); ferr != nil {
		return cancelRefused("fence-failed")
	}
case EpochCompleted:
	// A completed run cannot be cancelled: no-op refusal with the completed-run
	// explanation, never a state regression.
	return cancelRefused("run-completed")
```

(Replace the existing lone `case EpochActive:` arm; everything else in the flow is unchanged.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestRunCancel|TestGuardian' -count=1` then the package.
Expected: PASS, including all pre-existing cancel tests.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_cancel.go internal/app/rungate_cancel_test.go internal/app/agent_guardian_test.go
git commit -m "feat(app): cancellation wins from completing, refuses completed (change 0441)"
```

---

### Task 6: Resume interplay — completing/completed epochs are never cancelled predecessors

**Files:**
- Modify: `internal/app/rungate_before.go` (resume state switch + reason constants)
- Test: `internal/app/rungate_before_resume_test.go`

**Interfaces:**
- Produces (beside the existing `ReasonGateResume*` constants — match their exact naming/prose style):
  - `ReasonGateResumeRunCompleting` = `"resume-run-completing"`
  - `ReasonGateResumeRunCompleted` = `"resume-run-completed"`
- Behavior in the resume epoch-state switch (spec: "Resume cannot turn completing/completed into a cancelled predecessor or reserve a replacement"):
  - `EpochCompleting` → `gateUnarmedMsg(ReasonGateResumeRunCompleting, "change <id> completed its run and is closing out (epoch <epoch-id>); re-run the keyed 'docket run gate-verdict' to finish closeout, or cancel explicitly with 'docket run cancel'")`.
  - `EpochCompleted` → `gateUnarmedMsg(ReasonGateResumeRunCompleted, "change <id>'s run completed successfully (epoch <epoch-id>); there is nothing to resume — verify with 'docket run verify --id <id>' and finalize instead")`. Never quiescence-checked into a supersede, never a replacement reservation.

- [ ] **Step 1: Write the failing tests**

In `rungate_before_resume_test.go`, mirror the existing `EpochActive`-refusal resume test's fixture:

```go
func TestResumeRefusesCompletingEpochWithoutSuperseding(t *testing.T) {
	// Arrange a resumable in-progress change whose epoch is EpochCompleting.
	// Act: the gate-before resume path.
	// Assert: gate-unarmed with reason "resume-run-completing"; the epoch is
	// STILL EpochCompleting; ReplacementReserved is empty; no new gate key dir.
}

func TestResumeRefusesCompletedEpochWithoutSuperseding(t *testing.T) {
	// Same with EpochCompleted; reason "resume-run-completed"; state and
	// ReplacementReserved untouched.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestResumeRefusesCompleting|TestResumeRefusesCompleted' -count=1`
Expected: FAIL — the new states currently fall past the switch (verify what the current `default`/fallthrough does and pin the new behavior instead).

- [ ] **Step 3: Implement** the two switch arms and constants exactly as specified in Interfaces.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestResume' -count=1` then the package.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_before.go internal/app/rungate_before_resume_test.go
git commit -m "feat(app): resume refuses completing/completed epochs without superseding (change 0441)"
```

---

### Task 7: The success-closeout engine

**Files:**
- Create: `internal/app/rungate_complete.go`
- Modify: `internal/app/rungate_cancel.go` (extend `cancelSeams` + `productionCancelSeams` with the two observation seams)
- Test: `internal/app/rungate_complete_test.go`

**Interfaces:**
- Consumes: Task 1 `FenceEpochCompleting`/`CompleteEpoch`; Task 2 terminal-evidence fields; Task 4 `Driver.ObserveEpochLaunches`; existing `cancelSeams`, `classifySlotOwnership`, `retireWorktreeSlotOwnership`, `isNativeParticipant`, `isExecutionParticipant`, `mutationStatusCompleted`, `rawTeardownProven`, `gateService`.
- Produces:
  - `cancelSeams` gains two fields (production-wired in `productionCancelSeams`, nil fails closed):
    - `observer processObserver` — `type processObserver interface { observeProcessTerminal(runDir string) (bool, error) }`; production `appGateObserver{}` = `gateService()` + `svc.Observe(runDir)` + `rawTeardownProven(obs.State)`.
    - `launchObserver epochLaunchObserver` — `type epochLaunchObserver interface { observe(worktree, epochID string) (gatedrive.EpochLaunchReport, error) }`; production `appLaunchObserver{store}` = `gatedrive.NewSystemDriver(store, svc).ObserveEpochLaunches(...)` (resolve the service per call, exactly as `appLaunchReconciler.reconcile` does).
  - `func completeSuccessfulRun(seams cancelSeams, repoDir, gateKey string) (ok bool, reason string, findings []string)` — the whole closeout; `reason` is a bounded token for the `gate-unavailable` channel when `ok` is false: one of `"run-cancelled"`, `"stale-run-epoch"`, `"completion-unaccounted"`, `"completion-unpersisted"`, `"epoch-unreadable"`.

Flow (each numbered step maps to the spec's "Successful completion flow"; the caller — Task 8 — has already resolved the confirmed claim binding and the `run-complete` verdict):

1. `FenceEpochCompleting(repoDir, gateKey, "")`. Observed `EpochCompleted` → `(true, "", nil)` immediately (idempotent completed-receipt replay, safe after scratch cleanup). Observed cancelling/cancelled → `(false, "run-cancelled", nil)`; superseded → `(false, "stale-run-epoch", nil)`; store fault → `(false, "epoch-unreadable", nil)`. Never relabel.
2. Reload the fenced record (`LoadEpochRecord`). All remaining proof runs OUTSIDE the epoch lock.
3. Accounting pass over the reloaded record, purely observational, accumulating bounded findings (any finding ⇒ blocked):
   - every native participant (`isNativeParticipant`) must carry `TerminalStatus != ""` — else `participant-unobserved:<kind>` (terminal `failed` still counts as observed; missing-adapter evidence stays unresolved);
   - every execution participant (`isExecutionParticipant`): `seams.observer.observeProcessTerminal(p.NativeHandle)` must prove terminal — nil observer → `process-observer-unavailable`; error → `process-unobserved:<handle>`; live → `process-live:<handle>`;
   - the worktree slot (skip when `seams.store == nil || ep.Worktree == ""`): load it; `ErrNotFound` → acceptable ONLY because this whole pass is the independent old-run accounting; `slotOwned` requires `State == "released"` (else `slot-not-released`) and, when `RawRunDir != ""`, a proven-terminal observation of it; `slotForeign` → left untouched, accounted (`slot-replaced-by-successor` informational); `slotUnowned`/`slotLinkedLegacy` → follow the classifier: legacy-linked runs are already covered by the participant pass, unowned adds `slot-ownership-unresolved` and blocks (success cannot prove a slot it does not own);
   - launches: `seams.launchObserver.observe(ep.Worktree, ep.EpochID)` — nil/erroring observer → `launch-observer-unavailable`/`launch-observe-failed`, blocked; `!report.Accounted` → blocked, findings appended;
   - mutations: every `AdmittedMutation.Status == mutationStatusCompleted` — else `mutation-pending:<op>`.
4. RE-ENUMERATE before retirement: reload the epoch and repeat the participant + mutation checks (the completing fence rejects new registration, but an operation admitted pre-fence may have appended between the fence and step 3's read; the reload also catches a cancellation that won meanwhile — a state no longer `completing` returns `(false, "run-cancelled", findings)`).
5. Blocked ⇒ return `(false, "completion-unaccounted", findings)` — the epoch stays durably `completing`; the remedy is repeating the same keyed verdict once the named evidence settles. No retry, budget, or attempt is touched.
6. Retire: `retireWorktreeSlotOwnership(seams, ep)` (reused verbatim from cancellation — ownership-checked, expected-token/expected-epoch, successor-safe, idempotent on absent/detached). Not retired ⇒ `(false, "completion-unaccounted", findings+finding)`.
7. `CompleteEpoch(repoDir, gateKey)` — failure (cancellation won, or IO) ⇒ `(false, "run-cancelled", findings)` when the reloaded state is cancelling/cancelled/superseded, else `(false, "completion-unpersisted", findings)`. Success ⇒ `(true, "", findings)`.

- [ ] **Step 1: Write the failing tests**

`internal/app/rungate_complete_test.go`. Build one fixture helper `completionFixture(t)` returning repo, key, the epoch bound to a temp worktree with a released epoch-owned slot (reuse the store fixtures from `rungate_cancel_test.go`), permissive fake seams (`observer` proving terminal, `launchObserver` returning `Accounted: true`, `retire` delegating to the real store), a registered coordinator participant with terminal evidence recorded, and every mutation completed. Then:

```go
func TestCompleteSuccessfulRunHappyPath(t *testing.T) {
	// ok == true; epoch state EpochCompleted; slot RunEpochID == "" with
	// State still "released" and history fields intact (assert DriveID/
	// RawRunDir/ExecutionGen etc. unchanged — AC2 history preservation).
}

func TestCompleteSuccessfulRunIdempotentReplay(t *testing.T) {
	// Run twice; second call ok == true with no writes (compare the stored
	// epoch generation before/after the replay).
}

func TestCompleteSuccessfulRunNeverRelabelsCancellation(t *testing.T) {
	// For each of cancelling/cancelled/superseded: ok == false, reason
	// run-cancelled / stale-run-epoch; state unchanged.
}

func TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation(t *testing.T) {
	// Table-driven, one mutant fixture per row (AC3):
	//  - native participant without TerminalStatus       -> participant-unobserved
	//  - observer reporting a live execution participant -> process-live
	//  - nil observer                                     -> process-observer-unavailable
	//  - launchObserver !Accounted (busy claim / delayed ticket / pending
	//    relaunch findings passed through)                -> blocked
	//  - nil launchObserver                               -> launch-observer-unavailable
	//  - mutation admitted / uncertain                    -> mutation-pending
	//  - owned slot not released                          -> slot-not-released
	//  - unowned slot                                     -> slot-ownership-unresolved
	// Each row: ok == false, reason completion-unaccounted, the named finding
	// present, epoch still EpochCompleting, slot RunEpochID untouched.
}

func TestCompleteSuccessfulRunSendsNoStops(t *testing.T) {
	// Wire counting fakes for stopper/native/launches(reconcile): ZERO calls
	// to stopProcess, cancelNativeTask, or the reconcile (stop-capable) seam
	// on both the happy path and a blocked path (AC3: "sends no cancellation
	// or stop signals").
}

func TestCompleteSuccessfulRunLateParticipantBlocks(t *testing.T) {
	// Register an extra execution participant between the fence and the
	// accounting pass (simulate by appending via epochCAS after fencing but
	// before calling the account step — or simply include an extra
	// terminal-unproven participant): re-enumeration blocks it (AC3 "late
	// participant").
}

func TestCompleteSuccessfulRunReplayAfterRetireBeforeComplete(t *testing.T) {
	// Inject retire success + CompleteEpoch fault (force by flipping state to
	// cancelling? No — use a seams.retire that succeeds, then simulate the
	// interruption by calling completeSuccessfulRun with a fixture whose slot
	// is ALREADY detached and epoch still completing): ok == true — replay
	// accepts safe prior detachment and finishes (AC6 "interruption
	// before/after slot retirement").
}

func TestCompleteSuccessfulRunForeignSuccessorUntouched(t *testing.T) {
	// Slot carries a DIFFERENT nonempty RunEpochID: closeout completes,
	// successor slot untouched (AC6 "a successor reserving after safe
	// detachment"; AC5 successor protection).
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCompleteSuccessfulRun' -count=1`
Expected: FAIL — `completeSuccessfulRun` undefined.

- [ ] **Step 3: Implement** `rungate_complete.go` per the Interfaces/Flow block above. File header comment: the spec's "Successful completion flow" contract — only the attributed keyed verdict drives it; observation-only; fail closed on missing evidence; never a stop signal; lock ordering (fence and complete under `epochCAS`; ALL probes outside any lock). Extend `cancelSeams`/`productionCancelSeams` in `rungate_cancel.go` with `observer`/`launchObserver` (documented: "shared seam bundle, two flows — cancellation stops, completion only observes"). Add compile-time assertions beside the existing ones.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-test the completion preconditions (AC8, first half)**

For each guard, temporarily strip it and confirm the named test reddens (restore each via `git diff`/backup copy, never `git checkout --` over uncommitted work; always `-count=1`):
- delete the native-participant `TerminalStatus` check → `TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation` reddens;
- make `completeSuccessfulRun` skip re-enumeration → late-participant test reddens;
- make the slot pass accept a non-released owned slot → slot-not-released row reddens;
- make `CompleteEpoch` succeed from any state → `TestCompleteSuccessfulRunNeverRelabelsCancellation` (via Task 1's test) reddens.
Record the four probes in the commit message body.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_complete.go internal/app/rungate_complete_test.go internal/app/rungate_cancel.go
git commit -m "feat(app): successful-run ownership closeout engine (change 0441)"
```

---

### Task 8: Wire closeout into the keyed RunGateVerdict

**Files:**
- Modify: `internal/app/rungate_verdict.go`
- Modify: whichever deps struct carries the resume path's injectable `CancelSeams` factory (locate with `grep -rn 'CancelSeams' internal/app/ | grep -v _test`) so `RunGateVerdict` can reach the same injection; production defaults to `productionCancelSeams(repoDir)`.
- Test: the existing verdict test file (locate with `grep -rln 'RunGateVerdict(' internal/app/*_test.go`).

**Interfaces:**
- `RunGateVerdictResult` gains additive `CompletionFindings []string` (json `completion_findings,omitempty`) — diagnostics only; report-line tokens unchanged (`HumanText` untouched except that `gate-stop <key> gate-unavailable <reason>` now also occurs with the new reason tokens).
- New bounded reason constants beside `ReasonGate*`: `ReasonGateRunCancelled = "run-cancelled"`, `ReasonGateStaleRunEpoch = "stale-run-epoch"`, `ReasonGateCompletionUnaccounted = "completion-unaccounted"`, `ReasonGateCompletionUnpersisted = "completion-unpersisted"`, `ReasonGateReportUnpersisted = "report-unpersisted"` (reuse spellings already emitted by Task 7).
- `case VerdictRunComplete:` becomes:

```go
case VerdictRunComplete:
	return gateCompleteRun(repoDir, key, rec, id, seamsFor(repoDir))
```

`gateCompleteRun` contract:
1. `LoadEpochRecord(repoDir, key)`; `ErrEpochNotFound` → the keyless/standalone/legacy shape: EXACTLY today's behavior (best-effort `persistGateVerdict` + `gate-done run-complete`). Any other load fault → `gate-stop gate-unavailable epoch-unreadable` (fail closed; a record the store cannot read is never a free closeout).
2. Epoch found → `ok, reason, findings := completeSuccessfulRun(seams, repoDir, key)`.
3. `!ok` → `gate-stop <key> gate-unavailable <reason>` with `CompletionFindings = findings`, Terminal true, persisted best-effort — completion must lose without reporting success; the remedy (named in the result's findings) is to settle the evidence and repeat the same keyed verdict, or cancel explicitly. No retry is consumed (the fail-closed stop leaves the permit untouched, as `gateStopUnavailable` already guarantees).
4. `ok` → durably save the report FIRST as a checked write: `rec.Disposition/rec.Terminal` set as `persistGateVerdict` does, but `SaveGateRecord`'s error is CHECKED — on failure return `gate-stop gate-unavailable report-unpersisted` (spec: "Completion-path persistence failures must be reported, not hidden by the existing best-effort report save"; the epoch is already durably `completed`, so the replay path is `TestCompleteSuccessfulRunIdempotentReplay`'s). On success return `gate-done run-complete <id>`.
- Historical repair falls out for free (AC7): a historical `active` epoch whose change verifies `run-complete` passes through this exact flow on a keyed re-verdict; missing evidence produces `completion-unaccounted` findings plus the explicit-cancellation remedy. Unattributed observe mode is untouched (structurally cannot reach any of this).

- [ ] **Step 1: Write the failing tests**

In the verdict test file, mirroring its existing fixture style (fake `PlanningDeps`/`WorkspaceDeps` driving `RunVerify` to `run-complete` — copy the arrangement of the existing `VerdictRunComplete` test):

```go
func TestVerdictRunCompleteClosesOutEpochOwnership(t *testing.T) {
	// Epoch-backed fixture (Task 7's completionFixture wired under the key the
	// verdict loads): verdict returns gate-done run-complete; the epoch is
	// EpochCompleted; the slot's RunEpochID is cleared.
}

func TestVerdictRunCompleteWithoutEpochUnchanged(t *testing.T) {
	// No epoch.json beside the record: gate-done run-complete exactly as
	// before, and no rungate write beyond the existing record mirror.
}

func TestVerdictRunCompleteBlockedCloseoutStopsWithoutSuccess(t *testing.T) {
	// Seams with one live obligation: decision gate-stop, outcome
	// gate-unavailable, reason completion-unaccounted, CompletionFindings
	// non-empty, Terminal true; epoch left EpochCompleting; NO retry marker
	// consumed (GateRetryUsage unchanged) — AC2 budget preservation.
}

func TestVerdictRunCompleteCancelledEpochNeverReportsSuccess(t *testing.T) {
	// Epoch forced EpochCancelled + RunVerify run-complete: gate-stop
	// gate-unavailable run-cancelled; never gate-done.
}

func TestVerdictRunCompleteReportPersistFailureIsReported(t *testing.T) {
	// Inject a SaveGateRecord failure (read-only record dir after closeout, or
	// the file's existing store-fault fixture): gate-stop gate-unavailable
	// report-unpersisted; a SECOND verdict (fault removed) replays to
	// gate-done run-complete (AC6 gate-report write failure + replay).
}

func TestVerdictObserveModeNeverTouchesOwnership(t *testing.T) {
	// RunGateVerdictObserve over the same epoch-backed complete fixture: the
	// epoch state and slot are byte-identical afterwards (AC5).
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestVerdictRunComplete|TestVerdictObserveModeNeverTouches' -count=1`
Expected: new tests FAIL (today the verdict returns gate-done and leaves the epoch active).

- [ ] **Step 3: Implement** per the contract above. Update the file-header contract comment (the "It NEVER re-derives a run-* verdict" block) with a short COMPLETION paragraph citing change 0441: the verdict remains the sole authority mapper; on run-complete it additionally drives the ownership closeout, and a blocked closeout maps to `gate-stop gate-unavailable` — RunVerify's verdict is still reported as fact via the reason tokens, never re-derived.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS, including every pre-existing verdict/continuation/retry test (the run-complete arm is the only one touched — diff-check that the incomplete/waiting/halted arms are byte-identical).

- [ ] **Step 5: Commit**

```bash
git add internal/app/rungate_verdict.go internal/app/*_test.go internal/app/rungate_cancel.go
git commit -m "feat(app): keyed run-complete verdict drives successful ownership closeout (change 0441)"
```

---

### Task 9: Adapter terminal observation — codexentry records the exact turn's termination

**Files:**
- Modify: `internal/codexentry/client.go`
- Modify: `internal/cli/agent.go`
- Test: `internal/codexentry/client_test.go` (or the package's existing test file — locate with `ls internal/codexentry/*_test.go`), `internal/cli` if it has agent tests.

**Interfaces:**
- Produces in `codexentry`:
  - `type TerminalRecorder interface { RecordTerminal(handle, turnID, status string) error }` — beside `ParticipantRegistrar`; `Client` gains `Terminal TerminalRecorder` (nil records nothing — the honest no-lifecycle state).
  - `waitTurn` distinguishes TURN-TERMINAL outcomes from transport outcomes: introduce `type turnFailedError struct{ status, detail string }` (implements `error`) returned where today's `fmt.Errorf("coordinator turn %s", detail)` is built from a `turn/completed` frame with non-completed status. EOF, malformed frames, interactive-request rejection, and "completed without a final agent message" remain plain errors — NOT termination evidence (AC4: transport loss, yielded text, marker-only completion cannot satisfy).
  - `Client.Enter` records terminal evidence AFTER the invocation's terminal response and transport teardown are accounted: locate the transport's close in `Enter` (its `Connect`/`defer Close` shape) and perform the record after the transport is explicitly closed — restructure to close the transport before recording if the current shape is a bare defer. Record `RecordTerminal(threadID, turnID, participantTerminalCompleted)` on waitTurn success; `RecordTerminal(threadID, turnID, participantTerminalFailed)` when the error is a `turnFailedError` (then still return the run error). A RECORD failure on the success path must not fake evidence and must not flip the successful run to failure: `Result` gains `TerminalRecordFailed bool`; the CLI surfaces it.
- Produces in `internal/cli/agent.go`: `epochTerminalRecorder{repoDir, gateKey, epochID string}` implementing `RecordTerminal` via `app.RecordEpochParticipantTerminal(repoDir, gateKey, epochID, handle, turnID, status)`; wired beside `client.Registrar` under the same `runGateKey != "" && runEpoch != ""` condition; on `out.TerminalRecordFailed` append a bounded warning to the `AgentEnterResult` message (`"terminal evidence unrecorded; closeout will block until resolved"`).
- Requires exporting the status constants for the CLI boundary: move/alias `participantTerminalCompleted/Failed` to exported `app.ParticipantTerminalCompleted` / `app.ParticipantTerminalFailed` (adjust Task 2's spellings accordingly if this task lands after — keep ONE set of exported constants; `codexentry` passes literal `"completed"`/`"failed"` matching them and the recorder validates).

- [ ] **Step 1: Write the failing tests**

In codexentry's test file, reusing its fake `Transport` (read the existing Enter/waitTurn tests first):

```go
func TestEnterRecordsTerminalCompletionForExactTurn(t *testing.T) {
	// Fake transport driving a successful thread/turn/turn-completed exchange;
	// a recording fake TerminalRecorder.
	// Assert: exactly one RecordTerminal(threadID, turnID, "completed") call,
	// made AFTER the transport observed Close (have the fake transport set a
	// closed flag and the fake recorder assert it).
}

func TestEnterRecordsTerminalFailureAsEvidence(t *testing.T) {
	// turn/completed frame with status "failed": Enter returns the error AND
	// RecordTerminal(threadID, turnID, "failed") was called — terminal failure
	// IS termination evidence.
}

func TestEnterTransportLossRecordsNothing(t *testing.T) {
	// EOF mid-turn (the existing "ended before completion" fixture): ZERO
	// RecordTerminal calls (AC4 transport loss).
}

func TestEnterRecordFailureSurfacesWithoutFakingEvidence(t *testing.T) {
	// Recorder returns an error on the success path: Enter still returns the
	// output with TerminalRecordFailed == true.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/codexentry/ -run 'TestEnter.*Terminal|TestEnterTransportLoss' -count=1`
Expected: FAIL — `TerminalRecorder` undefined.

- [ ] **Step 3: Implement** per Interfaces. Keep `waitTurn`'s observation loop unchanged except the typed error; the record call lives in `Enter` only.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/codexentry/ ./internal/cli/ ./internal/app/ -count=1`
Expected: PASS (existing cancellation-signal tests in codexentry stay green — AC4 "existing native cancellation behavior remains intact").

- [ ] **Step 5: Commit**

```bash
git add internal/codexentry/client.go internal/codexentry/*_test.go internal/cli/agent.go internal/app/rungate_epoch.go
git commit -m "feat(codexentry): persist exact-turn terminal observation into the run epoch (change 0441)"
```

---

### Task 10: End-to-end acceptance — stale-run-epoch before closeout, standalone admission after

**Files:**
- Test: `internal/app/rungate_complete_test.go` (extend) and, if the finalize gate has an integration harness (`grep -rln 'processFinalizeGate' internal/app/*_test.go`), one assertion there.

**Interfaces:** consumes everything above; produces no new API — this is AC1/AC2's integration pin.

- [ ] **Step 1: Write the failing (or immediately-passing, then mutation-checked) tests**

```go
func TestStandaloneFinalizeAdmissionBlockedThenAdmittedAroundCloseout(t *testing.T) {
	// AC1, at the admission layer processFinalizeGate.RunLocalGate actually
	// hits: a released, epoch-owned slot (completionFixture).
	//
	// BEFORE closeout: an epoch-LESS reservation on the same worktree — the
	// standalone finalize gate's admission shape (gatedrive
	// reserveWorktreeExecution's "RunEpochID != rec.RunEpochID" check) — must
	// refuse ErrStaleRunEpoch. Use the store's standalone reserve entrypoint
	// exactly as the finalize path composes it.
	//
	// Run completeSuccessfulRun (or the full RunGateVerdict wiring).
	//
	// AFTER closeout: the same epoch-less reservation succeeds; release it;
	// then admitWorkflowMutation on the worktree returns a usable done
	// callback (subsequent workflow mutations are not trapped).
}

func TestOrdinaryReleaseStillRetainsEpochBetweenDrives(t *testing.T) {
	// AC2: ReleaseWorktreeExecution on an epoch-owned slot leaves RunEpochID
	// intact, and a foreign/epoch-less reserve between drives still refuses
	// ErrStaleRunEpoch. (This pins the ordinary-release fence the change must
	// NOT weaken.)
}
```

- [ ] **Step 2: Run, then mutation-check** (`-count=1`): strip the Task 3 `findEpochByWorktree` completed-skip → the after-closeout mutation-admission assert must redden; strip the closeout's retire step → the after-closeout reservation assert must redden; re-apply. Strip `ReleaseWorktreeExecution`'s epoch retention (clear `RunEpochID` there) → `TestOrdinaryReleaseStillRetainsEpochBetweenDrives` must redden (AC8 ordinary-release fence probe). Restore everything and record the probes in the commit body.

- [ ] **Step 3: Run the two packages**

Run: `go test ./internal/app/ ./internal/gatedrive/ -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/rungate_complete_test.go
git commit -m "test(app): pin finalize admission around successful closeout (change 0441)"
```

---

### Task 11: Record the ADR-0118 success-lifecycle extension

**Files:** metadata-branch ADR artifacts (owned by the ADR workflow, not this worktree).

- [ ] **Step 1:** Dispatch the registered `docket-adr` agent (repo rule: dispatch, don't run inline) to record the success-lifecycle extension to ADR-0118 — a NEW decision record referencing ADR-0118 (worktree-wide gate admission and explicit human cancellation) and documenting: the `completing`/`completed` epoch states; that only the attributed keyed verdict drives successful closeout; observation-only accounting (no stops); completed epochs excluded from ambient worktree-owner lookup while explicit references remain revoked; and that cancellation wins over completion from `completing`. Accepted history is never rewritten (no in-place edit of ADR-0118's decision text beyond the workflow's own linkage conventions). Ensure the new ADR id lands in change 0441's `adrs:` list per the workflow.
- [ ] **Step 2:** Verify the ADR file and index landed on the metadata branch (`git -C <metadata-worktree> log --oneline -3`; the hermetic suite cannot see this — record it in the results file at close-out per the metadata-branch learning).

---

### Task 12: Full-suite gate, remaining mutation probes, and documentation sweep

**Files:**
- Possibly modify: any caller/schema documentation the audit finds (`grep -rn 'run-cancelled\|stale-run-epoch' docs/ scripts/ *.md --include='*.md'` in the FEATURE tree only — point-in-time records on the metadata branch are never rewritten).

- [ ] **Step 1: Persistence-error mutation probes (AC8, remainder)** — with `-count=1`:
  - make Task 8's checked `SaveGateRecord` failure fall back to best-effort (ignore the error) → `TestVerdictRunCompleteReportPersistFailureIsReported` reddens;
  - make `retireWorktreeSlotOwnership`'s raced-CAS re-read treat a foreign owner as retry-with-successor-token (the successor-protection mutation) → Task 7's foreign-successor test or the existing cancel-side successor test reddens.
  Restore; record probes in the commit body.
- [ ] **Step 2: Documentation sweep.** Repo-wide grep for prose describing the epoch lifecycle's closed state set (e.g. "cancelling/cancelled/superseded") in maintained source comments and README/docs in the feature tree; update any sentence the new states falsify (verify each claimed behavior against the code before rewriting — a restated lifecycle list is exactly the drift surface).
- [ ] **Step 3: Full suite from source through the configured runner:**

Run: `go run ./cmd/docket development test`
Expected: SUITE green. Read the budget report even on green: any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (serial-confirm per `tests/README.md`).

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "test(app): mutation probes and doc sweep for successful-run closeout (change 0441)"
```

---

## Self-Review (performed while writing)

- **Spec coverage:** flow steps 1–8 → Tasks 1, 7, 8; participant terminal evidence → Tasks 2, 9; fencing/revocation/lookup exclusion → Task 3; observation-only launch accounting → Task 4; cancellation-wins + completed-refusal → Task 5; resume behavior → Task 6; failure/replay/consumer behavior → Tasks 7, 8, 10; historical repair-by-replay → Task 8 (contract note + AC7 covered by idempotent replay + blocked-diagnostic tests); AC1–AC8 → named tests in Tasks 3, 5, 7, 8, 9, 10, 12; ADR-0118 extension → Task 11; versioned-record compatibility → Global Constraints + Task 2's `omitempty` additive fields.
- **Placeholder scan:** test skeletons in Tasks 4, 5, 6, 9, 10 name the exact fixture to copy (the adjacent same-file test), the exact acts, and the exact asserts — the worker fills mechanics from the named neighbor, not from taste.
- **Type consistency:** `FenceEpochCompleting(repoDir, gateKey, expectEpoch) (epochState, error)`, `CompleteEpoch(repoDir, gateKey) error`, `RecordEpochParticipantTerminal(repoDir, gateKey, expectEpoch, handle, turn, status) error`, `completeSuccessfulRun(seams, repoDir, gateKey) (bool, string, []string)`, `ObserveEpochLaunches(worktreeRoot, epochID) (EpochLaunchReport, error)`, `TerminalRecorder.RecordTerminal(handle, turnID, status) error` — used with these exact shapes throughout.

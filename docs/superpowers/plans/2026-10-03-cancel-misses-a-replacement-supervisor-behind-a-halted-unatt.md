<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0493 — Retire the automatic gate relaunch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0493-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md)**
<!-- docket:backlink:end -->
# Retire the automatic gate relaunch — Implementation Plan

> **For agentic workers:** this plan is executed by **docket-build**: one tier-routed worker per
> task, strictly sequential in the feature worktree, one commit per task, no per-task review, and a
> single full-suite gate at the end. Each task names its **Risk** (the routing tier:
> economy / standard / premium / max). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the gate drive's automatic relaunch so that a supervisor death halts every drive
`supervisor-died`, and so that the launch census handles at most one launch per drive.

**Architecture:** The work moves through six layers in dependency order, and each task leaves the tree
green.
1. A new exported cause, `CauseSupervisorDied`, plus finalize's mapping test.
2. The driver: the death branch halts, and every relaunch function, the `Advance` recovery branch,
   the slice's claim plumbing, and `WorktreeLock.PriorHolder` are deleted. Every driver test that
   asserted relaunch behaviour is rewritten.
3. The census (`reconcile.go`) proves only `RawRunDir`, loses its reserved-relaunch branch, and
   gets its own CAS sentinel.
4. The record fields and the opt-in surface go: `IdempotentSuiteGate` on three structs, the CLI
   flag, finalize's `true`, and the signature pin.
5. The per-drive claim's Go identifiers are renamed. The file keeps its on-disk name,
   `relaunch.lock`.
6. Prose, budgets, and the embedded mirror.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), Cobra CLI, and `go test`, with
`-tags integration` for real-process tests. The full suite runs through
`go run ./cmd/docket development test`, the value of `build.test_command`.

**Spec:** `docs/superpowers/specs/2026-10-02-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-design.md`.
It is on the `docket` metadata branch; the synchronized copy is under `.docket/` in the primary
checkout. Read the spec beside this plan. Change file:
`docs/changes/active/0493-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md`.

## Global Constraints

- **This change must add no other risk.** Build drives behave exactly as before, apart from the
  renamed halt cause (`not-idempotent` → `supervisor-died`). Finalize and recertify gain only the
  halt that replaces the relaunch.
- **Must-pass-unmodified rule (spec *Tests*).** Tests sort into three groups:
  - relaunch-behaviour tests, which this plan deletes or rewrites;
  - tests that set or read a removed field incidentally, which drop that field or clause and keep
    every other assertion;
  - pins (`TestRepresentativeSignatures`, two budget rows, and the launch-site floor in
    `internal/repoguard/gatelaunch_admission_test.go`).

  This plan names every test it touches. If a test that is **not named in this plan** needs an
  assertion changed, production behaviour has moved beyond this design. **Stop and return BLOCKED**
  rather than adjust the test.
- **The per-drive claim file stays `relaunch.lock` on disk.** Only Go identifiers are renamed.
- **No schema bump and no migration.** `driveSchemaVersion` stays `4`,
  `driveSchemaVersionLegacy` stays `3`, and the store keeps plain `json.Unmarshal`, so unknown
  keys are ignored.
- **Never edited by this change:**
  - `internal/gatedrive/history.go`, the frozen schema-2 reader;
  - `internal/gatedrive/testdata/**`;
  - the bodies of `docs/adrs/**`;
  - `docs/superpowers/plans/**` (other plans), `docs/superpowers/specs/**`, `docs/results/**`, and
    `docs/changes/**`.
- **ADR work is out of scope for every task.** The coordinator records the new ADR, "Gate drives
  never relaunch automatically" (`relates_to: [87, 98, 107, 132]`), through `docket-adr` after
  the build. ADR-0132's dated `## Update` note is coordinator-owned metadata too. No task creates
  or edits an ADR.
- **Cross-references in maintained source anchor on a symbol name or a quoted clause, never a
  line number** (AGENTS.md, ADR-0054). Line numbers in this plan are orientation only.
- **Every test run that observes the current tree defeats the cache with `-count=1`** (learning
  cached-runner-serves-a-mutated-tree). Integration runs always pass `-run '^<Prefix>'`.
- **Every task leaves the tree buildable in both build modes.** Run
  `go build ./... && go vet ./... && go vet -tags integration ./...` before each commit (learning
  intermediate-task-state-buildable).
- **Mutation probes restore from a backup copy, never with `git checkout --`.** Use
  `cp f f.bak; <mutate>; <run>; mv -f f.bak f`. Confirm that the mutation landed with
  `grep -c` before trusting a green run (learnings mutation-restore-needs-a-backup-copy and
  assert-detects-removal-not-replacement).
- **Re-derive sites by grep at the task that removes them.** The site lists here were derived on
  `1fc28e872`. If a grep finds a site the task does not name, and it is a production or
  incidental site, handle it in the same task and say so in the commit body. If it is a test
  assertion outside the named groups, the must-pass-unmodified rule applies.

## Spec deviations found at planning (verified against `1fc28e872`)

1. **`launch-pending` loses its only emitter.** Spec Risk 5 says `launch-pending` "remain[s]
   emitted by the first-launch … paths". It is not. Its sole emitter is `reconcileReservation`'s
   verdict-mode branch, which this change deletes. Grep shows no production consumer that keys
   on it. `internal/app/runtracker_cancel_integration_test.go` and
   `runtracker_start_resume_integration_test.go` inject it as an opaque fake finding, so they
   stay unmodified and stay green. `replacement-stopped` is still emitted by `supervisorGone`'s
   stop leg.
2. **A pin the spec missed.** `TestGateLaunchAdmissionCoverage` has a population floor of `>= 3`
   launch sites, one of which is the relaunch. It drops to `>= 2` (Task 2).
3. **Relaunch-behaviour tests the spec's example list missed:**
   - `TestBarrierCancelBetweenAuthorizationAndLaunch`;
   - `TestClaimContentionBounded`;
   - `TestLoserAfterTerminalSettleReturnsRecordedState`;
   - `TestCorruptReservedDriveNeverLaunches/reserved-relaunch`;
   - `TestCensusChecksPriorRunDir`;
   - `TestCensusLeftoverOutranksOtherFindings`, change 0492's ranking across two run dirs. With
     one run dir its subject is gone, and `censusFindingRank` is deleted with it.

   They are handled in Tasks 2 and 3.
4. **Prose the spec did not list.** `docs/concepts/run-tracker.md` says "its single relaunch".
   Task 6 fixes it.
5. **No glossary entry lists the halt causes one by one.** `supervisor-died` is therefore added as
   a sentence in the *Drive disposition* entry rather than as a new heading (Task 6).

## Review Focus

1. **A finalize drive whose supervisor dies, reached on a later `finalize.rebase` continuation
   slice.** Finalize must report a halted gate in the unavailable class (`gate-halted`) and remove
   the exited run root, never call the result red or keep a stale root. The test is in Task 1
   (`TestMapDriveOutcomeSupervisorDiedIsGateHalted`).
2. **Installing over a drive left mid-relaunch**, a record with `relaunch_reserved: true` from a
   crashed pre-0493 CLI. The next `Advance` must halt `supervisor-died`. It must launch, resolve,
   and stop nothing, and must not fail to load. Tests are in Task 2
   (`TestOldRelaunchRecordAdvancesToSupervisorDied`) and Task 4
   (`TestOldRelaunchFieldsLoadAndAreDroppedOnWrite`).
3. **Two concurrent same-owner advances racing over one death.** Both must return the one
   recorded `HALTED supervisor-died` with no error and no launch. Tests are in Task 2
   (`TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce` and the rewritten
   `TestLoserAfterTerminalSettleReturnsRecordedState`).
4. **A hand-typed `gate drive start --idempotent-suite-gate`.** It must fail loudly with exit 2
   and an unknown-flag error, never be silently accepted. The test is in Task 4
   (`TestGateDriveStartRejectsIdempotentSuiteGateFlag`).
5. **A run-attributed build drive that died.** `run.cancel`'s census must account it from its one
   recorded run dir and stop nothing. It must never probe a leftover `prior_raw_run_dir` or
   resolve a leftover relaunch token. Tests are in Task 2
   (`TestRunBackedDeathHaltsAndCensusAccountsOneLaunch`) and Task 3
   (`TestCensusProvesOnlyRawRunDir`).

---

### Task 1: `CauseSupervisorDied` and finalize's mapping of it

**Risk:** economy. One constant and two tests. Finalize's existing default already maps the cause,
so no production branch changes.

**Files:**
- Modify: `internal/gatedrive/drive.go`. Add the constant to the exported cause `const` block.
- Modify: `internal/app/finalize_rebase.go`. Update the doc comment of `mapDriveHaltCause` only.
- Test: `internal/app/gate_drive_test.go`

**Interfaces:**
- Produces: `gatedrive.CauseSupervisorDied = "supervisor-died"` (untyped string const, beside
  `CauseDeadlineExpired`). Tasks 2–4 emit and assert it.

- [ ] **Step 1: Write the failing tests.** In `internal/app/gate_drive_test.go`, inside
  `TestMapDriveHaltCauseKeysOnGatedriveConstants`, **replace** the `gatedrive.CauseWorktreeBusy`
  row and its two-line comment with:

```go
		// Change 0493: a supervisor death halts the drive (never relaunched);
		// finalize reads it as an unavailable gate and a human re-runs finalize.
		gatedrive.CauseSupervisorDied: GateHaltUnavailable,
```

  The row being replaced is a pin on a token that Task 2 deletes. Then add this test after
  `TestMapDriveHaltCauseKeysOnGatedriveConstants`:

```go
// TestMapDriveOutcomeSupervisorDiedIsGateHalted (change 0493): a finalize or
// recertify drive whose supervisor died is a terminal HALTED gate in the
// unavailable class (finalize reports it as gate-halted), and its exited run
// root is removed like any other terminal halt's.
func TestMapDriveOutcomeSupervisorDiedIsGateHalted(t *testing.T) {
	g := &processFinalizeGate{}
	root := runRootFixture(t)
	doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseSupervisorDied, RunRoot: root}
	res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
	if res.Outcome != FinalizeGateHalted || res.HaltCause != GateHaltUnavailable {
		t.Fatalf("supervisor-died = %s/%s, want %s/%s", res.Outcome, res.HaltCause, FinalizeGateHalted, GateHaltUnavailable)
	}
	if dirExists(t, root) {
		t.Fatalf("a supervisor-died terminal must remove its exited run root %q", root)
	}
}
```

- [ ] **Step 2: Run them and watch them fail.**
  Run `go test -count=1 ./internal/app/ -run 'TestMapDriveHaltCauseKeysOnGatedriveConstants|TestMapDriveOutcomeSupervisorDiedIsGateHalted'`.
  Expected: a compile FAIL, `undefined: gatedrive.CauseSupervisorDied`.

- [ ] **Step 3: Add the constant.** In `internal/gatedrive/drive.go`, add this after the
  `CauseTakeoverAmbiguous` entry in the exported cause `const` block. Leave `CauseWorktreeBusy`
  in place; Task 2 deletes it.

```go
	// CauseSupervisorDied: the drive's run died without a verdict (signaled or
	// vanished) and no owned tree survives. A gate drive never relaunches (change
	// 0493): it HALTs, and a human re-runs the workflow, which re-runs the suite.
	CauseSupervisorDied = "supervisor-died"
```

  In `internal/app/finalize_rebase.go`, rewrite the doc comment of `mapDriveHaltCause` to read:

```go
// mapDriveHaltCause maps a driver HALTED cause token onto the closed finalize
// halt vocabulary. A deadline expiry is the running-at-budget analog; every other
// fail-closed cause (a changed worktree, uncertain ownership, malformed/unreadable
// state, a supervisor death — gatedrive.CauseSupervisorDied, never relaunched
// since change 0493) is reported as unavailable — a human is needed, never repair
// work. It never fabricates a decidable pass/fail.
```

- [ ] **Step 4: Run the tests and check that they pass.** Run the Step 2 command again. Expected: PASS.

- [ ] **Step 5: Mutation probe.** Back up `internal/app/finalize_rebase.go`. In `mapDriveHaltCause`, add
  `case cause == gatedrive.CauseSupervisorDied: return GateHaltRunningAtBudget` as the first
  `case`. Confirm the edit with `grep -c 'CauseSupervisorDied: return' internal/app/finalize_rebase.go` → 1.
  Run the Step 2 command. Expected: both tests FAIL. Restore with
  `mv -f internal/app/finalize_rebase.go.bak internal/app/finalize_rebase.go`.

- [ ] **Step 6: Build gate and commit.**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
git add internal/gatedrive/drive.go internal/app/finalize_rebase.go internal/app/gate_drive_test.go
git commit -m "feat(0493): add the supervisor-died halt cause; finalize maps it to a halted gate"
```

---

### Task 2: A supervisor death always halts; delete the driver's relaunch machinery

**Risk:** premium. This rewrites the gate-drive state machine's death branch and deletes its
crash-window machinery. A mistake here could relaunch, leak a claim, or change build-drive
behaviour. The change is correctable, but it matters.

**Files:**
- Modify: `internal/gatedrive/driver.go`
- Modify: `internal/gatedrive/drive.go`. Delete `CauseWorktreeBusy`, and update the `HALTED`
  outcome comment.
- Modify: `internal/gatedrive/worktree_lock.go`. Delete `PriorHolder`.
- Modify: `internal/repoguard/gatelaunch_admission_test.go`. This is the population-floor pin.
- Test: `internal/gatedrive/driver_test.go`, `driver_concurrency_test.go`,
  `driver_launch_claim_test.go`, `worktree_lock_test.go`, `store_test.go`, and
  `supervisor_integration_test.go`.

**Interfaces:**
- Consumes: `CauseSupervisorDied` (Task 1).
- Produces, all used by later tasks:
  - `func (d *Driver) driveSlice(rec driveRecord) sliceResult`, which has no `id`, `ownerGen`, or
    `claim` parameter any more;
  - `func (d *Driver) driveAndPersist(id, ownerGen string, rec driveRecord) (DriveDoc, error)`,
    the only persist path (`driveAndPersistClaim` is gone);
  - the test helpers in `store_test.go`:
    `func stampRecordKeys(t *testing.T, store *Store, id string, keys map[string]any)` and
    `func legacyRelaunchKeys() map[string]any`.
- Still present after this task, for Task 3 and later: `errRelaunchRaceLost` (the census uses it
  until Task 3), the record fields, `StartRequest.IdempotentSuiteGate`, `relaunchClaim`,
  `tryRelaunchClaim`, and `relaunchLockFileName`.

- [ ] **Step 1: Write the new death tests (RED).** In `internal/gatedrive/driver_test.go`:
  1. Rename the section banner "Death and the single relaunch." to
     "Death: a supervisor death always halts (change 0493)."
  2. **Delete** `TestSignaledDeathRelaunchAdmittedOnce` and `TestDeathRelaunchRefusals`.
  3. Add the following in their place:

```go
// deathAfterFirstSlice starts req with its first run live for the first slice;
// once *dead is set, that run observes state, so the next Advance reaches the
// death branch.
func deathAfterFirstSlice(t *testing.T, req StartRequest, state process.State) (*Driver, *Store, *fakeProc, DriveDoc, *bool) {
	t.Helper()
	clk := &fakeClock{now: startRun()}
	dead := false
	proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
		if dead && strings.HasSuffix(runDir, "run1") {
			return obs(state, runDir), nil
		}
		return obs(process.StateRunning, runDir), nil
	}}
	d, store := newTestDriver(t, clk, proc, stableGit())
	started, err := d.Start(req)
	if err != nil || started.Outcome != WAITING {
		t.Fatalf("Start = %s (%v), want WAITING", started.Outcome, err)
	}
	return d, store, proc, started, &dead
}

// TestDeathHaltsSupervisorDied (change 0493): every drive whose run dies — one
// started as finalize starts one or as a build drive, signaled or vanished,
// within or past its deadline, worktree unchanged or drifted, with or without
// another gate holding the worktree — HALTs supervisor-died with its one
// launch. Nothing launches again, the worktree lock is never re-taken, and the
// record keeps its first run.
func TestDeathHaltsSupervisorDied(t *testing.T) {
	for _, tc := range []struct {
		name       string
		owner      string
		idempotent bool
		state      process.State
		holdOther  bool
	}{
		{"finalize signaled", "finalize", true, process.StateSignaled, false},
		{"finalize vanished", "finalize", true, process.StateVanished, false},
		{"build signaled", "build", false, process.StateSignaled, false},
		{"build vanished", "build", false, process.StateVanished, false},
		{"another gate took the worktree", "finalize", true, process.StateVanished, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := sampleStart()
			req.Owner = tc.owner
			req.IdempotentSuiteGate = tc.idempotent
			d, store, proc, started, dead := deathAfterFirstSlice(t, req, tc.state)
			if tc.holdOther {
				holdWorktree(t, store, req.Cwd)
			}
			*dead = true

			doc, err := d.Advance(started.DriveID, started.Generation)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied {
				t.Fatalf("a death = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseSupervisorDied)
			}
			if proc.launchN != 1 {
				t.Fatalf("a death must never launch again: launches = %d, want 1", proc.launchN)
			}
			rec, err := store.Load(started.DriveID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.Attempt != 1 || rec.RawRunDir != "/runs/run1" || rec.LastCause != CauseSupervisorDied {
				t.Fatalf("record = attempt %d run %q cause %q, want 1 /runs/run1 %s", rec.Attempt, rec.RawRunDir, rec.LastCause, CauseSupervisorDied)
			}
			if !tc.holdOther && !worktreeFree(t, store, req.Cwd) {
				t.Fatalf("a halted drive must hold no worktree lock")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*driveRecord)
		drift  bool
	}{
		{name: "deadline already past", mutate: func(r *driveRecord) { r.Deadline = startRun().Add(-time.Minute) }},
		{name: "worktree drifted", drift: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := stableGit()
			proc := &fakeProc{
				observe: func(runDir string) (*process.Observation, error) { return obs(process.StateSignaled, runDir), nil },
				stop: func(runDir, reason string) (*process.StopOutcome, error) {
					return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
						Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
				},
			}
			d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, git)
			rec := seedRecord(t)
			if tc.mutate != nil {
				tc.mutate(&rec)
			}
			id, ownerGen := seedDrive(t, store, rec)
			if tc.drift {
				git.status = "DRIFTED"
			}
			doc, err := d.Advance(id, ownerGen)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied || proc.launchN != 0 {
				t.Fatalf("a death = %s/%q launches %d, want HALTED/%s and no launch", doc.Outcome, doc.Cause, proc.launchN, CauseSupervisorDied)
			}
		})
	}
}

// TestDeathUnprovenHaltsUncertainOwnership: a death whose tree cannot be proven
// gone (the probe stop errors) HALTs uncertain-ownership, as before change 0493,
// and launches nothing.
func TestDeathUnprovenHaltsUncertainOwnership(t *testing.T) {
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) { return obs(process.StateSignaled, runDir), nil },
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: stop cannot prove ownership")
		},
	}
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != "uncertain-ownership" || proc.launchN != 0 {
		t.Fatalf("an unproven death = %s/%q launches %d, want HALTED/uncertain-ownership and no launch", doc.Outcome, doc.Cause, proc.launchN)
	}
}
```

  Then, in `internal/gatedrive/store_test.go`, add the import
  `"github.com/danielhanold/docket/internal/process"` and this code:

```go
// stampRecordKeys rewrites drive id's persisted record to carry keys verbatim —
// the shape an older binary left on disk — keeping the envelope's generation so
// the record stays CAS-able by its current owner.
func stampRecordKeys(t *testing.T, store *Store, id string, keys map[string]any) {
	t.Helper()
	path := filepath.Join(store.root, id, recordFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rec := env["record"].(map[string]any)
	for k, v := range keys {
		rec[k] = v
	}
	if raw, err = json.Marshal(env); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// legacyRelaunchKeys is the relaunch state a pre-0493 binary wrote on a drive a
// crashed CLI left mid-relaunch: a consumed, never-attached reservation.
func legacyRelaunchKeys() map[string]any {
	return map[string]any{
		"idempotent_suite_gate": true,
		"relaunch_count":        1,
		"relaunch_reserved":     true,
		"relaunch_token":        "bbbbbbbbbbbbbbbb",
		"prior_raw_run_dir":     "/runs/run0",
	}
}

// TestOldRelaunchRecordAdvancesToSupervisorDied (change 0493, spec Design §5): a
// nonterminal record left mid-relaunch is advanced as an ordinary drive — its
// first run is dead, so it HALTs supervisor-died — and the advance launches,
// resolves, and stops nothing.
func TestOldRelaunchRecordAdvancesToSupervisorDied(t *testing.T) {
	proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
		return obs(process.StateVanished, runDir), nil
	}}
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	stampRecordKeys(t, store, id, legacyRelaunchKeys())

	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance over an old mid-relaunch record: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied {
		t.Fatalf("old mid-relaunch record = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseSupervisorDied)
	}
	if proc.launchN != 0 || proc.resolveN != 0 || proc.stopN != 0 {
		t.Fatalf("an old reservation is never acted on: launches=%d resolves=%d stops=%d", proc.launchN, proc.resolveN, proc.stopN)
	}
}
```

- [ ] **Step 2: Run the new tests and watch them fail.**
  Run `go test -count=1 ./internal/gatedrive/ -run 'TestDeathHaltsSupervisorDied|TestDeathUnprovenHaltsUncertainOwnership|TestOldRelaunchRecordAdvancesToSupervisorDied'`.
  Expected results:
  - the finalize cases FAIL (WAITING, because a relaunch happened);
  - the build cases FAIL (`not-idempotent` ≠ `supervisor-died`);
  - the seeded cases FAIL (`deadline-expired` and `worktree-changed`);
  - the old-record test FAILS, with a launch or `relaunch-exhausted`;
  - `TestDeathUnprovenHaltsUncertainOwnership` may already PASS, which is expected because it pins
    unchanged behaviour.

- [ ] **Step 3: Rewrite the driver.** Make these changes in `internal/gatedrive/driver.go`.
  1. **Package doc comment.** Replace "only an exact native running state is retryable; a death
     earns at most one relaunch under five conjoined conditions; every other uncertainty is
     HALTED" with "only an exact native running state is retryable; a supervisor death always
     HALTs (a gate drive never relaunches, change 0493); every other uncertainty is HALTED".
  2. **`ProcessSeam.ResolveReservation` doc.** Replace the sentences from "The relaunch's
     launch-error leg …" through to the end of the comment with "The launch census consults it to
     resolve a first launch whose run was never attached."
  3. **Delete the relaunch lock re-take.** Delete the `relaunchLockTries`/`relaunchLockPause`
     const block and its comment. Run `git grep -n isWorktreeBusy -- internal`. If the
     relaunch loop was its only caller, delete `isWorktreeBusy` too.
  4. **`revalidateAdmittedLaunch` doc.** Replace "(the SAME lock file
     tryRelaunchClaim/reserveRelaunch use)" with "(the SAME lock file the launch census probes)".
  5. **`launchScopeless` doc comment.** Replace the sentence that begins "The claimant flock
     revalidateAdmittedLaunch acquired is HELD across Launch and attach" with "The claimant flock
     revalidateAdmittedLaunch acquired is HELD across Launch and attach (so a concurrent
     cancellation observes pending work), then released before the drive slice; the deferred
     close is an idempotent safety net for every failure leg."
  6. **`launchScopeless` body comment.** Replace the comment above the in-body `claim.close()`
     with:

```go
	// Launch and attach are confirmed: release the launch claim before the drive
	// slice. Nothing in a slice takes it — a drive never relaunches (change 0493)
	// — and close is idempotent with the deferred safety net.
```

  7. **`Advance`.** After the terminal check, delete the `var claim *relaunchClaim` block and the
     `if rec.RelaunchReserved { … }` block. End the function with
     `return d.driveAndPersist(id, ownerGen, rec)`.
  8. **`driveAndPersist`.** Replace both `driveAndPersist` and `driveAndPersistClaim` with:

```go
// driveAndPersist runs one slice over rec, persists the resulting transition
// atomically under the owner CAS, and returns the outcome document built from
// the authoritative post-transition record. A concurrent same-owner writer that
// already settled the drive wins: this caller returns that recorded verdict
// rather than clobbering it.
func (d *Driver) driveAndPersist(id, ownerGen string, rec driveRecord) (DriveDoc, error) {
	res := d.driveSlice(rec)
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		r.UpdatedAt = res.lastClock
		r.LastClock = res.lastClock
		r.LastOutcome = res.outcome
		r.LastCause = res.cause
		return nil
	})
	if err != nil && !errors.Is(err, errAlreadyTerminal) {
		return DriveDoc{}, err
	}
	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.recordedDoc(id, ownerGen, cur), nil
}
```

  9. **`errAlreadyTerminal` doc.** Replace it with:

```go
// errAlreadyTerminal is a sentinel raised inside an owner CAS (the persist CAS,
// the launch-failed CAS, and the launch census's never-launched settle) to abort
// a write over a drive a concurrent writer already finished; it never escapes as
// a workflow error. Every caller treats it as "reload the authoritative recorded
// state".
```

  10. **Delete the relaunch functions.** Delete `reserveRelaunch`, `recoverReservedRelaunch`,
      `haltReservedRelaunch`, `haltReservedRelaunchCause`, `attachReservedRelaunch`,
      `relaunchRefusal`, and `ownerIf`, with their doc comments. **Keep** `errRelaunchRaceLost`
      for now; `reconcile.go` still uses it until Task 3.
  11. **`sliceResult`.** Replace it with:

```go
// sliceResult is one slice's decision: the outcome/cause to persist. lastClock
// is the freshly accepted clock value bound to the record.
type sliceResult struct {
	outcome   Outcome
	cause     string
	rawRunDir string // PASSED only

	lastClock time.Time
}
```

  12. **`driveSlice`.** Change its signature to `func (d *Driver) driveSlice(rec driveRecord) sliceResult`.
      Delete the `relaunchUsed` local. Replace the doc comment's "a death earns at most one
      relaunch under the five conjoined conditions" with "a supervisor death always HALTs (never
      relaunched, change 0493)", and "the persisted mutations travel back" with "the persisted
      transition travels back". Replace the whole `case process.StateSignaled, process.StateVanished:`
      arm with:

```go
		case process.StateSignaled, process.StateVanished:
			// A death without a verdict. A gate drive never relaunches (change
			// 0493): a proven death HALTs supervisor-died and a human re-runs the
			// workflow; a death whose tree cannot be proven gone HALTs
			// uncertain-ownership.
			gone, derr := d.proveNoTreeSurvives(runDir, observation)
			if derr != nil || !gone {
				return halt(&res, "uncertain-ownership")
			}
			return halt(&res, CauseSupervisorDied)
```

  13. **`proveNoTreeSurvives` doc.** Replace its first sentence with "proveNoTreeSurvives decides
      between the two death HALT causes: it establishes that no owned process tree survives a
      death (supervisor-died), or reports that it cannot (uncertain-ownership)."
  14. **`recordedDoc` doc.** Replace "WAITING retains it — a relaunch may still replay under it"
      with "WAITING retains it — the live run still writes under it".
  15. **`launchRequest` doc.** Replace it with:

```go
// launchRequest builds the deterministic raw launch input for the drive's one
// launch. token is the drive-minted launch token (AdmissionToken), carried as
// the manifest's reservation token so a lost launch response is resolvable to
// this exact run (ResolveReservation). worktreeLock is the held worktree lock
// handed to the supervisor (change 0490): process.Launch takes ownership of it
// on every path. This body is the only process.LaunchRequest literal in the
// package, and it always sets WorktreeLock.
```

  Then:
  - In `internal/gatedrive/drive.go`, delete the `CauseWorktreeBusy` constant and its comment. In
    the `HALTED` outcome comment, replace "or an unadmitted death" with "or a supervisor death".
  - In `internal/gatedrive/worktree_lock.go`, delete `PriorHolder` and its comment. Keep
    `readHolderNote`.
  - In `internal/repoguard/gatelaunch_admission_test.go`, change the header line "Today's
    population: app's raw GateLaunch (form a) and the gate driver's first launch and single
    relaunch, both through driveRecord.launchRequest (form b)." to "Today's population: app's raw
    GateLaunch (form a) and the gate driver's one launch, through driveRecord.launchRequest (form
    b); the single relaunch was retired by change 0493."
  - In that same file, change the floor to `if rep.sites < 2 {`, with the message
    `"population floor: found %d launch sites (want >= 2: app GateLaunch and the gate driver's launch); the launch-shape detector drifted"`.

- [ ] **Step 4: Remove or rewrite the driver tests that asserted relaunch behaviour.** This list is
  the complete set of edits for this step. Anything else that goes red is the must-pass-unmodified
  rule: return BLOCKED.
  1. **`driver_test.go`.**
     - Delete `TestRelaunchCrashBetweenReserveAndLaunchRecovers`,
       `TestRelaunchReservationNotRefundedOnUncertainty`, the `relaunchAfterDeath` helper,
       `TestRelaunchFindsWorktreeHeldHaltsWorktreeBusy`,
       `TestRelaunchWaitsOutItsOwnSupervisorsExit`, and
       `TestRelaunchRewritesHolderKeepingOwner`. Their properties are now
       `TestDeathHaltsSupervisorDied` (no re-take, so no `worktree-busy`) and
       `TestOldRelaunchRecordAdvancesToSupervisorDied`.
     - In `TestCorruptReservedDriveNeverLaunches`, replace the doc comment's tail "; a corrupt
       drive's reserved relaunch never launches either." with "; advancing a corrupt live drive
       never launches again."
     - Replace the `reserved-relaunch` subtest with:

```go
	t.Run("corrupt-live-drive", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		doc, err := d.Start(sampleStart())
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("Start: doc=%+v err=%v", doc, err)
		}
		corruptFile(t, filepath.Join(store.root, doc.DriveID, recordFileName))

		launches := proc.launchN
		if adv, err := d.Advance(doc.DriveID, doc.Generation); err == nil && adv.Outcome != HALTED {
			t.Fatalf("advancing a corrupt drive must halt or fail, got %+v", adv)
		}
		if proc.launchN != launches {
			t.Fatal("a corrupt drive must never launch again")
		}
	})
```

  2. **`driver_concurrency_test.go`.**
     - Replace `racingProc`, `newRacingProc`, and its methods with the version below. Delete
       `relaunchStopCount` and `liveRelaunchDirs`.

```go
// racingProc is a thread-safe ProcessSeam purpose-built to race two same-owner
// advances over one death deterministically: the seeded original run is dead
// (signaled), and its first two observations rendezvous both advances before
// either may persist. Launch only counts — a drive never relaunches (change
// 0493), so it must never be called. All bookkeeping is mutex-guarded so the
// test is clean under -race.
type racingProc struct {
	barrier *sync.WaitGroup // trips once both advances observed the dead run

	mu        sync.Mutex
	launchSeq int
	deathObs  int
}

func newRacingProc() *racingProc {
	var b sync.WaitGroup
	b.Add(2)
	return &racingProc{barrier: &b}
}

func (p *racingProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	defer releaseHandedLock(req)
	p.mu.Lock()
	p.launchSeq++
	id := fmt.Sprintf("relaunch%d", p.launchSeq)
	p.mu.Unlock()
	return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
}

func (p *racingProc) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		p.mu.Lock()
		block := p.deathObs < 2
		if block {
			p.deathObs++
		}
		p.mu.Unlock()
		if block {
			p.barrier.Done()
			p.barrier.Wait()
		}
		return &process.Observation{State: process.StateSignaled, RunDir: runDir}, nil
	}
	return &process.Observation{State: process.StateRunning, RunDir: runDir}, nil
}

func (p *racingProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	// The signaled original is already terminal: an ownership-proven no-op.
	return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
		Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
}

func (p *racingProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *racingProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}
```

     - Replace `TestConcurrentSameOwnerAdvanceRelaunchesOnce` with:

```go
// TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce (change 0493): two concurrent
// Advance calls presenting the SAME valid owner generation over a drive whose run
// has died both observe the death before either persists (racingProc's barrier),
// and both return the one recorded HALTED supervisor-died verdict with no error.
// Neither launches anything.
func TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	proc := newRacingProc()

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startRun().Add(time.Second)}
		d := NewDriver(store, clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}
	drivers := []*Driver{mkDriver(), mkDriver()}
	docs := make([]DriveDoc, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range drivers {
		go func(i int) {
			defer wg.Done()
			docs[i], errs[i] = drivers[i].Advance(id, ownerGen)
		}(i)
	}
	wg.Wait()

	for i := range docs {
		if errs[i] != nil {
			t.Fatalf("advance %d returned an error: %v", i, errs[i])
		}
		if docs[i].Outcome != HALTED || docs[i].Cause != CauseSupervisorDied {
			t.Fatalf("advance %d = %s/%q, want HALTED/%s", i, docs[i].Outcome, docs[i].Cause, CauseSupervisorDied)
		}
	}
	if proc.launchSeq != 0 {
		t.Fatalf("a death must never launch, got %d launches", proc.launchSeq)
	}
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != CauseSupervisorDied || rec.Attempt != 1 || rec.RawRunDir != "/runs/run1" {
		t.Fatalf("record = %s/%q attempt %d run %q, want HALTED/%s attempt 1 /runs/run1", rec.LastOutcome, rec.LastCause, rec.Attempt, rec.RawRunDir, CauseSupervisorDied)
	}
}
```

     - In `TestLoserAfterTerminalSettleReturnsRecordedState` and its seam types (`terminalSettleProc`,
       `gatedLoserSeam`), keep the gating machinery and change only the assertions and the
       relaunch wording.
       - Doc comments: the scenario is now "a same-owner advance whose persist CAS finds the
         winner's terminal HALTED supervisor-died record". `gatedLoserSeam`'s comment says "the
         loser's persist CAS finds a terminal record" in place of "reserveRelaunch CAS".
       - Assert that the winner is `HALTED`/`CauseSupervisorDied`, and that the loser returns no
         error with the same outcome and cause as the winner.
       - Assert `core.launchSeq == 0` ("a death must never launch").
       - Assert that `rec.Attempt == 1` and `rec.LastOutcome == HALTED`.
       - Keep the deadline-preserved assertion.
       - Delete the `RelaunchCount` assertion and the relaunch-stop loop; nothing is launched, so
         there is nothing to stop.
     - Replace `TestBarrierCancelBetweenAuthorizationAndLaunch` with:

```go
// TestRunBackedDeathHaltsAndCensusAccountsOneLaunch (change 0493): a drive
// started inside a run whose supervisor dies halts supervisor-died with its one
// launch — no replacement exists for a cancellation to race — and run.cancel's
// census then accounts it from that one recorded run dir, stopping nothing.
func TestRunBackedDeathHaltsAndCensusAccountsOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	clk := &fakeClock{now: startRun()}
	dead := false
	runsRoot := testsupport.TempDir(t) // the census probes run dirs that exist on disk
	var launchCount int32
	proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
		if dead {
			return obs(process.StateSignaled, runDir), nil
		}
		return obs(process.StateRunning, runDir), nil
	}}
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		n := atomic.AddInt32(&launchCount, 1)
		id := fmt.Sprintf("run%d", n)
		runDir := filepath.Join(runsRoot, id)
		if err := os.MkdirAll(runDir, 0o700); err != nil {
			return nil, err
		}
		return &process.LaunchOutcome{RunID: id, RunDir: runDir, State: process.StateRunning}, nil
	}
	d := storeTestDriver(store, clk, proc, stableGit())

	req := sampleStart()
	req.RunContext = "ctx-e1"
	started, serr := d.Start(req)
	if serr != nil || started.Outcome != WAITING {
		t.Fatalf("run-backed first slice must WAIT, got %+v (err=%v)", started, serr)
	}
	dead = true
	doc, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied {
		t.Fatalf("a run-backed death = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseSupervisorDied)
	}
	if got := atomic.LoadInt32(&launchCount); got != 1 {
		t.Fatalf("a death must never launch again, launches=%d", got)
	}

	sup := newSupervisors()
	run1 := filepath.Join(runsRoot, "run1")
	sup.state[run1] = process.StateSignaled
	dr := storeTestDriver(reopenStore(store), &fakeClock{now: startRun()}, sup.proc(), stableGit())
	report, err := dr.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "run-terminal", started.DriveID) {
		t.Fatalf("the halted drive's one dead run must account run-terminal, got %+v", report)
	}
	if len(sup.stopped) != 0 || len(sup.observed) != 1 || sup.observed[0] != run1 {
		t.Fatalf("the census must observe only %s and stop nothing: observed %v stopped %v", run1, sup.observed, sup.stopped)
	}
}
```

     - Delete `claimWindowProc` and its methods, and
       `TestRelaunchReservationHolderCannotBeStolenBeforeLaunch`. The rewrite below is its last
       user.
  3. **`driver_launch_claim_test.go`.**
     - In `TestStartAdmittedHoldsClaimAcrossLaunch`'s doc comment, replace "so the drive slice can
       reserve its own relaunch" with "before the drive slice".
     - Replace `TestClaimContentionBounded` with:

```go
// TestClaimContentionBounded proves the nonblocking per-drive claim bounds
// contention with no deadlock: StartAdmitted parks inside Launch holding the
// drive's claim while several reconcilers probe the drive. Every reconciler
// returns promptly with claim-busy (done-channel oracles, never a timing sleep
// as the ordering fact), and the launch completes after release with exactly one
// backend launch. A drive never relaunches (change 0493), so StartAdmitted is the
// claim's only launcher.
func TestClaimContentionBounded(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.RunContext = "ctx-e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	proc.launch = blockingLaunch(entered, release)
	launched := make(chan error, 1)
	go func() {
		_, serr := d.StartAdmitted(ticket)
		launched <- serr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("StartAdmitted did not enter Launch")
	}

	const reconcilers = 4
	reconcileDone := make(chan RunLaunchReport, reconcilers)
	for i := 0; i < reconcilers; i++ {
		go func() {
			dr := NewDriver(reopenStore(store), &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
			r, _ := dr.ReconcileRunLaunches(capHash(req.RunContext))
			reconcileDone <- r
		}()
	}
	for i := 0; i < reconcilers; i++ {
		select {
		case report := <-reconcileDone:
			if report.Accounted || !reconcileFindingPresent(report.Findings, "claim-busy:"+ticket.id) {
				t.Errorf("a held claim must read claim-busy:%s and stay unaccounted, got %+v", ticket.id, report)
			}
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("a reconcile blocked on the held claim; it must probe nonblocking")
		}
	}

	close(release)
	select {
	case serr := <-launched:
		if serr != nil {
			t.Fatalf("StartAdmitted: %v", serr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartAdmitted did not return after release")
	}
	if proc.launchN != 1 {
		t.Fatalf("exactly one backend launch under contention, got %d", proc.launchN)
	}
}
```

  4. **`worktree_lock_test.go`.**
     - Rename `TestWorktreeLockPriorHolderRoundTrips` to `TestWorktreeLockHolderNoteRoundTrips`.
     - Replace each `l.PriorHolder()` with `readHolderNote(l.dir)`. Change the failure strings
       from "PriorHolder" to "readHolderNote".
     - In `TestWriteHolderSkipsWhenRunNoLongerRunning`, replace `l.PriorHolder()` with
       `readHolderNote(l.dir)`. Every assertion is unchanged.
  5. **`supervisor_integration_test.go`** (`//go:build integration`). Replace
     `TestIntegrationGatedriveProcessDeathPermitsAtMostOneRelaunch` with the code below. Add
     `"syscall"` to the imports if it is not already there.

```go
// waitWorktreeFree polls until cwd's worktree lock is free, failing after 30s: a
// dead run's supervisor closes its copy of the lock a few durable writes after
// its terminal record becomes visible.
func waitWorktreeFree(t *testing.T, store *Store, cwd string) {
	t.Helper()
	end := time.Now().Add(30 * time.Second)
	for !worktreeFree(t, store, cwd) {
		if time.Now().After(end) {
			t.Fatalf("worktree %s still locked 30s after the drive halted", cwd)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestIntegrationGatedriveProcessDeathHaltsSupervisorDied (change 0493): a real
// gate whose run dies mid-drive is never relaunched. Whether the suite dies by a
// genuine signal (the child SIGKILLs itself; the supervisor records a signaled
// terminal) or the supervisor itself is killed (the run vanishes), the drive
// HALTs supervisor-died, exactly one raw run dir exists under the run root, and
// the worktree lock is free once the supervisor is gone.
func TestIntegrationGatedriveProcessDeathHaltsSupervisorDied(t *testing.T) {
	t.Run("child-signal-death", func(t *testing.T) {
		skipUnlessSupported(t)
		svc := mustService(t)
		runRoot := filepath.Join(testsupport.TempDir(t), "runs")
		store := OpenStore(testsupport.TempDir(t))
		t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
		reapSupervisors(t, runRoot)
		d := newIntDriver(store, svc)
		cwd := testsupport.TempDir(t)

		doc, err := d.Start(intStartRequest(mustExe(t), runRoot, cwd, "selfkill-after", "120"))
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("first slice over a live child = %s (%v), want WAITING", doc.Outcome, err)
		}
		term, _ := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
		if term.Outcome != HALTED || term.Cause != CauseSupervisorDied {
			t.Fatalf("terminal = %s/%q, want HALTED/%s", term.Outcome, term.Cause, CauseSupervisorDied)
		}
		runDir := soleRunDir(t, runRoot) // exactly one run: never relaunched
		if st := observeState(t, svc, runDir); st != process.StateSignaled {
			t.Fatalf("the dead run's state = %v, want signaled", st)
		}
		waitWorktreeFree(t, store, cwd)
	})
	t.Run("supervisor-killed", func(t *testing.T) {
		skipUnlessSupported(t)
		svc := mustService(t)
		runRoot := filepath.Join(testsupport.TempDir(t), "runs")
		store := OpenStore(testsupport.TempDir(t))
		// The reaper is registered FIRST so it outlives the cleanup stop.
		reapSupervisors(t, runRoot)
		t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
		d := newIntDriver(store, svc)
		cwd := testsupport.TempDir(t)

		doc, err := d.Start(intStartRequest(mustExe(t), runRoot, cwd, "sleep-forever", ""))
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("first slice over a live child = %s (%v), want WAITING", doc.Outcome, err)
		}
		runDir := soleRunDir(t, runRoot)
		id := readManifestIdentity(t, runDir)
		// The killed supervisor's suite survives it (change 0492); cleanup ends it.
		t.Cleanup(func() { _ = syscall.Kill(-id.PGID, syscall.SIGKILL) })
		if err := syscall.Kill(id.SupervisorPID, syscall.SIGKILL); err != nil {
			t.Fatalf("kill supervisor: %v", err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for pidAlive(id.SupervisorPID) {
			if time.Now().After(deadline) {
				t.Fatalf("the killed supervisor %d never went away", id.SupervisorPID)
			}
			time.Sleep(5 * time.Millisecond)
		}
		term, _ := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
		if term.Outcome != HALTED || term.Cause != CauseSupervisorDied {
			t.Fatalf("terminal = %s/%q, want HALTED/%s", term.Outcome, term.Cause, CauseSupervisorDied)
		}
		if got := len(runDirsUnder(t, runRoot)); got != 1 {
			t.Fatalf("want exactly one raw run dir (never relaunched), got %d", got)
		}
		waitWorktreeFree(t, store, cwd)
	})
}
```

  Remove any imports that became unused (the compiler names them).

- [ ] **Step 5: Run the package and the touched pins.**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/ ./internal/repoguard/
go test -count=1 -race ./internal/gatedrive/ -run 'TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce|TestLoserAfterTerminalSettleReturnsRecordedState|TestClaimContentionBounded|TestRunBackedDeathHaltsAndCensusAccountsOneLaunch'
go test -tags integration -count=1 -run '^TestIntegrationGatedrive' ./internal/gatedrive/
go test -count=1 ./internal/app/ -run 'TestMapDrive|TestService'
```

  Expected: PASS everywhere. The census tests in `reconcile_test.go` that seed
  `RelaunchReserved`/`PriorRawRunDir` still pass, because the census is unchanged until Task 3.

- [ ] **Step 6: Mutation probes.** Restore from the backup after each probe.
  1. **Re-adding a launch after a death.** In `driveSlice`'s death arm, insert
     `_, _ = d.proc.Launch(rec.launchRequest(rec.AdmissionToken, nil))` before
     `return halt(&res, CauseSupervisorDied)`. Expected: `TestDeathHaltsSupervisorDied` FAILS
     (launches = 2). Run
     `go test -count=1 ./internal/gatedrive/ -run TestDeathHaltsSupervisorDied` to confirm.
  2. **Halting an unproven death as `supervisor-died`.** Change `return halt(&res, "uncertain-ownership")`
     to `return halt(&res, CauseSupervisorDied)`. Expected: `TestDeathUnprovenHaltsUncertainOwnership`
     FAILS.
  3. **Re-taking the lock as the relaunch did.** In the death arm, insert
     `if _, kerr := d.lockWorktree(rec.Cwd); kerr != nil { return halt(&res, "worktree-busy") }`
     before the supervisor-died return. Expected: the
     `TestDeathHaltsSupervisorDied/another_gate_took_the_worktree` subtest FAILS. Without
     `holdOther`, the other subtests also fail on `worktreeFree`, because the re-taken lock is
     leaked.

- [ ] **Step 7: Commit.**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/drive.go internal/gatedrive/worktree_lock.go \
  internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go \
  internal/gatedrive/driver_launch_claim_test.go internal/gatedrive/worktree_lock_test.go \
  internal/gatedrive/store_test.go internal/gatedrive/supervisor_integration_test.go \
  internal/repoguard/gatelaunch_admission_test.go
git commit -m "feat(0493): a supervisor death always halts supervisor-died; delete the gate relaunch"
```

---

### Task 3: The launch census handles at most one launch

**Risk:** premium. `run.cancel` and the keyed `run.verdict` closeout both rest on this census. A
wrong settle could account a live supervisor, or strand a run.

**Files:**
- Modify: `internal/gatedrive/reconcile.go`
- Modify: `internal/gatedrive/driver.go`. Delete `errRelaunchRaceLost`.
- Test: `internal/gatedrive/reconcile_test.go`

**Interfaces:**
- Consumes: `stampRecordKeys`, `legacyRelaunchKeys` (Task 2), and the `supervisors` helper in
  `reconcile_test.go`.
- Produces: `var errLaunchStateMoved error`, the census's own CAS-lost sentinel in
  `reconcile.go`. It also produces
  `func (d *Driver) proveRunDirsGone(id string, rec driveRecord, mode censusMode) (bool, string)`,
  which now proves `RawRunDir` only.

- [ ] **Step 1: Write the failing census test.** In `reconcile_test.go`, replace
  `TestCensusChecksPriorRunDir` with the following:

```go
// TestCensusProvesOnlyRawRunDir (change 0493): a drive has at most one launch,
// so the census proves only RawRunDir. A HALTED drive whose recorded run dir is
// live is stopped by cancel (never probed for a leftover once stopped) and
// reported run-live by the verdict; a prior_raw_run_dir an old record still
// carries is never observed. An old nonterminal record still journaling a
// relaunch reservation is proven through RawRunDir, and its relaunch token is
// never resolved.
func TestCensusProvesOnlyRawRunDir(t *testing.T) {
	seedHalted := func(t *testing.T, store *Store, raw, prior string) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = raw
			r.LastOutcome = HALTED
			r.LastCause = CauseSupervisorDied
		})
		stampRecordKeys(t, store, id, map[string]any{"prior_raw_run_dir": prior, "relaunch_count": 1})
		return id
	}
	t.Run("cancel-stops-raw-only", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		raw, prior := liveRunDir(t, "run1"), liveRunDir(t, "run0")
		sup.state[raw], sup.state[prior] = process.StateRunning, process.StateRunning
		id := seedHalted(t, store, raw, prior)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("the live RawRunDir must be stopped and accounted, got %+v", report)
		}
		if len(sup.stopped) != 1 || sup.stopped[0] != raw {
			t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, raw)
		}
		if containsString(sup.observed, prior) {
			t.Fatalf("an old prior_raw_run_dir must never be probed; observed %v", sup.observed)
		}
		if containsString(sup.probed, raw) {
			t.Fatalf("a dir the census stopped is never probed for a leftover; probed %v", sup.probed)
		}
	})
	t.Run("verdict-run-live", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		raw, prior := liveRunDir(t, "run1"), liveRunDir(t, "run0")
		sup.state[raw], sup.state[prior] = process.StateRunning, process.StateRunning
		id := seedHalted(t, store, raw, prior)

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "run-live", id) {
			t.Fatalf("verdict must report the live RawRunDir run-live, got %+v", report)
		}
		if proc.stopN != 0 || containsString(sup.observed, prior) {
			t.Fatalf("verdict stops nothing and never probes prior_raw_run_dir: stops=%d observed=%v", proc.stopN, sup.observed)
		}
	})
	t.Run("old-reserved-relaunch-record", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		raw := liveRunDir(t, "run1")
		sup.state[raw] = process.StateRunning
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = raw
			r.AdmissionToken = censusAdmissionToken
		})
		stampRecordKeys(t, store, id, legacyRelaunchKeys())

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("an old reserved record is proven through RawRunDir, got %+v", report)
		}
		if proc.resolveN != 0 {
			t.Fatalf("an old relaunch token is never resolved, ResolveReservation called %d times", proc.resolveN)
		}
	})
}
```

- [ ] **Step 2: Run it and watch it fail.**
  Run `go test -count=1 ./internal/gatedrive/ -run TestCensusProvesOnlyRawRunDir`. Expected:
  - `cancel-stops-raw-only` FAILS, because two dirs are stopped and the prior dir is observed;
  - `old-reserved-relaunch-record` FAILS, because the relaunch token is resolved (`resolveN` is 1)
    and the drive is settled `run-cancelled` with no `replacement-stopped` finding;
  - `verdict-run-live` already PASSES, which is expected. The old loop returns at the first dir
    that is not gone, so the prior dir was never reached in verdict mode. The subtest pins that
    the verdict stops nothing.

- [ ] **Step 3: Rewrite the census.** In `internal/gatedrive/reconcile.go`:
  1. **Package doc comment.**
     - Delete ", and leaves a live supervisor run-live and a reserved relaunch launch-pending"
       and put ", and leaves a live supervisor run-live" in its place.
     - Replace "For each run dir a drive records (RawRunDir, PriorRawRunDir) a dir that no longer
       exists" with "For the one run dir a drive records (RawRunDir; a drive never relaunches since
       change 0493) a dir that no longer exists".
     - Replace "its run dirs are proven too" with "its run dir is proven too".
  2. **`censusVerdict` comment.** Replace "It settles only a proven never-launched FIRST launch,
     HALTED launch-abandoned; a running supervisor is run-live, and a reserved relaunch stays
     launch-pending (change 0493 retires finalize's relaunch)." with "It settles only a proven
     never-launched launch, HALTED launch-abandoned; a running supervisor is run-live."
  3. **`ReconcileRunLaunches` doc.** Replace "a first launch or reserved relaunch that never
     attached is resolved through its exact reservation token" with "a launch that never attached
     is resolved through its exact launch token".
  4. **`VerdictRunLaunches` doc.** Replace "a live supervisor is run-live and a reserved relaunch
     stays launch-pending, and both keep the report unaccounted." with "a live supervisor is
     run-live and keeps the report unaccounted."
  5. **`reconcileRunDrive`.** Delete the whole `if cur.RelaunchReserved { … }` block and the
     comment above it. Rewrite the nonterminal bullet of its doc to: "Nonterminal: the claimant
     flock is tried nonblocking (busy → claim-busy), the record re-read under it, and then a launch
     that never attached resolves its launch (admission) token, and an attached drive proves its
     run dir gone." In the in-body `busy` comment, replace "or a relaunch reservation resolving"
     with nothing, so that it reads "(StartAdmitted across launch/attach)".
  6. **`reconcileHaltedDrive` doc.** Replace "proves its recorded run dirs gone" with "proves its
     recorded run dir gone". Leave the body unchanged.
  7. **Delete** `reconcileReservation` and `settleNeverLaunchedCancelled`, with their doc
     comments.
  8. **`reconcileFirstLaunch` doc.** Replace "the way reconcileReservation resolves a reserved
     relaunch, through the drive's admission token" with "through the drive's admission token".
  9. **`settleNeverLaunchedFirstLaunch`.** Use the doc and body below:

```go
// settleNeverLaunchedFirstLaunch settles a launch that provably never ran, under
// the held per-drive claim, HALTED with the census mode's cause — "run-cancelled"
// in cancel mode, "launch-abandoned" in verdict mode (change 0491). Its CAS guard
// is that the drive still has no attached run dir; a record that moved on
// (errLaunchStateMoved) or a store fault stays pending, and an already-terminal
// record is accounted. A delayed StartAdmitted then re-reads a terminal record
// under the claim and refuses rather than launching.
func (d *Driver) settleNeverLaunchedFirstLaunch(id, ownerGen, cause string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, cause, func(r *driveRecord) bool {
		return r.RawRunDir == ""
	})
}
```

  10. **`settleNeverLaunched`.** Change `return errRelaunchRaceLost` to `return errLaunchStateMoved`.
      Add this sentinel directly above `settleNeverLaunched`:

```go
// errLaunchStateMoved is the census's CAS-lost sentinel: the drive it was about
// to settle as never launched attached a run dir after the census read it, so
// the settle is abandoned and the drive stays pending (resolution-unresolved).
// It never escapes the census.
var errLaunchStateMoved = errors.New("gatedrive: drive launch state moved under the census")
```

  11. **`proveRunDirsGone`.** Replace both `proveRunDirsGone` and `censusFindingRank` with the
      function below. Then drop the now-unused `"strings"` import; the compiler confirms it.

```go
// proveRunDirsGone applies the lock model's teardown proof to the drive's one
// recorded run dir (RawRunDir; a drive never relaunches since change 0493): the
// drive is torn down when that supervisor is gone. A drive with no recorded run
// dir names no supervisor and is settled.
func (d *Driver) proveRunDirsGone(id string, rec driveRecord, mode censusMode) (bool, string) {
	if rec.RawRunDir == "" {
		return true, ""
	}
	return d.supervisorGone(id, rec.RawRunDir, mode)
}
```

  In `internal/gatedrive/driver.go`, delete `errRelaunchRaceLost` and its comment.
  `git grep -n errRelaunchRaceLost` must return nothing.

- [ ] **Step 4: Remove the census tests that asserted relaunch behaviour.** In `reconcile_test.go`:
  1. Delete `TestCensusReservedRelaunchResolvesRelaunchToken`. Its subject, the reserved-relaunch
     branch, is gone, and `old-reserved-relaunch-record` above pins that the token is never
     resolved.
  2. Delete `TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow`. There is no
     relaunch recovery left to foreclose. The first-launch analog stays pinned by
     `TestBarrierCancelBetweenAdmitAndStartAdmitted` and `TestStartAdmittedRefusesAfterVerdictSettle`.
  3. Delete `TestCensusLeftoverOutranksOtherFindings`. Ranking across two run dirs has no subject
     with one dir. Its "census-stopped dir is never probed" property moved into
     `TestCensusProvesOnlyRawRunDir/cancel-stops-raw-only`.
  4. In `TestCensusReportsLeftoverOfDeadSupervisor`, change the seeded fixture value
     `r.LastCause = "relaunch-exhausted"` to `r.LastCause = CauseSupervisorDied`. This is a value
     no assertion reads; the HALTED drive a killed supervisor's slice leaves is now
     `supervisor-died`. Every assertion is unchanged.

- [ ] **Step 5: Run the census, the cancel paths, and their app consumers.**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/
go test -tags integration -count=1 -run '^TestIntegrationGatedrive' ./internal/gatedrive/
go test -tags integration -count=1 -timeout 30m -run '^TestIntegrationRunCancel|^TestIntegrationRunCompletion|^TestIntegrationRunVerdict' ./internal/app/
```

  Expected: PASS. In particular every first-launch census test (`TestCensusSettlesNeverAttachedFirstLaunch`,
  `TestCensusResolvesLaunchFailedFirstLaunch`, `TestCensusMissingRunRootIsNeverLaunched`, …)
  passes **unmodified**.

- [ ] **Step 6: Mutation probes.** Restore from the backup after each probe.
  1. **Re-adding the prior-dir leg.** `driveRecord.PriorRawRunDir` still exists until Task 4, and
     the stamped `prior_raw_run_dir` key decodes into it. In `proveRunDirsGone`, add before the
     return: `if rec.PriorRawRunDir != "" { if gone, f := d.supervisorGone(id, rec.PriorRawRunDir, mode); !gone { return false, f } }`.
     Expected: `TestCensusProvesOnlyRawRunDir/cancel-stops-raw-only` FAILS, because the prior dir
     is observed and stopped.
  2. **Restoring the reserved branch.** In `reconcileRunDrive`, insert
     `if cur.RelaunchReserved { _, _ = d.proc.ResolveReservation(cur.RunRoot, cur.RelaunchToken) }`
     before the `RawRunDir == ""` check. Expected: `old-reserved-relaunch-record` FAILS.

- [ ] **Step 7: Commit.**

```bash
git add internal/gatedrive/reconcile.go internal/gatedrive/driver.go internal/gatedrive/reconcile_test.go
git commit -m "feat(0493): the launch census proves only RawRunDir and has no relaunch branch"
```

---

### Task 4: Delete the relaunch record fields and the `IdempotentSuiteGate` opt-in surface

**Risk:** standard. This is a mechanical, compiler-checked removal across `gatedrive`, `app`, and
`cli`, plus one compatibility test. Old records still load because unknown JSON keys are
ignored.

**Files:**
- Modify: `internal/gatedrive/drive.go`, `internal/gatedrive/driver.go`, `internal/gatedrive/store.go` (a comment only)
- Modify: `internal/app/gate_drive.go`, `internal/app/finalize_rebase.go`
- Modify: `internal/cli/gate.go`
- Test (pin): `internal/cli/capability_production_test.go`
- Test (new): `internal/cli/gate_test.go`, `internal/gatedrive/store_test.go`
- Test (incidental, field dropped and assertions kept):
  - `internal/gatedrive/driver_test.go`, `drive_test.go`, `store_test.go`,
    `supervisor_integration_test.go`, `sequence_integration_test.go`;
  - `internal/app/gate_drive_test.go`, `runtracker_production_census_integration_test.go`,
    `runtracker_verdict_integration_test.go`.

**Interfaces:**
- Consumes: `stampRecordKeys`, `legacyRelaunchKeys` (Task 2).
- Produces: `driveRecord` without `IdempotentSuiteGate`, `RelaunchCount`, `RelaunchReserved`,
  `RelaunchToken`, or `PriorRawRunDir`. `gatedrive.StartRequest` and `app.GateDriveStartRequest`
  lose `IdempotentSuiteGate`. `gate drive start` loses `--idempotent-suite-gate`.

- [ ] **Step 1: Write the failing tests.** Add the following to `internal/gatedrive/store_test.go`:

```go
// TestOldRelaunchFieldsLoadAndAreDroppedOnWrite (change 0493, spec Design §5): a
// v4 record still carrying the retired relaunch fields loads without error (the
// store decodes with plain json.Unmarshal, which ignores unknown keys), and its
// next write omits every one of them.
func TestOldRelaunchFieldsLoadAndAreDroppedOnWrite(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	id, gen, err := store.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	stampRecordKeys(t, store, id, legacyRelaunchKeys())

	got, err := store.Load(id)
	if err != nil {
		t.Fatalf("a record carrying the retired relaunch fields must load: %v", err)
	}
	if got.RawRunDir != sampleRecord().RawRunDir {
		t.Fatalf("loaded RawRunDir = %q, want %q", got.RawRunDir, sampleRecord().RawRunDir)
	}
	if _, err := store.CAS(id, gen, func(*driveRecord) error { return nil }); err != nil {
		t.Fatalf("CAS over an old record: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(store.root, id, recordFileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env struct {
		Record map[string]any `json:"record"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Record) == 0 {
		t.Fatal("non-vacuity: the rewritten record decoded empty")
	}
	for k := range legacyRelaunchKeys() {
		if _, ok := env.Record[k]; ok {
			t.Fatalf("the next write must drop the retired key %q, record has it", k)
		}
	}
}
```

  Add the following to `internal/cli/gate_test.go`, after `TestGateDriveStartRejectsRunIDFlag`:

```go
// TestGateDriveStartRejectsIdempotentSuiteGateFlag (change 0493): gate drive
// start no longer takes --idempotent-suite-gate — no drive relaunches — so a
// hand-typed use fails on an unknown flag (exit 2) instead of being accepted.
func TestGateDriveStartRejectsIdempotentSuiteGateFlag(t *testing.T) {
	_, errS, code := runCLI(t, "gate", "drive", "start", "--owner", "finalize",
		"--run-root", "/tmp/docket-0493-run-root", "--idempotent-suite-gate")
	if code != 2 || !strings.Contains(errS, "unknown flag: --idempotent-suite-gate") {
		t.Fatalf("exit %d stderr %q, want exit 2 naming the unknown --idempotent-suite-gate flag", code, errS)
	}
}
```

- [ ] **Step 2: Run them and watch them fail.**
  Run `go test -count=1 ./internal/gatedrive/ -run TestOldRelaunchFieldsLoadAndAreDroppedOnWrite`.
  Expected: FAIL, because the write still emits `relaunch_count`, `relaunch_reserved`, and the
  other keys.
  Then run `go test -count=1 ./internal/cli/ -run TestGateDriveStartRejectsIdempotentSuiteGateFlag`.
  Expected: FAIL, because the flag is still accepted.

- [ ] **Step 3: Remove the fields and the surface.**
  1. **`internal/gatedrive/drive.go`.**
     - Delete the `IdempotentSuiteGate`, `RelaunchCount`, `RelaunchReserved`, `RelaunchToken`, and
       `PriorRawRunDir` fields and their comments from `driveRecord`.
     - In the `RunRoot` field comment, replace "It is the deterministic launch input the one
       admitted relaunch replays, so it is persisted" with "It is part of the deterministic launch
       input, so it is persisted".
     - In the `RawRunDir` group comment, replace "Current raw run dir + raw ownership identity +
       attempt + relaunch count + terminal receipt." with "Current raw run dir + raw ownership
       identity + attempt (always 1: a drive never relaunches, change 0493) + terminal receipt."
     - In the `driveRecord` doc comment, replace "attempt + relaunch count + terminal receipt"
       with "attempt + terminal receipt".
     - In the `driveSchemaVersion` comment, replace the sentence that begins "Bumped to 4 by change
       0375 Task 5" and runs to "older records still fail closed." with: "Bumped to 4 by change
       0375 Task 5, which added a relaunch-reservation journal. Change 0493 retired the relaunch
       and stopped writing its fields (idempotent_suite_gate, relaunch_count, relaunch_reserved,
       relaunch_token, prior_raw_run_dir) without a version bump: a record that still carries them
       loads with them ignored, and its next write drops them. A v3 record still LOADS (see
       driveSchemaVersionLegacy in readStored) and the next write stamps it forward to v4; older
       records still fail closed."
     - Replace the `driveSchemaVersionLegacy` comment's second sentence with "A v3 record carries
       every field a v4 reader needs."
     - In the `DriveDoc.RunRoot` comment, replace "a live drive may still relaunch under it, so a
       WAITING consumer must retain it" with "a live drive's run still writes under it, so a
       WAITING consumer must retain it".
  2. **`internal/gatedrive/driver.go`.**
     - In the `StartRequest` doc comment, replace "the launch environment/config provenance the
       record needs, and whether the gate is an idempotent suite gate eligible for the single
       relaunch." with "and the launch environment/config provenance the record needs."
     - Delete the `IdempotentSuiteGate` field and its comment.
     - In `Admit`, delete the `IdempotentSuiteGate: req.IdempotentSuiteGate,` line, then gofmt.
  3. **`internal/gatedrive/store.go`.** Replace the `readStored` comment sentence "A v3 record
     reads with RelaunchReserved false and an empty RelaunchToken, then upgrades to v4 on its next
     write (CAS re-stamps SchemaVersion), so a live drive survives the reservation-journal bump."
     with "A v3 record loads unchanged and upgrades to v4 on its next write (CAS re-stamps
     SchemaVersion)."
  4. **`internal/app/gate_drive.go`.** Delete `GateDriveStartRequest.IdempotentSuiteGate` and its
     mapping line in the request-to-`gatedrive.StartRequest` conversion. Run gofmt.
  5. **`internal/app/finalize_rebase.go`.**
     - In `RunLocalGate`, delete `IdempotentSuiteGate: true,`.
     - In `mapDriveOutcome`, replace the WAITING comment "Nonterminal: the run is still live and
       may relaunch under the run root, so the root MUST be retained." with "Nonterminal: the run
       is still live and writes under the run root, so the root MUST be retained."
  6. **`internal/cli/gate.go`.** Delete the `idempotent, _ := c.Flags().GetBool("idempotent-suite-gate")`
     line, the `IdempotentSuiteGate: idempotent,` field, and the
     `start.Flags().Bool("idempotent-suite-gate", …)` registration.
  7. **`internal/cli/capability_production_test.go` (the pin).** In the `"gate.drive.start"`
     signature, delete ` [--idempotent-suite-gate]` and leave the rest byte-identical. Add
     `// change 0493: --idempotent-suite-gate is gone (no drive relaunches).` to the comment above
     the line.

- [ ] **Step 4: Drop the removed field from the tests that used it incidentally.** Every other
  assertion stays byte-identical.
  1. **`internal/gatedrive/driver_test.go`.**
     - Delete `IdempotentSuiteGate: true,` from `sampleStart` and from `seedRecord`. In
       `sampleStart`'s comment, replace "for an idempotent suite gate" with "for a suite gate".
     - In `TestSeveralWaitingSlicesRetainDriveIdentity`, the condition
       `rec.RelaunchCount != 0 || rec.Attempt != 1` becomes `rec.Attempt != 1`, with the message
       `"identity drifted across slices: attempt=%d", rec.Attempt`.
     - In `TestVanishedProvenGoneWithoutStop` and
       `TestSignaledDeathConsumesTerminalViaStopNoOpAndReObserve`, delete the
       `rec.IdempotentSuiteGate = …` lines. In `TestVanishedProvenGoneWithoutStop`'s doc,
       replace "With relaunch refused it HALTs." with "It HALTs (a drive never relaunches)."
     - In its fatal message, replace "a vanished run with relaunch refused must HALT" with "a
       vanished run must HALT".
     - In `TestDeathHaltsSupervisorDied` (Task 2), delete the `idempotent` table column, the
       `req.IdempotentSuiteGate = tc.idempotent` line, and the column's values from each row.
  2. **`internal/gatedrive/drive_test.go`.** In `TestDriveSchemaV3LoadsAndUpgradesUnderV4`:
     - Delete the two `RelaunchReserved`/`RelaunchToken` assertions; they read removed fields.
     - Replace the doc comment's "(its missing RelaunchReserved reads false)" with nothing.
     - Replace the in-body comment "carrying no relaunch reservation or token — the shape persisted
       before the relaunch-reservation journal" with "— the shape persisted before schema v4".
     - Replace "A v3 record loads with RelaunchReserved false (not a fail-closed HALT)." with "A v3
       record loads (not a fail-closed HALT)."
     - Keep the load and upgrade assertions.
  3. **`internal/gatedrive/store_test.go`.** Delete `RelaunchCount: 0,` from `sampleRecord`.
  4. **`internal/gatedrive/supervisor_integration_test.go`.** Delete `IdempotentSuiteGate: true,`
     from `intStartRequest`.
  5. **`internal/gatedrive/sequence_integration_test.go`.** Delete `IdempotentSuiteGate: true,` from
     the sequence start request.
  6. **`internal/app/gate_drive_test.go`.** In `TestServiceStartInjectsAuthoritativeConfig`:
     - delete `IdempotentSuiteGate: true,`;
     - change the condition `eng.lastStart.ChangeID != "0342" || !eng.lastStart.IdempotentSuiteGate`
       to `eng.lastStart.ChangeID != "0342"`;
     - keep the message.
  7. **`internal/app/runtracker_production_census_integration_test.go`.** Remove the
     `IdempotentSuiteGate: true,` field (and `RunRoot: runRoot, IdempotentSuiteGate: true,`
     becomes `RunRoot: runRoot,`) from `startFinalizeGate` and from
     `TestIntegrationRunCompletionProductionCensusCancelResumeStartsReplacementGate`.
  8. **`internal/app/runtracker_verdict_integration_test.go`.** In
     `TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates`, change
     `RunContext: runContext, IdempotentSuiteGate: true,` to `RunContext: runContext,`.

  Then re-derive the sites and confirm that none remain:

```bash
git grep -n -E -e 'IdempotentSuiteGate|idempotent-suite-gate|RelaunchCount|RelaunchReserved|RelaunchToken|PriorRawRunDir' -- internal cmd
```

  Expected: the only hits are in `internal/gatedrive/history.go` (`PriorRawRunDir` on the frozen
  schema-2 reader, untouched). Fixture keys under `internal/gatedrive/testdata/**` and the
  `legacyRelaunchKeys` map are snake_case and do not match this pattern.

- [ ] **Step 5: Run the tests and check that they pass.**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/ ./internal/cli/ ./internal/app/
go test -tags integration -count=1 -run '^TestIntegrationGatedrive' ./internal/gatedrive/
go test -tags integration -count=1 -timeout 30m -run '^TestIntegrationRunCompletion|^TestIntegrationRunVerdict' ./internal/app/
```

  Expected: PASS, including `TestRepresentativeSignatures` and both new tests.

- [ ] **Step 6: Mutation probes.** Restore from the backup after each probe.
  1. **A strict decode.** In `readStored` (`store.go`), replace
     `json.Unmarshal(buf, &stored)` with
     `func() error { dec := json.NewDecoder(bytes.NewReader(buf)); dec.DisallowUnknownFields(); return dec.Decode(&stored) }()`,
     adding a `bytes` import. Expected: `TestOldRelaunchFieldsLoadAndAreDroppedOnWrite` FAILS.
  2. **Still writing a field.** Re-add a `RelaunchReserved bool` field to `driveRecord`, with the
     JSON tag `relaunch_reserved`. Expected: the same test FAILS, because the key is written back.
  3. **Re-registering the flag.** Re-add the flag registration in `gate.go`. Expected:
     `TestGateDriveStartRejectsIdempotentSuiteGateFlag` and `TestRepresentativeSignatures` FAIL.

- [ ] **Step 7: Commit.** Stage the explicit list of every file edited in Steps 3–4, never `git add -A`.

```bash
git commit -m "feat(0493): delete the relaunch record fields and the --idempotent-suite-gate opt-in"
```

---

### Task 5: Rename the per-drive claim's Go identifiers; keep `relaunch.lock` on disk

**Risk:** economy. This is a compiler-checked rename plus one pin test.

**Files:**
- Modify: `internal/gatedrive/store.go`, `internal/gatedrive/driver.go`, `internal/gatedrive/reconcile.go`
- Test: `internal/gatedrive/store_test.go` (new pin), plus a mechanical rename in
  `driver_launch_claim_test.go` and `reconcile_test.go`.

**Interfaces:**
- Produces:
  - `type driveClaim struct{ lock *os.File }` (the unused `token` field is deleted);
  - `func (c *driveClaim) close()`;
  - `func (s *Store) tryDriveClaim(id string) (*driveClaim, bool, error)`;
  - `const driveClaimLockFileName = "relaunch.lock"`.

- [ ] **Step 1: Write the pin test.** Add the following to `internal/gatedrive/store_test.go`:

```go
// TestDriveClaimKeepsRelaunchLockFileName (change 0493): the per-drive claim is
// still the relaunch.lock file on disk, so a pre-0493 CLI still running across
// the upgrade and a new one contend on the same flock.
func TestDriveClaimKeepsRelaunchLockFileName(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	id, _, err := store.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	c, busy, err := store.tryDriveClaim(id)
	if err != nil || busy {
		t.Fatalf("tryDriveClaim = busy %v err %v, want a free claim", busy, err)
	}
	defer c.close()
	path := filepath.Join(store.root, id, "relaunch.lock")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the per-drive claim must be created at %s: %v", path, err)
	}
	f, busy, err := tryAcquireExclusiveLock(path)
	if f != nil {
		f.Close()
	}
	if err != nil || !busy {
		t.Fatalf("an independent flock on relaunch.lock must be refused while the claim is held: busy %v err %v", busy, err)
	}
}
```

- [ ] **Step 2: Run it and watch it fail.** Run
  `go test -count=1 ./internal/gatedrive/ -run TestDriveClaimKeepsRelaunchLockFileName`.
  Expected: a compile FAIL, `undefined: tryDriveClaim`.

- [ ] **Step 3: Rename.** Derive the file list from a grep, then rename with word boundaries:

```bash
FILES=$(git grep -l -E -e 'relaunchClaim|tryRelaunchClaim|relaunchLockFileName' -- internal)
perl -pi -e 's/\btryRelaunchClaim\b/tryDriveClaim/g; s/\brelaunchClaim\b/driveClaim/g; s/\brelaunchLockFileName\b/driveClaimLockFileName/g' $FILES
gofmt -l internal/gatedrive
```

  Then, in `store.go`:
  - delete the `token string` field from `driveClaim`;
  - replace the constant's comment with:

```go
	// driveClaimLockFileName is the per-drive claim: a short-lived flock that a
	// launcher (StartAdmitted, across launch and attach) and the launch census take
	// nonblocking, so neither mistakes the other's in-flight work for a crash. The
	// file keeps its pre-0493 name, relaunch.lock: a CLI still running an older
	// binary across the upgrade opens that path, and a renamed file would let an
	// old and a new process each hold "the" claim (change 0493).
	driveClaimLockFileName = "relaunch.lock"
```

  Next, sweep comments: `git grep -n -i -e 'relaunch claim' -e 'relaunchclaim' -- internal`.
  Reword any remaining comment that calls this lock "the relaunch claim" to "the per-drive claim".
  Leave prose in `docs/**` alone.

- [ ] **Step 4: Run the tests and check that they pass.**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/
```

  Expected: PASS.

- [ ] **Step 5: Mutation probe.** Back up `store.go` and set `driveClaimLockFileName = "drive.lock"`.
  Expected: `TestDriveClaimKeepsRelaunchLockFileName` FAILS. Restore from the backup.

- [ ] **Step 6: Commit.** Stage only `$FILES` plus `internal/gatedrive/store_test.go`:

```bash
git commit -m "refactor(0493): rename the per-drive claim; its file stays relaunch.lock"
```

---

### Task 6: Prose, budgets, and the embedded mirror

**Risk:** standard. This is prose bound by budget and mirror drift guards.

**Files:**
- Modify: `skills/docket-build/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `docs/reference/glossary.md`, `docs/concepts/run-tracker.md`
- Modify (pin): `internal/repoguard/budgets_test.go`, the two rows only
- Regenerate (never hand-edit): `internal/assets/embedded/**` with `go generate ./internal/assets`

**Interfaces:** none (prose).

- [ ] **Step 1: Edit `skills/docket-build/SKILL.md`.** In the *Keying on the disposition*
  paragraph, replace this text:

```
malformed observation is `HALTED`, **not** a red suite and it **never** mints repair work. The one
bounded relaunch of a proven-dead **idempotent** suite gate, under the original deadline, is the
driver's own — the caller never relaunches, stops a raw run, or composes the raw verbs; a
non-idempotent gate earns no relaunch.
```

  with:

```
malformed observation is `HALTED`, **not** a red suite and it **never** mints repair work. The
caller never relaunches, stops a raw run, or composes the raw verbs; a gate whose supervisor dies
halts `supervisor-died`, never relaunched.
```

- [ ] **Step 2: Edit `skills/docket-finalize-change/references/gate-failure.md`.** In *The finalize
  gate shares the worktree's one lock*, replace this text:

```
gate's supervisor holds the lock — reason `worktree-busy` — and its single automatic relaunch
halts `worktree-busy` instead of relaunching when another gate took the lock first. This is a
```

  with:

```
gate's supervisor holds the lock — reason `worktree-busy`. This is a
```

  Then, after "The worktree frees itself when the holder ends." at the end of that paragraph, add
  this sentence:
  "A gate whose own supervisor dies mid-run is never relaunched: it halts `supervisor-died`
  (finalize reports `gate-halted`), and the remedy is to re-run finalize, which re-runs the
  suite." Re-wrap the paragraph to the file's line width.

- [ ] **Step 3: Edit `docs/reference/glossary.md`.**
  1. **The `### \`worktree-busy\`` entry.** Delete the sentence "A gate-drive relaunch that finds
     the worktree lock held by another gate HALTs with cause `worktree-busy` instead of relaunching
     over it." Replace the sentence "`launch-unconfirmed` survives only as a gate-drive HALT cause
     (a relaunch whose launch could not be established), never as an admission refusal." with
     "`launch-unconfirmed` is retired: it was the gate-drive HALT cause for a relaunch whose launch
     could not be established, and no driver path emits it since change 0493; it was never an
     admission refusal."
  2. **The `### Drive disposition: WAITING / PASSED / FAILED / HALTED` entry.** After the sentence
     that ends "bad state, or a process death.", add: "A process death halts `supervisor-died` (or
     `uncertain-ownership` when the death cannot be proven) and is never relaunched: a human
     re-runs the workflow, which re-runs the suite."

- [ ] **Step 4: Edit `docs/concepts/run-tracker.md`.** In *The worktree lock*, replace
  "— a drive's first launch, its single relaunch, and a raw `gate launch` —" with
  "— a drive's one launch and a raw `gate launch` —". Re-wrap only that paragraph.

- [ ] **Step 5: Re-baseline the two budget rows and see them stay green.**

```bash
wc -l -w skills/docket-build/SKILL.md skills/docket-finalize-change/references/gate-failure.md
```

  In `internal/repoguard/budgets_test.go`, set the `"docket-build/SKILL.md"` and
  `"docket-finalize-change/references/gate-failure.md"` rows to the **exact** new
  `lines, words` counts. Prepend each row's trailing comment with
  `0493: the gate relaunch is retired (<old l>/<old w> -> <new l>/<new w>); `.
  The old values are `404/4023` and `145/1892`.

- [ ] **Step 6: Regenerate the mirror and run the guards.**

```bash
go generate ./internal/assets
go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/repoguard/ ./internal/assets/
```

  Expected:
  - `genassets` rewrites `internal/assets/embedded/tree/skills/docket-build/SKILL.md`,
    `…/docket-finalize-change/references/gate-failure.md`, and `manifest.json`;
  - `-check` passes;
  - repoguard and assets PASS, including `TestSkillSizeBudgets`, `TestEmbeddedMatchesAuthored`, the
    retired-vocabulary guard, and `TestCommittedCodexDispatchMatchesGenerator`.

  `AGENTS.md` is not touched, because these skill bodies are not in the dispatch block. If that
  dispatch drift test fails, stop and return BLOCKED; do not hand-edit `AGENTS.md`.

- [ ] **Step 7: Residue sweep.** Classify every hit as prose or executable.

```bash
git grep -n -i -E -e 'relaunch|idempotent.suite' -- skills cursor-rules agents docs/reference docs/concepts internal cmd \
  ':!internal/gatedrive/history.go' ':!internal/gatedrive/testdata' ':!internal/assets/embedded'
```

  The allowed survivors are:
  - comments and test messages that state the **absence** of a relaunch ("never relaunches",
    "must not relaunch", "earns no relaunch", "no relaunch");
  - `driveClaimLockFileName`'s `relaunch.lock` value and its comment;
  - `legacyRelaunchKeys` and the old-record tests;
  - the run-tracker's unrelated "deadline/relaunch/budget/retry state" wording in
    `internal/app/runtracker_cancel.go` and `internal/cli/run.go`, which is about re-dispatch, not
    gate relaunch;
  - fixture strings in `internal/repoguard/retired_vocabulary_test.go`.

  Any other hit that describes a relaunch as **existing** must be fixed in this task.

- [ ] **Step 8: Commit.**

```bash
git add skills/docket-build/SKILL.md skills/docket-finalize-change/references/gate-failure.md \
  docs/reference/glossary.md docs/concepts/run-tracker.md internal/repoguard/budgets_test.go \
  internal/assets/embedded
git commit -m "docs(0493): retire the gate relaunch in the skills, glossary, and concepts; re-baseline budgets"
```

---

## Coordinator notes (not build tasks)

- **ADR.** After the build, record "Gate drives never relaunch automatically" through `docket-adr`
  (`relates_to: [87, 98, 107, 132]`; it supersedes or reverses none). Add the dated `## Update`
  note to ADR-0132, as spec Design §7 describes.
- **Results file, *Human actions and testing*.** Carry the spec's *Rollout* pre-install check for
  each repo that uses docket:
  `grep -lE '"relaunch_reserved": ?true' "$(git rev-parse --git-common-dir)"/docket/gate-drives/v2/*/record.json`.
  No output means there is nothing to handle.
- **Spec-deviation record.** Note the planning findings above in the results file, in particular
  that `launch-pending` lost its only emitter (deviation 1).

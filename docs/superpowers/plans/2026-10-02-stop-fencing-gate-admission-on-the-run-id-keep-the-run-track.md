<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0491 — Retire the run id; the run key becomes the run tracker's only handle](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0491-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md)**
<!-- docket:backlink:end -->
# Retire the run id; the run key becomes the run tracker's only handle — Implementation Plan

> **For agentic workers:** this plan is executed by **docket-build**: one tier-routed worker per
> task, strictly sequential in the feature worktree, one commit per task, no per-task review, and a
> single full-suite gate at the end. Each task names its **Risk** (the routing tier). Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the run id from the run tracker (the run key becomes its only handle), delete the
run launch check that refused gate starts on it, and let the keyed `run.verdict` close a proven
never-launched drive (HALTED `launch-abandoned`), with a missing run root read as never launched.

**Architecture:** Four code layers move in dependency order. (1) `internal/gatedrive` loses the
`RunLaunchGate` seam, and its launch census gains a second, stop-free mode (`censusVerdict`) that
settles a proven never-launched first launch; `ObserveRunLaunches` is folded into it because the
closeout was its only caller. (2) `internal/app` wires that mode into both keyed-verdict paths and
removes every run-id read: `run.cancel`, `agent.enter`, the death guardian, the participant APIs,
the id scan, and finally the `RunRecord.RunID` field and the three-token `run-started` line. (3)
The refusal tokens are renamed: `stale-run-id` becomes `run-superseded`, `ErrRunIDMismatch` becomes
`run-record-conflict`, and `unknown-run-id` is retired. (4) Prose, the generated mirror, the
`AGENTS.md` block, the glossary, and the retired-vocabulary seal follow.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), Cobra CLI, `go test` (with
`-tags integration` for real-git/process tests), the Go suite runner (`go run ./cmd/docket
development test`).

**Spec:** `docs/superpowers/specs/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-design.md`
(on the `docket` metadata branch; the synchronized copy is under `.docket/` in the primary
checkout). Executors read the spec beside this plan. Change file:
`docs/changes/active/0491-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md`.

## Global Constraints

- **Never touch the gate supervisor's own run id.** It has the same 32-hex shape and is a different
  identity. It lives in `internal/process/**`, `GateResult.RunID` / `RecoveryEntry.RunID`
  (`internal/app/gate.go`), `gateCleanupReceipt.RunID` (`internal/app/finalize_cleanup.go`),
  `obs.RunID` in `internal/app/run_waiting.go`, `IncumbentSnapshot.RawRunID` and the
  `incumbent-run:<id>` locator (`internal/gatedrive/ownership.go`, `worktree_lock.go`,
  `internal/app/gate_drive.go` `rawRunIDShape`), `process.LaunchOutcome.RunID`,
  `process.ReservationResolution.RunID`, and the `run_id` key in the `gate.launch`, `gate.observe`,
  `gate.recover`, and `gate.stop` results. No edit and no retired-vocabulary row may match these.
- **Hard cut.** No deprecated alias, no hidden flag. A caller still passing `--run-id` to `run
  cancel`, `gate drive start`, or `agent enter` fails on an unknown flag (exit 2).
- **No migration and no storage reset.** `runSchemaVersion` stays `1`. An old `run.json` with a
  `run_id` key decodes unchanged (unknown keys are ignored). Drive records never stored the id.
- **Derive every rename/removal site from a whole-repo grep at the task that does it** (AGENTS.md:
  never hand-list the sites of a literal). Use `git grep -n -E -e '<pattern>'`. Sort hits into
  executable and prose, and into the run tracker's id and the supervisor's id. The site lists in
  this plan are a starting point, not the complete list.
- **Frozen records are never edited:** `docs/superpowers/plans/**`, `docs/superpowers/specs/**`,
  `docs/results/**`, `docs/changes/**`, and the bodies of `docs/adrs/**`.
- **ADR work is out of scope for every build task.** The new ADR, the ADR-0129 in-place row
  amendment, and the Update notes on ADR-0124/0128/0132 are coordinator work on the `docket`
  metadata branch after the build. No task creates or edits an ADR.
- **Never hand-edit the `AGENTS.md` managed block.** Order (Task 8): edit the sources, run `go
  generate ./internal/assets`, run `go run ./cmd/genassets -check`, then regenerate `AGENTS.md`
  through the install path. `CLAUDE.md` is a symlink to `AGENTS.md`.
- **Test commands always defeat the cache:** `-count=1` on every run whose purpose is to observe the
  current tree (learning cached-runner-serves-a-mutated-tree). On `./internal/app/` with
  `-tags integration`, always pass `-run '^<Prefix>'` (the unfiltered corpus exceeds the default
  timeout; add `-timeout 30m` for a deliberately wide filter).
- **Every task leaves the tree buildable in both build modes:** `go build ./... && go vet ./... &&
  go vet -tags integration ./...` must pass before its commit (learning
  intermediate-task-state-buildable).
- **Mutation-test every new guard** (AGENTS.md): strip what it guards, watch it redden, restore it
  from a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --` over uncommitted
  work (learning mutation-restore-needs-a-backup-copy). Confirm each mutation landed with a
  before/after `grep -c`.
- **Go formatting** uses the toolchain's gofmt:
  `"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -w <files…>`.
- **Comments anchor on a symbol or a quoted clause, never a line number** (ADR-0054).
- **Runtime budgets are never raised** (`tests/README.md`). A task that adds integration tests
  measures its shard solo (`time bash tests/test_go_integration_app_<shard>.sh`) and returns
  `BLOCKED` naming the measurement if it exceeds the row's ceiling in `tests/runtime-budgets.tsv`.
- **Prose budgets** (`internal/repoguard/budgets_test.go`): the five ceiling-pinned prose rows
  (docket-build SKILL, gate-caller-loop, implement-next SKILL, edge-paths, gate-failure) may only
  shrink; Task 8 re-pins them at the exact new counts. `dispatchBudget` is a ceiling that must not
  rise.
- **Stage explicit paths only** (`git add <paths>`, never `git add -A`), and use `git -C
  <worktree>` or absolute paths.

## Review Focus

The five input classes the spec implies, but no listed spec test exercises, most likely to bite
first. Each has its pinning test in the owning task.

1. **A run root whose probe fails for a reason other than not-exist** (`ENOTDIR`, `EACCES`) must stay
   `resolution-unresolved` in both census modes and is never settled (learning
   probe-error-is-not-clean-absence). Pinned in Task 2 (`TestCensusRunRootProbeErrorStaysUnresolved`).
2. **A launcher still alive and holding the per-drive claim** (StartAdmitted mid-flight) must read
   `claim-busy` under the keyed verdict, and the verdict must not settle it. Pinned in Task 2
   (`TestCensusVerdictLeavesBusyClaimPending`).
3. **A never-launched drive of another run** (a different run context hash) is never settled by this
   run's keyed verdict. Pinned in Task 2 (`TestCensusVerdictSettlesOnlyItsOwnRun`).
4. **An `--unattributed` verdict stays read-only:** it never settles a never-launched drive. Pinned in
   Task 3 (`TestIntegrationRunVerdictUnattributedNeverSettlesNeverLaunchedDrive`).
5. **A run started by the old binary** (its `run.json` still carries `run_id`) loads, and `run.cancel`
   cancels it by key alone. Pinned in Task 7
   (`TestIntegrationRunCancelOldRecordWithRunIDCancelsByKey`).

---

## Spec coverage map

| Spec item | Task |
|---|---|
| Decision 2 — delete the run launch check (`runLaunchGate`, the gatedrive seam, `settleAdmittedAfterRunRefusal`, `fenceNextAction`, the run-error branches) | 1 |
| Decision 1 — `gate.drive.start` loses `--run-id` | 1 |
| Decision 3 — verdict census mode (`launch-abandoned`, relaunch left pending, observe folded); success path | 2 |
| Decision 4 — missing run root reads as never launched (`reconcileFirstLaunch`) | 2 |
| Decision 3 — run-incomplete path settles before `LocateOuterDrive` | 3 |
| Decision 1 — `run.cancel --key --reason` | 4 |
| Decision 1/6 — `agent.enter --run-key` alone; guardian key-only; id lookups deleted; `expectRunID` params deleted | 5 |
| Decision 5 — `run-id-mismatch` → `run-record-conflict`; `unknown-run-id` retired; classifiers renamed | 5 |
| Decision 5 — `stale-run-id` → `run-superseded` | 6 |
| Decision 1 — `RunRecord.RunID` deleted; `run-started <key> <run-context>`; locators/remedies | 7 |
| Decision 7 — `run.start` declares `process-control` | 7 |
| Decision 8 — old `run.json` still loads; cancel by key | 7 |
| Prose: run-tracker block (+0443 wording), skills, Codex clause, mirror, AGENTS.md, prose guards, prose budgets | 8 |
| Prose: glossary, `docs/concepts/run-tracker.md` | 9 |
| Retired vocabulary rows (re-point 10/11/20/31/32/38b/38d; new 0491 rows; negative controls) | 10 |
| T1, T3 (verdict + cancel + resume), T5 | 2 |
| T2, T5 extension | 3 |
| T4 | 2 |
| T6 | 1, 4, 5, 7 |
| New ADR, ADR-0129 amendment, ADR Update notes, results file | coordinator (not a build task) |

---

### Task 1: Delete the run launch check; `gate.drive.start` carries no run id

**Risk:** premium — deletes an admission fence across three packages and re-points concurrency
tests; consequential, but correctable.

**Files:**
- Modify: `internal/gatedrive/driver.go` (`StartRequest.RunID`, `Driver.runLaunch`, `RunLaunchGate`,
  `SetRunLaunchGate`, `RunLaunchGateWired`, `runLaunchGated`, `AdmissionTicket.runID`, `Admit`,
  `StartAdmitted` comment, `revalidateAdmittedLaunch`, `settleAdmittedAfterRunRefusal`)
- Delete: `internal/app/runtracker_launch_gate.go`
- Modify: `internal/app/gate_drive.go` (`GateDriveStartRequest.RunID`, `startRequest`,
  `newOwnedGateDriveService`, `NewCommandlessGateDriveService`, `startBudgetedBuild` doc,
  `mapDriveResult`, `mapDriveFailure`, delete `fenceNextAction`)
- Modify: `internal/app/runtracker_continuation.go` (`NewContinuationSeam`: drop `SetRunLaunchGate`)
- Modify: `internal/cli/gate.go` (`gate drive start`: drop the `--run-id` flag and `RunID:` field)
- Modify comments that describe the deleted check: `internal/app/runtracker_run_record.go` (header
  "IDENTITY vs. AUTHORITY" paragraph, `bindRunWorktree` doc), `internal/app/runtracker_start.go`
  (step "(6a)" comment bullet about `runLaunchGate`), `internal/app/runtracker_cancel.go` (header
  "once fenced no new participant, start, relaunch …" and the `appLaunchReconciler` doc),
  `tests/test_go_integration_app_concurrency.sh` (header comment naming the launch tests)
- Tests — delete: `internal/gatedrive/run_launch_gate_test.go` (move its `driveRecordCount` helper
  into `internal/gatedrive/driver_test.go` first), `internal/gatedrive/launch_sites_guard_test.go`,
  `internal/app/runtracker_launch_gate_integration_test.go`
- Tests — rename and trim: `git mv internal/gatedrive/driver_runfence_test.go
  internal/gatedrive/driver_launch_claim_test.go`
- Tests — modify: `internal/gatedrive/driver_concurrency_test.go`, `internal/gatedrive/driver_test.go`,
  `internal/gatedrive/old_records_test.go`, `internal/gatedrive/sequence_race_integration_test.go`,
  `internal/app/gate_drive_test.go`, `internal/app/runtracker_production_census_integration_test.go`,
  `internal/app/runtracker_start_resume_integration_test.go`, `internal/cli/gate_test.go`,
  `internal/cli/capability_production_test.go`, `internal/cli/run_tracker_rename_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `gatedrive.StartRequest` and `app.GateDriveStartRequest` have no `RunID` field;
  `(*gatedrive.Driver).Admit(req StartRequest) (*AdmissionTicket, error)` and
  `StartAdmitted(t *AdmissionTicket) (DriveDoc, error)` keep their signatures; `RunLaunchGate`,
  `SetRunLaunchGate`, and `RunLaunchGateWired` no longer exist. `mapDriveFailure` still maps a
  `*MutationFenceError` to `(ResultInvalidInput, fe.Reason)` (fail-safe); `fenceNextAction` is gone.
  `ClassifyRunIDError` / `RunIDNextAction` / `findRunDirByID` still exist (Task 5 removes them).

- [ ] **Step 1: Write the failing CLI test**

Append to `internal/cli/gate_test.go`:

```go
// TestGateDriveStartRejectsRunIDFlag (change 0491): gate drive start no longer
// takes --run-id. The run launch check it fed is gone, so a caller still passing
// it fails on an unknown flag (exit 2) instead of being silently accepted.
func TestGateDriveStartRejectsRunIDFlag(t *testing.T) {
	_, errS, code := runCLI(t, "gate", "drive", "start", "--owner", "build",
		"--run-root", "/tmp/docket-0491-run-root", "--run-id", "0790b760e26444866ef2e156ba383326")
	if code != 2 || !strings.Contains(errS, "unknown flag: --run-id") {
		t.Fatalf("exit %d stderr %q, want exit 2 naming the unknown --run-id flag", code, errS)
	}
}
```

In `internal/cli/run_tracker_rename_test.go` (`TestRunTrackerVocabularyHardCut`), replace the row
`{[]string{"gate", "drive", "start"}, "run-id", "run-epoch"},` with:

```go
		// change 0491: the run id is retired; gate drive start carries only the run context.
		{[]string{"gate", "drive", "start"}, "run-context", "run-id"},
		{[]string{"gate", "drive", "start"}, "run-context", "run-epoch"},
```

In `internal/cli/capability_production_test.go` (`TestRepresentativeSignatures`), change the
`gate.drive.start` pin to drop `[--run-id <id>] `:

```go
		"gate.drive.start": "--owner <role> --run-root <dir> [--branch <name>] [--change-id <id>] [--cwd <dir>] [--env-hash <hash>] [--idempotent-suite-gate] [--phase <name>] [--ref <ref>] [--repo-dir <dir>] [--run-context <token>] [--task-id <id>]",
```

Delete `TestGateDriveStartUnknownRunIDIsNamed` from `internal/cli/gate_test.go` (its subject, the
`--run-id` lookup, is gone).

- [ ] **Step 2: Run the CLI tests to verify they fail**

Run: `go test -count=1 ./internal/cli/ -run 'TestGateDriveStartRejectsRunIDFlag|TestRunTrackerVocabularyHardCut|TestRepresentativeSignatures'`
Expected: FAIL — the flag is still registered, so the command does not exit 2 on `--run-id`, the
hard-cut row reports `still registers the retired --run-id`, and the signature pin drifts.

- [ ] **Step 3: Delete the gatedrive run-launch seam**

In `internal/gatedrive/driver.go`:

- Delete the `RunID` field (and its comment) from `StartRequest`.
- Delete the `runLaunch RunLaunchGate` field and its comment from `Driver`.
- Delete the `RunLaunchGate` type, `SetRunLaunchGate`, `RunLaunchGateWired`, and `runLaunchGated`,
  with their comments.
- Delete the `runID` field and its comment from `AdmissionTicket`.
- Replace the tail of `Admit`, from the comment `// Fence the admission behind the app-owned run
  liveness read` through `return ticket, nil`, with:

```go
	// Admission takes the worktree lock and persists the reserved drive. No run is
	// checked (change 0491): a gate start is refused only by the worktree lock
	// (worktree-busy) and the drive protocol's own checks.
	return d.admitScopeless(rec, ownerGen, req.Owner)
}
```

- Replace `revalidateAdmittedLaunch` (and its doc comment) with:

```go
// revalidateAdmittedLaunch re-reads the RESERVED drive record this ticket minted
// — it must still exist under the ticket's owner generation and stay nonterminal
// — and acquires the drive's claimant flock NONBLOCKING. A busy claim, a lost
// owner, or a settled record (run.cancel or the keyed run.verdict settled a
// proven never-launched launch, change 0491) refuses with a typed error, closes
// the ticket's worktree lock, and launches nothing.
//
// The claim is taken as the nonblocking per-drive launch claimant (the SAME lock
// file tryRelaunchClaim/reserveRelaunch use), so a concurrent census that probes
// the claim reports busy — pending work, never proof of a crashed caller. The
// returned claim is retained by the caller across the launch.
func (d *Driver) revalidateAdmittedLaunch(t *AdmissionTicket) (*relaunchClaim, error) {
	refuse := func(c *relaunchClaim, err error) (*relaunchClaim, error) {
		if c != nil {
			c.close()
		}
		t.lock.Release()
		return nil, err
	}
	c, busy, cerr := d.store.tryRelaunchClaim(t.id)
	if cerr != nil {
		return refuse(nil, cerr)
	}
	if busy {
		return refuse(nil, ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted"))
	}
	cur, lerr := d.store.Load(t.id)
	if lerr != nil {
		return refuse(c, lerr)
	}
	if verr := verifyOwner(&cur, t.ownerGen); verr != nil {
		return refuse(c, verr)
	}
	if isTerminalOutcome(cur.LastOutcome) {
		return refuse(c, ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted"))
	}
	return c, nil
}
```

- Delete `settleAdmittedAfterRunRefusal` and its comment.
- In `StartAdmitted`'s body comment, change "Revalidate the run and the ticket's RESERVED drive
  record" to "Revalidate the ticket's RESERVED drive record", and drop the sentence "A fence that
  landed between Admit and here, a busy claim, or a settled record refuses" in favour of "A busy
  claim or a settled record refuses".
- Re-read the remaining `driver.go` comments with `git grep -n -E 'run launch gate|runLaunch|RunLaunchGate|run id' -- internal/gatedrive/*.go`
  and correct every one that still describes the run check (the `AdmissionTicket` and `Start`
  doc comments in particular). A comment naming the supervisor's raw run id stays.

- [ ] **Step 4: Delete the app-side gate and its wiring**

- `git rm internal/app/runtracker_launch_gate.go`.
- In `internal/app/gate_drive.go`: delete `GateDriveStartRequest.RunID` and its comment; delete
  `RunID: req.RunID,` from `startRequest`; delete the `engine.SetRunLaunchGate(runLaunchGate(gitCommonDir))`
  line and the three-line comment above it in both `newOwnedGateDriveService` and
  `NewCommandlessGateDriveService`. In the `startBudgetedBuild` doc, change "(worktree-busy), or the
  run launch gate refuses —" to "(worktree-busy) —".
- In `mapDriveResult`, delete the two `else if` branches after the ownership branch, so the block reads:

```go
		if oe, ok := gatedrive.AsOwnershipError(err); ok {
			if oe.Kind == gatedrive.ErrWorktreeBusy {
				// A worktree-busy refusal always names its site, and diagnoses from
				// the holder snapshot the lock refusal validated as running (never a
				// re-read) — or says the holder is unknown when there is none.
				result.Stage = stageWorktreeAdmission
				result.Locator = incumbentRefusalLocator(oe.Incumbent)
				result.Message = incumbentRemedyMessage(oe.Incumbent)
			} else {
				result.Message = ownershipNextAction(oe.Kind)
			}
		}
		return result
```

- In `mapDriveFailure`, delete the `ClassifyRunIDError` branch and its comment. Keep the
  `AsMutationFenceError` branch, and replace its comment with:

```go
	// A run mutation fence (runtracker_fence.go) is a distinct refusal type carrying
	// its OWN stable token. No gate-drive path raises it since change 0491 deleted
	// the run launch check, but classifying it stays fail-safe: a fenced-run error
	// that ever chains through this seam surfaces its bounded token instead of
	// leaking the wrapped refusal text or collapsing to the generic invalid-request.
```

- Delete `fenceNextAction` and its comment.
- In `internal/app/runtracker_continuation.go` `NewContinuationSeam`, delete the
  `driver.SetRunLaunchGate(runLaunchGate(gitCommonDir))` line and its three-line comment.
- In `internal/cli/gate.go` `gate drive start`: delete `runID, _ := c.Flags().GetString("run-id")`,
  `RunID: runID,`, and the `start.Flags().String("run-id", …)` line.
- Fix the comments listed under **Files** that describe the deleted check. For
  `internal/app/runtracker_run_record.go`, rewrite the header's IDENTITY paragraph clause "and is
  what a build-owned start presents to the run launch gate (runLaunchGate), so a cancelled or
  superseded run cannot start a gate" to drop it (Task 7 rewrites the paragraph again), and in the
  `bindRunWorktree` doc drop "and admitted by the run launch gate (runLaunchGate refuses an active
  run with no Worktree)". In `internal/app/runtracker_start.go` step "(6a)", delete the bullet
  "runLaunchGate refuses an active run that has no Worktree, so an unbound resume run would be
  refused on first use." and the clause "Its RunID is what each build-owned start presents to the
  run launch gate (runLaunchGate), so a cancelled or superseded run cannot start a gate." In
  `internal/app/runtracker_cancel.go`'s header, change "no new participant, start, relaunch,
  successor, resume claim, or mutation admission can attach to the run" to "no new participant,
  successor, resume claim, or mutation admission can attach to the run (gate starts are not fenced
  since change 0491; the census still accounts every drive the run's context names)".

- [ ] **Step 5: Delete and re-point the tests whose subject was the run check**

Decide each by what it guards (learning test-premise-deleted-not-regated): delete a test whose
subject is gone; re-point one whose mechanism stays.

`internal/gatedrive`:
- Move `driveRecordCount` from `run_launch_gate_test.go` into `driver_test.go` unchanged, then
  `git rm internal/gatedrive/run_launch_gate_test.go`.
- `git rm internal/gatedrive/launch_sites_guard_test.go` (`TestLaunchSitesBoundToRunLaunchGate`,
  `TestLaunchSiteGuardIsFalsifiable`). The surviving launch-site guard is
  `internal/repoguard` `TestGateLaunchAdmissionCoverage` (every launch site hands over a worktree
  lock); it must stay green.
- `git mv internal/gatedrive/driver_runfence_test.go internal/gatedrive/driver_launch_claim_test.go`.
  In it: delete `flippableGate` and its methods, `TestStartAdmittedRevalidatesRun`,
  `TestStartAdmittedNoRunRecordUnchanged`, `TestRelaunchCrossesNoRunGate`, and
  `TestNoRunAcquisitionWhileClaimHeld` (each guards only the run check). Keep
  `TestStartAdmittedRefusesBusyClaim`, `TestStartAdmittedHoldsClaimAcrossLaunch`,
  `TestClaimContentionBounded`, and `blockingLaunch`, deleting their `g := &flippableGate{}` /
  `permissive` lines, every `SetRunLaunchGate(...)` call, and every `req.RunID = …` line. Rewrite the
  file's opening comment to say it covers the per-drive launch claim.
- `driver_test.go`: delete `TestStartAdmittedRunRefusalFreesWorktree`; drop any remaining
  `StartRequest.RunID` use.
- `driver_concurrency_test.go`: delete `errRunFenced`, `fakeRunRegistry` (type and methods), and
  `TestBarrierCancelBeforeAdmit` (a fence at admission no longer exists). Re-point
  `TestBarrierCancelBetweenAdmitAndStartAdmitted`, `TestBarrierCancelBetweenAuthorizationAndLaunch`,
  and `TestBarrierCancelBetweenLaunchAndAttach`: delete `reg`, `SetRunLaunchGate`, `reg.fence(...)`,
  and `req.RunID` lines and keep every census and launch-count assertion. In
  `TestBarrierCancelBetweenAdmitAndStartAdmitted`, the delayed `StartAdmitted` now refuses because
  the census settled the record terminal, so replace the `errors.Is(serr, errRunFenced)` check
  with:

```go
	if oe, ok := AsOwnershipError(serr); !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted over a settled record must refuse ErrUnresolvedLaunchTransition, got %v", serr)
	}
```

  Rewrite the section banner comment above these tests: the census is driven directly; there is no
  run gate to fence.
- `old_records_test.go`: delete `req.RunID = runID` (keep the `runID` constant used in the scope JSON).
- `sequence_race_integration_test.go`: drop any `StartRequest.RunID` use.

`internal/app`:
- `git rm internal/app/runtracker_launch_gate_integration_test.go` (it holds
  `TestIntegrationRunRecordRunLaunchGate*`, `TestRaceIntegrationAppConcurrencyRunLaunchGateSerializesWithFence`,
  and `TestIntegrationRunRecordFindRunDirByID`). If it defines a helper another test file uses,
  move that helper into the file that uses it first.
- `gate_drive_test.go`: delete `TestProductionConstructorsWireRunLaunchGate`,
  `TestMapDriveFailureRunErrors`, `TestBuildStartRunRefusalChargesNoAttempt`, and
  `TestBuildStartChargedAttemptNotRefundedOnFencedLaunch`. Re-point `TestStartForwardsRunFields` to
  forward `RunContext` only (drop the `RunID` input and assertion). Re-point
  `TestMapDriveFailureFenceReasons` to its surviving property — `mapDriveFailure` maps each fence
  sentinel to `(invalid-input, <its token>)` without leaking the wrapped text: keep the
  `mapDriveFailure` and service-reason assertions and the no-leak checks, and delete the
  `got.Message == ""` and distinct-message (`seen`) assertions (the message came from the deleted
  `fenceNextAction`). Leave its tokens as `run-cancelled` / `stale-run-id` (Task 6 renames).
- `runtracker_production_census_integration_test.go` and `runtracker_start_resume_integration_test.go`:
  drop `RunID:` from every `GateDriveStartRequest` literal.
- Then run `git grep -n -E 'SetRunLaunchGate|RunLaunchGate|runLaunchGated|runLaunchGate\(|settleAdmittedAfterRunRefusal|fenceNextAction|errRunFenced|fakeRunRegistry|flippableGate' -- '*.go'`
  and expect no output.

- [ ] **Step 6: Format, build, vet, and run the focused tests**

Run:
```bash
"$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" -l internal/ cmd/
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/ ./internal/cli/
go test -tags integration -count=1 ./internal/gatedrive/
go test -count=1 ./internal/app/ -run 'TestStartForwardsRunFields|TestMapDriveFailure|TestBuildStart|TestGateDrive'
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunRecord|TestIntegrationRunCompletion|TestRaceIntegrationAppConcurrency|TestIntegrationGateLifecycle|TestIntegrationRunStart)' ./internal/app/
go test -count=1 ./internal/repoguard/ -run 'TestGateLaunchAdmissionCoverage|TestCommentAnchorStyle'
```
Expected: gofmt lists nothing; every command PASS. `TestGateDriveStartRejectsRunIDFlag` now passes.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive internal/app internal/cli tests/test_go_integration_app_concurrency.sh
git status --short   # must show only this task's paths
git commit -m "refactor(0491): delete the run launch check; gate.drive.start carries no run id"
```

---

### Task 2: The census gains a stop-free verdict mode; a missing run root reads as never launched

**Risk:** premium — the census now writes drive records on the success path; the probe-error
branch must stay fail-closed.

**Files:**
- Modify: `internal/gatedrive/reconcile.go` (mode type, `ReconcileRunLaunches`, new
  `VerdictRunLaunches` replacing `ObserveRunLaunches`, `accountRunLaunches`, `reconcileRunDrive`,
  `reconcileHaltedDrive`, `resolveHaltedFirstLaunch`, `reconcileReservation`,
  `reconcileFirstLaunch`, `settleNeverLaunchedCancelled`, `settleNeverLaunchedFirstLaunch`,
  `settleNeverLaunched`, `proveRunDirsGone`, `supervisorGone`, file header)
- Modify: `internal/app/runtracker_complete.go` (`appLaunchObserver.observe` calls
  `VerdictRunLaunches`; file header and `runLaunchObserver` / `appLaunchObserver` /
  `accountCompletionLaunches` docs)
- Modify: `internal/app/runtracker_cancel.go` (the `cancelSeams` field comment for
  `observer`/`launchObserver`)
- Test: `internal/gatedrive/reconcile_test.go`, `internal/gatedrive/old_records_test.go`,
  `internal/gatedrive/driver_launch_claim_test.go`,
  `internal/app/runtracker_production_census_integration_test.go`,
  `internal/app/runtracker_verdict_integration_test.go`

**Interfaces:**
- Consumes: Task 1's `revalidateAdmittedLaunch` (a settled record refuses `ErrUnresolvedLaunchTransition`).
- Produces: `func (d *Driver) VerdictRunLaunches(runContextHash string) (RunLaunchReport, error)`;
  `ObserveRunLaunches` no longer exists. `ReconcileRunLaunches` is unchanged in signature and
  behaviour except the missing-root rule. Drive HALT causes: cancel mode writes `run-cancelled`,
  verdict mode writes `launch-abandoned` (first launch only). The app keeps the names
  `runLaunchObserver`, `launchObserver`, `appLaunchObserver`, and method `observe` (renaming would
  churn every completion test); their doc comments now say they run the verdict-mode census.

- [ ] **Step 1: Write the failing gatedrive tests**

In `internal/gatedrive/reconcile_test.go`:

(a) Change `censusModes` to `{{"reconcile", d.ReconcileRunLaunches}, {"verdict", d.VerdictRunLaunches}}`.
Then grep the file for `ObserveRunLaunches` and `"observe"` and convert every observe-mode case to
verdict mode. The expectations of those cases do not change: a running supervisor is `run-live`
and never stopped, a reserved relaunch is `launch-pending`. The exception is (b).

(b) Replace the `observe-pending-unchanged` subtest of `TestCensusSettlesNeverAttachedFirstLaunch`
with:

```go
	t.Run("verdict-settles-launch-abandoned", func(t *testing.T) {
		var root, token string
		proc := newProc(&root, &token)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id := seed(t, store)

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if !report.Accounted || len(report.Findings) != 0 {
			t.Fatalf("the keyed verdict must settle a proven never-launched first launch, got %+v", report)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if after.LastOutcome != HALTED || after.LastCause != "launch-abandoned" {
			t.Fatalf("want HALTED launch-abandoned, got %v/%q", after.LastOutcome, after.LastCause)
		}
		if proc.launchN != 0 || proc.stopN != 0 {
			t.Fatalf("the verdict census launches and stops nothing: launch=%d stop=%d", proc.launchN, proc.stopN)
		}
	})
```

The `seed` helper of that test must give the drive an existing run root, because a missing root
now short-circuits the resolve (Step 3). Set `r.RunRoot = testsupport.TempDir(t)` in its mutate
func (it needs `t`, which it already takes), and compare `root` against the seeded value rather
than `seedRecord(t).RunRoot` in `cancel-settles`.

(c) Rename the `never-launched-observe-pending` subtest of
`TestCensusReservedRelaunchResolvesRelaunchToken` to `never-launched-verdict-pending`, and call
`d.VerdictRunLaunches`. Its expectation stays: `launch-pending:<id>`, record nonterminal (Change 0493
retires the relaunch).

(d) Append these tests:

```go
// TestCensusMissingRunRootIsNeverLaunched (change 0491, Decision 4): a first launch
// whose RunRoot does not exist (the launcher died before process.Launch made it, or
// a temp dir was cleaned) never resolves its token — ResolveReservation would refuse
// a missing root forever. Both modes read the clean absence as never launched and
// settle it: run-cancelled in cancel mode, launch-abandoned in verdict mode.
func TestCensusMissingRunRootIsNeverLaunched(t *testing.T) {
	for _, tc := range []struct{ mode, cause string }{
		{"reconcile", "run-cancelled"},
		{"verdict", "launch-abandoned"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			proc := &fakeProc{}
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
			missing := filepath.Join(testsupport.TempDir(t), "never-created")
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
				r.RawRunDir, r.RawOwnership = "", ""
				r.AdmissionToken = censusAdmissionToken
				r.RunRoot = missing
			})
			run := d.ReconcileRunLaunches
			if tc.mode == "verdict" {
				run = d.VerdictRunLaunches
			}
			report, err := run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", tc.mode, err)
			}
			if !report.Accounted || len(report.Findings) != 0 {
				t.Fatalf("%s: a missing run root is never launched and settled, got %+v", tc.mode, report)
			}
			if proc.resolveN != 0 {
				t.Fatalf("%s: a missing root must not be resolved, ResolveReservation called %d times", tc.mode, proc.resolveN)
			}
			after, err := store.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if after.LastOutcome != HALTED || after.LastCause != tc.cause {
				t.Fatalf("%s: want HALTED %s, got %v/%q", tc.mode, tc.cause, after.LastOutcome, after.LastCause)
			}
		})
	}
}

// TestCensusRunRootProbeErrorStaysUnresolved (change 0491, Review Focus 1): a run
// root whose Lstat fails for a reason other than not-exist is unknown, not absent —
// it stays resolution-unresolved in both modes and the record is never settled
// (learning probe-error-is-not-clean-absence).
func TestCensusRunRootProbeErrorStaysUnresolved(t *testing.T) {
	file := filepath.Join(testsupport.TempDir(t), "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(file, "root") // Lstat fails ENOTDIR, not ErrNotExist
	for _, mode := range []string{"reconcile", "verdict"} {
		t.Run(mode, func(t *testing.T) {
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
				r.RawRunDir, r.RawOwnership = "", ""
				r.AdmissionToken = censusAdmissionToken
				r.RunRoot = notDir
			})
			run := d.ReconcileRunLaunches
			if mode == "verdict" {
				run = d.VerdictRunLaunches
			}
			report, err := run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
				t.Fatalf("%s: a run-root probe error must stay unresolved, got %+v", mode, report)
			}
			after, err := store.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if isTerminalOutcome(after.LastOutcome) {
				t.Fatalf("%s: a probe error must never settle the record, got %v/%q", mode, after.LastOutcome, after.LastCause)
			}
		})
	}
}

// TestCensusVerdictLeavesBusyClaimPending (change 0491, Review Focus 2): a launcher
// still alive and holding the drive's claim (StartAdmitted mid-launch) is pending
// work. The keyed verdict reports claim-busy, settles nothing, and never waits.
func TestCensusVerdictLeavesBusyClaimPending(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
		r.RawRunDir, r.RawOwnership = "", ""
		r.AdmissionToken = censusAdmissionToken
		r.RunRoot = testsupport.TempDir(t)
	})
	held, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("precondition: take the claim: busy=%v err=%v", busy, err)
	}
	defer held.close()

	report, err := d.VerdictRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if report.Accounted || !findingFor(report.Findings, "claim-busy", id) {
		t.Fatalf("a held claim must read claim-busy, got %+v", report)
	}
	after, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if isTerminalOutcome(after.LastOutcome) {
		t.Fatalf("the verdict must not settle a drive whose launcher holds the claim, got %v", after.LastOutcome)
	}
}

// TestCensusVerdictSettlesOnlyItsOwnRun (change 0491, Review Focus 3): the keyed
// verdict settles only drives attributed to its own run context; another run's
// never-launched drive is untouched.
func TestCensusVerdictSettlesOnlyItsOwnRun(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	other, _ := seedRunDrive(t, store, censusCtxB, func(r *driveRecord) {
		r.RawRunDir, r.RawOwnership = "", ""
		r.AdmissionToken = censusAdmissionToken
		r.RunRoot = testsupport.TempDir(t)
	})
	report, err := d.VerdictRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if !report.Accounted || len(report.Findings) != 0 {
		t.Fatalf("a run with no drives of its own accounts vacuously, got %+v", report)
	}
	after, err := store.Load(other)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if isTerminalOutcome(after.LastOutcome) {
		t.Fatalf("another run's drive must not be settled, got %v/%q", after.LastOutcome, after.LastCause)
	}
}
```

(e) Every other census test that seeds an unattached, nonterminal first launch
(`RawRunDir == ""` with an `AdmissionToken`) **and** expects `ResolveReservation` to run (an
identified, unresolved, or resolve-error case) must now give it an existing run root:
`r.RunRoot = testsupport.TempDir(t)`. Find them with
`git grep -n -E 'RawRunDir, r.RawOwnership = "", ""' -- internal/gatedrive/` and the
`AdmissionToken = ` seeds near them. `TestCensusStopsIdentifiedFirstLaunch` is one. Tests over a
HALTED `launch-failed` drive (`resolveHaltedFirstLaunch`) already pass their own root and are
unchanged. The same applies to tests that create the reserved drive through `Admit` with
`sampleStart()`, whose `RunRoot` (`/repo/.git/docket/gate-runs`) does not exist. If such a test's
census must resolve the launch token (for example a `racingProc` that answers `identified` in
`driver_concurrency_test.go`), set `req.RunRoot = testsupport.TempDir(t)`. A test that only needs
the drive proven never launched may keep the missing root, because it now settles through the
Lstat rule.

(f) In `internal/gatedrive/old_records_test.go` `TestOldTaskScopeOnDiskChangesNothing`, change
`"closeout": d.ObserveRunLaunches,` to `"verdict": d.VerdictRunLaunches,`.

(g) T4 — append to `internal/gatedrive/driver_launch_claim_test.go`:

```go
// TestStartAdmittedRefusesAfterVerdictSettle (change 0491, spec T4): a launcher
// delayed between Admit and StartAdmitted, still holding its in-memory ticket,
// finds its reserved record settled HALTED launch-abandoned by the keyed verdict.
// StartAdmitted re-reads the record under the claim, refuses, launches nothing,
// and frees the worktree — exactly as after cancel's settle.
func TestStartAdmittedRefusesAfterVerdictSettle(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.RunContext = "ctx-0491"
	req.RunRoot = testsupport.TempDir(t) // exists, holds no reservation: never launched
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	report, err := d.VerdictRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched admission must be accounted, got %+v", report)
	}
	rec, err := store.Load(ticket.id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "launch-abandoned" {
		t.Fatalf("want HALTED launch-abandoned, got %v/%q", rec.LastOutcome, rec.LastCause)
	}

	_, serr := d.StartAdmitted(ticket)
	if oe, ok := AsOwnershipError(serr); !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted over a settled record must refuse ErrUnresolvedLaunchTransition, got %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a settled admission must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatal("a refused launch must free the worktree lock")
	}
}
```

If `fakeProc`'s default `ResolveReservation` (nil `resolve` func) does not return
`never-launched`, give this test's `fakeProc` a `resolve` func that does (mirror
`TestCensusSettlesNeverAttachedFirstLaunch`'s `newProc`).

- [ ] **Step 2: Run the gatedrive tests to verify they fail**

Run: `go test -count=1 ./internal/gatedrive/ -run 'TestCensus|TestOldTaskScope|TestStartAdmittedRefusesAfterVerdictSettle'`
Expected: compile FAIL — `d.VerdictRunLaunches undefined`.

- [ ] **Step 3: Implement the mode and the missing-root rule in `reconcile.go`**

Add below the imports:

```go
// censusMode selects what the run launch census may do (change 0491).
type censusMode int

const (
	// censusCancel is run.cancel's census: it stops a running supervisor and settles a
	// proven never-launched launch HALTED run-cancelled.
	censusCancel censusMode = iota
	// censusVerdict is the keyed run.verdict's census: it stops nothing and signals
	// nothing. It settles only a proven never-launched FIRST launch, HALTED
	// launch-abandoned; a running supervisor is run-live, and a reserved relaunch
	// stays launch-pending (change 0493 retires finalize's relaunch).
	censusVerdict
)

// The HALT causes the census writes on a proven never-launched launch.
const (
	causeRunCancelled    = "run-cancelled"
	causeLaunchAbandoned = "launch-abandoned"
)

// firstLaunchCause is the HALT cause a proven never-launched first launch settles to.
func (m censusMode) firstLaunchCause() string {
	if m == censusVerdict {
		return causeLaunchAbandoned
	}
	return causeRunCancelled
}
```

Replace the two entry points:

```go
// ReconcileRunLaunches reconciles, for an ALREADY-FENCED run, every drive whose
// RunContextHash equals runContextHash (cancellation mode): a running supervisor
// is stopped and must then observe as not running; a first launch or reserved
// relaunch that never attached is resolved through its exact reservation token,
// and a proven never-launched one is settled HALTED "run-cancelled" under the held
// claim so no later launch can follow the cancel. An empty runContextHash names no
// drive and accounts vacuously.
func (d *Driver) ReconcileRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, censusCancel)
}

// VerdictRunLaunches is the keyed run.verdict's view of one run's launch
// obligations (change 0491; it replaces change 0441's observe-only twin, whose
// only caller was the closeout): the same walk, attribution, and claimant probe as
// ReconcileRunLaunches, but it never stops or signals a process. A proven
// never-launched FIRST launch is settled HALTED "launch-abandoned" under the held
// claim, so a successful run is not stranded by a launcher killed between Admit
// and StartAdmitted; a live supervisor is run-live and a reserved relaunch stays
// launch-pending, and both keep the report unaccounted.
func (d *Driver) VerdictRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, censusVerdict)
}
```

Then thread `mode censusMode` through every helper that takes `observeOnly bool`
(`accountRunLaunches`, `reconcileRunDrive`, `reconcileHaltedDrive`, `resolveHaltedFirstLaunch`,
`reconcileReservation`, `reconcileFirstLaunch`, `proveRunDirsGone`, `supervisorGone`), replacing
each `observeOnly` test with `mode == censusVerdict`. The two behavioural changes:

```go
// reconcileReservation, "never-launched" arm — unchanged behaviour, new spelling:
	case "never-launched":
		if mode == censusVerdict {
			return false, "launch-pending:" + id
		}
		return d.settleNeverLaunchedCancelled(id, ownerGen)
```

```go
// reconcileFirstLaunch resolves a first launch whose run dir was never attached the
// way reconcileReservation resolves a reserved relaunch, through the drive's
// admission token (the token StartAdmitted hands the launch). A run root that does
// not exist was never created by a launch (or was cleaned), so it holds no
// reservation: clean absence, settled like a proven never-launched launch, as
// resolveHaltedFirstLaunch and supervisorGone already treat it (change 0491). Any
// other Lstat error is unknown, never absence, and stays pending. A proven
// never-launched first launch is settled HALTED with the mode's cause
// (run-cancelled / launch-abandoned); an identified run must prove its supervisor
// gone; anything else is pending.
func (d *Driver) reconcileFirstLaunch(id string, cur driveRecord, mode censusMode) (bool, string) {
	if cur.RunRoot != "" {
		if _, err := os.Lstat(cur.RunRoot); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return d.settleNeverLaunchedFirstLaunch(id, cur.OwnerGeneration, mode.firstLaunchCause())
			}
			return false, "resolution-unresolved:" + id
		}
	}
	res, rerr := d.proc.ResolveReservation(cur.RunRoot, cur.AdmissionToken)
	if rerr != nil || res == nil {
		return false, "resolution-unresolved:" + id
	}
	switch res.Disposition {
	case "never-launched":
		return d.settleNeverLaunchedFirstLaunch(id, cur.OwnerGeneration, mode.firstLaunchCause())
	case "identified":
		return d.supervisorGone(id, res.RunDir, mode)
	default:
		return false, "resolution-unresolved:" + id
	}
}
```

The settles take the cause:

```go
func (d *Driver) settleNeverLaunchedCancelled(id, ownerGen string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, causeRunCancelled, func(r *driveRecord) bool { return r.RelaunchReserved })
}

func (d *Driver) settleNeverLaunchedFirstLaunch(id, ownerGen, cause string) (bool, string) {
	return d.settleNeverLaunched(id, ownerGen, cause, func(r *driveRecord) bool {
		return r.RawRunDir == "" && !r.RelaunchReserved
	})
}

// settleNeverLaunched is the shared CAS behind every never-launched settle: it
// writes HALTED cause only while the owner generation still matches and
// stillUnlaunched holds, accounting an already-terminal record and keeping any
// other outcome pending (resolution-unresolved).
func (d *Driver) settleNeverLaunched(id, ownerGen, cause string, stillUnlaunched func(*driveRecord) bool) (bool, string) {
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if verr := verifyOwner(r, ownerGen); verr != nil {
			return verr
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		if !stillUnlaunched(r) {
			return errRelaunchRaceLost
		}
		r.LastOutcome = HALTED
		r.LastCause = cause
		return nil
	})
	if err == nil || errors.Is(err, errAlreadyTerminal) {
		return true, "" // settled terminal: provably idle AND foreclosed from launch
	}
	return false, "resolution-unresolved:" + id
}
```

Update the doc comments of `settleNeverLaunchedFirstLaunch` (it settles with the mode's cause),
`resolveHaltedFirstLaunch`, `supervisorGone` ("verdict mode reports it run-live without
stopping"), and the file header (the census has two modes: cancel stops and settles
`run-cancelled`; verdict stops nothing and settles only a never-launched first launch
`launch-abandoned`). Delete `ObserveRunLaunches`. Then run
`git grep -n -E 'observeOnly|ObserveRunLaunches|observation-only twin' -- internal/` and fix every
hit: code, comments, and tests.

- [ ] **Step 4: Swap the closeout onto verdict mode**

In `internal/app/runtracker_complete.go`, change `appLaunchObserver.observe` to call
`gatedrive.NewSystemDriver(o.store, svc).VerdictRunLaunches(contextHash)`. Rewrite the docs the
change falsifies:
- The file header's "OBSERVATION ONLY" paragraph becomes "STOPS NOTHING": closeout stops nothing
  and signals nothing (no native cancellation, no `process.Stop`). Its two writes are
  `settleUncertainPublications` (change 0444) and the verdict-mode census's settle of a proven
  never-launched first launch (HALTED `launch-abandoned`, change 0491).
- In step (3) of STEPS, "the observe-only launch census" becomes "the verdict-mode launch census".
- `runLaunchObserver`: "the verdict-mode counterpart of runLaunchReconciler: it stops nothing, but
  settles a proven never-launched first launch HALTED launch-abandoned (change 0491). Production
  appLaunchObserver wraps gatedrive.Driver.VerdictRunLaunches".
- `appLaunchObserver`: "observes one run's drives through VerdictRunLaunches (it stops nothing)".
- In `completeSuccessfulRun`'s step (1b) comment, drop "and settles no never-launched reservation".

In `internal/app/runtracker_cancel.go`, update the `cancelSeams` comment for
`observer`/`launchObserver` the same way ("launchObserver walks a run's launch obligations in
verdict mode: it stops nothing and settles only a proven never-launched first launch").

- [ ] **Step 5: Write the app-level tests (spec T1, T3, T5)**

In `internal/app/runtracker_production_census_integration_test.go` add the shared seed helpers
(integration-tagged; the verdict tests in the same package use them):

```go
// neverLaunchedToken is the launch token a seeded never-launched drive carries:
// lowercase hex, as process.ResolveReservation requires.
const neverLaunchedToken = "0491aaaabbbbccccddddeeeeffff0000"

// seedNeverLaunchedDrive seeds the record a tracked gate.drive.start killed between
// Admit and StartAdmitted leaves behind (change 0491, spec Problem fact 5): reserved
// — no outcome, no run dir — carrying its launch token, owner generation, run root,
// and the run's context hash. extra merges further fields (change_id for the outer
// scan).
func seedNeverLaunchedDrive(t *testing.T, common, id, worktree, runRoot, contextHash string, extra map[string]any) {
	t.Helper()
	fields := map[string]any{
		"worktree_path":    worktree,
		"run_root":         runRoot,
		"admission_token":  neverLaunchedToken,
		"owner_generation": "orphan-owner-generation",
		"run_context_hash": contextHash,
	}
	for k, v := range extra {
		fields[k] = v
	}
	seedCensusDrive(t, common, id, fields)
}

// driveOutcome reads one drive's persisted outcome and cause.
func driveOutcome(t *testing.T, store *gatedrive.Store, id string) (gatedrive.Outcome, string) {
	t.Helper()
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load drive %s: %v", id, err)
	}
	return rec.LastOutcome, rec.LastCause
}
```

T3 — append to the same file:

```go
// TestIntegrationRunCompletionProductionCensusMissingRunRoot (change 0491, spec T3):
// a never-attached first launch whose run root does not exist used to read
// resolution-unresolved forever, so cancel stayed cancellation-pending, the closeout
// blocked, and resume quiescence refused. Each now reads it as never launched.
func TestIntegrationRunCompletionProductionCensusMissingRunRoot(t *testing.T) {
	const orphan = "adddddddddddddddddddddddddd00491"
	seed := func(t *testing.T, fx cancelFixture) {
		missing := filepath.Join(testsupport.TempDir(t), "never-created-run-root")
		seedNeverLaunchedDrive(t, fx.common, orphan, fx.worktree, missing, fx.contextHash, nil)
	}
	t.Run("cancel-settles-run-cancelled", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		seed(t, fx)
		res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human stop")
		if res.Disposition != CancelDispositionCancelled {
			t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
		}
		if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "run-cancelled" {
			t.Fatalf("drive = %s/%q, want HALTED run-cancelled", out, cause)
		}
	})
	t.Run("closeout-settles-launch-abandoned", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		seed(t, fx)
		if ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key); !ok {
			t.Fatalf("closeout ok=false reason=%q findings=%v", reason, findings)
		}
		if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
			t.Fatalf("run state = %q, want completed", st)
		}
		if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "launch-abandoned" {
			t.Fatalf("drive = %s/%q, want HALTED launch-abandoned", out, cause)
		}
	})
	t.Run("resume-quiescence-admits", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		if res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human stop"); res.Disposition != CancelDispositionCancelled {
			t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
		}
		seed(t, fx) // left behind after the cancel, as a delayed launcher would
		oldEp, _, err := LoadRunRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("LoadRunRecord: %v", err)
		}
		if ok, detail := validateResumeQuiescence(productionCancelSeams(fx.repo), fx.repo, oldEp); !ok {
			t.Fatalf("resume quiescence refused over a missing run root: %s", detail)
		}
	})
}
```

The `fx.runID` argument to `runCancel` exists until Task 4 removes it; Task 4 updates this call. If
the `closeout` subtest's coordinator participant is required for completion (see
`TestIntegrationRunCompletionProductionCensusCompleteThenFinalize`), register and record it the
same way before the closeout.

T1 — append to `internal/app/runtracker_verdict_integration_test.go`:

```go
// TestIntegrationRunVerdictNeverLaunchedDriveCompletesRun (change 0491, spec T1, the
// stub's regression — 0490 review finding F3): a tracked gate.drive.start killed
// between Admit and StartAdmitted leaves a reserved drive record nothing can launch.
// The keyed run.verdict's verdict-mode census proves it never launched, settles it
// HALTED launch-abandoned, and the run completes — run-done run-complete, never
// run-stop … completion-unaccounted.
func TestIntegrationRunVerdictNeverLaunchedDriveCompletesRun(t *testing.T) {
	requireProcessSupervisorHere(t)
	fx := newVerdictCompletionFixture(t)
	common, err := runTrackerGitCommonDir(fx.repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	// The production verdict census over the real store replaces the fixture's fake.
	fx.wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: fx.store, observer: fx.observer, launchObserver: appLaunchObserver{store: fx.store}}
	}
	const orphan = "adddddddddddddddddddddddddd00492"
	// An existing, empty run root holds no reservation: ResolveReservation proves
	// never-launched. "ha" is the fixture run's context hash.
	seedNeverLaunchedDrive(t, common, orphan, fx.worktree, testsupport.TempDir(t), "ha", nil)

	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings=%v)", got, want, res.CompletionFindings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "launch-abandoned" {
		t.Fatalf("drive = %s/%q, want HALTED launch-abandoned", out, cause)
	}
}
```

Check the drive ids are 32 lowercase hex characters (`validateID`), and adjust the imports
(`testsupport`, `gatedrive`, `context`) as needed.

T5 — in `internal/app/runtracker_complete_integration_test.go`
`TestIntegrationRunCompletionCompleteSuccessfulRunSendsNoStops`, after each of the two
`completeSuccessfulRun` calls, also assert the closeout drove the verdict census exactly once (the
census seam the closeout uses is the stop-free one):

```go
	if got := fx.launchObserver.calls; len(got) != 1 {
		t.Fatalf("the closeout must drive the verdict census exactly once, got %v", got)
	}
```

(use `fx2.launchObserver` for the blocked path). Update the test's doc comment: "closeout stops
nothing — it drives only the stop-free verdict census, never the stopper, native canceller, or
cancel-mode reconcile seam". `TestIntegrationRunCompletionProductionCensusHaltedLiveDriveBlocks`
must stay green unchanged: a running supervisor still blocks as `run-live` and is never stopped.

- [ ] **Step 6: Run the tests**

Run:
```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/gatedrive/
go test -tags integration -count=1 ./internal/gatedrive/
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunCompletion|TestIntegrationRunVerdict|TestIntegrationRunCancel|TestIntegrationRunStart)' ./internal/app/
```
Expected: PASS.

- [ ] **Step 7: Mutation-test the new behaviour**

With a backup copy of `internal/gatedrive/reconcile.go`:
1. In `reconcileFirstLaunch`'s `"never-launched"` arm, replace the settle with
   `if mode == censusVerdict { return false, "launch-pending:" + id }` before it (observe mode's old
   behaviour). Run `go test -count=1 ./internal/gatedrive/ -run 'TestCensusSettlesNeverAttachedFirstLaunch|TestStartAdmittedRefusesAfterVerdictSettle'`
   and `go test -tags integration -count=1 -run '^TestIntegrationRunVerdictNeverLaunchedDriveCompletesRun$' ./internal/app/`.
   Expected: all three FAIL. Restore.
2. Delete the `os.Lstat(cur.RunRoot)` block in `reconcileFirstLaunch`. Run
   `go test -count=1 ./internal/gatedrive/ -run TestCensusMissingRunRootIsNeverLaunched` and
   `go test -tags integration -count=1 -run '^TestIntegrationRunCompletionProductionCensusMissingRunRoot$' ./internal/app/`.
   Expected: FAIL. Restore.
3. Change the Lstat error branch to treat every error as not-exist (`if err != nil { return d.settle… }`).
   Run `go test -count=1 ./internal/gatedrive/ -run TestCensusRunRootProbeErrorStaysUnresolved`.
   Expected: FAIL. Restore with `mv -f`.
Confirm each mutation landed with `grep -c` before and after.

- [ ] **Step 8: Measure the shards**

Run `time bash tests/test_go_integration_app_runcompletion.sh` and
`time bash tests/test_go_integration_app_runverdict.sh` solo. Each must stay within its
`tests/runtime-budgets.tsv` ceiling (15 s and 20 s). If either exceeds it, return `BLOCKED` with
the numbers; never raise a row.

- [ ] **Step 9: Commit**

```bash
git add internal/gatedrive internal/app
git commit -m "feat(0491): the keyed verdict settles a never-launched drive launch-abandoned; a missing run root is never launched"
```

---

### Task 3: The run-incomplete verdict settles never-launched drives before the outer scan

**Risk:** premium — ordering-sensitive change on the path ahead of the retry CAS.

**Files:**
- Modify: `internal/app/runtracker_verdict.go` (`RunVerdict` run-complete arm,
  `runTrackerOuterContinuation`, new `verdictSeams`, new `settleNeverLaunchedForVerdict`)
- Test: `internal/app/runtracker_verdict_integration_test.go`

**Interfaces:**
- Consumes: Task 2's verdict-mode census behind `cancelSeams.launchObserver`
  (`observe(contextHash string) (gatedrive.RunLaunchReport, error)`), and the helpers
  `seedNeverLaunchedDrive` / `driveOutcome` / `neverLaunchedToken`.
- Produces: `func verdictSeams(repoDir string, wdeps WorkspaceDeps) cancelSeams` and
  `func settleNeverLaunchedForVerdict(seams cancelSeams, contextHash string)`.

- [ ] **Step 1: Write the failing tests (spec T2, Review Focus 4)**

Append to `internal/app/runtracker_verdict_integration_test.go`:

```go
// TestIntegrationRunVerdictNeverLaunchedDriveEarnsRetry (change 0491, spec T2): the
// same orphan as T1, on a run-incomplete verdict. FindScopeDriveIDs used to offer
// the nonterminal reserved record for takeover, so the verdict printed run-stop …
// continuation-unverified and spent the run's single-use outer scope. The verdict
// now settles it first: the run earns run-retry-once, the drive is HALTED
// launch-abandoned, and the outer scope is not consumed.
func TestIntegrationRunVerdictNeverLaunchedDriveEarnsRetry(t *testing.T) {
	requireProcessSupervisorHere(t)
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	repo := f.repo.invocation
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	store := gatedrive.OpenStore(common)
	grant, err := store.PrepareScope(gatedrive.ScopeRequest{ChangeID: "3"})
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	ctxHash := runTrackerHashToken(grant.ChildCapability)
	key := runTrackerMintAttributedScoped(t, repo, grant.ScopeID, grant.ParentCapability, ctxHash, 3)

	svc, _, reason := gateService()
	if svc == nil {
		t.Skipf("gate service unavailable: %s", reason)
	}
	wdeps.Continuation = &gatedriveContinuationSeam{store: store, driver: gatedrive.NewSystemDriver(store, svc)}
	wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: store, launchObserver: appLaunchObserver{store: store}}
	}
	const orphan = "adddddddddddddddddddddddddd00493"
	seedNeverLaunchedDrive(t, common, orphan, repo, testsupport.TempDir(t), ctxHash,
		map[string]any{"change_id": "3"})

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, repo, key)
	if got, want := res.HumanText(), "run-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if out, cause := driveOutcome(t, store, orphan); out != gatedrive.HALTED || cause != "launch-abandoned" {
		t.Fatalf("drive = %s/%q, want HALTED launch-abandoned", out, cause)
	}
	sc, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if sc.Closed {
		t.Fatal("the outer recovery scope must not be consumed by a never-launched drive")
	}
}

// TestIntegrationRunVerdictUnattributedNeverSettlesNeverLaunchedDrive (change 0491,
// Review Focus 4): only the attributed, keyed verdict runs the verdict-mode census
// (ADR-0124 rule 1). An --unattributed verdict stays read-only: the orphan stays
// nonterminal.
func TestIntegrationRunVerdictUnattributedNeverSettlesNeverLaunchedDrive(t *testing.T) {
	requireProcessSupervisorHere(t)
	fx := newVerdictCompletionFixture(t)
	common, err := runTrackerGitCommonDir(fx.repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	fx.wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: fx.store, observer: fx.observer, launchObserver: appLaunchObserver{store: fx.store}}
	}
	const orphan = "adddddddddddddddddddddddddd00494"
	seedNeverLaunchedDrive(t, common, orphan, fx.worktree, testsupport.TempDir(t), "ha", nil)

	_ = RunVerdictObserve(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, []string{"3"})
	if out, _ := driveOutcome(t, fx.store, orphan); out != "" {
		t.Fatalf("an unattributed verdict settled a drive: outcome %s", out)
	}
}
```

If `gatedrive.ScopeRequest`, `PrepareScope`'s return, or `LoadScope`'s `Closed` field differ in
name, use the real ones (read `internal/gatedrive/scope.go`), keeping the assertions.

- [ ] **Step 2: Run them to verify T2 fails**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunVerdict(NeverLaunchedDriveEarnsRetry|UnattributedNeverSettlesNeverLaunchedDrive)$' ./internal/app/`
Expected: `NeverLaunchedDriveEarnsRetry` FAILS. The orphan is a takeover candidate, so HumanText is a
`run-stop … run-tracker-unavailable …` line. `Unattributed…` PASSES already; it is a regression
pin.

- [ ] **Step 3: Implement**

In `internal/app/runtracker_verdict.go`, add:

```go
// verdictSeams composes the census seam bundle the keyed run.verdict uses on both
// of its paths (change 0491): the injected bundle when a caller wires one, else
// the production bundle over repoDir.
func verdictSeams(repoDir string, wdeps WorkspaceDeps) cancelSeams {
	if wdeps.CancelSeams != nil {
		return wdeps.CancelSeams(repoDir)
	}
	return productionCancelSeams(repoDir)
}

// settleNeverLaunchedForVerdict runs the verdict-mode census for contextHash and
// discards its report: on the run-incomplete path only its settles matter (change
// 0491, Decision 3). It stops nothing. A nil census seam settles nothing.
func settleNeverLaunchedForVerdict(seams cancelSeams, contextHash string) {
	if seams.launchObserver == nil {
		return
	}
	_, _ = seams.launchObserver.observe(contextHash)
}
```

In `RunVerdict`'s `case VerdictRunComplete:` replace the four lines that compose `seams` with
`return runTrackerCompleteRun(repoDir, key, rec, id, verdictSeams(repoDir, wdeps))` and keep the
comment above it.

In `runTrackerOuterContinuation`, immediately after the `if seam == nil { … }` block and before
`ids, err := seam.LocateOuterDrive(…)`, insert:

```go
	// Settle the run's proven never-launched first launches BEFORE locating
	// candidates (change 0491, Decision 3). A gate.drive.start killed between Admit
	// and StartAdmitted leaves a nonterminal reserved record that FindScopeDriveIDs
	// would offer for takeover, stranding the run as continuation-unverified and
	// spending its single-use outer scope. The verdict-mode census settles it HALTED
	// launch-abandoned under its per-drive claim and stops nothing. Its findings are
	// ignored here: a live, busy, or unresolved drive is still found and handled below.
	settleNeverLaunchedForVerdict(verdictSeams(repoDir, wdeps), rec.ChildContextHash)
```

Add one line to `runTrackerOuterContinuation`'s doc comment: "It first settles the run's proven
never-launched first launches (settleNeverLaunchedForVerdict)."

- [ ] **Step 4: Run the verdict and completion shards**

Run:
```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunVerdict|TestIntegrationRunCompletion|TestRaceIntegrationAppConcurrency)' ./internal/app/
```
Expected: PASS, including `TestIntegrationRunVerdictVerdictIncompleteWithTrackedDriveContinuesWithoutRetry`
and `TestIntegrationRunVerdictVerdictIncompleteQuiescentStillRetriesOnce` (a live candidate is still
taken over; a quiescent run still retries).

- [ ] **Step 5: Mutation-test**

With a backup copy of `runtracker_verdict.go`, delete the `settleNeverLaunchedForVerdict(…)` call.
Run the Step 2 command. Expected: `NeverLaunchedDriveEarnsRetry` FAILS. Restore with `mv -f`.
Then move the call after `seam.LocateOuterDrive` (just before `switch len(ids)`); expected: FAILS
again. Restore.

- [ ] **Step 6: Measure the verdict shard**

`time bash tests/test_go_integration_app_runverdict.sh` solo must stay within 20 s; otherwise return
`BLOCKED` with the number.

- [ ] **Step 7: Commit**

```bash
git add internal/app/runtracker_verdict.go internal/app/runtracker_verdict_integration_test.go
git commit -m "feat(0491): the run-incomplete verdict settles never-launched drives before the outer scan"
```

---

### Task 4: `run.cancel` is keyed by the run key alone

**Risk:** standard — signature change with mechanical call-site updates; the authority checks
stay.

**Files:**
- Modify: `internal/app/runtracker_cancel.go` (`RunCancel`, `runCancel`, header AUTHORITY paragraph)
- Modify: `internal/cli/run.go` (`run cancel`: drop `--run-id`, its required mark, and the comment
  clause "validated against --run-id and a confirmed claim"; "All three flags are required" becomes
  "Both flags are required")
- Modify: `internal/cli/agent.go` (`runLifecycleCanceller`: drop the `runID` field and argument)
- Modify: `internal/app/runtracker_start.go` (`resumeIncumbentRemedy(runKey string)`, and the
  `RunSuperseded` settle remedy in `RunStart`)
- Modify: `internal/app/gate_drive.go` (`incumbentRemedyMessage` default arm)
- Test: every `runCancel(` / `RunCancel(` call site (`git grep -n -E '\b(run|Run)Cancel\(' -- '*.go'`);
  `internal/app/runtracker_cancel_integration_test.go`, `internal/cli/capability_production_test.go`,
  `internal/cli/run_tracker_rename_test.go`, the resume-locator tests that pin remedy text, and the
  `incumbentRemedyMessage` pins in `internal/app/gate_drive_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `func RunCancel(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key, reason string) RunCancelResult`;
  `func runCancel(seams cancelSeams, repoDir, key, reason string) RunCancelResult`;
  `func resumeIncumbentRemedy(runKey string) string`. The `run-id-mismatch` refusal no longer exists.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/run_tracker_rename_test.go` (a new function, so the hard-cut test keeps
its shape):

```go
// TestRunCancelIsKeyedByTheRunKeyAlone (change 0491): run cancel takes --key and
// --reason; --run-id is retired with no alias, so passing it exits 2.
func TestRunCancelIsKeyedByTheRunKeyAlone(t *testing.T) {
	if _, errS, code := runCLI(t, "--json", "run", "cancel", "--key", "k", "--run-id", "x", "--reason", "r"); code != 2 || !strings.Contains(errS, "unknown flag: --run-id") {
		t.Fatalf("run cancel --run-id: exit %d stderr %q, want exit 2 naming the unknown flag", code, errS)
	}
}
```

In `TestRunTrackerVocabularyHardCut`, change the row `{[]string{"run", "cancel"}, "run-id", "epoch"},`
to:

```go
		// change 0491: the run id is retired; run cancel is keyed by --key alone.
		{[]string{"run", "cancel"}, "key", "run-id"},
		{[]string{"run", "cancel"}, "key", "epoch"},
```

and change the existing `--epoch` exit-2 check's argv to drop `"--run-id"`-free form if it carries
one (it does not today; leave it).

In `TestRepresentativeSignatures`, change the `run.cancel` pin and its comment:

```go
		// change 0491: human Stop keyed by the run key alone — the two required
		// flags sorted, then the optional repo dir; no positional tail.
		"run.cancel": "--key <key> --reason <reason> [--repo-dir <dir>]",
```

Add `"strings"` to the imports if needed.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/cli/ -run 'TestRunCancelIsKeyedByTheRunKeyAlone|TestRunTrackerVocabularyHardCut|TestRepresentativeSignatures'`
Expected: FAIL (the flag is registered and required).

- [ ] **Step 3: Implement**

In `internal/app/runtracker_cancel.go`:
- `RunCancel(ctx, deps, wdeps, repoDir, key, reason string)` delegates to
  `runCancel(productionCancelSeams(repoDir), repoDir, key, reason)`.
- `runCancel(seams, repoDir, key, reason string)`: delete the `expectRunID == "" || ep.RunID !=
  expectRunID` check and its `cancelRefused("run-id-mismatch")`. Rewrite step (2)'s comment to:
  "(2) Validate the remaining authority conditions: the record must carry a parent-held authority;
  a CONFIRMED claim binding for the run's change must exist, or the resume-verified proof (ADR-0128
  Decision 1)."
- Header AUTHORITY paragraph: delete "the presented run id must equal the record's public RunID,".

In `internal/cli/run.go`: delete the `--run-id` flag, `runID` read, and `MarkFlagRequired("run-id")`,
and call `app.RunCancel(c.Context(), deps, wdeps, repoDir, key, reason)`.

In `internal/cli/agent.go`: `runLifecycleCanceller` loses `runID`, and `CancelRun` calls
`app.RunCancel(c.ctx, app.PlanningDeps{}, app.WorkspaceDeps{}, c.repoDir, c.runKey, reason)`. Its
construction site drops `runID: runID` (the `runID` variable stays until Task 5).

In `internal/app/runtracker_start.go`:

```go
// resumeIncumbentRemedy renders the two remedies for a resume refused over a live
// incumbent run. A no-run-record resume start binds its run when started (change 0463),
// so the incumbent may be a start that was never dispatched. Nothing records whether an
// agent is using the run, so the remedy names both cases rather than guessing.
func resumeIncumbentRemedy(runKey string) string {
	return "if it was never dispatched or its agent has exited, cancel it with 'docket run cancel --key " +
		runKey + " --reason <why>' and resume after confirmed cancellation; " +
		"if its agent is still running, continue the live run via 'docket run verdict'"
}
```

Update both callers (`resumeActiveLocator`, `resumeWorktreeOwnerLocator`) to
`resumeIncumbentRemedy(runKey)`. In `RunStart`'s `case RunSuperseded:` remedy, change
`"settle it with 'docket run cancel --key "+oldKey+" --run-id "+oldEp.RunID+" --reason <why>', …`
to `"settle it with 'docket run cancel --key "+oldKey+" --reason <why>', …`.

In `internal/app/gate_drive.go` `incumbentRemedyMessage`'s default arm, change
`(--key <key> --run-id <id> --reason <why>)` to `(--key <key> --reason <why>)`.

- [ ] **Step 4: Update every call site and pin**

- `git grep -n -E '\b(run|Run)Cancel\(' -- '*.go'`: drop the run-id argument everywhere
  (`fx.runID`, `ep.RunID`, literals). This includes the Task 2 test
  `TestIntegrationRunCompletionProductionCensusMissingRunRoot`.
- Delete `TestIntegrationRunCancelRunCancelRefusedWrongRun` (its subject, the id comparison, is
  gone). Keep every other refused-authority test.
- `git grep -n -e '--run-id' -- '*_test.go'`: update every test pinning the remedy text of
  `resumeIncumbentRemedy`, the superseded settle, or `incumbentRemedyMessage` to the new
  `--key <key> --reason <why>` form, and add a `!strings.Contains(msg, "--run-id")` check to each.
- Run `git grep -n -e 'run-id-mismatch' -- internal/app/runtracker_cancel*.go`: expect no output.

- [ ] **Step 5: Run the tests**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/cli/
go test -count=1 ./internal/app/ -run 'TestIncumbent|TestMapDriveFailure|TestGateDrive'
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunCancel|TestIntegrationRunStart|TestIntegrationRunCompletion|TestIntegrationGateLifecycle|TestRaceIntegrationAppConcurrency)' ./internal/app/
go test -tags integration -count=1 ./internal/cli/
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app internal/cli
git commit -m "refactor(0491): run.cancel is keyed by the run key alone"
```

---

### Task 5: `agent.enter --run-key` alone; key-only guardian; id lookups and `expectRunID` deleted; refusal classifier renamed

**Risk:** standard — refactor of a dormant opt-in linkage plus a mechanical API narrowing.

**Files:**
- Rename: `git mv internal/app/runtracker_run_id_refusal.go internal/app/runtracker_run_refusal.go`;
  `git mv internal/app/runtracker_run_id_refusal_test.go internal/app/runtracker_run_refusal_test.go`
- Modify: `internal/app/runtracker_run_refusal.go` (delete `ReasonUnknownRunID`, `runIDLocator`,
  `CheckRunIDExists`, `CheckRunIDLinkage`; rename `ClassifyRunIDError` → `ClassifyRunRecordError`,
  `RunIDNextAction` → `RunRecordNextAction`; add `CheckRunKey`)
- Modify: `internal/app/runtracker_run_record.go` (`ErrRunIDMismatch` → `ErrRunRecordConflict`
  `"run-record-conflict"`; `RegisterRunParticipant`, `RecordRunParticipantTerminal`,
  `FenceRunCompleting` drop `expectRunID`; delete `runDirMatch`, `scanRunsByID`, `findRunDirByID`)
- Modify: `internal/app/runtracker_complete.go` (`FenceRunCompleting(repoDir, runKey)` call)
- Modify: `internal/app/agent_guardian.go` (delete `guardianRunIDEnv`, `errGuardianRunIDMismatch`;
  `guardianFenceAndReap(repoDir, runKey string)`; `SpawnAgentGuardian(executable, repoDir, runKey, markerPath string)`;
  header AUTHORITY paragraph)
- Modify: `internal/app/agent_enter.go` (doc comment naming `--run-key`/`--run-id`)
- Modify: `internal/cli/agent.go` (drop `--run-id`; `--run-key` alone preflights and links; extract
  `runLinkage` / `runLinkageFor`; `runIDRefusal` → `runLinkageRefusal`; registrar/recorder lose `runID`;
  `spawnAgentDeathGuardian(repoDir, runKey string)`)
- Test: `internal/app/runtracker_run_refusal_test.go`, `internal/app/runtracker_run_record_integration_test.go`,
  `internal/app/agent_guardian_integration_test.go`, `internal/app/root_entry_integration_test.go`,
  every `RegisterRunParticipant` / `RecordRunParticipantTerminal` / `FenceRunCompleting` caller,
  `internal/cli/agent_test.go`, `internal/cli/run_tracker_rename_test.go`

**Interfaces:**
- Consumes: Task 4's `RunCancel(ctx, deps, wdeps, repoDir, key, reason)`.
- Produces:
  - `func RegisterRunParticipant(repoDir, runKey string, p RunParticipant) error`
  - `func RecordRunParticipantTerminal(repoDir, runKey, handle, turn, status string) error`
  - `func FenceRunCompleting(repoDir, runKey string) (runState, error)`
  - `const ErrRunRecordConflict RunErrorKind = "run-record-conflict"`
  - `func CheckRunKey(repoDir, runKey string) error`
  - `func ClassifyRunRecordError(err error) (Result, string, bool)`
  - `func RunRecordNextAction(reason string) string`
  - `func SpawnAgentGuardian(executable, repoDir, runKey, markerPath string) (*GuardianHandle, error)`
  - cli: `type runLinkage struct{ registrar codexentry.ParticipantRegistrar; terminal codexentry.TerminalRecorder; canceller codexentry.LifecycleCanceller }`,
    `func runLinkageFor(ctx context.Context, repoDir, runKey string, isRootCoordinator bool) runLinkage`
  - `RunRecord.RunID` still exists (Task 7 deletes it); nothing reads it after this task except
    `run.start`'s printing and locators.

- [ ] **Step 1: Write the failing tests**

(a) Replace the contents of `internal/app/runtracker_run_refusal_test.go` (after the `git mv`) with:

```go
package app

import (
	"fmt"
	"strings"
	"testing"
)

// TestClassifyRunRecordError (change 0491; was change 0463's ClassifyRunIDError):
// every run registry failure maps to a stable protocol result and its own kind as
// the reason token. A corrupt or unreadable record is an internal error. A
// presented value wrapped into the chain never leaks into the reason.
func TestClassifyRunRecordError(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	for _, tc := range []struct {
		kind RunErrorKind
		want Result
	}{
		{ErrRunNotFound, ResultInvalidInput},
		{ErrRunRecordConflict, ResultInvalidInput},
		{ErrRunNotActive, ResultInvalidInput},
		{ErrRunRecordCorrupt, ResultInternalError},
		{ErrRunRecordIO, ResultInternalError},
	} {
		err := fmt.Errorf("refused %s: %w", presented, runErr(tc.kind, "op", nil))
		res, reason, ok := ClassifyRunRecordError(err)
		if !ok || res != tc.want || reason != string(tc.kind) {
			t.Errorf("%s: got (%s, %q, %v), want (%s, %q, true)", tc.kind, res, reason, ok, tc.want, tc.kind)
		}
		if strings.Contains(reason, presented) {
			t.Errorf("%s: reason leaked the presented value", tc.kind)
		}
	}
	if _, _, ok := ClassifyRunRecordError(fmt.Errorf("plain")); ok {
		t.Error("an error with no *RunError must fall through (ok=false)")
	}
}

// TestRunRecordNextAction (change 0491): run-not-found tells the caller to pass the
// run key run.start printed, never the run context; it names no retired run id.
// Other reasons carry no invented message.
func TestRunRecordNextAction(t *testing.T) {
	msg := RunRecordNextAction(string(ErrRunNotFound))
	if !strings.Contains(msg, "--run-key") || !strings.Contains(msg, "run context") {
		t.Fatalf("run-not-found next action must name --run-key and warn off the run context, got %q", msg)
	}
	if strings.Contains(msg, "--run-id") || strings.Contains(msg, "<run-id>") {
		t.Fatalf("the next action names the retired run id: %q", msg)
	}
	if got := RunRecordNextAction(string(ErrRunRecordConflict)); got != "" {
		t.Errorf("run-record-conflict carries no invented message, got %q", got)
	}
}
```

(b) In `internal/app/runtracker_run_record_integration_test.go`, delete
`TestIntegrationRunRecordCheckRunIDLinkage` and `TestIntegrationRunRecordFenceRunCompletingRejectsStaleLocator`,
and add:

```go
// TestIntegrationRunRecordCheckRunKey (change 0491): agent.enter's --run-key
// preflight. A key with no record, a malformed key, or a key whose run record was
// never minted is run-not-found; a corrupt run record keeps its kind; a key whose
// run record loads passes. It reads only and never checks liveness.
func TestIntegrationRunRecordCheckRunKey(t *testing.T) {
	repo := newGitRepoForRunRecord(t)
	bare := runTrackerMintStarted(t, repo, nil, 1, "ha")
	withRun := runTrackerMintStarted(t, repo, nil, 1, "hb")
	if _, err := MintRunRecord(repo, withRun, "491"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	for _, tc := range []struct{ name, key string }{
		{"no run record", bare},
		{"unknown key", "00000000000000000000000000000491"},
		{"malformed key", "not/a-key"},
	} {
		if err := CheckRunKey(repo, tc.key); !isRunKind(err, ErrRunNotFound) {
			t.Errorf("%s: CheckRunKey = %v, want run-not-found", tc.name, err)
		}
	}
	if err := CheckRunKey(repo, withRun); err != nil {
		t.Fatalf("a key with a run record must pass, got %v", err)
	}
	corruptRunRecord(t, repo, withRun)
	if err := CheckRunKey(repo, withRun); !isRunKind(err, ErrRunRecordCorrupt) {
		t.Fatalf("a corrupt run record must keep its kind, got %v", err)
	}
}
```

Use the repository/fixture helpers the surrounding run-record integration tests already use: the
names `newGitRepoForRunRecord` and `corruptRunRecord` above are placeholders for them. Read the
file, and if no helper corrupts `run.json`, write `{not json` to
`<common>/docket/run-tracker/<key>/run.json`.

(c) In `internal/cli/agent_test.go`:
- Change `TestAgentEnterCapabilitySignature`'s `want` to
  `"--approval-policy <policy> --cwd <dir> --request <file> --role <name> --sandbox <mode> [--run-key <key>] [--worktree <dir>]"`.
- Replace `TestAgentEnterRefusesBadRunIDLinkageBeforeLaunch` with
  `TestAgentEnterRefusesUnknownRunKeyBeforeLaunch`. Keep the same stub-codex marker setup; the
  cases are `{"run key with no run record", bare}` and `{"unknown run key", "00000000000000000000000000000491"}`,
  both expecting `(ResultInvalidInput, "run-not-found")`. Pass `--run-key <key>` only. The human
  output must contain `--run-key` and must not contain `--run-id`. Codex is never launched.
- Replace `TestAgentEnterLoneRunIDIsPreflightedBeforeLaunch` with:

```go
// TestAgentEnterLoneRunKeyReachesLaunch (change 0491): a lone --run-key that names a
// run passes the preflight and reaches the launch (the stub codex records the
// invocation). Before 0491 a lone --run-key was silently ignored.
func TestAgentEnterLoneRunKeyReachesLaunch(t *testing.T) {
	seedAgentInstallation(t)
	repo := gateDriveRepo(t)
	bin := testsupport.TempDir(t)
	marker := filepath.Join(bin, "codex-invoked")
	stub := "#!/bin/sh\ntouch '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	key, err := app.MintRunTrackerRecord(repo, app.RunTrackerRecord{Target: "docket-implement-next", AttemptLimit: 1, Retry: app.RetryUnused, Disposition: "run-started"})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	if _, err := app.MintRunRecord(repo, key, "491"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	var out, stderr bytes.Buffer
	Run([]string{"agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", repo,
		"--approval-policy", "never", "--sandbox", "workspace-write", "--run-key", key, "--json"},
		strings.NewReader("req"), &out, &stderr, devInfo(), hostFacts())
	var res app.AgentEnterResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decode %q: %v (stderr %q)", out.String(), err, stderr.String())
	}
	if res.Reason == "run-not-found" {
		t.Fatalf("a lone --run-key naming a run was refused: %+v", res)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("a lone --run-key naming a run must reach launch; codex not invoked (result %+v)", res)
	}
}

// TestAgentEnterRejectsRunIDFlag (change 0491): --run-id is retired with no alias.
func TestAgentEnterRejectsRunIDFlag(t *testing.T) {
	_, errS, code := runCLI(t, "agent", "enter", "--role", "docket-implement-next", "--request", "-",
		"--cwd", testsupport.TempDir(t), "--approval-policy", "never", "--sandbox", "workspace-write",
		"--run-id", "0790b760e26444866ef2e156ba383326")
	if code != 2 || !strings.Contains(errS, "unknown flag: --run-id") {
		t.Fatalf("exit %d stderr %q, want exit 2 naming the unknown --run-id flag", code, errS)
	}
}

// TestRunLinkageFor (change 0491): --run-key alone wires what both flags did
// together — participant registration and terminal recording by key, and for a
// root coordinator the lifecycle cancel. An empty key wires nothing. A feature
// child gets no cancellation authority: the flag registers, it does not confer.
func TestRunLinkageFor(t *testing.T) {
	ctx := context.Background()
	if l := runLinkageFor(ctx, "/repo", "", true); l.registrar != nil || l.terminal != nil || l.canceller != nil {
		t.Fatalf("an empty --run-key must wire nothing, got %+v", l)
	}
	child := runLinkageFor(ctx, "/repo", "k1", false)
	if reg, ok := child.registrar.(runParticipantRegistrar); !ok || reg.runKey != "k1" || reg.kind != "task" {
		t.Fatalf("a lone --run-key must register a task participant by key, got %#v", child.registrar)
	}
	if term, ok := child.terminal.(runTerminalRecorder); !ok || term.runKey != "k1" {
		t.Fatalf("a lone --run-key must record terminal evidence by key, got %#v", child.terminal)
	}
	if child.canceller != nil {
		t.Fatalf("a feature child must receive no cancellation authority, got %#v", child.canceller)
	}
	root := runLinkageFor(ctx, "/repo", "k1", true)
	if reg, ok := root.registrar.(runParticipantRegistrar); !ok || reg.kind != "coordinator" {
		t.Fatalf("a root coordinator registers as coordinator, got %#v", root.registrar)
	}
	if c, ok := root.canceller.(runLifecycleCanceller); !ok || c.runKey != "k1" {
		t.Fatalf("a root coordinator gets the lifecycle canceller by key, got %#v", root.canceller)
	}
}
```

- In `TestRunTrackerVocabularyHardCut`, replace `{[]string{"agent", "enter"}, "run-id", "run-epoch"},`
  with `{[]string{"agent", "enter"}, "run-key", "run-id"},` and
  `{[]string{"agent", "enter"}, "run-key", "run-epoch"},`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestClassifyRunRecordError|TestRunRecordNextAction'` and
`go test -count=1 ./internal/cli/ -run 'TestAgentEnter|TestRunLinkageFor|TestRunTrackerVocabularyHardCut'`
Expected: compile FAIL (`ClassifyRunRecordError`, `ErrRunRecordConflict`, `runLinkageFor` undefined).

- [ ] **Step 3: Implement the app side**

In `internal/app/runtracker_run_record.go`:

```go
	// ErrRunRecordConflict: the run record is already bound to a different change
	// or worktree (bind-run-change, bind-run-worktree), or the presented terminal
	// evidence is malformed or conflicts with what is recorded
	// (record-participant-terminal). Recorded state is never silently re-pointed.
	ErrRunRecordConflict RunErrorKind = "run-record-conflict"
```

replacing the `ErrRunIDMismatch` constant. Replace every `ErrRunIDMismatch` use with
`ErrRunRecordConflict`, and correct the doc comments that call it "a stale locator". Drop the
`expectRunID` parameter and its `rec.RunID != expectRunID` check from `RegisterRunParticipant`,
`RecordRunParticipantTerminal`, and `FenceRunCompleting`, along with the sentences in their doc
comments about the expected locator. Delete `runDirMatch`, `scanRunsByID`, and `findRunDirByID`
with their comments. In `internal/app/runtracker_complete.go`, call
`FenceRunCompleting(repoDir, runKey)`.

`internal/app/runtracker_run_refusal.go` becomes:

```go
package app

// This file is the run refusal vocabulary (change 0463; key-only since change 0491).
// agent.enter's optional --run-key names a run; when the presented key cannot be
// resolved, the caller learns WHICH mistake it made through a stable token — the
// run registry error's own kind. Tokens are a fixed vocabulary; nothing here echoes
// the presented value, a path, or record content.

// ClassifyRunRecordError maps a run registry failure (a *RunError anywhere in
// err's chain) to a protocol result and a bounded reason token, the error's own
// kind: a corrupt or unreadable record is internal-error; every other
// readable-but-unusable state (run-not-found, run-record-conflict, run-not-active,
// …) is invalid-input. ok is false when err carries no *RunError, so callers fall
// through to their own classification.
func ClassifyRunRecordError(err error) (Result, string, bool) {
	ee, ok := AsRunError(err)
	if !ok {
		return "", "", false
	}
	switch ee.Kind {
	case ErrRunRecordCorrupt, ErrRunRecordIO:
		return ResultInternalError, string(ee.Kind), true
	default:
		return ResultInvalidInput, string(ee.Kind), true
	}
}

// RunRecordNextAction maps a run refusal reason to a one-line, credential-free next
// action (the ownershipNextAction pattern). It never echoes the presented value. A
// reason with no specific remedy yields "", and callers then omit the message.
func RunRecordNextAction(reason string) string {
	switch reason {
	case string(ErrRunNotFound):
		return "the --run-key value names no run in this repository; pass the run key run.start printed, " +
			"never the run context (that goes to --run-context on change claim and the gate drive)"
	default:
		return ""
	}
}

// CheckRunKey verifies, before agent.enter spawns anything, that --run-key names a
// run in repoDir's repository (change 0491; it replaces change 0463's
// CheckRunIDLinkage). It returns nil when the key's run record loads and otherwise
// ALWAYS a *RunError: ErrRunNotFound for a key with no directory, a malformed key,
// or no run record; ErrRunRecordCorrupt for a corrupt record; ErrRunRecordIO for
// any other fault. It only reads and never checks liveness: participant
// registration still refuses a non-active run.
func CheckRunKey(repoDir, runKey string) error {
	_, _, err := LoadRunRecord(repoDir, runKey)
	if err == nil {
		return nil
	}
	if _, ok := AsRunError(err); ok {
		return err
	}
	if ge, ok := AsRunTrackerStoreError(err); ok && (ge.Kind == ErrRunTrackerNotFound || ge.Kind == ErrRunTrackerMalformedKey) {
		return runErr(ErrRunNotFound, "check-run-key", nil)
	}
	return runErr(ErrRunRecordIO, "check-run-key", err)
}
```

In `internal/app/agent_guardian.go`: delete `guardianRunIDEnv` and `errGuardianRunIDMismatch` (drop
the `errors` import only if nothing else uses it — `SpawnAgentGuardian` uses `errors.Is`);
`RunAgentGuardianFromEnv` stops reading the run id and calls `guardianFenceAndReap(repoDir, runKey)`;
`guardianFenceAndReap(repoDir, runKey string)` deletes the id comparison (the CAS keeps only the
`RunActive → RunCancelling` flip). `SpawnAgentGuardian(executable, repoDir, runKey, markerPath string)`
stops setting the run-id env. Rewrite the doc comments: the guardian "carries the run key (a
locator, not a credential)"; "it verifies the run id before writing so a stale guardian cannot
fence a successor run" becomes "keys are bind-once and never reused, so the key alone cannot fence
a successor run".

In `internal/app/agent_enter.go`, change the comment's `--run-key`/`--run-id` to `--run-key`.

- [ ] **Step 4: Implement the CLI side**

In `internal/cli/agent.go`:
- Change `var runKey, runID string` to `var runKey string`. Delete the `--run-id` flag line, and
  change the `--run-key` help to `"run `key` for lifecycle registration (optional; a locator, not a credential)"`.
- Replace the whole linkage block (from `isRootCoordinator := …` through the closing `}` of
  `if runKey != "" && runID != "" { … }`) with:

```go
			// Optional lifecycle linkage (change 0375 Task 13; key-only since change
			// 0491): --run-key registers this entry's thread as a run participant and,
			// for a root coordinator only, connects a catchable Stop to the run's
			// cancellation path and spawns the detached death guardian for an
			// uncatchable death. No documented flow passes it, so the linkage stays
			// dormant unless a caller opts in.
			isRootCoordinator := contract.LaunchPosture == harness.LaunchRootCoordinator
			if runKey != "" {
				// Preflight the key BEFORE anything is spawned: an unknown key refuses
				// run-not-found instead of surfacing as a generic root-entry failure
				// after Codex already started a thread.
				if lerr := app.CheckRunKey(effectiveCWD, runKey); lerr != nil {
					res, reason, _ := app.ClassifyRunRecordError(lerr)
					setResult(runLinkageRefusal(role, res, reason))
					return nil
				}
				link := runLinkageFor(c.Context(), effectiveCWD, runKey, isRootCoordinator)
				client.Registrar, client.Terminal, client.Canceller = link.registrar, link.terminal, link.canceller
				if isRootCoordinator {
					if guardian, gerr := spawnAgentDeathGuardian(effectiveCWD, runKey); gerr == nil {
						defer guardian.Complete()
					}
				}
			}
```

- In the `client.Enter` error branch, call `app.ClassifyRunRecordError` and `runLinkageRefusal`.
- Add:

```go
// runLinkage is the optional lifecycle linkage --run-key wires into one entry.
type runLinkage struct {
	registrar codexentry.ParticipantRegistrar
	terminal  codexentry.TerminalRecorder
	canceller codexentry.LifecycleCanceller
}

// runLinkageFor returns the linkage for runKey (change 0491): none for an empty key;
// otherwise participant registration and terminal recording by key, plus — for a
// root coordinator only — the catchable-Stop cancellation. A feature child
// registers but receives no cancellation authority.
func runLinkageFor(ctx context.Context, repoDir, runKey string, isRootCoordinator bool) runLinkage {
	if runKey == "" {
		return runLinkage{}
	}
	kind := "task"
	if isRootCoordinator {
		kind = "coordinator"
	}
	l := runLinkage{
		registrar: runParticipantRegistrar{repoDir: repoDir, runKey: runKey, kind: kind},
		terminal:  runTerminalRecorder{repoDir: repoDir, runKey: runKey},
	}
	if isRootCoordinator {
		l.canceller = runLifecycleCanceller{ctx: ctx, repoDir: repoDir, runKey: runKey}
	}
	return l
}
```

- `runParticipantRegistrar` and `runTerminalRecorder` lose `runID`, and call
  `app.RegisterRunParticipant(r.repoDir, r.runKey, app.RunParticipant{…})` and
  `app.RecordRunParticipantTerminal(r.repoDir, r.runKey, handle, turnID, status)`. Fix their doc
  comments (no expected locator).
- Rename `runIDRefusal` to `runLinkageRefusal` and have it use `app.RunRecordNextAction`.
- `spawnAgentDeathGuardian(repoDir, runKey string)` calls
  `app.SpawnAgentGuardian(exe, repoDir, runKey, marker)`.

- [ ] **Step 5: Update the remaining callers and pins**

- `git grep -n -E 'RegisterRunParticipant\(|RecordRunParticipantTerminal\(|FenceRunCompleting\(|SpawnAgentGuardian\(' -- '*.go'`:
  drop the id argument everywhere (tests pass `fx.runID`, `ep.RunID`, `runID`, `""`). In
  `internal/app/root_entry_integration_test.go`, `runLifecycleFixture` loses `runID`.
- `git grep -n -E 'ErrRunIDMismatch|run-id-mismatch|ClassifyRunIDError|RunIDNextAction|CheckRunID|ReasonUnknownRunID|unknown-run-id|findRunDirByID|scanRunsByID|runIDLocator|runIDRefusal|guardianRunIDEnv|DOCKET_AGENT_GUARDIAN_RUN_ID|errGuardianRunIDMismatch' -- '*.go'`:
  re-point every bind/terminal pin from `run-id-mismatch` to `run-record-conflict`, and delete the
  remaining hits. Expect no output after.

- [ ] **Step 6: Run the tests**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/cli/
go test -count=1 ./internal/app/ -run 'TestClassifyRunRecordError|TestRunRecordNextAction|TestRunTracker'
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunRecord|TestIntegrationGateLifecycle|TestIntegrationWorkflowLifecycle|TestIntegrationRunCompletion|TestIntegrationRunCancel|TestIntegrationRunVerdict)' ./internal/app/
go test -tags integration -count=1 ./internal/cli/
```
Expected: PASS.

- [ ] **Step 7: Mutation-test the linkage**

With a backup copy of `internal/cli/agent.go`, change `runLinkageFor`'s first line to
`if runKey == "" || true {` (the pre-0491 "lone --run-key is ignored" behaviour). Run
`go test -count=1 ./internal/cli/ -run TestRunLinkageFor`. Expected: FAIL. Restore with `mv -f`.

- [ ] **Step 8: Commit**

```bash
git add internal/app internal/cli
git commit -m "refactor(0491): agent.enter links by --run-key alone; guardian and participant APIs are key-only; run-record-conflict"
```

---

### Task 6: `stale-run-id` becomes `run-superseded`

**Risk:** economy — a pure token rename with pinned tests.

**Files:**
- Modify: `internal/app/runtracker_fence.go` (`ErrStaleRunID` → `ErrRunSuperseded`, `Reason:
  "run-superseded"`; `fenceRefusalReasonMessage` case; `admitWorkflowMutation`; comments)
- Modify: `internal/app/runtracker_verdict.go` (`ReasonStaleRunID` → `ReasonRunSuperseded =
  "run-superseded"`; comments naming the token)
- Modify: `internal/app/runtracker_complete.go` (return `ReasonRunSuperseded`; comments)
- Modify: `internal/app/gate_drive.go` (`mapDriveFailure` comment, if it names the token)
- Test: every pin — `git grep -n -E 'stale-run-id|StaleRunID' -- '*.go'`
  (`TestIntegrationRunFenceFenceRefusesSupersededRunAsStale`,
  `TestIntegrationRunFenceRunCarryingFencesUnchangedByOwnerSelection`,
  `TestIntegrationRunCompletionCompleteSuccessfulRunNeverRelabelsCancellation`,
  `TestPRFenceRefusalMessageIsReasonAware`, `TestWorkspaceFenceRefusalMessageIsReasonAware`,
  `TestMapDriveFailureFenceReasons`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `var ErrRunSuperseded = &MutationFenceError{Reason: "run-superseded"}`;
  `const ReasonRunSuperseded = "run-superseded"`.

- [ ] **Step 1: Re-point the pins first (RED)**

In each pinning test, replace the expected token `stale-run-id` with `run-superseded` and the
symbols `ErrStaleRunID` / `ReasonStaleRunID` with `ErrRunSuperseded` / `ReasonRunSuperseded`.
Rename `TestIntegrationRunFenceFenceRefusesSupersededRunAsStale` to
`TestIntegrationRunFenceFenceRefusesSupersededRun`. In `TestMapDriveFailureFenceReasons`, the case
becomes `{"run-superseded", ErrRunSuperseded}`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestMapDriveFailureFenceReasons|TestPRFenceRefusalMessageIsReasonAware|TestWorkspaceFenceRefusalMessageIsReasonAware'`
Expected: compile FAIL (`ErrRunSuperseded` undefined).

- [ ] **Step 3: Rename**

```go
	// ErrRunSuperseded: the owning run was superseded by a confirmed resume —
	// its replacement now owns the worktree, and this run's mutations are refused.
	ErrRunSuperseded = &MutationFenceError{Reason: "run-superseded"}
```

```go
	// ReasonRunSuperseded: a superseded run — a confirmed resume replaced it.
	ReasonRunSuperseded = "run-superseded"
```

In `completeSuccessfulRun`, `case RunSuperseded: return false, ReasonRunSuperseded, nil`. In
`fenceRefusalReasonMessage`, `case "run-cancelled", "run-superseded":`. Then run
`git grep -n -E 'stale-run-id|StaleRunID|stale run identity' -- '*.go'` and rename or reword every
remaining hit, comments included. Expect no output after.

- [ ] **Step 4: Run the tests**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/app/ -run 'TestMapDriveFailure|FenceRefusalMessage'
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunFence|TestIntegrationRunCompletion|TestIntegrationRunVerdict)' ./internal/app/
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "refactor(0491): stale-run-id becomes run-superseded"
```

---

### Task 7: Retire `RunRecord.RunID`; `run-started <key> <run-context>`; `run.start` declares `process-control`

**Risk:** standard — the last run-id reader and the start line's shape; many fixtures.

**Files:**
- Modify: `internal/app/runtracker_run_record.go` (`RunRecord.RunID` field and doc, header IDENTITY
  paragraph, `MintRunRecord`)
- Modify: `internal/app/runtracker_start.go` (`RunStartResult.RunID`, `HumanText`,
  `startedRunResult`, `resumeActiveLocator`, `resumeWorktreeOwnerLocator`, the `RunCancelling` /
  `RunCompleting` / `RunCompleted` messages in `RunStart`, `armResumeReplacement`, the file header
  and step comments, `runUntrackedMsg` doc)
- Modify: `internal/app/runtracker_fence.go` (header "AUTHORITY vs. LOCATOR")
- Modify: `internal/cli/run.go` (`run start` Short text and comment; `Annotations:
  capability("run.start", EffectLocalWrite, EffectProcessControl)` with a comment)
- Test: `internal/app/runtracker_start_integration_test.go`,
  `internal/app/runtracker_start_resume_integration_test.go`,
  `internal/app/runtracker_no_run_record_resume_e2e_integration_test.go`,
  `internal/app/runtracker_storage_reset_integration_test.go`,
  `internal/app/runtracker_production_census_integration_test.go`,
  `internal/app/runtracker_cancel_helpers_test.go` (`cancelFixture.runID`),
  `internal/app/runtracker_fence_helpers_test.go`, `internal/app/runtracker_verdict_helpers_test.go`
  (`verdictCompletionFixture.runID`), `internal/app/agent_guardian_integration_test.go`,
  `internal/app/runtracker_start_result_json_test.go`, `internal/cli/agent_test.go`,
  `internal/cli/capability_production_test.go`, plus whatever the compiler names

**Interfaces:**
- Consumes: Tasks 4–5 (no remaining reader of `RunRecord.RunID` outside `run.start`).
- Produces: `RunRecord` has no `RunID`; `RunStartResult` has no `RunID` / `run_id`;
  `func startedRunResult(key, runContext string) RunStartResult`; `HumanText` prints
  `run-started <key> <run-context>`; `run.start` effects are `local-write process-control`.

- [ ] **Step 1: Write the failing tests**

(a) In `internal/app/runtracker_start_integration_test.go`, rename
`TestIntegrationRunStartStartedLineIsAlwaysThreeTokens` to
`TestIntegrationRunStartStartedLineIsTwoTokens` and change its assertion to exactly two tokens after
`run-started` (`<key> <run-context>`), with the key equal to `res.Key` and the context equal to
`res.RunContext`. Rename `TestIntegrationRunStartFreshStartSurfacesRunID` to
`TestIntegrationRunStartFreshStartSurfacesKeyAndContext`: assert `res.Key` and `res.RunContext`
non-empty, and that the JSON (`json.Marshal(res)`) contains no `"run_id"`.

(b) In `internal/app/runtracker_start_result_json_test.go` add:

```go
// TestRunStartResultCarriesNoRunID (change 0491): the run id is retired; the
// run.start result names only the key and the run context, and its started line is
// two tokens.
func TestRunStartResultCarriesNoRunID(t *testing.T) {
	res := startedRunResult("k0491", "ctx-token")
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(buf), `"run_id"`) {
		t.Errorf("run.start JSON still carries the retired run_id key: %s", buf)
	}
	if got := strings.SplitN(res.HumanText(), "\n", 2)[0]; got != "run-started k0491 ctx-token" {
		t.Errorf("started line = %q, want %q", got, "run-started k0491 ctx-token")
	}
	if r := startedRunResult("", "ctx-token"); r.Started || r.Reason != ReasonRunMintFailed {
		t.Errorf("a start with no key must fail closed run-untracked mint-failed, got %+v", r)
	}
}
```

(c) Review Focus 5 — in `internal/app/runtracker_cancel_integration_test.go` add:

```go
// TestIntegrationRunCancelOldRecordWithRunIDCancelsByKey (change 0491, Decision 8,
// Review Focus 5): a run started by the pre-0491 binary carries run_id in its
// run.json. No migration runs: the record still loads (unknown keys are ignored),
// and run.cancel cancels it by its key alone.
func TestIntegrationRunCancelOldRecordWithRunIDCancelsByKey(t *testing.T) {
	fx := newCancelFixture(t)
	path := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key, runRecordFileName)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(buf, &stored); err != nil {
		t.Fatalf("decode run.json: %v", err)
	}
	stored["record"].(map[string]any)["run_id"] = "0790b760e26444866ef2e156ba383326"
	old, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadRunRecord(fx.repo, fx.key); err != nil {
		t.Fatalf("an old run.json carrying run_id must still load, got %v", err)
	}
	res := runCancel(okCancelSeams(fx), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel by key = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
}
```

Use the permissive seam constructor the other cancel integration tests use in place of
`okCancelSeams(fx)` (read the file), or `productionCancelSeams(fx.repo)` behind
`requireProcessSupervisorHere(t)`.

(d) In `internal/cli/capability_production_test.go`, add after the signature loop in
`TestRepresentativeSignatures`:

```go
	// change 0491: run.start's resume quiescence check runs the stop-capable census
	// on a cancelled or superseded predecessor, so it declares process-control.
	if e, ok := entryByID(entries, "run.start"); !ok || strings.Join(e.Effects, " ") != "local-write process-control" {
		t.Errorf("run.start effects = %v, want [local-write process-control]", e.Effects)
	}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestRunStartResult'` and
`go test -count=1 ./internal/cli/ -run TestRepresentativeSignatures`
Expected: compile FAIL (`startedRunResult` takes three arguments) and the effect pin FAILS.

- [ ] **Step 3: Implement**

`internal/app/runtracker_run_record.go`:
- Delete the `RunID` field from `RunRecord` and "RunID is the random public locator." from its doc.
- Rewrite the header's IDENTITY paragraph to: "IDENTITY vs. AUTHORITY: the run key locates the run
  (bind-once, never reused) and authorizes nothing — the run context's child capability continues
  to carry authority, per ADR-0111. Since change 0491 the run carries no separate run id; an old
  run.json that still has a run_id key decodes unchanged, because unknown keys are ignored."
- `MintRunRecord`: delete the `runID, err := runToken()` block and the `RunID: runID,` field; its doc
  ends "On success it returns the persisted active record (its Generation stamped)."

`internal/app/runtracker_start.go`:
- Delete `RunStartResult.RunID` and its comment.
- `HumanText`: `line := "run-started " + r.Key + " " + r.RunContext`. Its doc: "A started run prints
  `run-started <key> <run-context>`".
- Replace `startedRunResult`:

```go
// startedRunResult builds the started report for key. A started run always carries
// its key and run context, so the positional `run-started <key> <run-context>` line
// is always two tokens; an empty either fails closed as run-untracked mint-failed.
func startedRunResult(key, runContext string) RunStartResult {
	if key == "" || runContext == "" {
		return runUntracked(ReasonRunMintFailed)
	}
	return newRunStartResult(ResultApplied, RunStartResult{
		Started:        true,
		Key:            key,
		Target:         runStartStoredTarget,
		RunContext:     runContext,
		OwnerLifecycle: ReasonOwnerLifecycleUnavailable,
	})
}
```

- `resumeActiveLocator`: `"change " + ep.ChangeID + " has an active run (run key " + runKey + "); " + resumeIncumbentRemedy(runKey)`.
- `resumeWorktreeOwnerLocator`: `… + " (state " + string(ep.State) + ", run key " + runKey + "); " + resumeIncumbentRemedy(runKey)`.
- In `RunStart`, the three messages that print `(run "+oldEp.RunID+")` print `(run key "+oldKey+")` instead.
- `armResumeReplacement` and `RunStart` step (6a)/(7): `if _, eerr := MintRunRecord(…); eerr != nil {…}`
  and `return startedRunResult(key, grant.ChildCapability)`. Rewrite the comments that mention the
  run id: (6a) "Every started run tracker binds a run beside the just-minted gate record, keyed by
  the run key"; (7) "Report the started run with its key, its run context, and the honest
  owner-lifecycle caveat … the line is always two tokens."
- File header and `RunStart` doc: `run-started <key> <run-context>`. `runUntrackedMsg` doc: "only
  public locators (a run key, a change id)".

`internal/app/runtracker_fence.go` header: "AUTHORITY vs. LOCATOR. The run key is a locator, never a
credential (ADR-0111)".

`internal/cli/run.go`: Short `"Start a tracked run for a dispatched workflow and print run-started <key> <run-context>"`;
the comment above it likewise; and:

```go
		// local-write: mints the durable run-tracker record AND the outer recovery-scope
		// record under the Git common dir; the re-sync is a read-only fetch.
		// process-control: a resume over a cancelled or superseded predecessor re-proves
		// its quiescence with the stop-capable launch census (change 0491).
		Annotations: capability("run.start", EffectLocalWrite, EffectProcessControl),
```

- [ ] **Step 4: Update every remaining run-id reader**

Let the compiler list them: `go vet ./... && go vet -tags integration ./...`. Delete
`cancelFixture.runID`, `verdictCompletionFixture.runID`, and the fixture fields in
`runtracker_fence_helpers_test.go`; replace every `res.RunID` / `started.RunID` /
`rec.RunID` / `ep.RunID` use (for example `!started.Started || started.RunID == ""` becomes
`!started.Started || started.Key == ""`). Re-point the resume-locator tests and
`TestIntegrationRunStartNoRunRecordResumeMintsBoundRun` (its JSON) to the key-only locators.
Re-point `TestIntegrationRunStartStorageResetIgnoresRetiredRoots` and the no-run-record resume e2e
parser to two tokens.

Then run the closing sweep, and sort each remaining hit by owner:

```bash
git grep -n -E '\bRunID\b|\brunID\b|run_id|run-id|<run-id>|run id' -- 'internal/**/*.go' 'cmd/**/*.go' 'tests/*.sh'
```

Every remaining hit must be the gate supervisor's id (see Global Constraints) or the
`old run.json` test above. Fix any other hit, comments included.

- [ ] **Step 5: Run the tests**

```bash
go build ./... && go vet ./... && go vet -tags integration ./...
go test -count=1 ./internal/cli/
go test -count=1 ./internal/app/ -run 'TestRunStartResult|TestRunTracker'
go test -tags integration -count=1 -timeout 30m -run '^(TestIntegrationRunStart|TestIntegrationRunCancel|TestIntegrationRunCompletion|TestIntegrationRunRecord|TestIntegrationRunFence|TestIntegrationGateLifecycle|TestIntegrationRunVerdict|TestIntegrationWorkflowLifecycle)' ./internal/app/
go test -tags integration -count=1 ./internal/cli/
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app internal/cli
git commit -m "refactor(0491): retire the run id; run-started prints <key> <run-context>; run.start declares process-control"
```

---

### Task 8: Prose — the run-tracker block, skills, the Codex clause, the generated mirror, `AGENTS.md`, the prose guards, and the prose budgets

**Risk:** standard — prose and generated artifacts bound by drift guards; the generation order is
load-bearing.

**Files:**
- Modify: `cursor-rules/run-tracker.md` (the source of the `docket:dispatch` block)
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-implement-next/references/edge-paths.md`,
  `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`,
  `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `internal/harness/dispatch.go` (`CodexRootEntryClause`)
- Regenerate: `internal/assets/embedded/**` (`go generate ./internal/assets`)
- Regenerate (never hand-edit): `AGENTS.md` managed `docket:dispatch` block
- Rename and modify: `git mv internal/repoguard/gatedrive_run_id_thread_test.go internal/repoguard/gatedrive_change_id_thread_test.go`
- Modify: `internal/repoguard/gatedrive_task_absence_test.go` (planted line), `internal/repoguard/budgets_test.go`

**Interfaces:**
- Consumes: Tasks 1–7 (the code the prose now describes).
- Produces: the block says `run-started <key> <run-context>`, keeps both, copies only `<run-context>`,
  stops by `--key <key> --reason <why>`, and states that an operation id is not a command.

- [ ] **Step 1: Re-point the prose guards first (RED)**

In `internal/repoguard/gatedrive_change_id_thread_test.go` (after the `git mv`):
- Rewrite the header: "Every build-owned `gate.drive.start` paragraph (`--owner build`) in maintained
  workflow markdown carries `--change-id` — `GateDriveService.Start` charges `build.max_attempts`
  only for a build-owned start with a non-empty ChangeID, and `FindScopeDriveIDs` matches drives on
  ChangeID, so a build-owned start without it is unbudgeted and invisible to run.verdict (change
  0488 review). Change 0491 retired the run id; the retired-vocabulary seal keeps `--run-id` out."
  Keep the floor and site-discovery sentences.
- Rename `TestGateDriveRunIDThreaded` to `TestGateDriveBuildStartCarriesChangeID`. Delete
  `runIDFlagRe`, `carriesRunID`, and the `--run-id` violation; keep the `--change-id` violation and
  both population floors. In `non_vacuity`, use
  ``build := "the `gate.drive.start` operation with `--owner build --change-id <id> --json`"``,
  assert stripping `--change-id <id> ` is detected by `changeIDRe`, keep the `builder` / `task`
  boundary checks and the `--change-idx` check, delete the `--run-id-x` check, and change the
  wrapped example to carry `--change-id <id>`.
- Delete `runTrackerRunIDCopyRe`, `TestRunTrackerCopiesRunIDIntoDispatchPrompt`, `codexRequestRunIDRe`,
  and `TestCodexRequestFileCarriesRunID`. Their absence check moves to Task 10's seal (`<run-id>` and
  `--run-id` rows scan `AGENTS.md`, `cursor-rules/**`, and the Go string literal
  `CodexRootEntryClause`). Drop the `harness` import if it is now unused.
- In `internal/repoguard/gatedrive_task_absence_test.go`, change the planted line
  ``"the `gate.drive.start` operation with `--owner build --run-id <run-id> --json`"`` to
  ``"the `gate.drive.start` operation with `--owner build --change-id <id> --json`"``.

Run: `go test -count=1 ./internal/repoguard/ -run 'TestGateDriveBuildStartCarriesChangeID|TestNoTaskOwnedDriveInstructions'`
Expected: PASS (the guard no longer demands `--run-id`). The RED for this task is Step 4's
`TestCommittedCodexDispatchMatchesGenerator`.

- [ ] **Step 2: Rewrite `cursor-rules/run-tracker.md`**

Make exactly these edits; wrap to the file's existing ~100-column style.

(a) Intro paragraph — replace "The `docket` binary is on `PATH`; resolve each operation below from
the capability catalog. If it is missing, the install is broken: surface it, never rebuild the run
tracker by hand." with:

```
The `docket` binary is on `PATH`. An operation id below, such as `run.start`, is not a command:
look up its entry in `docket capabilities --json` and run that entry's `argv`. If the binary is
missing, the install is broken: surface it, never rebuild the run tracker by hand.
```

(b) Step 1 — replace its first five lines (through "flag (`agent.enter`, `gate drive start`). Add
`--resume <id>` to") with:

```
1. Before dispatching `docket-implement-next`, run `run.start` with `implement-next`. It prints
   `run-started <key> <run-context>`; keep both (they won't survive the next tool call) and copy
   the `<run-context>` into the dispatch prompt. Add `--resume <id>` to
```

and keep the rest of step 1 ("start a run that resumes …") unchanged.

(c) Stopping section — replace "with the key and run id\n`run.start` gave you, plus a human reason —
`--key <key> --run-id <id> --reason <why>`." with "with the key `run.start` gave\nyou, plus a human
reason — `--key <key> --reason <why>`." Replace "It fences the run so nothing new can attach to it,
then stops" with "It fences the run, then stops". Replace "- `refused` — the key, run id, or
repository did not match; nothing was touched." with "- `refused` — the key or repository did not
match; nothing was touched."

(d) Resuming section — replace "prints a locator naming the change, run id, and key, plus the exact
remedy:" with "prints a locator naming the change and key, plus the exact remedy:", and
"(`--key <key> --run-id <id> --reason <why>`)" with "(`--key <key> --reason <why>`)".

Then confirm: `grep -n -E -e 'run-id|run id' cursor-rules/run-tracker.md` prints nothing.

- [ ] **Step 3: Edit the skills and the Codex clause**

Each edit below names the phrase to replace. Line wraps in the files may split a phrase, so match
across the break and keep the file's wrap style.

- `skills/docket-implement-next/SKILL.md`, Step 6 evidence re-mint: replace
  "`--change-id <id> --run-id <run-id> --run-context <token> --json`" with
  "`--change-id <id> --run-context <token> --json`", and replace "`--run-id`/`--run-context` carry
  your dispatch prompt's values, omitted only when it carried none" with "`--run-context` carries
  your dispatch prompt's value, omitted only when it carried none".
- `skills/docket-implement-next/SKILL.md`, Step 7: replace the passage from "A gated parent's prompt
  may also carry a **run context** token;" through "Build-task workers receive neither value: they
  run their tests directly and call no gate operation." with:

```
A gated parent's prompt may also carry a **run context** token; pass it, always as `--run-context`, into every build-owned `gate.drive.start` (`--owner build` — Step 6's evidence re-mint and re-gates, the build role's final suite gate, and each post-repair attempt the build role starts) this run performs — each invoked with `--json` per the shared capture requirement and each always carrying `--change-id <id>` — and into the Step-2 claim, omitting it only when the prompt carried none (a `run-untracked` or ungated run). Build-task workers never receive it: they run their tests directly and call no gate operation.
```

- `skills/docket-implement-next/references/edge-paths.md`: "naming the change, run id, and run key"
  → "naming the change and run key"; "(`--key <key> --run-id <id> --reason <why>`)" →
  "(`--key <key> --reason <why>`)".
- `skills/docket-build/SKILL.md`: "No run context, run id, or capability goes into a worker prompt" →
  "No run context or capability goes into a worker prompt". In the `build_gate: local` item, replace
  "`--owner build --change-id <id> --run-id <run-id> --run-context <token> --json` (`--run-id`/`--run-context`
  only when your prompt carried them;" with "`--owner build --change-id <id> --run-context <token>
  --json` (`--run-context` only when your prompt carried it;". In the Red-path item 1, replace "it
  never runs the full suite, and its dispatch payload carries no run id." with "it never runs the
  full suite." In item 2, replace "`--owner build --change-id <id> --run-id <run-id> --run-context
  <token> --json`" with "`--owner build --change-id <id> --run-context <token> --json`".
- `skills/docket-build/references/gate-caller-loop.md`, the `start` row: "plus `--run-context
  <token>` and `--run-id <id>` when its prompt carried them;" → "plus `--run-context <token>` when
  its prompt carried it;".
- `skills/docket-finalize-change/references/gate-failure.md`: "with `--key <key> --run-id <id>
  --reason <why>` for a tracked run" → "with `--key <key> --reason <why>` for a tracked run".
- `internal/harness/dispatch.go` `CodexRootEntryClause`: replace "labeled for `--run-context` on
  `change.claim` and the gate drive, and the unchanged run id, labeled for `--run-id` on build-owned
  starts. Preserve" with "labeled for `--run-context` on `change.claim` and the gate drive.
  Preserve".

Then run `git grep -n -E -e '--run-id|<run-id>|run id' -- skills cursor-rules internal/harness/dispatch.go`
and expect no output.

- [ ] **Step 4: Regenerate the mirror, see the AGENTS.md drift (RED), then regenerate AGENTS.md**

```bash
go generate ./internal/assets
go run ./cmd/genassets -repo . -check
go test -count=1 ./internal/repoguard -run 'TestCommittedCodexDispatchMatchesGenerator$'
```
Expected: `genassets` writes `internal/assets/embedded/…` (the five skill mirrors, the run-tracker
mirror, and `manifest.json`), `-check` passes, and the drift test FAILS with `AGENTS.md dispatch
block is stale; regenerate it from the reposeed dispatch interior`.

Regenerate the block through the install path (verified at change 0488 against a scratch clone):
install refuses to overwrite a block it has no ownership record for, and its sanctioned remedy is to
delete the block and rerun. With a throwaway `HOME`, nothing outside the worktree's `AGENTS.md` is
touched. From the feature worktree root:

```bash
WT="$(git rev-parse --show-toplevel)"
S="$(mktemp -d "${TMPDIR:-/tmp}/regen-0491.XXXXXX")"
go build -o "$S/docket" ./cmd/docket
python3 - "$WT/AGENTS.md" <<'EOF'
import sys
p = sys.argv[1]
s = open(p).read()
start, end = '<!-- docket:dispatch:start', '<!-- docket:dispatch:end -->'
assert s.count(start) == 1 and s.count(end) == 1, 'dispatch markers missing or duplicated; refusing to edit'
a, b = s.index(start), s.index(end)
assert a < b, 'dispatch markers out of order; refusing to edit'
b += len(end) + (1 if s[b + len(end):b + len(end) + 1] == '\n' else 0)
open(p, 'w').write(s[:a] + s[b:])
EOF
H="$S/home"; mkdir -p "$H"
HOME="$H" XDG_CONFIG_HOME="$H/.config" XDG_DATA_HOME="$H/.local/share" \
  XDG_STATE_HOME="$H/.local/state" XDG_CACHE_HOME="$H/.cache" \
  "$S/docket" install --repo-dir "$WT" --harness opencode
git diff --stat -- AGENTS.md
git status --short
```

Expected: `install: applied`. `AGENTS.md` changes only inside the run-tracker sections, matching
Step 2's edits. `git status --short` shows only this task's paths. If anything else changed, stop
and return `BLOCKED` naming the extra paths.

- [ ] **Step 5: Re-pin the prose budgets**

Measure the five touched skill files as `TestSkillSizeBudgets` counts them (lines, and
`len(strings.Fields(content))` words; read `skillBudgets` and the counter in
`internal/repoguard/budgets_test.go`; `wc -l` / `wc -w` agree for ASCII prose):

```bash
for f in docket-build/SKILL.md docket-build/references/gate-caller-loop.md docket-implement-next/SKILL.md docket-implement-next/references/edge-paths.md docket-finalize-change/references/gate-failure.md; do
  printf '%s %s %s\n' "$f" "$(wc -l < skills/$f)" "$(wc -w < skills/$f)"; done
```

Each count must be ≤ its current row. Set each row to the measured counts and prepend
`0491: the run id is retired (<old> -> <new>); ` to its comment. Measure the AGENTS.md dispatch
block's words (`dispatchBlockWords`). It must be below 1154; set `dispatchBudget` to the exact new
count and prepend `0491: the run id is retired from the run-tracker block; the 0443 operation-id
wording is added (was 1154); ` to its comment. If any count grew, return `BLOCKED` with the
numbers; never raise a ceiling.

- [ ] **Step 6: Run the guards**

```bash
go build ./... && go vet ./...
go test -count=1 ./internal/repoguard/ ./internal/harness/ ./internal/assets/
go test -count=1 ./internal/install/
```
Expected: PASS — including `TestCommittedCodexDispatchMatchesGenerator`,
`TestEmbeddedMatchesAuthored` (or its equivalent), `TestSkillSizeBudgets`, `TestDispatchBlockBudget`,
`TestGateDriveBuildStartCarriesChangeID`, and the capability-surface guard (`docket capabilities
--json` is its permitted bootstrap spelling).

- [ ] **Step 7: Commit**

```bash
git add cursor-rules/run-tracker.md skills internal/harness/dispatch.go internal/assets/embedded AGENTS.md internal/repoguard
git status --short   # only this task's paths
git commit -m "docs(0491): the run-tracker block, skills, and Codex clause name the run key only; an operation id is not a command"
```

---

### Task 9: Glossary and the run-tracker concept doc

**Risk:** economy — documentation only.

**Files:**
- Modify: `docs/reference/glossary.md`
- Review (edit only if it describes the deleted check): `docs/concepts/run-tracker.md`

**Interfaces:**
- Consumes: Tasks 1–8's names: `run-started <key> <run-context>`, `run cancel --key <key> --reason <why>`,
  `launch-abandoned`, `run-superseded`, `run-record-conflict`.
- Produces: the anchor `#start--run-key--run-context`.

- [ ] **Step 1: Edit the glossary**

(a) Rename the heading `### Start / run key / run id / run context` to
`### Start / run key / run context`. Replace its first paragraph with:

```
**Starting** a run (`run.start`) mints two values before a dispatch: the **run key** (ties a finish
to this launch, and is the run tracker's only handle — cancel, verdict, and continue all take it)
and the **run context** (a token). The run context is copied into the implement-next dispatch
prompt; build-task workers never receive it, because they run their focused tests directly and call
no gate operation. It prints `run-started <key> <run-context>`; `run-untracked` still allows a
keyless dispatch that can never authorise a re-dispatch.
```

(b) *Cancel*: replace "It fences the run so nothing new attaches, tears down" with "It fences the
run, tears down", and the example with
`docket run cancel --key <key> --reason "superseded by 413"`.

(c) *Run fence*: replace "located by its [run id](#start--run-key--run-id--run-context), so nothing
new can attach to it. A fenced run is never restored." with:

```
located by its [run key](#start--run-key--run-context), so no new participant, successor, or
workflow mutation can attach to it (gate starts are not fenced; cancel accounts every gate the run
started). A fenced run is never restored. A superseded run's mutations are refused
`run-superseded`; a run record already bound to a different change, worktree, or terminal evidence
refuses a re-bind with `run-record-conflict`.
```

(d) *Drive disposition*: after "…deadline expiry, bad state, or a process death." add: "A drive whose
launch provably never started is settled HALTED too: `launch-abandoned` when the keyed `run.verdict`
closes it, `run-cancelled` when `run.cancel` does."

(e) The gate-drive example: drop ` --run-id <run-id>` from the `docket gate drive start` line. The
*Agent enter* example: drop ` --run-id <run-id> --run-key <key>` (no documented flow passes the
lifecycle linkage, spec Decision 6).

(f) Update every link to the old anchor, including the table-of-contents entry:
`git grep -n -e 'start--run-key--run-id--run-context' -- docs skills cursor-rules AGENTS.md README.md`
(excluding frozen records) → `start--run-key--run-context`; the TOC text becomes
`Start / run key / run context`.

Then `grep -n -E -e '--run-id|<run-id>|run id' docs/reference/glossary.md` must print only
supervisor run-id prose, if any.

- [ ] **Step 2: Review the concept doc**

Read `docs/concepts/run-tracker.md` in full. If any sentence says a gate start is refused because the
run is cancelled, completing, or superseded, or names the run id or a launch check, rewrite it: a
gate start is refused only by the worktree lock (`worktree-busy`). Otherwise leave the file
untouched.

- [ ] **Step 3: Run the doc guards**

```bash
go test -count=1 ./internal/repoguard/
```
Expected: PASS (anchor, link, and doc guards).

- [ ] **Step 4: Commit**

```bash
git add docs/reference/glossary.md docs/concepts/run-tracker.md
git commit -m "docs(0491): glossary names the run key as the run tracker's only handle"
```

(Stage `docs/concepts/run-tracker.md` only if Step 2 changed it.)

---

### Task 10: The retired-vocabulary seal retires the run id

**Risk:** standard — guard code: a new scoped kind plus rows that must never match the supervisor's
id; each row is mutation-tested.

**Files:**
- Modify: `internal/repoguard/retired_vocabulary_test.go`

**Interfaces:**
- Consumes: the surfaces Tasks 1–9 cleaned (a row reddens on any surviving site).
- Produces: `kindSchemaPath` (an exact `"<op> <REQ|RES> <key path>"` absent from the live schema)
  and the change-0491 rows.

- [ ] **Step 1: Re-point the rows whose replacements are now retired**

In `retiredVocabulary`, change only the `New` of these rows:

```go
	{Row: "10", Kind: kindToken, Old: "--run-epoch", New: "no flag — the run key is the run tracker's only handle (change 0491)"},
	{Row: "10", Kind: kindGoFlag, Old: "run-epoch", New: "run-key"},
	{Row: "11", Kind: kindToken, Old: "--epoch", New: "run cancel --key"},
	{Row: "11", Kind: kindGoFlag, Old: "epoch", New: "key"},
	{Row: "20", Kind: kindToken, Old: "epoch-mismatch", New: "run-record-conflict"},
	{Row: "31", Kind: kindToken, Old: "stale-run-epoch", New: "run-superseded"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-epoch", New: "run-not-found"},
	{Row: "38b", Kind: kindJSONKey, Old: "epoch", New: "none — run.start reports key and run_context (change 0491)"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_EPOCH", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY"},
```

- [ ] **Step 2: Add the change-0491 rows and the scoped schema kind**

Add to the `retiredKind` constants (after `kindWord`):

```go
	// kindSchemaPath: an exact "<operation> <REQ|RES> <key path>" that must be
	// absent from the live schema registry (change 0491, row 38b: the run.start
	// result's run_id). It is exact, never a suffix, so the gate supervisor's own
	// run_id keys — gate.launch, gate.observe, gate.recover, and gate.stop results —
	// can never match.
	kindSchemaPath
```

Append to `retiredVocabulary`, after the family (e) rows:

```go
	// Change 0491 — the run id is retired; the run key is the run tracker's only
	// handle (ADR-0129 rows 3, 10, 11, 20, 31, 32, 38b, 38d amended in place). No row
	// may match the gate supervisor's own run id (internal/process, gate.* results'
	// run_id, incumbent-run:<id>): the negative controls pin that.
	{Row: "10, 11", Kind: kindToken, Old: "--run-id", New: "no flag — the run key is the handle (run cancel --key)"},
	{Row: "10, 11", Kind: kindGoFlag, Old: "run-id", New: "run-key / key"},
	{Row: "3", Kind: kindToken, Old: "<run-id>", New: "<key> (the run key)"},
	{Row: "20", Kind: kindToken, Old: "run-id-mismatch", New: "run-record-conflict"},
	{Row: "31", Kind: kindToken, Old: "stale-run-id", New: "run-superseded"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-id", New: "run-not-found"},
	{Row: "38b", Kind: kindSchemaPath, Old: "run.start RES run_id", New: "none — run.start reports key and run_context"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_RUN_ID", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY (the run id env is retired)"},
```

Wire `kindSchemaPath`:
- `textMatcher` returns `nil` for it, as for `kindSchemaKey`, so the text, Go, and prefilter scans
  skip it. Read `textMatcher` and mirror the `kindSchemaKey` case.
- Add:

```go
// schemaPathHits reports every kindSchemaPath row whose exact path the walked
// schema carries (seen, from schemaVersionKeyHits), naming the replacement.
func schemaPathHits(seen map[string]bool) []string {
	var hits []string
	for _, r := range retiredVocabulary {
		if r.Kind == kindSchemaPath && seen[r.Old] {
			hits = append(hits, fmt.Sprintf("schema %s: ADR-0129 rows %s: retired — use %s", r.Old, r.Row, r.New))
		}
	}
	sort.Strings(hits)
	return hits
}
```

- In `testRetiredSchemaWalk`, after the kept-key loop:

```go
	if ph := schemaPathHits(seen); len(ph) != 0 {
		t.Errorf("retired ADR-0129 schema paths in the live schema (%d):\n%s", len(ph), strings.Join(ph, "\n"))
	}
	// Non-vacuity of the path spelling: the scoped row must name a path shape the
	// walk produces. run.start's kept run_context key proves the scope spelling.
	if !seen["run.start RES run_context"] {
		t.Errorf("the schema walk no longer yields %q; the run.start RES scope spelling drifted", "run.start RES run_context")
	}
```

- In `testRetiredNonVacuity`'s kind switch:

```go
		case kindSchemaPath:
			op, rest, _ := strings.Cut(r.Old, " ")
			_, key, _ := strings.Cut(rest, " ")
			doc := app.SchemaResult{Operations: []app.OperationSchema{{ID: op, Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: key}}}}}}
			_, _, seen := schemaVersionKeyHits(doc)
			if ph := schemaPathHits(seen); len(ph) != 1 || !strings.Contains(ph[0], r.New) {
				t.Errorf("row %s: a planted %q was not detected naming %q: %v", r.Row, r.Old, r.New, ph)
			}
			continue
```

Check that `app.OperationSchema.Result` is a value of `app.TypeDescriptor`: `schemaVersionKeyHits`
reads `op.Result.Fields`, while `op.Request` is a pointer. Adjust the literal to the real field
types.

- Raise `testRetiredTableIntegrity`'s `floor` to the new row count (`len(retiredVocabulary)` after
  this step), so a truncated table cannot pass.
- Update the file header: add a paragraph "Change 0491 retires the run id (rows 3, 10, 11, 20, 31,
  32, 38b, 38d): it re-points the rows whose replacements were run-id spellings and adds
  kindSchemaPath, the first exact-path kind, because the run.start result's run_id shares its key
  name with the gate supervisor's run_id."

- [ ] **Step 3: Update the negative controls and planted lines**

- In the rows-12/38e block (`for _, line := range []string{ "docket gate drive prepare-scope --gate-context <ctx>", …`),
  remove ` --run-id <id>` and ` [--run-id <id>]` from the planted lines and ", and the run id" from
  the third line. They exist to hit row 12; they must not also hit the new rows.
- In `testRetiredNegativeControls`, change `"docket gate drive start --run-context <ctx> --run-id <id>"`
  to `"docket gate drive start --run-context <ctx>"`.
- Add negative controls that prove the rows cannot match the supervisor's id. Each must produce no
  hit from any row:

```go
		{"skills/x/SKILL.md", "the `gate.observe` operation reports `run_id: 0790b760e26444866ef2e156ba383326` for the raw run"},
		{"skills/x/SKILL.md", "a worktree-busy refusal names `incumbent-run:<id>` while that raw run holds the lock"},
		{"internal/app/gate.go", "package p\ntype R struct {\n\tRunID string `json:\"run_id\"`\n}\n"},
		{"internal/app/gate.go", "package p\nfunc f() { lines = append(lines, \"run_id: \"+r.RunID) }\n"},
		{"skills/x/SKILL.md", "the gate supervisor's raw run id is the base name of its run dir"},
```

Place each in the negative-control table whose shape it matches (text lines vs Go source). Add a
schema negative control: a planted doc with operation `gate.launch` and a RES field `run_id` must
yield zero `schemaPathHits`.

- [ ] **Step 4: Run the seal**

Run: `go test -count=1 ./internal/repoguard/ -run 'TestRetiredVocabularySeal'`
Expected: PASS. If `maintained_surfaces` or `generator_output` reports a surviving site, it is a
real leftover from an earlier task: fix that site (never weaken the row), then re-run.

- [ ] **Step 5: Mutation-test every new row**

For each change-0491 row, plant one violation in a maintained surface, run the seal with
`-count=1`, watch it report that row naming its replacement, and restore the file from a backup copy
with `mv -f`. Use `grep -c` before and after to confirm each plant landed. Plants:
- `--run-id`: append `` the `gate.drive.start` operation with `--run-id <x>` `` to
  `skills/docket-build/references/gate-caller-loop.md`.
- `run-id` (GoFlag): add `_ = "run-id"` inside a function in `internal/cli/run.go`.
- `<run-id>`: append `` `run-started <key> <run-id> <run-context>` `` to `cursor-rules/run-tracker.md`.
- `run-id-mismatch`, `stale-run-id`, `unknown-run-id`, `DOCKET_AGENT_GUARDIAN_RUN_ID`: add each as a Go
  string literal in `internal/app/runtracker_fence.go` (one at a time).
- `run.start RES run_id`: add back `RunID string \`json:"run_id,omitempty"\`` to `RunStartResult` in
  `internal/app/runtracker_start.go`; the `schema_walk` subtest must redden.

Then run the whole seal once more on the restored tree: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repoguard/retired_vocabulary_test.go
git commit -m "test(0491): the retired-vocabulary seal retires the run id, scoped clear of the supervisor's run_id"
```

---

## After the last task (docket-build's single full-suite gate)

docket-build runs the whole suite through `build.test_command` (`go run ./cmd/docket development
test`). Read every `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:` line
even on a green run. The coordinator then records the new ADR, amends ADR-0129's rows in place, adds
the Update notes to ADR-0124, ADR-0128, and ADR-0132, and writes the results file. The results file
must carry the upgrade contract: install after the merge only while no run is in flight, then
restart every agent session. Optional notes: a run left `completing` by a never-launched drive
completes on its next keyed `run.verdict`, and a cancel stuck `cancellation-pending` on a missing run
root finishes when re-run. None of that is a build task.

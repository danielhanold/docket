<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0489 — Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0489-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md)**
<!-- docket:backlink:end -->
# Delete the Task-Owned Gate-Drive Machinery Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`: one tier worker per `### Task N`
> heading, each running the `docket-build-task` contract (one task, one commit). Steps use checkbox
> (`- [ ]`) syntax for readability only; nobody ticks them.

**Goal:** Delete the unreachable task-owned gate-drive path (`--owner task`, scoped starts with
predecessor receipts, the scope slot lifecycle and pending-ack journal, terminal acknowledgement,
and the `gate.drive.prepare-scope` / `gate.drive.acknowledge` / `gate.drive.takeover` operations),
keep a minimal in-process outer recovery scope for the run tracker, and make the outer takeover
recover only a still-running drive.

**Architecture:** The work runs in compile-safe layers. Task 1 fixes the live defect first (the
outer candidate scan stops counting finished drives). Tasks 2–3 cut the CLI and app surfaces
(operations, task owner, scope/receipt flags), so nothing outside `internal/gatedrive` reaches the
task path. Task 4 narrows `Takeover` to the outer path and deletes the run-revocation seam. Task 5
deletes scoped starts and terminal acknowledgement. Task 6 deletes the scope slot lifecycle, the
census's scope walk, and the dead record fields, and adds the old-record tests. Task 7 prunes the
error kinds and next-action messages. Task 8 sweeps prose and comments. Task 9 re-budgets the
shrinking test files. Every task leaves the tree compiling and its focused tests green.

**Tech Stack:** Go 1.27 (stdlib `encoding/json`, `crypto/sha256`, `sync`, `testing`), cobra CLI,
the docket Go suite runner (`internal/suiterunner`), `go test -tags integration` shards.

**Spec:** `docs/superpowers/specs/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri-design.md`
on the `docket` metadata branch (worktree copy:
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri-design.md`).
Read it before starting any task; it is the design authority. Its Decision 1 symbol lists are the
**known** sites, not the full set.

## Global Constraints

- **Run every focused test directly, in the foreground, in the feature worktree, wrapped in GNU
  `timeout`:** `timeout --kill-after=10s 10m go test -count=1 ./internal/<pkg>/ -run '<regex>'` (use
  `gtimeout` if `timeout` is missing). Exit `124`/`137` means the limit was hit; `125`–`127` mean the
  command could not run. Neither is a red test. Always pass `-count=1`: a cached `ok` is not evidence
  (learning: cached-runner-serves-a-mutated-tree). No task runs the full suite
  (`go run ./cmd/docket development test`); that is the controller's build gate.
- Integration tests are behind `-tags integration` and run by prefix, never unfiltered (the
  `internal/app` integration binary refuses an unfiltered run):
  `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationRunVerdict' ./internal/app/`.
  The gatedrive race shard runs with `-race`:
  `timeout --kill-after=10s 10m go test -count=1 -race -tags integration -run '^TestRaceIntegrationGatedrive' ./internal/gatedrive/`.
- Every task ends with both of these green before its commit:
  `go build ./... && go vet ./... && go vet -tags integration ./...` and
  `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ ./internal/app/ ./internal/cli/ ./internal/repoguard/`.
  The second command is the default-tag corpus of every package this change touches. A compile error
  in any of them means the task is not done (learning: intermediate-task-state-buildable).
- **Derive every deletion site from a whole-repo grep, never from this plan's lists alone.** Before
  deleting a symbol, run `git grep -n '<symbol>'` and sort the hits into executable code, tests, and
  prose. Delete a symbol only when the grep shows no remaining production caller. Frozen records
  (`docs/results/`, `docs/superpowers/plans/` files other than this plan, archived changes,
  Accepted ADRs) are never edited.
- **Decide each broken test by what it guards, not what it asserts** (spec "Tests"; learning:
  test-premise-deleted-not-regated). Delete a test whose subject is gone. Re-point a test whose
  property survives onto a build-owned scopeless drive (carrying `RunContext`/`RunID`) or onto an
  outer scope. Never restore deleted code or prose to keep a test green. When a file loses its last
  test, delete the file. When a helper loses its last caller, delete the helper.
- **Kept behavior is out of scope:** build/finalize start/advance, owner generation, handoff/claim,
  fingerprinting, the single relaunch, the suite-attempt budget, worktree admission, finished-incumbent
  reconciliation (which **keeps accepting the legacy `"scoped"` slot kind**), legacy-history
  assessment, `gate.history.cleanup`, the run tracker's attribution/retry model, `run.continue`, the
  run-context claim binding, and the `--task-id`/`--phase` recording flags. Do not change them.
- No schema bump, no store reset, no migration, no deletion of record files on disk.
- Mutation probes restore from a backup copy (`cp f f.bak; <mutate>; <run>; mv -f f.bak f`), never
  `git checkout --`. Confirm the mutation landed before trusting a green result.
- A cross-reference in a comment anchors on a symbol name or a quoted clause, never a line number.
- Stage by explicit path only (`git add <path>...`, `git rm <path>` for deletions); never
  `git add -A` / `git add .` / `git commit -a`.
- **No plan task writes an ADR.** The new ADR (*The run tracker's outer takeover recovers only a
  still-running drive*, `relates_to: [107, 130]`) is recorded by implement-next at Step 6 through
  `docket-adr`. Do not cite its number in code.
- Skills are expected not to change. If any file under `skills/` or `cursor-rules/` does change,
  run `go generate ./internal/assets` and stage the regenerated `internal/assets/embedded/` paths in
  the same commit.

## Review Focus

1. **A run in flight across the upgrade.** An outer scope written by the pre-0489 binary (with
   `task_id`, `phase`, `drive_count`, and the other fields this change drops) must still load, bind
   its change, and take over a live drive. Test: Task 6 `TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver`.
2. **Leftover open task scopes that name the run.** An open task scope on disk with slot fields, a
   pending-ack journal, and a `run_id` must not block `run.cancel`'s census, the successful-run
   closeout census, or admission. Test: Task 6 `TestOldTaskScopeOnDiskChangesNothing`.
3. **A leftover nonterminal task drive carrying `scope_id`.** It must decode and resolve as a
   scopeless, no-run-record drive, never as `run-record-unreadable`. Test: Task 6
   `TestOldDriveRecordWithScopeIDSettlesScopeless`.
4. **A stale caller still using the retired surface.** `--owner task`, `--scope-id`, `--child-cap`,
   the predecessor flags, an argv after `--`, and the three retired subcommands must each be a usage
   error that launches nothing — never a silent build-owned run. Tests: Task 2
   `TestGateDriveRetiredScopeCommandsAreUnknown`, Task 3 `TestGateDriveStartRetiredTaskSurfaceIsUsageError`.
5. **A suite that finishes between the outer scan and the takeover.** `Takeover` given an explicit
   drive id must still accept a drive that has since finished and hand its verdict over unchanged.
   Test: Task 4 `TestTakeoverAcceptsDriveThatFinishedAfterScan`.

---

### Task 1: The outer takeover's candidates are live drives only

**Build tier:** premium. This is the change's one behavior fix (spec Decision 3) and the R1 tests.

**Files:**
- Modify: `internal/gatedrive/run_waiting.go` (`FindScopeDriveIDs` and its doc comment)
- Modify: `internal/app/runtracker_continuation.go` (doc comments on `RunDecisionContinue` and `ContinuationSeam.LocateOuterDrive`)
- Modify: `internal/gatedrive/takeover_test.go` (`TestFindScopeDriveIDs`; add `TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates`)
- Modify: `internal/gatedrive/driver_faults_test.go` (delete `TestFaultLaunchFailsThenRestart`)
- Modify or delete: `internal/gatedrive/sequence_race_integration_test.go` (delete `TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork`)
- Modify: `internal/app/runtracker_verdict_integration_test.go` (add `TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `(*Store).FindScopeDriveIDs(changeID, runContextHash string) ([]string, error)` keeps
  its signature; it now returns only drives whose `LastOutcome` is nonterminal.

A pre-probe of this exact edit showed exactly three red tests: `TestFindScopeDriveIDs` (its premise
inverts), `TestFaultLaunchFailsThenRestart` (a task-scope test asserting a HALTED launch-failed drive
is a candidate), and the race-shard `TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork`
(a task-scope sequence test). All `internal/app` and `internal/cli` tests stayed green.

- [ ] **Step 1: Write the failing R1 gatedrive test**

Add to `internal/gatedrive/takeover_test.go`, right after `TestFindScopeDriveIDs`:

```go
// TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates pins change 0489's R1: a
// run whose only drives are a FAILED and a PASSED build drive — both still owned,
// both carrying the run's change id and run context — has NO outer-takeover
// candidate, so run.verdict falls through to its retry path instead of stopping
// takeover-ambiguous. A live drive beside them is the single candidate.
func TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	h := capHash("run-ctx-0489")
	seed := func(outcome Outcome) string {
		t.Helper()
		rec := seedRecord(t)
		rec.ChangeID = "0342"
		rec.RunContextHash = h
		rec.LastOutcome = outcome
		rec.OwnerGeneration = "owner-" + string(outcome) // still owned: never consumed
		id, _, err := store.NewDrive(rec)
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		return id
	}

	seed(FAILED) // the red gate
	seed(PASSED) // the green re-gate
	ids, err := store.FindScopeDriveIDs("0342", h)
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("finished build drives must never be takeover candidates, got %v", ids)
	}

	live := seed(WAITING)
	ids, err = store.FindScopeDriveIDs("0342", h)
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != live {
		t.Fatalf("a live drive beside finished ones must be the single candidate, got %v want [%s]", ids, live)
	}
}
```

Also rewrite `TestFindScopeDriveIDs`'s expectation: the `termUnconsumed` seed (PASSED, owner still
set) is now **excluded**. Rename the variable to `finishedOwned`, change the final assertion to
`len(got) != 1 || !got[waiting]` with the message `want exactly {waiting}`, and change the doc
comment to "lists drives matching change + run-context hash that are still live; excludes every
finished drive (owned or consumed), a wrong run context, a wrong change, and unreadable records".

- [ ] **Step 2: Run it to verify it fails**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestFindScopeDriveIDs'`
Expected: FAIL — both tests report the PASSED/FAILED owned drives as candidates.

- [ ] **Step 3: Change the candidate rule**

In `internal/gatedrive/run_waiting.go`, replace the clause in `FindScopeDriveIDs`:

```go
		// Only a still-running drive is a takeover candidate (change 0489). A
		// finished drive — PASSED, FAILED, or HALTED, owned or not — is never
		// recovered: its verdict is not reused, and the run's retry re-runs the gate.
		if isTerminalOutcome(rec.LastOutcome) {
			continue
		}
```

Rewrite the function's doc comment: it lists the drive ids for `changeID` whose `RunContextHash`
equals `runContextHash` and whose `LastOutcome` is nonterminal; drop every sentence about
"terminal-unconsumed", sequential scopes, `retirePredecessor`, and `Driver.Acknowledge`; keep the
sentences about skipping unreadable records and returning only a root-read fault as an error; end
with "It is run.verdict's outer candidate scan: exactly one live match authorizes an outer takeover;
zero falls through to the retry path, more than one stops `takeover-ambiguous`."

In `internal/app/runtracker_continuation.go`: `RunDecisionContinue`'s comment says "owns live
tracked work" (drop "or terminal-unconsumed"); `LocateOuterDrive`'s interface comment says "whose
outcome is nonterminal (a still-running drive)". The file header's "(or wrote a verdict for and then
died before its parent consumed it)" goes.

- [ ] **Step 4: Delete the two task-path tests whose premise this removes**

- `TestFaultLaunchFailsThenRestart` in `internal/gatedrive/driver_faults_test.go` (a task-scope test
  whose subject is a reserved scope slot; Task 5 would delete it anyway). Delete helpers only it used.
- `TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork` in
  `internal/gatedrive/sequence_race_integration_test.go` (two task scopes driven to terminal). If the
  file has no test left, `git rm` it. The race shard still selects `TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity`
  and the history race tests, so `tests/test_go_integration_contract.sh` check (7) stays satisfied.

- [ ] **Step 5: Add the real-seam R1 integration test**

Add to `internal/app/runtracker_verdict_integration_test.go` (it is `//go:build integration`; the
`TestIntegrationRunVerdict` prefix routes it to `tests/test_go_integration_app_runverdict.sh`). It
reuses `initGitRepo`, `requireProcessSupervisor`, `guardianExecutable`, `buildEffWithMaxAttempts`,
`runDriveToTerminal`, and `stopRunsUnder`, which already exist in this package's test files. Add
`crypto/sha256` and `encoding/hex` imports if missing.

```go
// TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates (change 0489,
// R1) drives a REAL red gate and a REAL green re-gate — build-owned drives carrying
// the run's change id and run context, as implement-next starts them — through the
// production gate-drive service, then asks the PRODUCTION continuation seam for
// outer-takeover candidates. Finished drives must yield none, so run.verdict takes
// its retry path (TestIntegrationRunVerdictVerdictIncompleteQuiescentStillRetriesOnce
// pins zero candidates -> run-retry-once) instead of stopping takeover-ambiguous.
func TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates(t *testing.T) {
	requireRealGit(t)
	requireProcessSupervisor(t)
	worktree, gitDir := initGitRepo(t, "")
	const runContext = "run-ctx-0489-r1"
	sum := sha256.Sum256([]byte(runContext))
	ctxHash := hex.EncodeToString(sum[:])

	finish := func(command string) gatedrive.Outcome {
		t.Helper()
		runRoot := filepath.Join(testsupport.TempDir(t), "runs")
		t.Cleanup(func() { stopRunsUnder(runRoot) })
		svc, res, reason := NewBuildGateDriveService(gitDir, guardianExecutable(t), buildEffWithMaxAttempts(command, 4))
		if svc == nil {
			t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
		}
		got := svc.Start(GateDriveStartRequest{
			RepoDir: worktree, Worktree: worktree, ChangeID: "0003", Phase: "build",
			Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: worktree, RunRoot: runRoot,
			RunContext: runContext, IdempotentSuiteGate: true,
		})
		if got.Result != ResultApplied || got.Drive == nil {
			t.Fatalf("build start refused: result=%s reason=%q message=%q", got.Result, got.Reason, got.Message)
		}
		return runDriveToTerminal(t, svc, got)
	}
	if out := finish("/usr/bin/false"); out != gatedrive.FAILED {
		t.Fatalf("red gate outcome = %s, want FAILED", out)
	}
	if out := finish("/bin/echo green"); out != gatedrive.PASSED {
		t.Fatalf("green re-gate outcome = %s, want PASSED", out)
	}

	seam, err := NewContinuationSeam(gitDir, guardianExecutable(t))
	if err != nil {
		t.Fatalf("NewContinuationSeam: %v", err)
	}
	ids, err := seam.LocateOuterDrive(3, ctxHash)
	if err != nil {
		t.Fatalf("LocateOuterDrive: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("a red-then-green run must leave NO takeover candidate (else run.verdict stops takeover-ambiguous), got %v", ids)
	}
}
```

This code is a draft (learning: plan-supplied-test-code-is-unverified): if a helper's name or
signature differs, use the real one; keep the assertion.

- [ ] **Step 6: Run everything this task touches**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestFindScopeDriveIDs'` — PASS
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationRunVerdict' ./internal/app/` — PASS
- `timeout --kill-after=10s 10m go test -count=1 -race -tags integration -run '^TestRaceIntegrationGatedrive' ./internal/gatedrive/` — PASS
- the two Global Constraints commands — PASS

- [ ] **Step 7: Mutation-test the R1 assertions**

Restore the old clause (`if isTerminalOutcome(rec.LastOutcome) && rec.OwnerGeneration == "" {`)
from a backup copy and confirm `TestFindScopeDriveIDsFinishedDrivesAreNeverCandidates` **and**
`TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates` both go red, then restore.
Record both reds in the task report.

- [ ] **Step 8: Commit**

```bash
git add internal/gatedrive/run_waiting.go internal/app/runtracker_continuation.go \
  internal/gatedrive/takeover_test.go internal/gatedrive/driver_faults_test.go \
  internal/app/runtracker_verdict_integration_test.go
git add internal/gatedrive/sequence_race_integration_test.go   # or: git rm it if emptied
git commit -m "fix(gatedrive): the outer takeover's candidates are live drives only (change 0489)"
```

---

### Task 2: Retire the three scope operations from the CLI, the app service, and the catalog

**Build tier:** standard.

**Files:**
- Modify: `internal/cli/gate.go` (delete the `acknowledge`, `prepare-scope`, and `takeover` cobra commands and their `AddCommand` wiring)
- Modify: `internal/cli/install.go` (delete the `"gate drive acknowledge"`, `"gate drive prepare-scope"`, `"gate drive takeover"` `assetIndependent` entries)
- Modify: `internal/app/gate_drive.go` (delete `OperationGateDriveAcknowledge`, `OperationGateDrivePrepareScope`, `OperationGateDriveTakeover`, `GateScopeResult` + its `HumanText`, `GateDriveService.Acknowledge`/`PrepareScope`/`Takeover`, and `Acknowledge`/`Takeover`/`PrepareScope` from the `driveEngine` interface; delete the `runLocate` field and its assignment in `NewCommandlessGateDriveService`)
- Modify: `internal/app/schema_registry.go` (delete the `gate.drive.acknowledge`, `gate.drive.prepare-scope`, `gate.drive.takeover` rows)
- Modify tests: `internal/cli/gate_test.go`, `internal/cli/capability_production_test.go`, `internal/cli/run_tracker_rename_test.go`, `internal/app/gate_drive_test.go`, `internal/app/gate_drive_history_integration_test.go`, `internal/app/runtracker_production_census_integration_test.go`, plus any other test the compile surfaces

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `driveEngine` no longer has `Acknowledge`, `Takeover`, or `PrepareScope`; the `gatedrive`
  methods of those names still exist (Tasks 4–5 delete/narrow them). `runIDLocator` **stays**: it is
  still used by `CheckRunIDExists` for `agent.enter`.

- [ ] **Step 1: Write the failing absence test**

Add to `internal/cli/gate_test.go`:

```go
// TestGateDriveRetiredScopeCommandsAreUnknown (change 0489): the recovery-scope
// operations are gone from the CLI. The leaf is not registered, and a stale caller
// invoking one gets a usage error and no gate.drive.* protocol document — never a
// silently different operation.
func TestGateDriveRetiredScopeCommandsAreUnknown(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"prepare-scope", "acknowledge", "takeover"} {
		if cmd, _, err := root.Find([]string{"gate", "drive", sub}); err == nil && cmd.Name() == sub {
			t.Errorf("docket gate drive %s is still registered", sub)
		}
		out, _, code := runCLI(t, "--json", "gate", "drive", sub)
		if code == 0 {
			t.Errorf("docket gate drive %s exited 0, want a usage error", sub)
		}
		if strings.Contains(out, `"operation":"gate.drive.`) {
			t.Errorf("docket gate drive %s emitted a protocol document: %s", sub, out)
		}
	}
}
```

`captureTree` is the cobra-tree helper `TestRunTrackerVocabularyHardCut` already uses. The `Find`
check is what makes the test non-vacuous: today a bare invocation already exits non-zero on its
missing required flags.

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/cli/ -run '^TestGateDriveRetiredScopeCommandsAreUnknown$'`
Expected: FAIL on the `Find` check (the leaves are still registered). Once they are deleted, note
the exit code the CLI returns for the unknown subcommand; if it is a fixed `2`, tighten the exit
assertion to `code != 2`.

- [ ] **Step 2: Delete the three commands and their service methods**

Grep first: `git grep -n 'gate.drive.acknowledge\|gate.drive.prepare-scope\|gate.drive.takeover\|OperationGateDriveAcknowledge\|OperationGateDrivePrepareScope\|OperationGateDriveTakeover\|GateScopeResult\|runLocate\|\.PrepareScope(\|\.Acknowledge(\|\.Takeover(' -- internal cmd`.
Delete the CLI commands, the install-map entries, the registry rows, the constants, `GateScopeResult`,
the three service methods, the three interface methods, and `runLocate`. Keep `Store.PrepareScope`
(used by `run.start` through `internal/cli/run.go`) and `gatedrive.Driver.Acknowledge`/`Takeover`
(Tasks 4–5 own them). Rewrite comments in `gate.go` and `gate_drive.go` that describe the deleted
commands; `NewCommandlessGateDriveService`'s comment no longer says it serves prepare-scope.

- [ ] **Step 3: Fix the pins and the tests**

- `TestRepresentativeSignatures` (`internal/cli/capability_production_test.go`): delete the
  `gate.drive.acknowledge`, `gate.drive.prepare-scope`, and `gate.drive.takeover` rows and their
  comments. Leave the `gate.drive.start` row for Task 3.
- `TestRunTrackerVocabularyHardCut` (`internal/cli/run_tracker_rename_test.go`): delete the two
  `{"gate","drive","prepare-scope"}` rows and the `prepare-scope --gate-context` exit-2 probe with
  its comment. The `gate drive start` rows stay.
- `TestSchemaCatalogCorrespondence`, `TestEveryRequestAndResultStructIsBound`,
  `TestAssetIndependentSetExact`: run them; they must pass with the rows gone and no other edit. If
  one fails, fix the registry/install map, never the guard.
- `internal/cli/gate_test.go`: delete `TestGateDrivePrepareScopeGrantAndRedaction`,
  `TestGateDriveTakeoverWired`, `TestGateDriveTakeoverRequiresFlags`,
  `TestGateDrivePrepareScopeRunIDGatesTakeover`, `TestGateDriveAcknowledgeWired`,
  `TestGateDriveAcknowledgeRequiresFlags`, `TestGateDrivePrepareScopeUnknownRunIDIsNamed`, and
  `makeCancelledRun` if it has no caller left. `TestGateDriveScopeBoundStartRoundTrips` and
  `TestGateDriveRunContextWiredThroughScope` call `prepare-scope`; delete them here (Task 3 adds the
  build-owned run-context wiring test that replaces the second one).
- `internal/app/gate_drive_test.go`: delete `TestPrepareScopeHumanTextRedactsCapabilities`,
  `TestAcknowledgeForwardsArgsAndMapsDoc`, `TestAcknowledgeScopeTransferredEnvelope`,
  `TestTakeoverMapsDoc`, `TestPrepareScopeRefusesUnknownRunID`. In
  `TestAdvanceRecoverTakeoverDoNotCharge` and `TestCancellationDoesNotCharge`, remove only the
  takeover/acknowledge legs (the property "these operations never charge the suite budget" still
  holds for the rest). Remove the fake engine's `Acknowledge`/`Takeover`/`PrepareScope` methods if no
  test calls them.
- `internal/app/gate_drive_history_integration_test.go`
  (`TestIntegrationBuildStartAdmitsOverLegacyPassedHistoryOneLaunch`): re-point it to a plain
  build-owned start. Delete the `svc.PrepareScope(...)` block and the `ScopeID`/`ChildCapability`
  request fields, and change "one ordinary build-owned scoped Start" in its comment to "one ordinary
  build-owned Start". Every assertion stays.
- `internal/app/runtracker_production_census_integration_test.go`
  (`TestIntegrationRunCompletionProductionCensusCancelResumeStartsReplacementGate`): the outer
  `sdeps.Prepare` becomes `gatedrive.OpenStore(fx.common).PrepareScope` (production's composition);
  delete the replacement `svc.PrepareScope(...)` block and the `ScopeID`/`ChildCapability` start
  fields, keeping `RunID: started.RunID`. Every assertion stays.

- [ ] **Step 4: Run the task's tests**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/cli/ ./internal/app/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationBuildStart|^TestIntegrationRunCompletion' ./internal/app/`
- the two Global Constraints commands
Expected: PASS, including `TestGateDriveRetiredScopeCommandsAreUnknown`.

- [ ] **Step 5: Mutation-test the new absence test**

Temporarily re-add an empty `prepare-scope` leaf (any cobra command with that `Use` that sets a
`gate.drive.prepare-scope` result) from a backup copy of `gate.go`; confirm
`TestGateDriveRetiredScopeCommandsAreUnknown` goes red; restore.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/gate.go internal/cli/install.go internal/app/gate_drive.go internal/app/schema_registry.go \
  internal/cli/gate_test.go internal/cli/capability_production_test.go internal/cli/run_tracker_rename_test.go \
  internal/app/gate_drive_test.go internal/app/gate_drive_history_integration_test.go \
  internal/app/runtracker_production_census_integration_test.go
git commit -m "refactor(gate): retire gate.drive.prepare-scope, acknowledge, and takeover (change 0489)"
```

---

### Task 3: Retire the task-intent owner and the start's scope and receipt flags

**Build tier:** standard.

**Files:**
- Modify: `internal/cli/gate.go` (`gate drive start`: `Use`, owner switch, argv handling, flags; delete `buildTaskGateDriveService`)
- Modify: `internal/app/gate_drive.go` (delete `NewTaskGateDriveService`, the `taskIntent`/`argv` fields and every branch reading them; delete `ScopeID`, `ChildCapability`, `PredecessorDriveID`, `PredecessorOwnerGen` from `GateDriveStartRequest`; `startRequest` stops forwarding them; delete `AdvisoryRunID` from `driveEngine`; `startBudgetedBuild` uses `startReq.RunID`)
- Modify tests: `internal/cli/gate_test.go`, `internal/cli/capability_production_test.go`, `internal/app/gate_drive_test.go`, `internal/app/runtracker_no_run_record_resume_e2e_integration_test.go`, plus whatever the compile surfaces

**Interfaces:**
- Consumes: Task 2's trimmed `driveEngine`.
- Produces: `gate drive start` accepts `--owner build|finalize` only and no positional or `--` argv.
  `GateDriveStartRequest` keeps `RepoDir, Worktree, ChangeID, TaskID, Phase, Branch, Ref, Cwd,
  EnvHash, RunRoot, IdempotentSuiteGate, RunContext, RunID`. The CLI no longer sets
  `gatedrive.StartRequest.ScopeID`/`ChildCapability`/`PredecessorDriveID`/`PredecessorOwnerGen`
  (those fields are deleted in Task 5).

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/gate_test.go`:

```go
// TestGateDriveStartRetiredTaskSurfaceIsUsageError (change 0489): a stale caller of
// the retired task-owned surface gets a usage error and launches nothing — never a
// silent build-owned run of the configured suite.
func TestGateDriveStartRetiredTaskSurfaceIsUsageError(t *testing.T) {
	marker := filepath.Join(testsupport.TempDir(t), "suite-ran")
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: touch "+marker+"\n")
	root := testsupport.TempDir(t)
	base := []string{"--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root}
	cases := map[string][]string{
		"owner task":            {"--owner", "task", "--", "/bin/echo", "hi"},
		"argv after dash":       {"--owner", "build", "--", "/bin/echo", "hi"},
		"scope id":              {"--owner", "build", "--scope-id", "s"},
		"child cap":             {"--owner", "build", "--child-cap", "c"},
		"predecessor drive id":  {"--owner", "build", "--predecessor-drive-id", "d"},
		"predecessor owner gen": {"--owner", "build", "--predecessor-owner-gen", "g"},
	}
	for name, extra := range cases {
		out, _, code := runCLI(t, append(append([]string{}, base...), extra...)...)
		if code != 2 {
			t.Errorf("%s: exited %d, want 2 (usage error): %s", name, code, out)
		}
		if strings.Contains(out, `"drive"`) {
			t.Errorf("%s: emitted a drive document: %s", name, out)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("a refused start launched the configured suite")
	}
}

// TestGateDriveStartBuildOwnedStoresRunContextHash (change 0489): with the task
// owner gone, this proves a build-owned start's --run-context reaches the drive
// record as the hash run.verdict's outer scan matches a run's drives on.
func TestGateDriveStartBuildOwnedStoresRunContextHash(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")
	root := testsupport.TempDir(t)
	out, errS, code := runCLI(t, "--json", "gate", "drive", "start", "--repo-dir", wt, "--run-root", root,
		"--owner", "build", "--change-id", "489", "--run-context", "ctx-0489")
	if code != 0 || errS != "" {
		t.Fatalf("start: out=%q err=%q code=%d", out, errS, code)
	}
	id, _ := driveDoc(t, decodeOneJSON(t, out))["drive_id"].(string)
	if id == "" {
		t.Fatalf("start produced no drive id: %s", out)
	}
	common := gitCommonDirForTest(t, wt) // resolve `git rev-parse --path-format=absolute --git-common-dir` with this package's git helper
	buf, err := os.ReadFile(filepath.Join(common, "docket", "gate-drives", "v2", id, "record.json"))
	if err != nil {
		t.Fatalf("read drive record: %v", err)
	}
	var env struct {
		Record struct {
			RunContextHash string `json:"run_context_hash"`
		} `json:"record"`
	}
	if err := json.Unmarshal(buf, &env); err != nil {
		t.Fatalf("decode drive record: %v", err)
	}
	sum := sha256.Sum256([]byte("ctx-0489"))
	if want := hex.EncodeToString(sum[:]); env.Record.RunContextHash != want {
		t.Fatalf("drive run_context_hash = %q, want sha256(--run-context) %q", env.Record.RunContextHash, want)
	}
}
```

`gitCommonDirForTest` stands for whatever git helper this package already has (for example
`statusGit` if it returns output); add a three-line helper only if none fits. The `code != 2`
expectation matches the existing usage-error tests (`TestGateDriveStartRejectsPositionalBeforeDash`
expects 2 for a `RunE` error and cobra returns 2 for an unknown flag); verify it and keep it strict —
a weaker `code != 0` would pass the flag cases vacuously today, because a bare `--scope-id` start
already refuses with a non-usage result. Run both:
`timeout --kill-after=10s 10m go test -count=1 ./internal/cli/ -run '^TestGateDriveStartRetiredTaskSurfaceIsUsageError$|^TestGateDriveStartBuildOwnedStoresRunContextHash$'`
Expected: the first FAILS (`--owner task` and the scope flags are still accepted); the second passes
already — it is the coverage the deletion would otherwise lose, pinned before the deletion.

- [ ] **Step 2: Cut the CLI start surface**

In `internal/cli/gate.go`'s `start` command:
- `Use: "start --repo-dir <dir> --run-root <dir> --owner build|finalize"`.
- Replace the `ArgsLenAtDash` block with a single check: `if len(args) > 0 { return errors.New("gate drive start takes no arguments; it runs the owner's configured suite command") }`.
- Delete the predecessor-pair validation, the `case "task"` branch, and `buildTaskGateDriveService`.
  The default error reads `gate drive start --owner must be build or finalize, got %q`.
- Delete the `scope-id`, `child-cap`, `predecessor-drive-id`, `predecessor-owner-gen` flags and their
  reads; the `--owner` flag help says "build or finalize (required)".
- Rewrite the comment above `RunE` (no owner takes an argv) and the `RepoDir`/common-dir comment
  (it no longer mentions `prepare-scope`; the common dir is the repository identity `run.start`'s
  outer scope and `Takeover` compare).

- [ ] **Step 3: Cut the app task owner and scope fields**

In `internal/app/gate_drive.go`: delete `NewTaskGateDriveService`, `taskIntent`, `argv`, the
`!s.taskIntent &&` guard (it becomes `if s.command == ""`), the idempotent forcing in `startRequest`,
the `commandArgv` task branch, the four request fields and their forwarding, and `AdvisoryRunID`
from `driveEngine`. In `startBudgetedBuild`, replace `runID := s.engine.AdvisoryRunID(startReq)` and
its comment with `runID := startReq.RunID` and a one-line comment ("reconcile under the run this
start presents"). Rewrite comments that name the task-intent owner or task-owned starts (`Start`,
`startRequest`, `reserveBuildSuiteAttempt`, the `budgetStore` field, `GateDriveStartRequest.RunID`).

- [ ] **Step 4: Fix the pins and tests**

- `TestRepresentativeSignatures`: `gate.drive.start` becomes
  `--owner <role> --run-root <dir> [--branch <name>] [--change-id <id>] [--cwd <dir>] [--env-hash <hash>] [--idempotent-suite-gate] [--phase <name>] [--ref <ref>] [--repo-dir <dir>] [--run-context <token>] [--run-id <id>] [--task-id <id>]`
  (no `-- <argv...>` tail). Run the test and use the rendered string if it differs only in order or
  spacing; rewrite the row's comment to say the task owner and receipt flags are gone (change 0489).
  `internal/cli/capability_test.go`'s synthetic `leaf("start ... -- <argv...>")` fixture tests the
  walker's argv-tail rule, not the production command: leave it.
- `internal/cli/gate_test.go`: delete `TestGateDriveStartPredecessorPairBothOrNeither`,
  `TestGateDriveStartOwnerTaskRunsArgv`, `TestGateDriveStartOwnerTaskRequiresArgv`. Re-point
  `TestGateDriveStartRejectsPositionalBeforeDash` to `--owner build` (both probes still exit 2).
  Re-point `TestGateDriveStartUnknownRunIDIsNamed` to `--owner build` over
  `gateDriveConfiguredRepo(t, "metadata_branch: main\nbuild:\n  gate: local\n  test_command: /bin/echo hi\n")`,
  dropping the `-- /bin/echo hi` tail; every assertion stays.
- `internal/app/gate_drive_test.go`: delete `TestTaskServiceForcesNonIdempotent`,
  `TestTaskServiceRequiresArgv`, `TestTaskServiceResolvesObservationBudget`,
  `TestStartForwardsScopeFields` (keep a `RunContext`/`RunID` forwarding assertion if the test also
  pinned those; otherwise delete), `TestStartForwardsPredecessorFields`, `TestTaskOwnedStartNotBudgeted`,
  `TestBudgetedBuildAdvisoryReconcilesWithScopeRun`; drop the `"task"` row from
  `TestProductionConstructorsWireRunLaunchGate`; delete the fake engine's `AdvisoryRunID`.
- `internal/app/runtracker_no_run_record_resume_e2e_integration_test.go`: replace
  `NewTaskGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("go test ./...", 4), []string{"/bin/echo", "ok"})`
  with `NewBuildGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("/bin/echo ok", 4))`
  and drop `TaskID: "task-6"` from the request. Every assertion stays (it only `Admit`s).

- [ ] **Step 5: Run the task's tests**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/cli/ ./internal/app/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationRunStart' ./internal/app/`
- the two Global Constraints commands
Expected: PASS.

- [ ] **Step 6: Mutation-test the two new tests**

(a) Re-add `case "task":` routing to the build service from a backup of `gate.go`: the owner-task case
of `TestGateDriveStartRetiredTaskSurfaceIsUsageError` must go red. (b) Delete `RunContext: runContext,`
from the CLI's `GateDriveStartRequest` literal: `TestGateDriveStartBuildOwnedStoresRunContextHash`
must go red. Restore both. Record the reds.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/gate.go internal/app/gate_drive.go internal/cli/gate_test.go \
  internal/cli/capability_production_test.go internal/app/gate_drive_test.go \
  internal/app/runtracker_no_run_record_resume_e2e_integration_test.go
git commit -m "refactor(gate): retire the task-intent owner and the start's scope flags (change 0489)"
```

---

### Task 4: Narrow `Takeover` to the outer recovery path

**Build tier:** premium. It changes the recovery authority's code path; mistakes are correctable but
consequential.

**Files:**
- Modify: `internal/gatedrive/takeover.go` (`Takeover`, `claimScopeForTakeover`; delete `Driver.PrepareScope` and `resolveTakeoverDrive`; rewrite the file header)
- Modify: `internal/gatedrive/driver.go` (delete `RunRevokedFunc`, `SetRunRevokedResolver`, the `runRevoked` field and its comment)
- Modify: `internal/gatedrive/drive.go` (delete `CauseTakeoverNoCandidate`; reword `CauseTakeoverAmbiguous`'s and `CauseRunRecordUnreadable`'s comments)
- Modify: `internal/app/runtracker_run_record.go` (delete `runRevokedResolver`)
- Modify: `internal/app/gate_drive.go`, `internal/app/runtracker_continuation.go` (delete every `SetRunRevokedResolver(runRevokedResolver(...))` wiring call and its comment)
- Modify: `internal/app/runtracker_launch_gate.go` (its comment cites `runRevokedResolver`; anchor it on `runSettledResolver`)
- Modify tests: `internal/gatedrive/takeover_test.go`, `internal/gatedrive/takeover_run_test.go`, `internal/gatedrive/takeover_integration_test.go`, `internal/gatedrive/takeover_race_integration_test.go`, `internal/gatedrive/driver_faults_test.go`, `internal/gatedrive/sequence_integration_test.go`, `internal/app/runtracker_fence_integration_test.go`, `internal/app/runtracker_launch_gate_integration_test.go`

**Interfaces:**
- Consumes: Task 2 (no app caller of `Driver.Takeover` other than `gatedriveContinuationSeam.TakeoverAndHandoff`, which always passes the id `LocateOuterDrive` returned).
- Produces: `(*Driver).Takeover(scopeID, parentCapability, driveID string) (DriveDoc, error)` —
  same signature; an empty `driveID` is a command error. `(*Store).claimScopeForTakeover(scopeID string) error`.
  `CauseTakeoverAmbiguous` stays (it is `app.ReasonRunTakeoverAmbiguous`). `CauseRunRecordUnreadable`
  stays until Task 6 (it still has the `resolveDriveRun` scope-branch producer).

- [ ] **Step 1: Add the outer-scope test helpers and re-point the kept takeover tests**

In `internal/gatedrive/takeover_test.go`, add:

```go
// outerScopeReqFor builds the outer recovery scope run.start prepares for a
// dispatch whose drives carry req's identity: repo, change, branch, and worktree —
// never a task, phase, run id, or run context (change 0489).
func outerScopeReqFor(req StartRequest) ScopeRequest {
	return ScopeRequest{RepoIdentity: req.RepoDir, ChangeID: req.ChangeID, Branch: req.Branch, Worktree: req.Worktree}
}

// startUnderOuterScope prepares an outer scope matching sampleStart's identity and
// starts a drive whose RunContext is that scope's child capability — exactly how
// run.start's run context links implement-next's suite gates to the run — and
// asserts the first slice WAITs.
func startUnderOuterScope(t *testing.T, d *Driver, store *Store) (ScopeGrant, DriveDoc) {
	t.Helper()
	req := sampleStart()
	grant, err := store.PrepareScope(outerScopeReqFor(req))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.RunContext = grant.ChildCapability
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("a drive under the outer scope must WAIT on its first slice, got %s (%s)", started.Outcome, started.Cause)
	}
	return grant, started
}
```

Re-point onto `startUnderOuterScope` (replacing `bindWaiting`): `TestTakeoverInvalidatesChildAndMintsParentOwner`,
`TestTakeoverRace`, and every remaining row of `TestTakeoverFailClosedTable`. Rename
`TestTakeoverTerminalUnconsumed` to `TestTakeoverAcceptsDriveThatFinishedAfterScan`, re-point it,
and change its doc comment: "the outer scan saw the drive live, then the suite finished before the
takeover: `Takeover`, given the drive id explicitly, still accepts it and hands the recorded verdict
over unchanged — the suite that completed in that window is not lost (Review Focus 5)."

In `TestTakeoverFailClosedTable`, delete these rows: `identity mismatch task` and `identity mismatch
phase` (an outer scope pins neither), `two candidate drives for one outer scope` and `zero candidate
drives` (candidate resolution is the caller's: `FindScopeDriveIDs` plus
`runTrackerOuterContinuation`, pinned by Task 1's tests and
`TestIntegrationRunVerdictVerdictAmbiguousDrivesStops`). Delete `prepareOuterScope` and
`startNested` once unused.

Add one row-independent test:

```go
// TestTakeoverRequiresExplicitDriveID (change 0489): Takeover no longer resolves a
// drive itself; its only caller passes the id the outer scan found. An empty id is
// a command error that mutates neither record.
func TestTakeoverRequiresExplicitDriveID(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	grant, _ := startUnderOuterScope(t, d, store)
	scopePath := filepath.Join(store.scopeRoot, grant.ScopeID, recordFileName)
	before := mustReadBytes(t, scopePath)
	if _, err := d.Takeover(grant.ScopeID, grant.ParentCapability, ""); err == nil {
		t.Fatalf("an empty drive id must be a command error")
	}
	if got := mustReadBytes(t, scopePath); string(got) != string(before) {
		t.Fatalf("a refused takeover must not mutate the scope record")
	}
}
```

Re-point the two integration tests the same way: in `TestIntegrationGatedriveTerminalConsumedFromFreshProcess`
and `TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity`, prepare the scope with
`outerScopeReqFor(req)` and set `req.RunContext = grant.ChildCapability` instead of
`req.ScopeID`/`req.ChildCapability`. Every identity/relaunch assertion stays; reword "scope-bound" to
"outer-scope" in their comments.

- [ ] **Step 2: Run the re-pointed tests (green before the production change)**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestTakeover'`
Expected: PASS except `TestTakeoverRequiresExplicitDriveID`, which FAILS (today an empty id resolves
through the scan). The re-pointed tests pass on the old code: the outer path already behaves this way.

- [ ] **Step 3: Narrow `Takeover`**

In `internal/gatedrive/takeover.go`:
- `Takeover` begins with `if driveID == "" { return DriveDoc{}, fmt.Errorf("gatedrive: takeover requires an explicit drive id") }` (add the `fmt` import).
- Delete the run-revocation block (`d.runRevoked` / `scope.RunID`), the reserved/pending-ack slot
  check, and the `resolveTakeoverDrive` call; load `driveID` directly. Every other check stays in its
  current order: scope load failure mapping, closed, parent capability, drive load failure mapping,
  identity (`scopeIdentityMatch`), outstanding handoff, fingerprint, live-drive deadline, superseded
  owner present, single-use scope claim, owner CAS.
- Delete `resolveTakeoverDrive` and `Driver.PrepareScope` (no caller since Task 2; `run.start` uses
  `Store.PrepareScope`).
- `claimScopeForTakeover(scopeID string) error`: delete the `driveID` parameter and the
  `CurrentDriveID` revalidation; it only refuses an already-closed scope and closes it. Keep its
  single-use paragraph.
- Rewrite the file header and `Takeover`'s doc: recovery scopes exist only at the coordinator →
  implement-next boundary; `run.verdict` resolves the one live candidate (`FindScopeDriveIDs`) and
  calls `Takeover` with its id; a drive that finished after the scan is still accepted.

In `internal/gatedrive/driver.go`, delete `RunRevokedFunc`, `SetRunRevokedResolver`, and the
`runRevoked` field; fix the `SetRunLaunchGate` comment that says "mirrors SetRunRevokedResolver". In
`internal/gatedrive/drive.go`, delete `CauseTakeoverNoCandidate` (no producer remains) and reword
`CauseTakeoverAmbiguous` ("the run tracker's outer scan found more than one live candidate").

In `internal/app`: delete `runRevokedResolver` and every wiring call to it (`newOwnedGateDriveService`,
`NewCommandlessGateDriveService`, `NewContinuationSeam`) with their comments. `findRunByID` stays (it
is used by `runtracker_fence.go`).

- [ ] **Step 4: Delete the tests whose subject is gone**

Grep `git grep -n 'SetRunRevokedResolver\|runRevokedResolver\|resolveTakeoverDrive\|CauseTakeoverNoCandidate\|claimScopeForTakeover\|\.Takeover(' -- internal`
and decide each hit by the Global Constraints rule. Known deletions:
- `internal/gatedrive/takeover_run_test.go`: `TestTakeoverCannotReviveCancelledRun` and
  `bindWaitingWithRun` — `git rm` the file if empty.
- `internal/gatedrive/takeover_test.go`: `TestTakeoverResolvesCurrentNotAcknowledgedPredecessor`,
  `TestTakeoverUnacknowledgedTerminalResult`, `TestTakeoverExplicitStaleDriveIDRefused`,
  `TestTakeoverReservedOrPendingSlotHalts`, `TestClaimScopeForTakeoverRevalidates`,
  `TestTakeoverRaceVsSuccessorStart`, `TestTakeoverRaceVsFinalAcknowledge`, and helpers left unused
  (`scopedSequenceProc`, `scopedPredecessorThenSuccessor`, `overwriteScopeRecord` if unused).
- Any task-scope test that calls `Takeover` with `""` and now fails (expected:
  `TestFaultPredecessorRetirementInterruptedThenRestart` in `driver_faults_test.go`,
  `TestIntegrationGatedriveSequenceCredentialTheftRejected` in `sequence_integration_test.go`): delete
  it; its subject is the task scope.
- `internal/app/runtracker_fence_integration_test.go`: delete only the `runRevokedResolver` assertion
  inside the stale-run case; the launch-gate assertions stay.
- `internal/app/runtracker_launch_gate_integration_test.go`: delete
  `TestIntegrationRunRecordRunRevokedResolverRevokesCompletingAndCompleted` (its subject is gone).

Keep `bindWaiting` and `scopeReqFor` if tests outside this task still use them (Task 5 deletes them
with the scoped-start tests).

- [ ] **Step 5: Run the task's tests**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ ./internal/app/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -race -tags integration -run '^TestRaceIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationRunVerdict|^TestIntegrationRunFence|^TestIntegrationRunRecord' ./internal/app/`
- the two Global Constraints commands
Expected: PASS.

- [ ] **Step 6: Mutation-test**

(a) `TestTakeoverRequiresExplicitDriveID`'s evidence is Step 2's red on the old resolve-by-scan code
(an empty id took over the one live candidate). Removing only the new guard leaves it green, because
`Store.Load("")` already refuses an invalid id; the guard stays for its explicit message — say so in
the report rather than calling the test vacuous. (b) Make `Takeover` halt when
`isTerminalOutcome(rec.LastOutcome)`: `TestTakeoverAcceptsDriveThatFinishedAfterScan` must go red.
Restore; record the red.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/takeover.go internal/gatedrive/driver.go internal/gatedrive/drive.go \
  internal/gatedrive/takeover_test.go internal/gatedrive/takeover_integration_test.go \
  internal/gatedrive/takeover_race_integration_test.go internal/gatedrive/driver_faults_test.go \
  internal/gatedrive/sequence_integration_test.go \
  internal/app/runtracker_run_record.go internal/app/gate_drive.go internal/app/runtracker_continuation.go \
  internal/app/runtracker_launch_gate.go internal/app/runtracker_fence_integration_test.go \
  internal/app/runtracker_launch_gate_integration_test.go
git rm internal/gatedrive/takeover_run_test.go   # if emptied
git commit -m "refactor(gatedrive): narrow Takeover to the outer recovery path (change 0489)"
```

---

### Task 5: Delete scoped starts, slot rotation, and terminal acknowledgement

**Build tier:** premium. Largest deletion; it collapses the admission ticket the build owner's
two-phase start depends on.

**Files:**
- Delete: `internal/gatedrive/acknowledge.go`, `internal/gatedrive/acknowledge_test.go`, `internal/gatedrive/admission_successor_test.go` (after moving `TestOrdinaryReleasePreservesRunID`)
- Modify: `internal/gatedrive/driver.go`, `internal/gatedrive/admission.go`, `internal/gatedrive/store.go` (comments only)
- Modify tests: `driver_test.go`, `driver_concurrency_test.go`, `driver_faults_test.go`, `driver_runfence_test.go`, `slot_run_fence_test.go`, `run_launch_gate_test.go`, `handoff_test.go`, `driver_transfer_test.go`, `takeover_test.go`, `reconcile_test.go`, `admission_test.go`, `sequence_integration_test.go` (all under `internal/gatedrive/`)
- Modify: `tests/test_go_integration_gatedrive_race.sh` header (it no longer runs "concurrent scopes driven to terminal")

**Interfaces:**
- Consumes: Tasks 3–4 (no caller sets `StartRequest.ScopeID`; `Takeover` never reads a task slot).
- Produces: `StartRequest` keeps `RunContext` and `RunID` and loses `ScopeID`, `ChildCapability`,
  `PredecessorDriveID`, `PredecessorOwnerGen`. `AdmissionTicket` keeps `id, ownerGen, rec, token,
  legacy, runID`. `Driver.Admit`/`StartAdmitted`/`AbandonAdmission` keep their signatures. The scope
  store's lifecycle helpers (`reserveScopeDrive`, `confirmScopeLaunch`, `clearPendingAck`,
  `closeScope`, `retirePredecessor`, `predecessorReusableError`, `predecessorReceipt`), the census
  walk, `driveRecord.ScopeID`, and `Claim`'s scope close **stay until Task 6** — `reconcile_test.go`'s
  census fixtures still use them.

- [ ] **Step 1: Move the one general admission test out of the successor file**

`TestOrdinaryReleasePreservesRunID` in `admission_successor_test.go` pins a general admission
property (an ordinary release keeps the slot's `RunID`). Move it verbatim into `admission_test.go`
(with any helper it needs that is not already there), then `git rm internal/gatedrive/admission_successor_test.go`.
Run `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestOrdinaryReleasePreservesRunID$'` — PASS.

- [ ] **Step 2: Re-point the relaunch run-fence tests**

In `driver_runfence_test.go`, `startScopedWaitingWithRun` links a drive to its run through a scope.
Replace it with a scopeless start carrying `req.RunID = runID` (the worktree slot records the run, so
`resolveDriveRun` answers it through the exact-reservation match). Keep each caller's assertions
(`TestRelaunchAuthorizedUnderGateThenLaunchedOutside`, `TestRecoveredRelaunchValidatesRunBeforeClaim`,
`TestNoRunAcquisitionWhileClaimHeld`, `TestRelaunchRefusedWhenRunRevoked`); only the linkage changes.
Run `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestRelaunch|^TestRecoveredRelaunch|^TestNoRunAcquisition'` — PASS on the old code.

- [ ] **Step 3: Delete the scoped start path and acknowledgement**

Grep: `git grep -n 'precheckScopedStart\|scopedIdentityMatch\|scopedRunID\|AdvisoryRunID\|admitScoped\|launchScoped\|admitScopedWorktree\|isSameScopeRaceLoss\|siblingMayHoldReservation\|scopedAdmissionHook\|rotateWorktreeExecutionForSuccessor\|reservedFresh\|ownsSlot\|releasable()\|t\.scoped\|\.Acknowledge(\|closeScopeFinal\|PredecessorDriveID\|PredecessorOwnerGen\|ChildCapability' -- internal`.

- `git rm internal/gatedrive/acknowledge.go`.
- `driver.go`: delete the four `StartRequest` fields and their comments (rewrite `RunContext`'s and
  `RunID`'s comments for a scopeless start); in `Admit`, delete the scope pre-check block and the
  `rec.ScopeID` stamping, and the admit closure calls only `admitScopeless`; delete
  `precheckScopedStart`, `scopedIdentityMatch`, `scopedRunID`, `AdvisoryRunID`, `scopedAdmissionHook`,
  `admitScoped`, `launchScoped`, `admitScopedWorktree`, `isSameScopeRaceLoss`,
  `siblingMayHoldReservation`. Collapse `AdmissionTicket` to `id, ownerGen, rec, token, legacy, runID`
  and delete `releasable`. `StartAdmitted` returns `d.launchScopeless(t, claim)`.
  `verifyAdmittedSlot` always expects `admissionReserved`. `settleAdmittedAfterRunRefusal` always
  releases the ticket's slot. `AbandonAdmission` returns nil only when `t == nil || t.token == ""`.
  Rewrite every comment that mentions scoped starts, the task-intent owner, scope slots, or
  "scope-busy" (`Start`, `Admit`, `AdmissionTicket`, `StartAdmitted`, `revalidateAdmittedLaunch`,
  `launchScopeless`, `admitScopeless`).
- `admission.go`: delete `rotateWorktreeExecutionForSuccessor`. Leave the slot `ScopeID` field for
  Task 6. Change the `Kind` field comment to `"scopeless"|"raw"; "scoped" is a legacy kind only
  pre-0489 slots carry, still settled by finished-incumbent reconciliation`.
- `store.go`: rewrite the `NewDrive`/`NewReservedDrive`/`removeReservedDrive`/`attachLaunch` comments
  that cite `admitScoped` or a "scoped start" (they describe the reserve-then-launch start every
  drive uses now).

- [ ] **Step 4: Delete the scoped-start tests**

Compile `go vet ./internal/gatedrive/` and work through the errors. Every test that sets
`StartRequest.ScopeID`/`ChildCapability`/predecessor fields, calls `Acknowledge`, the scoped admission
hook, or `AdvisoryRunID`, or whose subject is a task scope's sequence, is deleted. Known set (from the
spec and a pre-scan): in `driver_test.go` — `prepareScopedStart`, `prepareScopedStartAt`,
`scopedTestDriver`, `TestScopedStartReservesBeforeLaunch`, `TestScopedStartLaunchFailureFailsClosed`,
`TestScopedStartReservationFailureCleansOrphanedReservedDrive`, `TestScopedStartAttachLaunchFailureStopsOrphan`,
`TestTwoScopesOneWorktreeSecondRefused`, `TestScopedSequentialStarts`, `TestScopedSequenceBaselineRedGreen`,
`scopedSuccessorFixture`, `TestScopedSuccessorRejectionMatrix`, `TestScopedSuccessorPredecessorStateRejections`;
in `driver_concurrency_test.go` — `TestDriverConcurrencyScopedStartReservationRace`,
`TestDriverConcurrencySuccessorStartRace`, `TestBarrierSameScopeFirstStartContention`,
`TestSameScopeFirstStartLateLoserDoesNotRotate`, `TestSameScopeSuccessorStaleReceiptDoesNotRotate`,
`TestSameScopeSuccessorScopeReadFailureFailsClosed`, `TestSameScopeSuccessorGuardAppliesWholeReservePredicate`,
`successorOf`, `setScopedAdmissionHook`, `TestStaleSuccessorLeavesSiblingAdoptedReservation`,
`TestBarrierSuccessorUnderCancel`, `TestReconcileSuccessorRaceLeavesSuccessorUntouched`; in
`driver_faults_test.go` — `successorReq`, `TestFaultAdmissionThenScopeReservationLostReleasesSlot`,
`TestFaultSuccessorReservationWriteFailsThenRestart`, `TestFaultAttachLaunchFailsThenRestart`,
`TestFaultFinalAckInterruptedThenRestart`, and `TestFaultLaunchLostResponseLeavesUnresolved` if it is
scoped; in `slot_run_fence_test.go` — `prepareRunScopedStart`, `TestScopedStartCarriesRunIntoSlot`,
`TestScopedStartInheritsScopeRun`, `TestScopedStartPresentingScopeRunAdmits`,
`TestScopedStartForeignRunRefused`, `TestScopedSuccessorStartInheritsScopeRun`,
`TestNoRunRecordScopeKeepsPresentedRun`, `TestAdvisoryRunIDMatchesAdmit`; in `run_launch_gate_test.go`
— the `scoped` subtests only; in `handoff_test.go` — `TestScopedWaitingHandoffClaimClosesScope`,
`TestClaimedScopeAcknowledgeAndStartAreTransferred`; in `driver_transfer_test.go` — `TestClaimClosesScope`;
in `takeover_test.go` — `TestStartBindsScope`, `TestFindScopeDriveIDsExcludesAcknowledgedHistory`,
`bindWaiting`, `scopeReqFor` if unused; in `reconcile_test.go` — the `scoped` variants of
`admitRunDrive` callers (drop the `scoped` parameter; the scopeless twin stays); in
`sequence_integration_test.go` — every remaining test and helper (it is the task-scope sequence
suite; `git rm` the file if empty). Tests in these files with no task-path hit (for example
`TestRunOmissionCannotDetachOwnedWorktree`, `TestStandaloneSlotFencesNoRun`,
`TestDistinctWorktreesProgressConcurrently`) stay; if one used a scoped helper only incidentally,
re-point it to a scopeless start.

`TestProcessSeamSatisfiedByRealService` mentions `reserveScopeDrive`/`confirmScopeLaunch`: read it
and remove only the scope clause.

Edit `tests/test_go_integration_gatedrive_race.sh`'s header: drop "concurrent scopes driven to
terminal" from the list of what the shard runs.

- [ ] **Step 5: Run the task's tests**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -race ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -race -tags integration -run '^TestRaceIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationBuildStart|^TestIntegrationRunStart|^TestIntegrationRunCancel' ./internal/app/`
- the two Global Constraints commands
Expected: PASS. The build owner's admission-precedes-charging tests in `internal/app` must stay green
unchanged (they exercise `Admit`/`StartAdmitted`/`AbandonAdmission`).

- [ ] **Step 6: Commit**

```bash
git rm internal/gatedrive/acknowledge.go internal/gatedrive/acknowledge_test.go internal/gatedrive/admission_successor_test.go
git add internal/gatedrive/driver.go internal/gatedrive/admission.go internal/gatedrive/store.go \
  internal/gatedrive/admission_test.go internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go \
  internal/gatedrive/driver_faults_test.go internal/gatedrive/driver_runfence_test.go internal/gatedrive/slot_run_fence_test.go \
  internal/gatedrive/run_launch_gate_test.go internal/gatedrive/handoff_test.go internal/gatedrive/driver_transfer_test.go \
  internal/gatedrive/takeover_test.go internal/gatedrive/reconcile_test.go tests/test_go_integration_gatedrive_race.sh
git add internal/gatedrive/sequence_integration_test.go   # or: git rm it if emptied
git commit -m "refactor(gatedrive): delete scoped starts, slot rotation, and terminal acknowledgement (change 0489)"
```

---

### Task 6: Shrink the scope store to the outer scope, drop the census's scope walk, and pin old records

**Build tier:** premium. The census is cancellation's safety accounting; re-pointing its fixtures
must keep every surviving rule covered.

**Files:**
- Modify: `internal/gatedrive/scope.go` (records, request, reader, lifecycle deletions)
- Modify: `internal/gatedrive/takeover.go` (`scopeIdentityMatch` loses task/phase)
- Modify: `internal/gatedrive/store.go` (delete `retirePredecessor`, `predecessorReusableError`)
- Modify: `internal/gatedrive/driver.go` (`Claim` loses its scope close; `resolveDriveRun` loses its scope branch)
- Modify: `internal/gatedrive/drive.go` (delete `driveRecord.ScopeID`; delete `CauseRunRecordUnreadable` if no producer remains)
- Modify: `internal/gatedrive/reconcile.go` (delete the scope-registry walk and what only it feeds)
- Modify: `internal/gatedrive/admission.go`, `internal/gatedrive/admission_retire.go` (delete the slot `ScopeID` field; comment)
- Modify tests: `scope_test.go`, `reconcile_test.go`, `admission_test.go`, `slot_run_fence_test.go`, `takeover_test.go`, plus a new `internal/gatedrive/old_records_test.go`; app tests that still build a `ScopeRequest` with dropped fields (compile surfaces them)
- Modify: `internal/app/runtracker_production_census_integration_test.go` (only `seedUnrelatedDamagedHistory`'s comment for the `scope_id` record: it is now "an old drive carrying `scope_id`, read as scopeless")

**Interfaces:**
- Consumes: Task 5 (no production code reads the slot lifecycle).
- Produces:
  - `type ScopeRequest struct { RepoIdentity, ChangeID, Branch, Worktree string }`
  - `scopeRecord` = `SchemaVersion int`, `RepoIdentity`, `ChangeID`, `Branch`, `Worktree`,
    `ChildCapHash`, `ParentCapHash string`, `Closed bool` — same JSON tags as today
    (`schema_version`, `repo_identity`, `change_id`, `branch`, `worktree`, `child_cap_hash`,
    `parent_cap_hash`, `closed`). `scopeSchemaVersion` stays `3`.
  - `scopeIdentityMatch(scope scopeRecord, repo, branch, worktree, change string) bool`
  - `(*Driver).resolveDriveRun(rec driveRecord) (runID string, ok bool, cause string)` — scopeless only.

- [ ] **Step 1: Write the old-record tests (they pin Decision 4)**

Create `internal/gatedrive/old_records_test.go`:

```go
package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// writeRawScope writes a scope record exactly as a pre-0489 binary left it on
// disk: the storedScope envelope around an arbitrary record map.
func writeRawScope(t *testing.T, store *Store, id string, record map[string]any) {
	t.Helper()
	dir := filepath.Join(store.scopeRoot, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir scope: %v", err)
	}
	buf, err := json.Marshal(map[string]any{"generation": "pre-0489-gen", "record": record})
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("write scope: %v", err)
	}
}

// TestOldTaskScopeOnDiskChangesNothing (change 0489, Decision 4; Review Focus 2):
// an OPEN task scope a pre-0489 binary left on disk — slot fields, a pending-ack
// journal, and a run id naming the run under test — is never read, so it blocks
// neither cancellation's census, the successful-run closeout census, nor admission.
func TestOldTaskScopeOnDiskChangesNothing(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	const runID = "run-0489-old-task-scope"
	writeRawScope(t, store, "abababababababababababababababab", map[string]any{
		"schema_version": 3, "repo_identity": "/repo", "change_id": "0342",
		"task_id": "task-6", "phase": "build", "branch": "feat/x", "worktree": sampleWorktree(),
		"child_cap_hash": capHash("old-child"), "parent_cap_hash": capHash("old-parent"),
		"run_id":              runID,
		"current_drive_id":    "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd",
		"current_drive_state": "reserved",
		"pending_ack_drive_id": "efefefefefefefefefefefefefefefef", "pending_ack_owner_gen": "old-gen",
		"drive_count": 2, "closed": false,
	})

	for name, census := range map[string]func(string, string) (RunLaunchReport, error){
		"cancel":   d.ReconcileRunLaunches,
		"closeout": d.ObserveRunLaunches,
	} {
		rep, err := census(sampleWorktree(), runID)
		if err != nil {
			t.Fatalf("%s census: %v", name, err)
		}
		if !rep.Accounted || len(rep.Findings) != 0 {
			t.Fatalf("%s census over an old task scope = accounted %v findings %v, want accounted with no findings", name, rep.Accounted, rep.Findings)
		}
	}

	req := sampleStart()
	req.RunID = runID
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("admission beside an old task scope must succeed: %v", err)
	}
	_ = d.AbandonAdmission(ticket)
}

// TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver (change 0489, Decision 4;
// Review Focus 1): an outer scope written in the pre-0489 on-disk shape — every
// field the old binary wrote, including the ones this change drops — still loads,
// binds its change, and authorizes a takeover of the run's live drive, so a run in
// flight across the upgrade keeps its continuation path.
func TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	req := sampleStart()
	const runContext = "pre-0489-run-context"
	const parentCap = "pre-0489-parent-capability"
	const scopeID = "abababababababababababababababab"
	writeRawScope(t, store, scopeID, map[string]any{
		"schema_version": 3, "repo_identity": req.RepoDir, "change_id": "",
		"task_id": "", "phase": "", "branch": req.Branch, "worktree": req.Worktree,
		"child_cap_hash": capHash(runContext), "parent_cap_hash": capHash(parentCap),
		"drive_count": 0, "closed": false,
	})
	if _, err := store.LoadScope(scopeID); err != nil {
		t.Fatalf("LoadScope of a pre-0489 outer scope: %v", err)
	}
	if err := d.BindScopeChange(scopeID, req.ChangeID); err != nil {
		t.Fatalf("BindScopeChange: %v", err)
	}
	req.RunContext = runContext
	started, err := d.Start(req)
	if err != nil || started.Outcome != WAITING {
		t.Fatalf("Start under the old outer scope: %v (%s)", err, started.Outcome)
	}
	ids, err := store.FindScopeDriveIDs(req.ChangeID, capHash(runContext))
	if err != nil || len(ids) != 1 || ids[0] != started.DriveID {
		t.Fatalf("outer scan = %v, %v; want [%s]", ids, err, started.DriveID)
	}
	took, err := d.Takeover(scopeID, parentCap, started.DriveID)
	if err != nil || took.Outcome == HALTED {
		t.Fatalf("takeover under a pre-0489 outer scope: err=%v outcome=%s cause=%q", err, took.Outcome, took.Cause)
	}
}

// TestOldDriveRecordWithScopeIDSettlesScopeless (change 0489, Decision 4; Review
// Focus 3): a pre-0489 task drive record still carrying scope_id — naming a scope
// that is not on disk — decodes and resolves as a scopeless, no-run-record drive,
// never run-record-unreadable, so the census treats it as history.
func TestOldDriveRecordWithScopeIDSettlesScopeless(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	rec := seedRecord(t)
	rec.LastOutcome = WAITING
	id, _, err := store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	path := filepath.Join(store.root, id, recordFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	env["record"].(map[string]any)["scope_id"] = "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
	if raw, err = json.Marshal(env); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load of an old drive carrying scope_id: %v", err)
	}
	if runID, ok, cause := d.resolveDriveRun(got); !ok || runID != "" || cause != "" {
		t.Fatalf("old drive must resolve as scopeless no-run-record, got (%q, %v, %q)", runID, ok, cause)
	}
	rep, err := d.ReconcileRunLaunches("", "run-0489-any")
	if err != nil || !rep.Accounted {
		t.Fatalf("census over an old scope_id drive = %+v, %v; want accounted", rep, err)
	}
}
```

Use 32-hex ids that `validateID` accepts (adjust if the real id length differs). Draft code: verify
it compiles and can pass.

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestOld'`
Expected: `TestOldTaskScopeOnDiskChangesNothing` FAILS (today's census walk reads the scope:
`record-missing:` findings); `TestOldDriveRecordWithScopeIDSettlesScopeless` FAILS (today's scope
branch reports `run-record-unreadable`); `TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver` passes
already (it pins behavior that must survive).

- [ ] **Step 2: Re-point the census fixtures before deleting the walk**

In `reconcile_test.go`, replace `seedScopedRunDrive` with a slot-linked seed — the way production
links a build-owned drive to its run:

```go
// seedSlotLinkedRunDrive persists a drive linked to runID the way production links
// a build-owned drive: a worktree execution slot reserved for runID on the drive's
// worktree, with the drive carrying that slot's reservation token. resolveDriveRun
// answers runID for it, and the slot's current token is the census's positive
// current reference to it (change 0446 spec §4). mutate tweaks the record before it
// is persisted.
func seedSlotLinkedRunDrive(t *testing.T, store *Store, worktree, runID string, mutate func(*driveRecord)) (id, ownerGen string) {
	t.Helper()
	rec := seedRecord(t)
	rec.WorktreePath = worktree
	token, err := store.ReserveWorktreeExecution(admissionRecord{
		RepoIdentity: rec.RepoIdentity, WorktreeRoot: worktree, RunID: runID, Kind: "scopeless",
	})
	if err != nil {
		t.Fatalf("ReserveWorktreeExecution: %v", err)
	}
	rec.AdmissionToken = token
	if mutate != nil {
		mutate(&rec)
	}
	id, _, err = store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return id, rec.OwnerGeneration
}
```

A test that seeds more than one run-linked drive gives each its own worktree (the package's
`mkWorktree(t)` helper or an equivalent), because one worktree slot holds one reservation; pass that
worktree to `ReconcileRunLaunches`/`ObserveRunLaunches` where the test passes one. Keep every
assertion of each re-pointed test. `TestReconcileLostLinkageFailsClosed` loses linkage by making the
slot unreadable or reassigning its token (the scopeless lost-linkage cause, `CauseRunLinkLost`)
instead of a missing scope. Delete the census tests whose subject is the scope walk:
`TestCensusScopeNamedMissingDriveBlocks`, `TestCensusWithdrawnReservationStaysAccounted`,
`seedLaunchedScopePredecessor`, and any `scope-unreadable:` / `scope-registry-unreadable` /
`reservation-withdrawn:` cases. `TestCensusSupersededRunStillEnumerates` depended on scope
enumeration with an empty worktree: re-point it so the census runs against the replacement's worktree
(the app's `resolveTerminalRunSlot` path supplies it), or delete it if its only subject was the
registry walk; say which in the report.

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ -run '^TestReconcile|^TestCensus|^TestObserveRunLaunches'`
Expected: PASS on the still-unchanged production code.

- [ ] **Step 3: Delete the census walk and the scope lifecycle**

Grep: `git grep -n 'reserveScopeDrive\|scopeReserveRefusal\|confirmScopeLaunch\|clearPendingAck\|closeScope\b\|retirePredecessor\|predecessorReusableError\|predecessorReceipt\|scopeStateReserved\|scopeStateLaunched\|scopeSchemaVersionLegacy\|CurrentDriveID\|PendingAck\|FinalAcked\|DriveCount\|PriorDriveID\|censusRefs\|reservedBy\|journalOpen\|releasedScope\|withdrawn\|accountMissing\|ScopeID\b\|RunContextHash\|TaskID\|CauseRunRecordUnreadable' -- internal`.

- `scope.go`: delete `scopeSchemaVersionLegacy`, `scopeStateReserved`, `scopeStateLaunched`,
  `predecessorReceipt` (+ `empty`, `halfFilled`), `reserveScopeDrive`, `scopeReserveRefusal`,
  `confirmScopeLaunch`, `clearPendingAck`, `closeScope`. Trim `scopeRecord` and `ScopeRequest` to the
  Interfaces shapes; `PrepareScope` writes only those fields. `readStoredScope` accepts schema `3`
  only. Rewrite the file header: one outer recovery scope per run, prepared by `run.start`, bound by
  `run.verdict`, read by `run.cancel`, closed once by `Takeover`; no per-test lifecycle.
- `takeover.go`: `scopeIdentityMatch` drops `task, phase` and their clauses; update its caller.
- `store.go`: delete `retirePredecessor` and `predecessorReusableError`.
- `driver.go`: `Claim` drops the `rec.ScopeID` close and its comment; `resolveDriveRun` drops the
  scope branch and its doc sentences.
- `drive.go`: delete `driveRecord.ScopeID` (keep `RunContextHash` and reword their shared comment);
  delete `CauseRunRecordUnreadable` if the grep shows no producer.
- `reconcile.go`: delete the scope-registry walk in `censusReferences`, the slot-named scope load,
  `censusRefs.ids`/`reservedBy`/`journalOpen`/`releasedScope`, `withdrawn`, `accountMissing` and its
  call sites, and the rule-4/rule-7 text. `names` keys only on the slot token; `namesUnreadable` keeps
  `slotOccupied && !holderFound`. Renumber the attribution rules in `accountRunLaunches`'s comment and
  rewrite the file header ("the target worktree's slot" is the only current reference).
- `admission.go`: delete the slot `ScopeID` field; `admission_retire.go`'s comment drops `ScopeID`.
- Tests: `scope_test.go` keeps `TestPrepareScopeMintsSeparatedCapabilities` (trimmed to the outer
  request), `TestScopeIdentityFailClosed`, `TestBindScopeChangeOnce` (its `closeScope` setup becomes
  `claimScopeForTakeover`), and a schema test that unknown versions — including `2` — fail closed;
  delete the slot/receipt/pending-ack tests (`TestScopeV2RoundTripZeroSlot`,
  `TestScopeSchemaV1FailClosed` if its subject is the slot, `TestScopeReserve*`,
  `TestScopeConfirmLaunch`, `TestScopeClearPendingAck`, `TestScopeCASConcurrent`, `launchOne`).
  `slot_run_fence_test.go`: delete `TestScopeSchemaV2LegacyTolerated`. `admission_test.go`:
  `sampleAdmission` drops `ScopeID`. `takeover_test.go`: the `closed scope` row of
  `TestTakeoverFailClosedTable` closes the scope with `claimScopeForTakeover` instead of
  `closeScope`; delete `overwriteScopeRecord` if unused.

- [ ] **Step 4: Replace `TestScopeCASConcurrent` with a kept-transition race**

Add to `scope_test.go`:

```go
// TestScopeCASKeptTransitionsSerialize (change 0489) replaces TestScopeCASConcurrent:
// the scope store's conflict-checked write is exercised through the two transitions
// that survive — run.verdict's change bind racing Takeover's single-use close. No
// write is lost: the close always lands, and a bind that reported success is
// always visible. Run under -race.
func TestScopeCASKeptTransitionsSerialize(t *testing.T) {
	for i := 0; i < 50; i++ {
		store := OpenStore(testsupport.TempDir(t))
		grant, err := store.PrepareScope(ScopeRequest{RepoIdentity: "/repo", Branch: "feat/x", Worktree: "/repo"})
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		const binders = 8
		var wg sync.WaitGroup
		bindErrs := make([]error, binders)
		var closeErr error
		wg.Add(binders + 1)
		for b := 0; b < binders; b++ {
			go func(b int) { defer wg.Done(); bindErrs[b] = store.bindScopeChange(grant.ScopeID, "0342") }(b)
		}
		go func() { defer wg.Done(); closeErr = store.claimScopeForTakeover(grant.ScopeID) }()
		wg.Wait()
		if closeErr != nil {
			t.Fatalf("iteration %d: the single takeover close must land: %v", i, closeErr)
		}
		got, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if !got.Closed {
			t.Fatalf("iteration %d: a bind overwrote the takeover close (lost write)", i)
		}
		for b, berr := range bindErrs {
			switch {
			case berr == nil && got.ChangeID != "0342":
				t.Fatalf("iteration %d: bind %d reported success but its write was lost", i, b)
			case berr != nil && !isOwnershipKind(berr, ErrScopeClosed):
				t.Fatalf("iteration %d: bind %d failed with %v, want nil or scope-closed", i, b, berr)
			}
		}
	}
}
```

- [ ] **Step 5: Run the task's tests**

Run:
- `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -race ./internal/gatedrive/ -run '^TestScopeCAS|^TestTakeover|^TestOld'`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -race -tags integration -run '^TestRaceIntegrationGatedrive' ./internal/gatedrive/`
- `timeout --kill-after=10s 10m go test -count=1 -tags integration -run '^TestIntegrationRunCancel|^TestIntegrationRunCompletion|^TestIntegrationRunStart|^TestIntegrationRunVerdict' ./internal/app/`
- the two Global Constraints commands
Expected: PASS, including all three `TestOld*` tests.

- [ ] **Step 6: Mutation-test the new tests**

Each test must be shown red with its fixture's effect live (restore from backup after each
mutation; record each red):
1. `TestOldTaskScopeOnDiskChangesNothing` and `TestOldDriveRecordWithScopeIDSettlesScopeless`: Step 1
   ran them red on the unchanged code (the census walk read the old scope; the scope branch reported
   `run-record-unreadable`). That before/after pair is their evidence, because re-adding the deleted
   walk or branch needs the deleted fields. Quote both Step 1 failure messages in the report.
2. `TestOldOuterScopeOnDiskStillLoadsBindsAndTakesOver`: decode the scope with
   `json.Decoder.DisallowUnknownFields()` in `readStoredScope` — `LoadScope` must fail. Separately,
   change the fixture's `schema_version` to `2` — it must fail (schema 2 is no longer tolerated).
3. `TestScopeCASKeptTransitionsSerialize`: in `scopeCASOnce`, drop the flock and the generation
   comparison; run it with `-race -count=3`; it must go red at least once. If it never reddens,
   report that as a finding (learning: residual-is-for-undetectable-not-unprobed) rather than
   weakening the test.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/scope.go internal/gatedrive/takeover.go internal/gatedrive/store.go \
  internal/gatedrive/driver.go internal/gatedrive/drive.go internal/gatedrive/reconcile.go \
  internal/gatedrive/admission.go internal/gatedrive/admission_retire.go \
  internal/gatedrive/old_records_test.go internal/gatedrive/scope_test.go internal/gatedrive/reconcile_test.go \
  internal/gatedrive/admission_test.go internal/gatedrive/slot_run_fence_test.go internal/gatedrive/takeover_test.go \
  internal/app/runtracker_production_census_integration_test.go
git commit -m "refactor(gatedrive): shrink the scope store to the outer scope; census walks no scopes (change 0489)"
```

(Add any app test file the compile forced you to touch to the `git add` line.)

---

### Task 7: Prune unreachable ownership kinds and reword next-action messages

**Build tier:** standard.

**Files:**
- Modify: `internal/gatedrive/ownership.go` (delete unreachable `OwnershipErrorKind` constants)
- Modify: `internal/app/gate_drive.go` (`ownershipNextAction`; any surviving message that sends the reader to "the parent" or an "identity bundle")
- Modify tests: `internal/gatedrive/ownership_test.go` (`TestOwnershipKindSpellings`), `internal/app/gate_drive_test.go` (`TestMapDriveFailureOwnershipKinds`, `TestMapDriveFailureOwnershipNextAction`)

**Interfaces:**
- Consumes: Tasks 4–6 (no producer of the task-path kinds remains).
- Produces: the remaining kinds. Expected deletions: `ErrScopeSecondDrive` (`scope-second-live-drive`),
  `ErrScopeTransferred` (`scope-transferred`), `ErrScopeBusy` (`scope-busy`), `ErrStalePredecessor`
  (`stale-predecessor`), `ErrPredecessorNotReusable` (`predecessor-not-reusable`). Expected keeps:
  `ErrScopeCapabilityMismatch`, `ErrScopeClosed`, `ErrScopeIdentityMismatch` (Takeover/bind halt
  causes; `mapDriveHaltCause` still classifies `scope-identity-mismatch`), `ErrUnresolvedLaunchTransition`,
  and every non-scope kind.

- [ ] **Step 1: Prove each kind is unreachable**

For each candidate kind, `git grep -n '<ConstName>' -- internal ':!*_test.go'`. Delete a kind only
when its sole remaining non-test hit is its own declaration. Report the grep result per kind.

- [ ] **Step 2: Delete the kinds and their next-action cases**

Delete the unreachable constants and their comments. In `ownershipNextAction`, delete the case for
every deleted kind **and** for every kind no `gate.drive.*` command can return any more:
`ErrScopeClosed`, `ErrScopeCapabilityMismatch`, `ErrScopeIdentityMismatch` (only `Takeover` and the
in-process bind produce them, and no CLI command reaches either). Reword the surviving cases that
point at the deleted recovery path, for the scopeless caller:
- `ErrHandoffOutstanding`: `"claim the outstanding handoff instead of starting another drive"`
- `ErrUnresolvedLaunchTransition`: `"a prior launch transition is unresolved; settle it with run.cancel or wait for it, never a blind retry"`
- `ErrLaunchUnconfirmed`: `"a prior execution in this worktree is unresolved; recover it through run.cancel, never a blind re-start"`

Then `git grep -n -i 'parent\|identity bundle\|dispatch prompt' -- internal/app/gate_drive.go internal/gatedrive/ownership.go`
and reword any other surviving message or comment that sends the reader there.

- [ ] **Step 3: Update the pins**

- `TestOwnershipKindSpellings`: keep only the kinds that remain (`unresolved-launch-transition`,
  `scope-closed`), and update its comment.
- `TestMapDriveFailureOwnershipKinds`: drop the deleted kinds from its list.
- `TestMapDriveFailureOwnershipNextAction`: drop the deleted kinds and the three kinds whose case was
  removed. The "distinct non-empty message per kind" assertion stays.

- [ ] **Step 4: Run the task's tests**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/gatedrive/ ./internal/app/ -run 'Ownership|MapDrive'`
and the two Global Constraints commands. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/ownership.go internal/app/gate_drive.go internal/gatedrive/ownership_test.go internal/app/gate_drive_test.go
git commit -m "refactor(gatedrive): prune unreachable task-path ownership kinds and messages (change 0489)"
```

---

### Task 8: Sweep the glossary, the Codex dispatch clause, and kept-code comments

**Build tier:** standard.

**Files:**
- Modify: `docs/reference/glossary.md`
- Modify: `internal/harness/dispatch.go` (the Codex root-entry clause)
- Modify: `internal/repoguard/gatedrive_task_absence_test.go` (header comment only)
- Modify: comments in kept code under `internal/gatedrive/`, `internal/app/` (`gate_drive.go`, `runtracker_*.go`, `workspace_ops.go`), `internal/cli/` (`gate.go`, `run.go`)

**Interfaces:** none (prose and comments only).

- [ ] **Step 1: Glossary**

In *Gate drive / slice / owner generation / handoff / takeover*, replace the sentences from "The
catalog still carries the recovery-scope operations" through "Change 0489 decides their fate." with:

> The run tracker keeps one recovery scope per run internally: `run.start` prepares it, and
> `run.verdict` can take over a drive that is still running through it when implement-next stopped
> early. A drive that already finished is never taken over; the run's retry re-runs the gate. No
> operation exposes scopes.

In *Worktree changed / certified input changed*, "change, task or phase is not the scope's" becomes
"or change is not the scope's".

- [ ] **Step 2: Codex dispatch clause**

In `internal/harness/dispatch.go`, change "labeled for `--run-id` on prepare-scope and build-owned
starts" to "labeled for `--run-id` on build-owned starts". This repo does not enable Codex, so no
managed block regenerates. Run
`timeout --kill-after=10s 10m go test -count=1 ./internal/harness/ ./internal/repoguard/ -run 'Codex|Dispatch|RunID'`
— `TestCodexRequestFileCarriesRunID` must stay green (its regex needs only "run id … `--run-id`").

- [ ] **Step 3: Repoguard header**

In `internal/repoguard/gatedrive_task_absence_test.go`'s header, "the Go catalog keeps these
operations until change 0489 deletes them" becomes past tense ("change 0489 deleted these operations
from the Go catalog"). Do not change the guard's regex or cases: the absence guard stays as it is.

- [ ] **Step 4: Comment sweep**

Run:
`git grep -n -i -E 'task scope|task-owned|task-intent|worker scope|successor|predecessor|prepare-scope|acknowledg|terminal-unconsumed|terminal-but-unconsumed|unconsumed|pending-ack|scope slot|FinalAcked|sequential scope|scoped start|owner task|child capability' -- internal cmd ':!*_test.go'`.
For each hit in kept code, rewrite the comment to describe what the code does now (scopeless build
and finalize drives; the outer recovery scope). Hits that describe still-true history with a change
number may stay if they are accurate. Expected files include `takeover.go`, `scope.go`, `store.go`,
`drive.go`, `driver.go`, `run_waiting.go`, `admission.go`, `incumbent.go` (keep its legacy `"scoped"`
kind note), `internal/app/gate_drive.go`, `runtracker_start.go`, `runtracker_store.go`,
`runtracker_run_id_refusal.go` (prepare-scope is gone; `runIDLocator` now serves only
`CheckRunIDExists`/`agent.enter`), `runtracker_continuation.go`, `workspace_ops.go`,
`internal/cli/gate.go`, `internal/cli/run.go`. Anchor any cross-reference on a symbol name, never a
line number.

- [ ] **Step 5: Skills and maintained prose**

Run `git grep -n -E 'prepare-scope|gate\.drive\.(acknowledge|takeover)|gate drive (acknowledge|takeover|prepare-scope)|--owner task|--child-cap|--scope-id|predecessor-(drive-id|owner-gen)' -- ':!docs/results' ':!docs/superpowers' ':!docs/changes' ':!docs/adrs' ':!internal/repoguard'`.
Expected: no hits outside test fixtures that deliberately spell the retired surface (the
`internal/cli` absence tests from Tasks 2–3). If a skill or cursor rule hits, fix it and run
`go generate ./internal/assets`, staging the regenerated paths.

- [ ] **Step 6: Run the task's tests**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ ./internal/harness/` and the
two Global Constraints commands. Expected: PASS (repoguard pins prose; a red here names a guard that
greps a sentence you changed — repoint the assert at the owning text, never restore the old prose).

- [ ] **Step 7: Commit**

```bash
git add docs/reference/glossary.md internal/harness/dispatch.go internal/repoguard/gatedrive_task_absence_test.go
git add <each kept-code file whose comments changed>
git commit -m "docs(gatedrive): describe the outer recovery scope; drop task-path prose (change 0489)"
```

---

### Task 9: Re-budget the shrinking test files

**Build tier:** economy.

**Files:**
- Modify: `tests/runtime-budgets.tsv` (rows for `tests/test_go_toolchain.sh`, `tests/test_go_race.sh`, `tests/test_go_integration_gatedrive_process.sh`, `tests/test_go_integration_gatedrive_race.sh`)

**Interfaces:** none.

- [ ] **Step 1: Measure serially**

From the feature worktree root, run each file alone, twice, and record the wall-clock seconds:

```bash
for f in tests/test_go_toolchain.sh tests/test_go_race.sh \
         tests/test_go_integration_gatedrive_process.sh tests/test_go_integration_gatedrive_race.sh; do
  for i in 1 2; do /usr/bin/time -p bash "$f" >/dev/null 2>"/tmp/budget.$$.err"; grep '^real' "/tmp/budget.$$.err"; done
done
```

(Template the temp file under `${TMPDIR:-/tmp}` with `mktemp` if you keep it.) Each run must end
green; a red run is a finding to report, not a number to budget.

- [ ] **Step 2: Lower only rows that shrank**

For each file, `new = ceil5(max measured) + 5`, minimum `10` (the table's seeding rule in its
header). Change a row only when `new` is **below** its current ceiling; never raise a row. Record each
file's measurements, current ceiling, and new ceiling in the task report.

- [ ] **Step 3: Run the budget correspondence guard**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/ -run 'RuntimeBudgets'` — PASS.

- [ ] **Step 4: Commit**

```bash
git add tests/runtime-budgets.tsv
git commit -m "test: lower runtime budgets for the shrunken gate-drive corpus (change 0489)"
```

(If no row shrank, make no commit and report the measurements.)

---

## After the plan (not build tasks)

- **Build gate:** the controller runs the whole suite through `build.test_command`
  (`go run ./cmd/docket development test`) and reads every `BUDGET WATCH:` and
  `SERIAL CONFIRMED OVER BUDGET:` line, green or not.
- **ADR:** implement-next Step 6 records *The run tracker's outer takeover recovers only a
  still-running drive* through `docket-adr`, `relates_to: [107, 130]`, with the spec's Decision
  record content (context: Problem facts 1 and 3; decision: Decisions 2–3, recovery scopes exist only
  at the coordinator → implement-next boundary with no per-test lifecycle; consequences: the accepted
  loss; alternatives: Decision 3's rejected list). No ADR for Decision 4.
- **Results file:** no Important human action beyond the standard post-merge reinstall. One Optional
  check: the next time an implement-next run dies after its gate finished, `run.verdict` answers
  `run-retry-once`, not `run-stop … takeover-ambiguous`. Note for change 0491's spec: this change
  deleted the takeover's run-revocation check.

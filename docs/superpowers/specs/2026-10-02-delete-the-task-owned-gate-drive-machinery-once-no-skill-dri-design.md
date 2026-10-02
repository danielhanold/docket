<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0489 — Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-02-0489-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md)**
<!-- docket:backlink:end -->

# Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives — design

## Problem

Change 0488 (ADR-0130) moved build-task workers off the gate driver: they run focused tests directly under `timeout`, and no skill starts a task-owned drive. ADR-0130 left the Go machinery "in place but unused until change 0489 deletes it". A whole-repo grep of maintained skills, agents, and cursor rules finds no caller of:

- `--owner task`;
- `--scope-id` or `--child-cap`;
- `--predecessor-drive-id` or `--predecessor-owner-gen`;
- `gate.drive.prepare-scope`, `gate.drive.acknowledge`, or `gate.drive.takeover`.

The unreachable code is:

- the task-intent owner;
- scoped starts with predecessor receipts;
- the scope's slot lifecycle and pending-ack journal;
- terminal acknowledgement;
- the CLI surface for all of it.

That is about 2,000 production lines and 6,850 test lines across `internal/gatedrive`, `internal/app`, and `internal/cli`. The fix chain 0405 → 0416 → 0452 → 0453 → 0459 → 0467 lived there, and open change 0457 still targets it. It keeps the admission paths and the lock order (worktree admission → scope → drive) complex for the owners that remain. Every refusal it can produce is one more a reader has to reason through.

The trace found three facts the stub did not have.

1. **The run tracker still uses a recovery scope, but only a small part of the code.**
   - `run.start` prepares one outer scope per run (`RunStart` step 5); the CLI composes `Store.PrepareScope`.
   - `run.verdict` binds the attributed change to that scope (`runTrackerAdoptOwnership` → `BindScopeChange`). On a `run-incomplete` it takes over a drive the dead implement-next left behind and synthesizes a handoff, which becomes `run-continue` (`runTrackerOuterContinuation` → `TakeoverAndHandoff`).
   - `run.cancel` reads the scope's worktree (`storedScopeWorktree`).

   None of these touch the slot lifecycle, receipts, acknowledgement, scoped starts, slot rotation, or the `prepare-scope`/`takeover` commands.
2. **The outer scope never carries a run id.** `RunStart` prepares the scope (step 5) before it mints the run (step 6a), and its `ScopeRequest` sets only `ChangeID`, `Branch`, and `Worktree`. All 16 outer scopes on this machine have an empty `run_id`. So the run-id paths have only ever fired for task scopes:
   - `Takeover`'s "a parent takeover cannot revive a cancelled run" check, which fires only when `scope.RunID != ""`;
   - `resolveDriveRun`'s scope branch;
   - the cancel census's scope walk.

   A cancelled run's relaunch stays fenced through the drive's worktree-slot linkage (`authorizeRelaunch` → `resolveDriveRun` → `runLaunchGated`), which this change keeps.
3. **The outer takeover's candidate rule lost its premise.** `FindScopeDriveIDs` returns a run's drives that are live or "terminal-unconsumed", meaning terminal with the owner generation still set. Only `retirePredecessor` and `Acknowledge` ever clear a terminal drive's owner generation. Both are task-only, and this change deletes them.

   Until 0488 that was harmless, because build-owned drives carried no change id and never matched. 0488's review fix (`9acf686fa`) made every build-owned start pass `--change-id`. Now implement-next's own full-suite drives match, and nothing ever consumes them, so every finished gate stays a candidate for the rest of the run:
   - **Implement-next dies after a red-then-green gate.** There are two candidates, so the verdict is `run-stop <key> run-tracker-unavailable takeover-ambiguous`. That is terminal, and a human resumes by hand. It is the same failure as the 2026-09-08 incident on change 0410, where the candidates were task drives.
   - **It dies after one finished gate and a later commit** (the results checkpoint, a review fix). The takeover halts on the fingerprint mismatch, which is again a terminal stop.

Persisted state on this machine on 2026-10-02, under `<git-common-dir>/docket/`:

| Store | Contents |
|---|---|
| `gate-scopes/v2` | 16 outer scopes (all open, no run id); 76 task scopes (9 open, 8 of them with a run id) |
| `gate-drives/v2` | 182 task drives (1 `WAITING`); 30 scopeless drives |
| `gate-admission/v2` | 11 slots, all scopeless and released |

## Decision

### 1. Delete the task path end to end

Delete the following, with their tests.

- **Operations.** `gate.drive.acknowledge`, `gate.drive.prepare-scope`, and `gate.drive.takeover`, which means:
  - the cobra commands;
  - the `GateDriveService` methods and the `driveEngine` methods;
  - the operation constants and `GateScopeResult`;
  - the schema-registry bindings;
  - the `assetIndependent` entries in `internal/cli/install.go`.
- **Start flags.** `gate.drive.start` loses `--owner task`, `--scope-id`, `--child-cap`, `--predecessor-drive-id`, and `--predecessor-owner-gen`, along with the receipt-pair validation.
  - `--owner` accepts `build` or `finalize`.
  - No owner takes a `-- <argv>` any more, so any argv after `--` is a usage error.
  - The cataloged signature drops the `-- <argv...>` tail.
- **The task-intent owner.** `NewTaskGateDriveService`, `buildTaskGateDriveService`, `GateDriveService.taskIntent` and `argv`, and every branch that reads them.
- **Scoped starts** (`internal/gatedrive/driver.go`):
  - the scope and receipt fields of `StartRequest` and `GateDriveStartRequest`;
  - `precheckScopedStart`, `scopedIdentityMatch`, `scopedRunID`, `admitScoped`, `launchScoped`, `admitScopedWorktree`, `isSameScopeRaceLoss`, `siblingMayHoldReservation`, and `scopedAdmissionHook`;
  - the `AdmissionTicket` fields that exist only for them (`scoped`, `reservedFresh`, `ownsSlot`, `rotated`, `releasable`), so that `verifyAdmittedSlot`, `settleAdmittedAfterRunRefusal`, and `AbandonAdmission` collapse to the scopeless shape;
  - `AdvisoryRunID`, which collapses to the presented run id; `startBudgetedBuild` uses that id directly.

  In `admission.go`, delete `rotateWorktreeExecutionForSuccessor` and the slot's `ScopeID` field. Nothing writes the `scoped` kind any more.
- **The scope's per-test lifecycle** (`scope.go`, `acknowledge.go`, `store.go`):
  - `predecessorReceipt` and the reserved/launched slot states;
  - `reserveScopeDrive`, `scopeReserveRefusal`, `confirmScopeLaunch`, `clearPendingAck`, and `closeScope`;
  - all of `acknowledge.go` (`Driver.Acknowledge`, `closeScopeFinal`);
  - `retirePredecessor` and `predecessorReusableError`.
- **Task-only branches in shared code:**
  - `Driver.Claim`'s scope close, and `driveRecord.ScopeID`;
  - `resolveDriveRun`'s scope branch;
  - in `Takeover`: the run-revocation check (with `RunRevokedFunc`, `Driver.runRevoked`, `SetRunRevokedResolver`, the app's `runRevokedResolver`, and their wiring); the reserved/pending-ack slot check; the current-drive and stale-predecessor branches of `resolveTakeoverDrive`; and the empty-drive-id resolution, which only the deleted CLI used;
  - `claimScopeForTakeover`'s current-drive revalidation;
  - `Driver.PrepareScope` (`run.start` uses `Store.PrepareScope`);
  - the cancel and closeout census's scope-registry walk, and everything only it feeds: `censusRefs.ids`, `reservedBy`, `journalOpen`, `releasedScope`, `withdrawn`, `accountMissing`, the slot-named scope load, and the findings only those emit;
  - `GateDriveService.runLocate`, prepare-scope's run-id pre-check. `runIDLocator` stays, because `CheckRunIDExists` uses it for `agent.enter`.
- **Error kinds and messages:**
  - each `OwnershipErrorKind` that no remaining path can return (expected: `scope-second-live-drive`, `scope-transferred`, `scope-busy`, `stale-predecessor`, `predecessor-not-reusable`);
  - each `ownershipNextAction` case that no `gate.drive.*` command can reach any more.

  A surviving message that still sends the reader to a worker identity bundle or to parent recovery gets reworded for the scopeless caller.

The plan derives every site from a whole-repo grep for these symbols, flags, and operation ids, and sorts executable sites from prose (AGENTS.md). The lists above are the known sites, not the full set. A symbol is deleted only when the grep shows no remaining production caller.

### 2. Keep a minimal outer recovery scope, in-process only

The outer scope stays only to the extent that `run.start`, `run.verdict`, and `run.cancel` use it.

- **Records.**
  - `ScopeRequest` keeps `RepoIdentity`, `ChangeID`, `Branch`, and `Worktree`.
  - `scopeRecord` keeps `SchemaVersion`, `RepoIdentity`, `ChangeID`, `Branch`, `Worktree`, `ChildCapHash`, `ParentCapHash`, and `Closed`. It drops `TaskID`, `Phase`, `RunContextHash`, `RunID`, and the slot and journal fields.
- **Functions kept.** `Store.PrepareScope`, `LoadScope`, `bindScopeChange` with `Driver.BindScopeChange`, `claimScopeForTakeover` (the single-use open → closed close), `scopeCAS`, and `capHash`.
- **`Takeover`.** Its outer checks behave exactly as before:
  - the scope is open;
  - the exact parent capability is presented;
  - identity matches (repo, branch, worktree, change);
  - no handoff is outstanding;
  - the fingerprint is unchanged;
  - a live drive's deadline hasn't expired;
  - the scope closes once (single use).

  It takes the explicit drive id that its only caller has already resolved.
- **Unchanged.** The run-tracker record (`ScopeID`, `ParentCap`, `ChildContextHash`), the continuation seam, `run.continue`, and `change.claim`'s run-context binding.

Removing the scope's run id and the takeover's revocation check changes no remaining behavior, because no outer scope ever carried a run id (Problem, fact 2).

### 3. The outer takeover recovers only a still-running drive

`FindScopeDriveIDs` returns only the run's nonterminal drives, and the "terminal with owner set" clause goes. A finished build drive is never a takeover candidate.

| Implement-next dies … | Today (since 0488) | After |
|---|---|---|
| while the suite is still running | `run-continue` (takes over the live drive) | unchanged |
| during review, after a red-then-green gate | terminal `run-stop … takeover-ambiguous` | `run-retry-once` while attempts remain |
| after one finished gate and a later commit | terminal stop on the fingerprint mismatch | `run-retry-once` while attempts remain |
| right after one finished gate, nothing committed since | `run-continue`, reusing that verdict | `run-retry-once`; the suite runs again |

`Takeover` still accepts a drive that finished between the scan and the takeover. A suite that completes in that window is handed over, not lost.

**Accepted loss.** A gate verdict that finished before implement-next died is not reused. The retry re-runs the suite, which costs one `build.max_attempts` attempt and the run's retry. If the build budget is already spent, the retry halts for a human, the same outcome as today's terminal stop.

**Rejected alternatives:**

- **Keep the newest finished drive when the worktree is unchanged.** That needs an ordering rule plus a fingerprint pre-check, for a rare window.
- **Mark build drives consumed when evidence is recorded.** That adds a new write path into evidence.
- **Leave the rule for a follow-up.** Until the follow-up landed, an early-stopped run with a red gate or a re-gate would stop for a human.

### 4. Records already on disk stay where they are, unread

There is no schema bump, no store reset, and no migration.

- **Scopes.**
  - An outer scope's on-disk shape is unchanged, because the kept fields are a subset of today's. A run in flight across the upgrade keeps its continuation path.
  - The reader accepts schema 3 only. The schema-2 tolerance (`scopeSchemaVersionLegacy`) goes: change 0471's reset created the `gate-scopes/v2` root, and that root has never held a schema-2 record.
  - Task scopes are never opened again. `LoadScope` is called only with an id from a run-tracker record, and once the census walk is gone, nothing lists the scope directory.
- **Drives.** Old task drives still decode, because the stores use plain `json.Unmarshal` and ignore the dropped `scope_id`. They are handled like any scopeless record, and they can't match a new run's context. A nonterminal one is informational census history, never an obligation.
- **Slots.** Admission ignores the dropped `ScopeID`. Finished-incumbent reconciliation keeps accepting the legacy `scoped` kind, so an old scoped slot on another machine still settles exactly as it does today.

Old files stay as inert evidence. This applies ADR-0120/0125's assess-and-ignore posture; nothing new is decided.

### 5. Change 0457 is killed as superseded

0457 fixes a race inside `siblingMayHoldReservation` and in `admitScoped`'s reservation-failure leg. No workflow has reached that code since 0488, and Decision 1 deletes it. At this groom the human killed 0457 as "Superseded by 0489".

## Unchanged

- **The build and finalize gates:** scopeless start, advance, handoff, claim, fingerprinting, the single relaunch, the suite-attempt budget, worktree admission, finished-incumbent reconciliation, legacy-history assessment, and `gate.history.cleanup`.
- **The run tracker:** the key, the run-context binding, retry accounting, observe mode, `run.continue`, and `run.cancel` apart from the census's scope walk.
- **0488's work:** the worker contract and the build controller. ADR-0024's never-yield rule.
- **ADR-0107** stays Accepted. Decision 3 narrows it through `relates_to`, as ADR-0130 did.

## Prose and generated sites

| Site | Change |
|---|---|
| `docs/reference/glossary.md` | In *Gate drive / slice / owner generation / handoff / takeover*, drop "The catalog still carries the recovery-scope operations … Change 0489 decides their fate." Instead, say that the run tracker keeps one recovery scope per run internally and can take over a still-running drive through it, and that no operation exposes scopes. In *Worktree changed*, the takeover identity list loses "task or phase". |
| `internal/harness/dispatch.go` | The Codex root-entry clause becomes "labeled for `--run-id` on build-owned starts". This repo does not enable Codex, so no managed block regenerates here. |
| Comments in kept code | Comments that still describe worker sequences or task scopes, in `FindScopeDriveIDs`, `takeover.go`, `scope.go`, `store.go`, `drive.go`, `gate_drive.go`, and the run-tracker start, store, and run-id refusal files. Anchor them on symbols, never line numbers. |
| `internal/repoguard/gatedrive_task_absence_test.go` | Its header's "the Go catalog keeps these operations until change 0489 deletes them" moves to the past tense. |
| `tests/test_go_integration_gatedrive_race.sh` | Its header's "concurrent scopes driven to terminal" description. |
| Skills | No change expected. A whole-repo grep for the retired vocabulary decides. Regenerate the embedded mirror (`go generate ./internal/assets`) only if a mirrored file changes. |

Plans, results, archived changes, and Accepted ADRs are frozen records and are never edited.

## Tests

Decide each test by what it guards, not what it asserts (learnings: test-premise-deleted-not-regated). Never restore deleted code or prose to keep a test green.

- **Delete** tests whose subject is gone, about 6,850 lines:
  - all of `acknowledge_test.go`;
  - the scoped-start, successor, receipt, slot-rotation, sibling-race, task-owner, prepare-scope, acknowledge, and takeover-CLI tests in `driver_test.go`, `driver_concurrency_test.go`, `driver_faults_test.go`, `admission_successor_test.go`, `scope_test.go`, `slot_run_fence_test.go`, `takeover_test.go`, `handoff_test.go`, `driver_transfer_test.go`, `run_launch_gate_test.go`, `sequence_integration_test.go`, `sequence_race_integration_test.go`, `internal/app/gate_drive_test.go`, and `internal/cli/gate_test.go`;
  - `reconcile_test.go`'s census scope-walk tests;
  - `takeover_run_test.go`'s cancelled-run takeover test;
  - the takeover fail-closed table's task-mismatch and phase-mismatch rows, because an outer scope pins neither.
- **Re-point** kept tests whose property survives but whose fixture builds a worker scope, about 2,650 lines. Move them onto build-owned scopeless drives carrying `--run-context`, or onto an outer scope (`prepareOuterScope`/`startNested`):
  - the takeover tests: `TestTakeoverInvalidatesChildAndMintsParentOwner`, `TestTakeoverFailClosedTable`, `TestTakeoverRace`, and the takeover integration and race-integration tests;
  - the relaunch run-fence tests in `driver_runfence_test.go`, which link to the run through the worktree slot's run id instead of a scope;
  - the cancel and closeout census tests that use `seedScopedRunDrive`;
  - the app and CLI tests that borrow the task owner or a task scope only as a fixture: the run-start e2e test, the build-start history and production-census integration tests, and `TestGateDriveStartUnknownRunIDIsNamed`.
- **New tests:**
  - **R1.** A run whose drives are a FAILED and a PASSED build drive, both carrying its change id and run context, has no takeover candidate, so `run.verdict` takes the retry path instead of stopping `takeover-ambiguous`. A live drive next to finished ones resolves to that one drive and gets `run-continue`. Restoring the terminal clause must turn the first test red.
  - **Old records.**
    - An open task-scope file with slot fields and a run id on disk changes nothing for cancel, the successful-run closeout, or admission.
    - An outer scope written in today's on-disk shape still loads, binds, and takes over.
    - An old drive record carrying `scope_id` decodes and settles through the scopeless path.

    Each of these must be shown to turn red when its fixture's effect is removed.
  - **Coverage the deletion would otherwise lose.**
    - A build-owned `gate.drive.start --run-context` stores the run-context hash on the drive. Today only task-tied tests prove that wiring.
    - The scope store's concurrent-write discipline is tested through kept transitions (a takeover close racing a change bind), replacing `TestScopeCASConcurrent`.
- **Pins.** These pins are what stop the operations from silently coming back, so no new guard is added.
  - Drop the three operations from `TestRepresentativeSignatures`, and update `gate.drive.start`'s pinned signature.
  - Update the schema-registry correspondence (`TestSchemaCatalogCorrespondence`, `TestEveryRequestAndResultStructIsBound`).
  - Drop the three `assetIndependent` entries (`TestAssetIndependentSetExact`).
  - Drop the prepare-scope rows of `TestRunTrackerVocabularyHardCut`.
  - Update `TestOwnershipKindSpellings`, `TestMapDriveFailureOwnershipKinds`, and `TestMapDriveFailureOwnershipNextAction` to the kinds that remain.
- **Guards.**
  - 0488's absence guard (`TestNoTaskOwnedDriveInstructions`) stays as it is, and its companion still finds the build-owned start.
  - The launch-site guard derives its population from the AST and needs no change.
- **Budgets.** After the deletions, re-measure the `tests/runtime-budgets.tsv` rows that run the shrinking corpus, and lower the ones that shrink, per `tests/README.md`:
  - `tests/test_go_toolchain.sh`;
  - `tests/test_go_race.sh`;
  - `tests/test_go_integration_gatedrive_process.sh`;
  - `tests/test_go_integration_gatedrive_race.sh`.

  Read every `BUDGET WATCH` and `SERIAL CONFIRMED OVER BUDGET` line, whether or not the run is green.
- **Build gate.** Run the whole suite through `build.test_command`.

## Decision record

- **New ADR, recorded through docket-adr:** *The run tracker's outer takeover recovers only a still-running drive.* `relates_to: [107, 130]`.
  - Context: Problem facts 1 and 3.
  - Decision: Decisions 2 and 3, including that recovery scopes now exist only at the coordinator → implement-next boundary, with no per-test lifecycle.
  - Consequences: the accepted loss.
  - Alternatives: Decision 3's rejected list.
- **No ADR for Decision 4.** It applies ADR-0120/0125 and changes no schema.
- **Results file.** No Important human action beyond the standard post-merge reinstall. One Optional check: the next time an implement-next run dies after its gate finished, `run.verdict` should answer `run-retry-once`, not `run-stop … takeover-ambiguous`.

## Effect on follow-ups

- **0490 (supervisor-held worktree lock):** unaffected. Its worktree slot no longer has a scoped kind to model.
- **0491 (retire the run id):** its stub lists "the revocation checks at launch, recovery, and takeover". This change deletes the takeover check, and 0491's spec should say so.

## Out of scope

- Replacing the worktree admission slot (0490) and retiring run-id fencing (0491). The one exception is the takeover revocation check, which is unreachable today.
- Build-owned drive mechanics: start/advance, owner generation, handoff/claim, fingerprinting, the idempotent relaunch, and the suite-attempt budget.
- The run tracker's attribution and retry model, `run.continue`, and the run-context claim binding.
- The `--task-id` and `--phase` recording flags on `gate.drive.start`. Finalize's in-process gate and run-waiting receipts still use phase.
- Deleting old scope or drive files from disk.
- The controller-side background-and-yield cases (0412).

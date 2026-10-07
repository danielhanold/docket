<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0490 — Replace the durable worktree admission slot with a supervisor-held kernel lock](../../changes/archive/2026-10-02-0490-replace-the-durable-worktree-admission-slot-with-a-superviso.md)**
<!-- docket:backlink:end -->

# Replace the durable worktree admission slot with a supervisor-held kernel lock — design

## Problem

Change 0375 (ADR-0118) enforces "one live gate per worktree" with a durable JSON state machine per canonical worktree: `<git-common-dir>/docket/gate-admission/v2/<sha256(root)>/record.json`, states `reserved`, `executing`, `stopping`, `unresolved`, and `released`. The file header of `internal/gatedrive/admission.go` states the design directly: "the flock is a critical-section primitive here, never the lifetime guarantee — the persisted state … is the authority on whether the worktree is busy."

The state machine fails closed, so every ambiguous outcome leaves the worktree blocked until something recovers the slot. The fix chain that followed: 0428 (a repo-wide legacy census vetoed admission; 0446's spec counted 623 PASSED/FAILED records in it), 0435 (cancel left a stale run stamp that blocked finalize and resume), 0437, 0439 (finalize hit a completed raw run still holding its slot — by design, since a raw `gate.launch` held the slot until `gate.stop`; the defect was a swallowed, locator-less refusal), 0441, 0446 (one orphaned HALTED drive vetoed every worktree's first admission), 0452, and 0453. 0457 was killed as superseded by 0489.

The trace found these facts.

1. **The lock-handoff pattern already runs in production.** `process.Service.Launch` takes `<runDir>/live.lock` with a non-blocking `LOCK_EX` before spawning, passes the locked file to the re-executed supervisor through `ExtraFiles` (fd 3), and closes its own copy after `cmd.Start`. `RunSupervisorFromEnv` marks fd 3 close-on-exec and closes it last, after `terminal.json` (or `failure.json`). ADR-0095 rests on that order: a released `live.lock` implies a durable terminal record.
2. **On the normal path the slot and a lock release together.** PASSED/FAILED releases the slot when the supervisor's terminal record is observed (`releaseAdmissionIfProven`), which is the moment the supervisor exits. They differ only on the ambiguous paths, and there the slot wedges:
   - a crash between reserve and drive creation, or between `Admit` and `StartAdmitted`, leaves the slot `reserved` with no recovery (`incumbent-drive-unresolved`, `incumbent-nonterminal`, `launch-pending` forever);
   - a launch error not proven never-launched, or an attach/confirm failure without a proven stop, marks it `unresolved`, and `run.cancel`'s `reconcileWorktreeSlot` has no branch that ever resolves `unresolved` (`slot-state-unknown`);
   - a raw launch that crashes between reserve and confirm records no run dir, so `gate.stop` cannot match it;
   - cancel treats a signaled or vanished run as unproven (`rawTeardownProven`), so it stays `cancellation-pending`.
3. **The slot never proved process-tree teardown either.** The gate tree is supervisor → `go run` → test runner → one target per process group (`suiterunner` starts each with `Setpgid`). The supervisor waits for its direct child only. Slot release keys on a terminal record, or on "vanished" (`releaseAdmissionIfProven`, `proveNoTreeSurvives`, `stopProvesTeardown`), which proves only that the supervisor exited. So a supervisor killed alone, or a KILL escalation that kills the runner before it stops its targets, can leave test processes running today, with the slot released.
4. **The slot carries the run linkage.** A drive start stamps the run id on the slot; the drive record carries no run id. These read the stamp:
   - `run.cancel` and the agent death guardian find a run's suites only through the slot (`reconcileWorktreeSlot`, and the launch census via `censusReferences` and `resolveDriveRun`, which attributes at most the one drive whose `AdmissionToken` the slot still holds);
   - `run.verdict`'s success closeout reads and retires it (`accountCompletionSlot`, `durableExecutionProof`, `retireWorktreeSlotOwnership`);
   - `run.start --resume` retires it in its quiescence check (`validateResumeQuiescence`);
   - between a run's gates the released slot keeps the run id, so any other start is refused `stale-run-id` (the slot run fence, `rawStaleRunRefusal`, `settleStaleReleasedRun`, `RetireWorktreeExecutionRun`);
   - the mutation fence uses it as a positive owner reference when no readable run record owns the worktree (`slotNamedRunUnresolved`);
   - the relaunch and crash-window relaunch recovery check the run's liveness through it (`authorizeRelaunch`, `recoveryRunRevoked`).
5. **A slot-independent run link already exists.** A drive started with `--run-context` stores `run_context_hash`, equal to the run-tracker record's `child_context_hash`; `run.verdict`'s takeover scan finds a run's drives with it (`FindScopeDriveIDs`). The build controller passes `--run-context` on every tracked `gate.drive.start` (docket-build SKILL, "drive it through the native gate driver").
6. **The relaunch run checks are unreachable in production.** The single automatic relaunch requires `IdempotentSuiteGate`. Only `finalize_rebase.go` sets it, finalize's gate carries no run, and no skill passes `--idempotent-suite-gate`. So no production drive both carries a run and can relaunch.
7. **Drive starts key on the caller's spelling.** `gate drive start` keys the slot on `--repo-dir` or `os.Getwd()` through `EvalSymlinks` only. On this machine's case-insensitive volume `/users/homer/dev/DOCKET` survives `EvalSymlinks` unchanged and yields a different key from `/Users/homer/dev/docket`. Raw launches already key on `gitcli.DiscoverWorktree` (git's `--show-toplevel` plus `EvalSymlinks`), which returns the on-disk spelling.
8. **flock semantics hold on both platforms for this use.** Verified on this Mac (APFS) and per Linux `flock(2)`:
   - a lock belongs to the open file description; `fork` and descriptor inheritance across `exec` share it unless close-on-exec is set;
   - it is released when the last descriptor referencing the description closes, or when any holder of a shared description calls `LOCK_UN`;
   - a holder that has exited but is not yet reaped no longer holds it;
   - every linked worktree (primary, `.docket`, `.worktrees/<slug>`, an out-of-tree worktree) resolves the same `<git-common-dir>` and sees a lock held from another;
   - NFS and CIFS emulate flock differently; docket gates run on local disks.

   The real-process tests run in CI only on `macos-15`; Linux equivalence rests on the documented semantics, which are identical for the subset this design uses.

The stub's other premises are corrected here: "989 legacy records" has no source; "0457: still open" is wrong; the rejection of "a worktree flock alone" ("short-lived CLI calls release it while detached work survives") is in 0375's spec, while ADR-0118 rejected "release the worktree slot on process death or a liveness heartbeat" as fail-open.

## Decision

### 1. A per-worktree lock, held by the gate supervisor for its whole life

**Location and key.** `<git-common-dir>/docket/worktree-locks/<key>/busy.lock`, beside the holder note `holder.json` (Decision 2). The directory is created owner-only (0700) and the files are 0600, like the other stores.

- `<key>` is the lowercase hex sha256 of the canonical worktree root, where the root is `gitcli.DiscoverWorktree(<launch working directory>).Root` — git's `--show-toplevel` with every symlink hop resolved. Never key on a caller-supplied spelling (`--repo-dir`, `os.Getwd()`), a subdirectory, or a run root. This fixes Problem fact 7 and the `/tmp` vs `/private/tmp` alias in one place. Nested worktrees get their own keys.
- Lock files and their directories are never deleted. Unlinking a lock file races a concurrent opener into holding a lock on an orphaned inode.
- It is a new root; nothing is migrated (Decision 5).

**Acquisition.** Every production launch takes the lock first with a non-blocking exclusive `flock`, at all three `Launch` call sites:

- **Drive start** (`gate.drive.start`, finalize's local gate via `composeLocalGate`, `evidence.recertify`): `Driver.Admit` takes the lock before it creates the drive record. In a build start the suite-attempt charge (`ReserveSuiteAttempt`) still follows admission, so a busy worktree creates no drive and charges nothing. The held lock travels in-process with the admission ticket to `StartAdmitted`; every path that abandons the admission before launching (`AbandonAdmission`, a refused revalidation, a failed drive write) closes it.
- **Raw `gate.launch`:** `GateLaunch` takes the lock when `--cwd` resolves to a registered git worktree (`resolveWorktreeAdmission`); outside git there is no lock, as there is no slot today.
- **Relaunch** (the single automatic relaunch of an idempotent drive, in `driveSlice`, and a never-launched replacement completed by `recoverReservedRelaunch`): the relaunching process takes the lock again before launching the replacement. If another gate holds it, the drive HALTs with cause `worktree-busy` instead of relaunching. Finalize's gate-outcome mapping carries that cause like any other HALT cause: a halt, never repair work.

A held lock refuses the start with the reason `worktree-busy` (stage `worktree-admission`). Any other open or lock error refuses the start as an I/O failure and is never read as "free". A refusal is never queued and never stops the holder. It leaves nothing behind, so a retry after the holder exits succeeds with no recovery step.

**Handoff and release.** The lock follows the `live.lock` pattern (Problem fact 1).

- `process.LaunchRequest` carries the already-locked file. `Service.Launch` takes ownership of it: it passes it to the supervisor as one more `ExtraFiles` descriptor and closes the caller's copy before returning on every path. A launch that fails before spawn therefore frees the worktree, and one that spawned leaves the supervisor as the only holder.
- The supervisor adopts the descriptor only when the launcher says it passed one (a private variable stripped from the child's environment, like the two that exist today), marks it close-on-exec, and closes it last: after the terminal or failure record and after `live.lock`. So an observer that finds the worktree free also finds that gate's `live.lock` free and its terminal record durable, or the supervisor dead.
- Release is by closing only. No code calls `LOCK_UN` on it. In particular, the workspace and transaction `release()` helpers (which unlock before closing) are not reused for it.
- The supervisor is the only holder: the supervised command never inherits the lock. The kernel releases it when the supervisor exits or dies, and when a launching CLI dies before the handoff.
- `internal/process` stays standard-library-only (`TestImportBoundaryStdlibOnly`). It owns the flock primitives (an exported non-blocking acquire with a typed busy result, built on `acquireFlock`); `internal/gatedrive` and `internal/app` own the path and the key.

**Lock order.** The worktree lock is only ever tried, never waited on, so it adds no deadlock edge. Where a start already holds the run registry's `run.lock` (`runLaunchGated` around `Admit`), it takes the worktree lock inside it; it takes the drive store's own CAS lock after.

### 2. The holder note is a diagnostic, validated before it is printed

After a successful launch handshake, the process that took the lock writes `holder.json` atomically (temporary file plus rename, beside — never over — `busy.lock`): `kind` (`drive` or `raw`), `drive_id`, `run_dir`, `change_id`, `owner` (`build`, `finalize`, or `raw`), and `written_at`. A relaunch rewrites it with the replacement's run dir. A failed note write is ignored and never fails the launch.

On a `worktree-busy` refusal, the refusal reads the note and calls `process.Service.Observe` on its `run_dir`. Only a `running` answer prints it, as today's locators: `incumbent-drive:<id>` with the change id, or `incumbent-run:<dir>`. Any other answer, a missing note, or an unreadable note prints "holder unknown" — a stale note from an earlier holder is never shown. The remedy names the right tool for the holder's kind: stop the owning run with `run.cancel` for a build drive, wait for finalize's gate, or `gate.stop <run-dir>` for a raw launch; with an unknown holder, wait, or find the holding process with `lsof` on `busy.lock`.

Admission, cancel, and every other decision ignore the note. It replaces the slot's `IncumbentSnapshot` source; the Reason/Stage/Locator plumbing 0439 added (`GateReport`, `mapDriveOutcome`) carries it unchanged.

### 3. Delete the slot and everything that runs or recovers it

- **The slot store:** `admissionRecord`, its states, reservation tokens, and `ReserveWorktreeExecution`, `ReserveRawWorktreeExecution`, `ReserveWorktreeExecutionForRun`, `ConfirmWorktreeExecution`, `ReleaseWorktreeExecution`, `MarkWorktreeExecutionUnresolved`, `MarkWorktreeExecutionStopping`, `LoadWorktreeExecution`, `WorktreeAdmissionRefusal`, and the admission root in `OpenStore`.
- **`launch-unconfirmed` as an admission refusal** (`ErrLaunchUnconfirmed`), with `ErrUnresolvedLaunchTransition` and gatedrive's slot-side `ErrStaleRunID`. `ErrWorktreeBusy` stays, produced by the lock. The drive-level HALT cause spelled `launch-unconfirmed` (a reserved relaunch whose launch cannot be established, `haltReservedRelaunch`) stays; it describes a drive, not admission.
- **Every slot transition in the driver and the raw launch:** `verifyAdmittedSlot`, `releaseOrUnresolveWorktree`, `resolveWorktreeAfterLaunchFailure`'s slot legs, `settleAdmittedAfterRunRefusal`'s release, `releaseAdmissionIfProven`, and `DriveDoc.ReleaseFinding`; in `internal/app/gate.go`, the slot legs of `resolveRawLaunchFailure` and `rawStopIfOwned`, and `releaseRawSlotForStop` (`gate.stop` now only stops). A terminal drive always exposes its run root, so finalize always removes its temporary run root.
- **Finished-incumbent reconciliation** (`internal/gatedrive/incumbent.go`), and the advisory busy pre-check in `startBudgetedBuild` (`WorktreeAdmissionRefusal` plus `ReconcileFinishedIncumbent`), which existed to refuse before the attempt charge — the lock is now taken before the charge.
- **The first-admission legacy inventory** (`inventoryLegacyDrives`, `classifyLegacyDrive`, `LegacyHistorySummary`, `LegacyFinding`, `startDocWithLegacy`, and the `legacy_history` field on `gate.drive.start` and `gate.launch` results and refusals).
- **`gate.history.cleanup`:** the CLI command, its capability entry, `GateHistoryCleanup` (`internal/app/gate_history.go`), and `Driver.CleanupHistory` with `cleanupHistory` and `assessLegacyRecord`. Its only effect was writing abandoned markers so a blocked first admission could proceed. `loadHistoricalDrive` stays: the launch census uses it to skip historical drives with a terminal outcome.
- **The slot's run stamp and its readers that only fenced:** the slot run fence in reservation, `settleStaleReleasedRun` with the app's `runSettledResolver`/`RunSettledFunc` seam, `RetireWorktreeExecutionRun`, `rawStaleRunRefusal`, `classifySlotOwnership`, and `resolveDriveRun` with the two checks it feeds: `authorizeRelaunch`'s run-liveness check and `recoveryRunRevoked`. Problem fact 6: no production drive reaches them. `resolveDriveRun`'s third consumer, the launch census, moves to context-hash attribution (Decision 4).

A drive still mints a launch token and passes it to `Launch` as the manifest's reservation token, so its owner can still resolve a lost launch response (`ResolveReservation`). The driver mints it itself now instead of receiving the slot's token. The field keeps its JSON key `admission_token`; its Go name and comment may change.

### 4. The run tracker without the slot

**Cancel and the death guardian** (`reconcileRunTeardown`, shared by `run.cancel`, `guardianFenceAndReap`, and the `agent.enter` lifecycle cancel):

- Delete the slot steps: `markWorktreeSlotStopping`, `reconcileWorktreeSlot`, `retireWorktreeSlotOwnership`/`retireSlotOwnership`, and in the terminal path `resolveTerminalRunSlot` and `storedScopeWorktree`. The dormant registered-participant stop path stays, minus its slot call.
- The launch census (`ReconcileRunLaunches`, `ObserveRunLaunches`, `accountRunLaunches`) attributes drives by `run_context_hash` equal to the run-tracker record's `child_context_hash`, instead of through the slot. It accounts every nonterminal drive so attributed — today it can attribute at most one. `censusReferences` goes. A drive without a run context is never attributed, as raw launches are not today. An unreadable drive record is informational (`history-unattributed`), because nothing positively names it any more.
- For each attributed drive, `reconcileRunDrive` keeps its claim check (`claim-busy` while `relaunch.lock` is held) and its reserved-relaunch handling. A first launch whose run dir was never attached now resolves the same way a reserved relaunch does: `ResolveReservation(RunRoot, launch token)`; an identified run is stopped, a never-launched one is settled by writing the drive HALTED (`run-cancelled`, as `settleNeverLaunchedCancelled` does for relaunches), and an unresolvable one keeps cancel pending. This removes the `launch-pending` finding.
- **Teardown proof is the lock model's proof: the supervisor is gone.** For each of a drive's run dirs (`RawRunDir`, `PriorRawRunDir`), `Observe` decides: `running` is stopped with `process.Service.Stop`, then must observe as not running; every other state (passed, failed, signaled, stopped, vanished) counts as torn down; an unprovable probe keeps cancel pending.
- `cancellation-pending` is left with these causes: a supervisor still running after the stop, a probe or resolution error, `claim-busy`, a pending mutation in the journal, and a failed final write. The slot findings and `stop-unproven` on a vanished or signaled run are gone.
- A repeat cancel of an already-terminal run (`repairTerminalRun`) reruns the census for that run's own context hash and the journal check; it no longer looks for a slot. The disposition vocabulary (`cancelled`, `cancellation-pending`, `already-cancelled`, `refused`) is unchanged.

**Resume** (`run.start --resume`): one-live-run-per-worktree already rests on run records and is unchanged. `validateResumeQuiescence` keeps its stop-capable census for a cancelled or superseded predecessor, now attributed by that predecessor's context hash, and drops the slot retire.

**Success closeout** (`completeSuccessfulRun` on a keyed `run-complete`): keeps `FenceRunCompleting`, publication settlement, participant status, the journal, the observe-only census (now by context hash), and `CompleteRun`. Drops `accountCompletionSlot`, the slot branch of `durableExecutionProof`, and the slot retire.

**The mutation fence** (`admitWorkflowMutation`): drops `slotNamedRunUnresolved`. It finds the worktree's owner from run records only.

**Kept for 0491:** `runLaunchGate` at `Admit` and `StartAdmitted` (its `stale-run-id` and cancelled refusals), every `--run-id` flag, the run id in `run.start` and `run.cancel`, the `completing`/`completed` lifecycle, and the run-tracker prose.

### 5. Records already on disk, and the upgrade

There is no migration and no schema bump for kept stores.

- `gate-admission/v2/` stays on disk, inert; nothing opens it. Deleting it is optional and out of scope.
- Drive records keep their shape: the launch token keeps its key, and the dropped `legacy_history`/`release_finding` fields live only in results. Old records decode unchanged.
- A run left `cancelling` by a wedged slot finding is re-run by `run.cancel` after the upgrade and settles through the new census.
- **Upgrade contract** (carried from ADR-0118): install the new binary only when no gate is running. A supervisor started by the old binary holds no worktree lock, so the new binary would not see it.

## Accepted losses

- **Between-gate run ownership.** Between two gates of a live run, another start (finalize, a raw `gate.launch`, an untracked drive) is no longer refused `stale-run-id`. If it starts, the run's next gate is refused `worktree-busy`. 0491 already accepts the related loss for a cancelled run's leftover agents; until 0491, `runLaunchGate` still refuses a start that presents a cancelled run's id.
- **The mutation fence's fallback for a corrupted run record.** A metadata write in a worktree whose owning run record is unreadable is no longer refused through the slot.
- **The record of who last held the worktree.** The holder note names a live holder only.
- **Unreadable drive records don't block cancel.** A drive whose record cannot be read is not attributed or stopped by cancel. The worktree lock still keeps a second suite out while it runs.
- **Process-tree teardown gaps, unchanged from today's actual guarantee** (Problem fact 3): a supervisor that dies alone leaves its suite running with the worktree free; a KILL escalation leaves test targets running in their own process groups; the relaunch trusts "vanished" (`proveNoTreeSurvives`); and on a graceful stop `go run` exits on TERM, so the worktree frees while the runner spends up to about 5s stopping targets. The new ADR records them; change 0492 tracks them.

## Unchanged

- The drive protocol: start/advance, owner generation, handoff, claim, takeover, fingerprinting, the stored terminal result, the single idempotent relaunch (apart from re-taking the lock), and the suite-attempt budget.
- The supervisor's `live.lock` contract (ADR-0095), its identity, observe, stop, and recover logic, and `gate.observe`, `gate.recover`, and `gate.cleanup`.
- The run tracker's key, run-context binding, retry accounting, observe mode, `run.continue`, resume admission, and `run.cancel`'s authority checks, fence, and journal.
- The workflow-mutation fence apart from its slot fallback.

## Prose and generated sites

| Site | Change |
|---|---|
| `skills/docket-build/references/gate-caller-loop.md`, *Worktree admission — one live gate per worktree, and what `worktree-busy` means* | The worktree's lock replaces the execution slot. Drop the `launch-unconfirmed` bullet. `worktree-busy` means another gate's supervisor holds this worktree's lock; the refusal names the holder when it can; it frees itself when that gate ends; the remedy is to wait or stop the holder (`run.cancel` for a tracked run, `gate.stop <run-dir>` for a raw launch). Keep: a blocking diagnostic, never a retry trigger or `FAILED`, no attempt charged, never a poll loop on start. |
| `skills/docket-finalize-change/references/gate-failure.md`, *The finalize gate shares the worktree's one execution slot* | Retitle around the lock (and update the section terminator pinned in `internal/repoguard/prose_contracts_test.go`). Drop `launch-unconfirmed` and "must be recovered or cancelled, never cleared by a blind re-start". Same meaning and remedy as above. |
| `docs/reference/glossary.md` | Replace *Worktree slot* with a *Worktree lock* entry. *`worktree-busy` / `launch-unconfirmed`* becomes *`worktree-busy`* (note that `launch-unconfirmed` survives only as a drive HALT cause). *Process recovery / gate history cleanup* loses the history-cleanup half and its examples. Update the cross-reference that says a start "can be refused with `worktree-busy`" and the index entries. |
| Embedded mirror | Regenerate `internal/assets/embedded/…` with `go generate ./internal/assets` for every mirrored file that changes. |
| `internal/repoguard/budgets_test.go` | Update the comment that names the slot sections, and any word-count rows the rewrites move. |
| `tests/test_go_integration_app_buildstart.sh` | Its header's description of the worktree-busy admission refusal. |
| Comments in kept code | Comments that describe the slot, its token, or slot ownership — in `drive.go` (`AdmissionToken`), `reconcile.go` (the census rules), `store.go`, `ownership.go`, `gate_drive.go`, `gate.go`, and the run-tracker cancel, complete, fence, and start files. Anchor on symbols, never line numbers. |

A whole-repo grep for `execution slot`, `worktree slot`, `launch-unconfirmed`, `gate history cleanup`, `gate.history.cleanup`, `gate-admission`, and the deleted symbol names decides the full list; sort hits into prose and executable. Plans, results, archived changes, and Accepted ADRs are frozen records and are never edited.

## Tests

Decide each test by what it guards, not what it asserts (learnings: test-premise-deleted-not-regated). Never restore deleted code or prose to keep a test green.

- **Re-target, don't delete,** the tests that pin the invariant itself: `TestTwoScopelessStartsOneWorktreeOneLaunch` and `TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner` (`driver_concurrency_test.go`), and `TestGateLaunchInsideWorktreeSecondRefused` (`internal/cli/gate_test.go`). Each must still produce exactly one launch.
- **New tests:**
  - **L1.** A supervisor killed with SIGKILL frees the worktree: the next start is admitted with no recovery step.
  - **L2.** A launching process that dies after taking the lock and before spawning frees the worktree.
  - **L3.** Path aliases (`/tmp` vs `/private/tmp`, a symlinked root) and a case variant on a case-insensitive volume (skipped where the volume is case-sensitive) map to one lock; a subdirectory `--repo-dir` maps to its worktree's lock.
  - **L4.** A relaunch that finds the worktree held by another gate HALTs `worktree-busy` and launches nothing.
  - **L5.** A busy build start creates no drive and charges no suite attempt.
  - **L6.** The supervised command never inherits the lock descriptor. Mutation-test it: dropping the close-on-exec must turn it red.
  - **L7.** The supervisor releases the worktree lock after `live.lock` and the terminal record: an observer that finds the worktree free finds the terminal record.
  - **L8.** A holder note is printed only while its run is running; a stale or missing note prints "holder unknown".
  - **L9.** Cancel with no slot anywhere: it finds every nonterminal drive carrying the run's context hash, stops a running one, reaches `cancelled` when a drive's supervisor already vanished, settles a never-attached first launch as never-launched, and stays pending on `claim-busy`.
  - **L10.** A drive of another run (a different context hash) in the same worktree is never stopped by cancel.
- **Delete** tests whose subject is gone: the slot store and its transitions, finished-incumbent reconciliation, the slot's run stamp (fence, settlement, retirement), the relaunch and recovery run checks, the legacy inventory and its result summary, and history cleanup. Whole files: `admission_test.go`, `admission_retire_test.go`, `incumbent_test.go`, `slot_run_fence_test.go`, `gate_history_test.go`, `gate_drive_history_integration_test.go`, and the history integration and race tests. Cases inside mixed files: `storage_reset_test.go`'s slot cases, `driver_test.go`'s slot and legacy-summary cases (`TestStartCarriesLegacyHistorySummary`), `driver_faults_test.go`, `driver_runfence_test.go`'s relaunch and recovery cases, `history_test.go`, `old_records_test.go`'s slot cases, `sequence_race_integration_test.go`, and the cli `TestGateHistoryCleanup*` tests.
- **Re-point, don't delete,** cases whose property survives: the start-time run check (`runLaunchGate`, kept for 0491) in `run_launch_gate_test.go` and `driver_runfence_test.go`; abandoning an admission, and a failed launch, freeing the worktree (now the lock, not a release); taking admission before launch; an old drive record still decoding.
- **Rewrite** the shared fixtures that build slots — `newCancelFixture` (`runtracker_cancel_helpers_test.go`) and `newVerdictCompletionFixture` (`runtracker_verdict_helpers_test.go`) — onto drives carrying a run context, and re-point the cancel, complete, verdict, fence, resume, and production-census integration tests that use them. `reconcile_test.go` moves from slot-token attribution to context-hash attribution.
- **Pins:** `TestGateLaunchAdmissionCoverage` (`internal/repoguard/gatelaunch_admission_test.go`) is rewritten to require that every production `Launch` call site hands over a held worktree lock; it keys on syntactic shape, derived from the AST, not on a list of spellings. Update `TestRepresentativeSignatures`, the schema-registry correspondence tests, the asset-independence set, and `TestOwnershipKindSpellings` and the `mapDriveFailure` kind tests for the removed capability, result fields, and kinds. The gatedrive `launch_sites_guard_test.go` population stays three.
- **Budgets.** After the deletions, re-measure the `tests/runtime-budgets.tsv` rows that run the shrinking corpus and lower the ones that shrink, per `tests/README.md`. Read every `BUDGET WATCH` and `SERIAL CONFIRMED OVER BUDGET` line, green run or not.
- **Build gate.** Run the whole suite through `build.test_command`.

## Decision record

- **New ADR, recorded through docket-adr:** *Worktree admission is a supervisor-held kernel lock.* `supersedes: [118]`, `relates_to: [95, 120, 124, 125]`.
  - **Context:** Problem facts 1–8.
  - **Decision:** Decisions 1–5. Restate what ADR-0118 established and this keeps: one live gate per canonical worktree; every launch site acquires before launch; a busy start charges no suite attempt, is never queued, and never stops the holder; distinct worktrees are independent. ADR-0118's cancellation and resume rules stand until 0491.
  - **Consequences:** the accepted losses, including the process-tree gaps and change 0492, which tracks them.
  - **Alternatives:**
    - Keep the slot and fix the remaining wedges one by one — every ambiguous path needs its own recovery, which is the fix chain.
    - Let the whole tree inherit the lock — a leaked or daemonized descendant pins the worktree, and `gate.stop`/`run.cancel` cannot free it.
    - Fold 0491 in — a much larger change, including the always-loaded run-tracker prose.
    - Copy the run id onto drive records — new code 0491 would delete.
    - No holder note, or a drive-record scan — loses the 0439 locator, or cannot see raw launches.
  - **Answers to the earlier rejections:** 0375's spec rejected "a worktree flock alone" because short-lived CLI calls released it; here the supervisor holds it for the run's whole life. ADR-0118 rejected release on process death as fail-open; here the kernel proves the holder is gone, and what remains open is the process-tree gap the slot did not close either.
- **ADR-0120, ADR-0125, and ADR-0124 get dated Update notes** naming the rules the new ADR replaces: ADR-0120's use in first admission and history cleanup (its assessment-only historical reader stays for the census); ADR-0125's finished-incumbent settlement, removed-path slot addressing, and the slot legs of its relevance and owner rules (relevance and "a probe error is never absence" stay); ADR-0124's closeout slot retirement.
- **Results file.** Important: reinstall after the merge only while no gate is running, on every machine (the upgrade contract). Optional: once the new binary is installed, `.git/docket/gate-admission/` can be deleted by hand; nothing reads it. Optional check: a `run.cancel` that was stuck `cancellation-pending` on a slot finding reaches `cancelled` when re-run.

## Effect on follow-ups

- **0491 (retire the run id).** Its stub lists several items this change does:
  - stamping the run id on slots, and `settleStaleReleasedRun`;
  - the revocation checks at relaunch and crash-window recovery (the takeover check was already deleted by 0489);
  - the `run.verdict` closeout steps that exist only to release slots;
  - most of "simpler cancel": after this change cancel is the fence, a stop of the run's drives found by run context, the journal, and the final write.

  Still 0491's: `runLaunchGate` at admission and launch with its `stale-run-id`/cancelled refusals and `fenceNextAction`'s message, the `--run-id` flags (`gate.drive.start`, `agent.enter`, `run.cancel`), the run id in `run.start`'s output, the run-tracker prose, ADR-0124/0128, and the 0422 re-check. Its out-of-scope note on the mutation fence ("unless grooming shows it depends on the slot") is settled: it did, through `slotNamedRunUnresolved`, which this change deletes. An observation for 0491: `run.start` is cataloged `local-write` only, but its resume quiescence check can stop process groups through the census.
- **0492 (suite teardown can outlive its supervisor).** Captured at this groom as a proposed change with `discovered_from: [490]` and `depends_on: [490]`. It covers the four process-tree gaps under *Accepted losses*.

## Out of scope

- The run id and its start-time check (0491).
- The drive protocol, fingerprinting, the stored terminal result, and the suite-attempt budget.
- Finalize's rebase logic apart from its admission call.
- Suite-teardown completeness (0492).
- Linux CI coverage for the real-process tests.
- Deleting `gate-admission/v2/` or old drive records from disk.
- `run.start`'s capability effects (passed to 0491).

---
id: 133
slug: 'the-run-key-is-the-run-tracker-s-only-handle-gate-starts-car'
title: 'The run key is the run tracker''s only handle; gate starts carry no run check'
status: 'Accepted'
date: '2026-10-02'
supersedes: []
reverses: []
relates_to: [95, 111, 118, 124, 128, 129, 132]
change: 491
---

## Context

At 0488's groom the human set the direction: retire the run id entirely, so the run key becomes the run tracker's only handle. Changes 0489 and 0490 have since removed the takeover revocation check, the worktree slot, and every reader of the slot's run stamp. This trace is of `main` at 756fea9fe, after both.

1. **The run id identifies nothing the key doesn't.** `MintRunRecord` (`internal/app/runtracker_run_record.go`) is the only place a `RunRecord.RunID` is minted, once per run key under `run.lock` (bind-once, `ErrRunExists`). Every `run.start` and every resume arm (`armResumeReplacement`) mints a new key, run record, run id, and run context together. A retry (`run-retry-once`) keeps both the key and the id, so the id never told attempts apart. Supersession is a state on the old key's record, with `ReplacementReserved` naming the new key. Keys are never reused.
2. **What still reads the id:**
   - **The run launch check.** `runLaunchGate` (`internal/app/runtracker_launch_gate.go`) is wired through `SetRunLaunchGate` into gatedrive's `runLaunchGated`, at `Driver.Admit` and at `revalidateAdmittedLaunch` (from `StartAdmitted`). It runs only when the start carries `--run-id`, and finds the run by scanning for the id (`findRunDirByID`, `scanRunsByID`).
   - **`run.cancel`.** Its `expectRunID` comparison refuses `run-id-mismatch` (`runCancel`). Every other cancel step works from the key, and since 0490 cancel attributes drives by `run_context_hash`.
   - **`agent.enter`:**
     - A lone `--run-id` is only an existence preflight (`CheckRunIDExists`).
     - `--run-id` with `--run-key` preflights the pair (`CheckRunIDLinkage`). It then wires participant registration and terminal recording, and for the root coordinator the lifecycle cancel and the death guardian (`SpawnAgentGuardian`; env `DOCKET_AGENT_GUARDIAN_RUN_ID`, compared in `guardianFenceAndReap`).
     - A lone `--run-key` is silently ignored.
     - The run-tracker block documents the lone `--run-id`, so in practice the linkage is dormant.
   - **`expectRunID` parameters** on `RegisterRunParticipant`, `RecordRunParticipantTerminal`, and `FenceRunCompleting`. The last is always `""` in production.
   - **Printing.** The `run-started <key> <run-id> <run-context>` line and `RunStartResult.RunID` (JSON `run_id`). Also the resume locators and remedies (`resumeActiveLocator`, `resumeIncumbentRemedy`, `resumeWorktreeOwnerLocator`, the settle remedy in `RunStart`), the worktree-busy remedy (`incumbentRemedyMessage`), and `RunIDNextAction`.
   - **Threading only.** `GateDriveStartRequest.RunID` → `gatedrive.StartRequest.RunID` → `AdmissionTicket.runID` lives in memory only. No drive record or holder note stores the id.
3. **The launch check strands runs.**
   - It refuses `stale-run-id` for four different causes:
     - `--repo-dir` cannot be canonicalized (a subdirectory is refused too);
     - the run was superseded;
     - the run is active but not bound to a worktree (a claim made without `--run-context`, or a failed best-effort bind);
     - the run is bound to another worktree.
   - `fenceNextAction` (`internal/app/gate_drive.go`) gives all four the same next action, "the run was superseded by a resume", and has no case for `run-completed`.
   - When the check refuses at `StartAdmitted`, `settleAdmittedAfterRunRefusal` writes the drive HALTED `run-cancelled` whatever the cause, including for a completing run.
   - A start without `--run-id` skips the check entirely. Since 0490 the check therefore guards only callers that remembered to thread the id.
4. **Names that outlive the id:**
   - `stale-run-id` comes from three places: the success closeout for a superseded run (`completeSuccessfulRun`, `ReasonStaleRunID`); the mutation fence for a superseded owner (`admitWorkflowMutation`, `ErrStaleRunID`); and `ClassifyRunIDError`'s mapping of `run-id-mismatch`.
   - Outside cancel, `ErrRunIDMismatch` (`run-id-mismatch`) is raised for conflicts that have nothing to do with ids: `bindRunChange` and `bindRunWorktree` re-pointing a run that is already bound elsewhere, and malformed or conflicting terminal evidence in `RecordRunParticipantTerminal`.
5. **The never-launched drive (0490 review finding F3).** `Driver.Admit` persists a reserved drive record before `StartAdmitted` launches it (`NewReservedDrive`: no outcome, no `RawRunDir`, an `AdmissionToken`, the `RunContextHash`). If `gate.drive.start` is killed between the two, the record stays forever, and nothing can launch it, because the ticket lived in memory. Both keyed-verdict paths then stop for good:
   - **Success path.**
     - The call chain: `completeSuccessfulRun` → `accountCompletionLaunches` → `ObserveRunLaunches` → `reconcileFirstLaunch`. It resolves `never-launched` and, in observe mode, returns `launch-pending:<drive>`.
     - The verdict prints `run-stop <key> run-tracker-unavailable completion-unaccounted` on every replay. The finding appears only in the JSON `completion_findings`.
     - The run stays `completing`.
     - The only remedy is `run.cancel`. It settles the drive HALTED `run-cancelled`, so a successful build is recorded as cancelled.
   - **Run-incomplete path.**
     - The call chain: `runTrackerOuterContinuation` → `LocateOuterDrive` → `FindScopeDriveIDs`, which lists the record because it is nonterminal.
     - `TakeoverAndHandoff` takes the record over, but `RunVerify` cannot then report `run-waiting`. The verdict prints `run-stop … continuation-unverified` instead of a retry.
     - The run's single-use outer scope is spent, so the human must `run.start --resume`.
     - `AbandonAdmission`'s `removeReservedDrive` exists to keep such a record out of that scan, but a killed CLI never reaches it.

   A live caller whose `revalidateAdmittedLaunch` fails on a busy per-drive claim leaves the same record behind.
6. **A missing run root never resolves.**
   - `reconcileFirstLaunch` and `reconcileReservation` call `process.Service.ResolveReservation(RunRoot, token)`. That call fails with "root must be an existing directory" when the root is gone: either it was never created (the launcher died before `process.Launch` made it) or it was later cleaned from a temp directory.
   - The census then reports `resolution-unresolved:<drive>` forever, in both modes. The closeout blocks, `run.cancel` stays `cancellation-pending`, and resume quiescence refuses.
   - `resolveHaltedFirstLaunch` already treats a missing root as clean absence, as `supervisorGone` treats a removed run dir.
7. **What the success closeout still protects.**
   - Finalize's local gate carries no run, so since 0490 it needs only the worktree lock (`TestIntegrationRunCompletionCompleteThenScratchCleanupThenFinalizeAdmits`). ADR-0124's original motivation is gone.
   - The closeout still:
     - retires the run as the worktree's ambient owner for the mutation fence;
     - avoids `run-owner-ambiguous` when a later run binds the same worktree;
     - makes cancel and resume refuse a completed run.
   - Its launch census is the only evidence on the Claude route that no gate is still running.
8. **`run.start` under-declares its effects.** It is cataloged `local-write`. But on a cancelled or superseded predecessor, its resume quiescence check runs the stop-capable census (`validateResumeQuiescence` → `verifyTerminalRunQuiescence` → `ReconcileRunLaunches`), which can stop process groups.

## Decision

### 1. The run key is the run tracker's only handle

- **`run.start` stops minting a run id.**
  - `RunRecord.RunID` is deleted. `runToken` stays for the CAS `Generation`.
  - The line becomes `run-started <key> <run-context>`. `RunStartResult.RunID` and its `run_id` JSON key are removed.
  - A start that cannot mint its run record still reports `run-untracked mint-failed`.
  - Resume refusals and remedies name the change and the run key only, with the remedy `docket run cancel --key <key> --reason <why>`.
- **`run.cancel` takes `--key <key> --reason <why>`.**
  - The `--run-id` flag, the `expectRunID` parameter, and the `run-id-mismatch` refusal are removed.
  - Every other authority check is unchanged: the repository, the parent-held authority, and the claim binding or the resume-verified proof (ADR-0128 Decision 1).
  - The disposition vocabulary is unchanged.
- **`gate.drive.start` loses `--run-id`:** `GateDriveStartRequest.RunID` (schema key `RunID`), `gatedrive.StartRequest.RunID`, and `AdmissionTicket.runID`. `--run-context` and `--change-id` are unchanged and still attribute the drive.
- **Delete the id lookup:** `findRunDirByID`, `scanRunsByID`, `runIDLocator`, `CheckRunIDExists`, `CheckRunIDLinkage`, and the `expectRunID` parameters of `RegisterRunParticipant`, `RecordRunParticipantTerminal`, and `FenceRunCompleting`.
- **Hard cut.** There are no deprecated aliases: a caller still passing `--run-id` fails on an unknown flag.

### 2. Delete the run launch check

- **What goes:**
  - In `internal/app`: `runLaunchGate` and its wiring (`SetRunLaunchGate` in `newOwnedGateDriveService`, `NewCommandlessGateDriveService`, and `NewContinuationSeam`).
  - In gatedrive: the `RunLaunchGate` type, `SetRunLaunchGate`, `RunLaunchGateWired`, `runLaunchGated` and its calls in `Admit` and `revalidateAdmittedLaunch`, and `settleAdmittedAfterRunRefusal`.
  - The run-error branches the check fed in `mapDriveFailure` and `mapDriveResult`, and `fenceNextAction`.
- **What stays:** `revalidateAdmittedLaunch` keeps its per-drive claim check.
- **After this, a gate start is refused only by:**
  - worktree admission (`worktree-busy`, ADR-0132);
  - the drive protocol's own checks.
- **Lifecycle states no longer block starts.** A run that is `completing`, `cancelling`, `cancelled`, or `superseded` no longer refuses a gate start. ADR-0124's clause that `completing` "admits no new … start" lapses (see Decision record).
- **Delayed launchers are still contained.** Cancel and the keyed verdict still account every drive they can attribute by run context, under the per-drive claim. A delayed launcher that finds its record settled still refuses.

### 3. The keyed verdict closes a proven never-launched drive

- **Add a third census mode, verdict mode,** beside cancel and observe.
  - For a proven never-launched first launch (`reconcileFirstLaunch`), it does what cancel mode does: it settles the drive under the held per-drive claim, through the same owner CAS (`settleNeverLaunchedFirstLaunch` → `settleNeverLaunched`).
  - It writes the HALT cause `launch-abandoned` instead of `run-cancelled`. `settleNeverLaunched` takes the cause as a parameter, and cancel keeps writing `run-cancelled`.
  - **Reserved relaunches are left alone.** In verdict mode `reconcileReservation` keeps observe behaviour (`launch-pending`). The census never attributes a relaunch in production: only finalize's gate sets `IdempotentSuiteGate`, and finalize's drives carry no run context. Change 0493 retires finalize's automatic relaunch (direction from the 2026-10-02 backlog review).
  - Everything else behaves as in observe mode:
    - a running supervisor is reported `run-live` and is never stopped;
    - nothing is signalled;
    - `claim-busy`, `resolution-unresolved`, and every other pending finding block as they do today.
- **Success path.** `accountCompletionLaunches` runs verdict mode instead of observe mode. A run whose only obstacle was a never-launched drive completes with `run-done <key> run-complete`.
- **Run-incomplete path.**
  - Before `LocateOuterDrive`, `runTrackerOuterContinuation` runs verdict mode for the run's context hash and ignores its findings; only its settles matter.
  - A settled drive is terminal, so `FindScopeDriveIDs` never offers it.
  - The verdict reaches `run-retry-once`, or takes over a genuinely live drive as today.
- **Scope of the mode.** Only the attributed, keyed verdict runs verdict mode (ADR-0124 rule 1); `--unattributed` stays read-only. If observe mode has no production caller left, fold it into verdict mode rather than keep an unused mode.
- **A delayed launcher still alive with its in-memory ticket** takes the same per-drive claim in `StartAdmitted`, re-reads the record, finds it terminal, and refuses, exactly as it does after cancel's settle.

### 4. A missing run root counts as never launched

- In `reconcileFirstLaunch`, `Lstat` the drive's `RunRoot` before resolving the token, as `resolveHaltedFirstLaunch` already does. `reconcileReservation` is left to 0493, for the reason given in Decision 3.
- **Not-exist** is treated as `never-launched`: it is settled in cancel and verdict modes, and is `launch-pending` in observe mode if that mode survives.
- **Any other `Lstat` error** is `resolution-unresolved`.
- **An empty `RunRoot`** is handled as today.

### 5. Names

Derive every site from a whole-repo grep at build time (AGENTS.md: never hand-list the sites of a literal). This table is the starting point, not the complete list.

| Today | Meaning after this change | New |
|---|---|---|
| `stale-run-id` (`ReasonStaleRunID`, `ErrStaleRunID`, the closeout's return value) | the run was superseded by a resume | `run-superseded` (`ReasonRunSuperseded`, `ErrRunSuperseded`) |
| `unknown-run-id` (`ReasonUnknownRunID`) | no run has that id; nothing accepts an id any more | retired; an unknown key reports the existing `run-not-found` |
| `run-id-mismatch` as `run.cancel`'s refusal | the id does not belong to this key | retired with the comparison |
| `run-id-mismatch` as `ErrRunIDMismatch` from `bind-run-change`, `bind-run-worktree`, and `record-participant-terminal` | the run record is already bound to a different change, worktree, or terminal evidence | `run-record-conflict` (`ErrRunRecordConflict`) |
| `ErrRunAmbiguous` from `find-dir-by-id` | two run records share an id | removed with the id scan; `find-by-change` keeps `run-ambiguous` |
| `DOCKET_AGENT_GUARDIAN_RUN_ID` | the guardian's run id | retired |
| `run_id` in the `run.start` result | the run id | retired |
| `--run-id` on `run cancel`, `gate drive start`, and `agent enter` | the run id | retired |

**Do not touch the gate supervisor's own run id.** It is a different identity with the same 32-hex shape, and no retirement row may match it. It appears in:

- `internal/process`;
- `GateResult.RunID`, `RecoveryEntry.RunID`, and `gateCleanupReceipt.RunID`;
- `IncumbentSnapshot.RawRunID` and the `incumbent-run:<id>` locator;
- the `run_id` key in the `gate.launch`, `gate.observe`, `gate.recover`, and `gate.stop` results.

Rename `ClassifyRunIDError`, `RunIDNextAction`, and `runIDRefusal` for what they classify once the id is gone. Their next-action texts stop quoting the three-token `run-started` line.

### 6. `agent.enter` and the death guardian

- **`agent.enter` drops `--run-id`.** `--run-key` alone now does what both flags did together:
  - preflight before spawning anything: an unknown key refuses `run-not-found`, and any other run fault keeps its named token;
  - participant registration and terminal recording by key;
  - for the root coordinator, the lifecycle cancel and the death guardian.
- **The linkage stays opt-in.** The run-tracker block no longer lists `agent.enter`, so no documented flow passes `--run-key`. The linkage stays dormant unless a caller opts in, as it is in practice today. Documenting it would switch on participant accounting, a new way to block the closeout on the Codex route.
- **The death guardian goes key-only.** It loses `DOCKET_AGENT_GUARDIAN_RUN_ID` and `errGuardianRunIDMismatch`, and `guardianFenceAndReap` fences by key. Keys are bind-once and never reused, so the id comparison protected nothing the key does not.

### 7. `run.start` declares `process-control`

- Its capability annotation goes from `local-write` to `local-write` plus `process-control`, pinned beside the representative signatures.
- No skill bounds the coordinator's `run.start` effects, so the declaration blocks nothing.

### 8. Records on disk, and the upgrade

- **No migration and no storage reset.**
  - Run records keep `runSchemaVersion` 1. An old `run.json` with a `run_id` key decodes unchanged, because unknown keys are ignored.
  - Drive records and holder notes never stored the id.
- **Old runs.** A run started by the old binary and still active after the upgrade is cancelled by its key alone.
- **Upgrade contract:**
  - Install only while no run is in flight.
  - Then restart every agent session. A session that loaded the old run-tracker block would copy a run id and pass `--run-id`, which now fails as an unknown flag.

## Consequences

- **A cancelled or superseded run's leftover agent can start a suite.** Nothing refuses a start that carries a dead run's context. What still contains it:
  - The worktree lock keeps it to one suite at a time.
  - The replacement run's next gate is refused `worktree-busy`, naming the holder while it runs.
  - A repeat `run.cancel` of the old key stops it: `repairTerminalRun` re-runs that run's census in cancel mode.

  The stub accepted this loss.
- **A gate started after the verdict's census goes unaccounted.** `completing` no longer refuses starts, so such a gate runs with no run accounting it. The worktree lock still serializes it against finalize's gate.
- **A run root deleted from under a live supervisor** is treated as never launched (Decision 4), just as `supervisorGone` already treats a removed run dir. The worktree lock still keeps a second suite out while that supervisor runs.
- **A keyed verdict can settle a live child's reserved drive (found at review).** On the run-incomplete path the keyed verdict's verdict-mode census runs against an active run. If a keyed verdict runs while the dispatched child is still alive and between `Admit` and `StartAdmitted`, that child's reserved drive is settled HALTED `launch-abandoned`, its already-charged build attempt is not refunded, and its `StartAdmitted` refuses. The window is milliseconds wide; the run then reaches `run-retry-once`.

## Alternatives considered

- **Keep a narrow "is this run over?" check keyed on `--run-context`.** Rejected: it keeps refusing gate starts on bookkeeping state, which is the failure class this change removes, and caller identity is not enforceable anyway, because the mutation fence keys on the worktree.
    - **Keep the run id as an optional locator.** Rejected: every consumer can use the key, and threading a second token is where the 0463, 0467, and 0477 bugs came from.
    - **F3: name `run.cancel` in the verdict, or only document it.** Rejected: it records a success as cancelled and needs a human every time.
    - **F3: settle only on the success path.** Rejected: it leaves the run-incomplete path's `continuation-unverified`.
    - **F3: stop counting a never-launched drive without writing anything.** Rejected: with the launch check gone the drive stays launchable, and it remains a takeover candidate.
    - **`agent.enter`: delete the dormant linkage.** Rejected: it removes the Codex automatic Stop that 0375 built and 0490 kept.
    - **`agent.enter`: document `--run-key`.** Rejected: it switches on participant accounting, a new way to block the closeout on the Codex route.

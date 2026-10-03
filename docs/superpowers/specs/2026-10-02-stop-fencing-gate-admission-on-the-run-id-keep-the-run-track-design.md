<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0491 — Retire the run id; the run key becomes the run tracker's only handle](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0491-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md)**
<!-- docket:backlink:end -->

# Retire the run id; the run key becomes the run tracker's only handle — design

## Problem

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

## Accepted losses

- **A cancelled or superseded run's leftover agent can start a suite.** Nothing refuses a start that carries a dead run's context. What still contains it:
  - The worktree lock keeps it to one suite at a time.
  - The replacement run's next gate is refused `worktree-busy`, naming the holder while it runs.
  - A repeat `run.cancel` of the old key stops it: `repairTerminalRun` re-runs that run's census in cancel mode.

  The stub accepted this loss.
- **A gate started after the verdict's census goes unaccounted.** `completing` no longer refuses starts, so such a gate runs with no run accounting it. The worktree lock still serializes it against finalize's gate.
- **A run root deleted from under a live supervisor** is treated as never launched (Decision 4), just as `supervisorGone` already treats a removed run dir. The worktree lock still keeps a second suite out while that supervisor runs.

## Unchanged

- **The rest of the run tracker:**
  - the run key, the run context and its binding on `change.claim` (ADR-0111);
  - retry accounting;
  - read-only `--unattributed` verdicts, `run.continue`, and `run.verify`;
  - resume admission (one live run per worktree, exactly one replacement) and the per-change resume lock (ADR-0128);
  - the `## Run halted` marker.
- **`run.cancel`:** its fence, journal handling, cancel-mode census, and disposition vocabulary. It still writes `run-cancelled` on a never-launched drive.
- **The workflow-mutation fence** (`admitWorkflowMutation`), apart from the `run-superseded` rename.
- **The gate machinery:** worktree admission (ADR-0132), the drive protocol, and the supervisor contract (ADR-0095).
- **`resolution-unresolved` stays fail-closed in every mode**, because a reservation that cannot be proven either way may be running.

## Prose and generated sites

| Site | Change |
|---|---|
| `cursor-rules/run-tracker.md` (the source of the CLAUDE.md/AGENTS.md `docket:dispatch` block) | **Step 1:** `run-started <key> <run-context>`; "keep both"; copy only `<run-context>` into the dispatch prompt; delete the sentence that threads `<run-id>` into `run.cancel --run-id` and the `--run-id` dispatch flags. **Stopping:** "with the key `run.start` gave you" and `--key <key> --reason <why>`; drop "so nothing new can attach to it", because gate starts are no longer fenced; `refused` names "the key or repository". **Resuming, `resume-active-run`:** the locator names the change and the key; the cancel argv has no `--run-id`. **Operation ids (0443's wording fix, folded in at the 2026-10-02 backlog review):** say explicitly that an operation id such as `run.start` is not a command. Look up its entry in `docket capabilities --json` and run that entry's `argv`. (0443 recorded an agent running `docket run.gate-before implement-next` itself.) |
| `AGENTS.md` / `CLAUDE.md` (CLAUDE.md is a symlink to AGENTS.md) | Re-render the managed block from the generator after the embedded mirror is regenerated (`TestCommittedCodexDispatchMatchesGenerator` reads the embedded catalog). Never hand-edit it. |
| `internal/harness/dispatch.go` `CodexRootEntryClause` | Drop "and the unchanged run id, labeled for `--run-id` on build-owned starts"; keep the run-context labeling. |
| `skills/docket-implement-next/SKILL.md` | Step 6's evidence re-mint argv and Step 7's run-id sentence drop `--run-id`. Move the list of build-owned starts and "always `--change-id`" onto the run-context sentence; "neither value" becomes singular. |
| `skills/docket-implement-next/references/edge-paths.md` | The `resume-active-run` refusal names the change and the run key; the cancel argv is `--key <key> --reason <why>`. |
| `skills/docket-build/SKILL.md` | The build-gate and Red items drop `--run-id`; "no run context, run id, or capability" becomes "no run context or capability". |
| `skills/docket-build/references/gate-caller-loop.md` | The `start` row drops "and `--run-id <id>`". |
| `skills/docket-finalize-change/references/gate-failure.md` | The `run.cancel` argv in the worktree-lock section. |
| `docs/reference/glossary.md` | Rename the heading and anchor to *Start / run key / run context*, and update its cross-references and table-of-contents entry. "Mints three values … both copied … never receive either" becomes two values. Update the cancel, gate-drive, and agent-enter examples. *Run fence* is located by the run key. Add `launch-abandoned`, `run-superseded`, and `run-record-conflict` where causes and tokens are listed. |
| `docs/concepts/run-tracker.md` | Re-read for any description of the launch check, or of gate starts refused by a cancelled or completing run. |
| Go messages and comments | `resumeActiveLocator`, `resumeIncumbentRemedy`, `resumeWorktreeOwnerLocator`, the settle remedy in `RunStart`, and `incumbentRemedyMessage`. The help strings of `run start`, `run cancel`, `agent enter`, and `gate drive start`. The `RunStartResult` doc comment. Every comment that says cancel finds suites by run id, starting with the header of `internal/repoguard/gatedrive_run_id_thread_test.go`. |
| Embedded mirror | Run `go generate ./internal/assets` for the six mirrored files that change, then `go run ./cmd/genassets -check`. Order: edit the sources, regenerate, then re-render AGENTS.md. |

- **Derive the full list by grep.** A whole-repo grep for `run-id`, `run_id`, `<run-id>`, `run id`, `RunID`, `stale-run-id`, `unknown-run-id`, and `run-id-mismatch` decides it. Sort the hits into prose and executable, and into the run tracker's id and the gate supervisor's id.
- **Frozen records are never edited:** plans, results, archived changes, and the bodies of Accepted ADRs.
- **Budgets** (`internal/repoguard/budgets_test.go`):
  - Five prose rows sit at their ceilings: docket-build SKILL, gate-caller-loop, implement-next SKILL, edge-paths, and gate-failure. Pure deletions pass, and rewording must not grow them; re-pin them per `tests/README.md`.
  - `dispatchBudget` is a ceiling: the block is 923 words against 1154. The run-id deletions outweigh the 0443 sentence, so the block shrinks and the ceiling must not be raised. Re-pinning it at the new count is optional; nothing queued needs the headroom now that 0422 is killed.

## Tests

Decide each test by what it guards, not by what it asserts (learning: test-premise-deleted-not-regated). Never restore deleted code or prose to keep a test green. The names below are the ones the trace found; derive the full set by grep.

- **New:**
  - **T1 (success path, the stub's regression).** End to end through `run.verdict <key>`:
    - Start a tracked drive with `--run-context`.
    - Kill the CLI between `Admit` and `StartAdmitted`, or fault-inject the same state.
    - Let `RunVerify` report run-complete.
    - Assert `run-done <key> run-complete`, the run `completed`, and the drive HALTED `launch-abandoned`.
    - Mutation-test it: restoring observe mode's `launch-pending` return must turn it red.
  - **T2 (run-incomplete path).** The same orphan, with `RunVerify` reporting run-incomplete. Assert `run-retry-once` (not `continuation-unverified`), the drive HALTED `launch-abandoned`, and the outer scope not consumed.
  - **T3 (missing run root).** A never-attached first launch whose `RunRoot` does not exist. Assert that:
    - cancel reaches `cancelled`, with the drive HALTED `run-cancelled`;
    - the keyed verdict completes, with the drive HALTED `launch-abandoned`;
    - resume quiescence admits.
  - **T4 (a delayed live launcher).** Settle a drive in verdict mode while its ticket is held. `StartAdmitted` refuses and launches nothing.
  - **T5 (verdict mode stops nothing).**
    - Verdict mode never calls the stop-capable seam: extend `TestIntegrationRunCompletionCompleteSuccessfulRunSendsNoStops`.
    - A running supervisor still blocks as `run-live`: `TestIntegrationRunCompletionProductionCensusHaltedLiveDriveBlocks` stays green.
  - **T6 (surface).** Assert that:
    - `run-started` has two tokens;
    - `run cancel`, `gate drive start`, and `agent enter` reject `--run-id`;
    - `agent enter --run-key` alone wires the linkage and preflights `run-not-found`;
    - `run.start` declares `process-control`.
- **Flip:** `TestCensusSettlesNeverAttachedFirstLaunch/observe-pending-unchanged` (`internal/gatedrive/reconcile_test.go`). Move it to verdict mode and assert the `launch-abandoned` settle. `TestCensusReservedRelaunchResolvesRelaunchToken/never-launched-observe-pending` keeps its expectation (`launch-pending`, record untouched) and is re-pointed to verdict mode if observe mode is folded in. Keep an observe-mode case only if observe mode survives.
- **Delete** (their subject is gone):
  - In `internal/gatedrive`:
    - `run_launch_gate_test.go`;
    - the run-gate cases in `driver_runfence_test.go` (re-point any case that tests the per-drive claim alone);
    - `TestStartAdmittedRunRefusalFreesWorktree`;
    - the run-gate parts of `driver_concurrency_test.go` (`fakeRunRegistry`, `TestBarrierCancel*`);
    - `TestLaunchSitesBoundToRunLaunchGate` and `TestLaunchSiteGuardIsFalsifiable`. The launch-site population test and `TestGateLaunchAdmissionCoverage` stay.
  - In `internal/app`:
    - `runtracker_launch_gate_integration_test.go`;
    - the removed-token cases in `runtracker_run_id_refusal_test.go`;
    - `TestIntegrationRunRecordCheckRunIDLinkage`, `TestIntegrationRunRecordFindRunDirByID`, and `TestIntegrationRunRecordFenceRunCompletingRejectsStaleLocator`;
    - `TestIntegrationRunCancelRunCancelRefusedWrongRun`;
    - `TestRaceIntegrationAppConcurrencyRunLaunchGateSerializesWithFence`;
    - `TestProductionConstructorsWireRunLaunchGate` and `TestMapDriveFailureRunErrors`;
    - `TestBuildStartRunRefusalChargesNoAttempt` and `TestBuildStartChargedAttemptNotRefundedOnFencedLaunch`.
  - In `internal/cli`: `TestAgentEnterLoneRunIDIsPreflightedBeforeLaunch` and `TestGateDriveStartUnknownRunIDIsNamed`.
  - In `internal/repoguard`: `TestRunTrackerCopiesRunIDIntoDispatchPrompt` and `TestCodexRequestFileCarriesRunID`, or invert both into absence checks.
- **Re-point:**
  - `TestGateDriveRunIDThreaded` keeps only its `--change-id` check and its minimum site counts; rename it.
  - `TestIntegrationRunStartStartedLineIsAlwaysThreeTokens` becomes two tokens.
  - `…FreshStartSurfacesRunID`, `…NoRunRecordResumeMintsBoundRun` (its JSON), and the resume-locator tests.
  - The CLI pins: `TestAgentEnterRefusesBadRunIDLinkageBeforeLaunch` (now by key), `TestAgentEnterCapabilitySignature`, `TestRepresentativeSignatures`, and `TestRunTrackerVocabularyHardCut`.
  - The `stale-run-id` token pins:
    - `…FenceRefusesSupersededRunAsStale`;
    - `…RunCarryingFencesUnchangedByOwnerSelection`;
    - `…CompleteSuccessfulRunNeverRelabelsCancellation`;
    - `TestPRFenceRefusalMessageIsReasonAware` and `TestWorkspaceFenceRefusalMessageIsReasonAware`.
  - `TestMapDriveFailureFenceReasons`, `TestStartForwardsRunFields`, and `TestIntegrationRunStartStorageResetIgnoresRetiredRoots`.
  - The fixtures that carry a run id: `cancelFixture.runID`, `runtracker_fence_helpers_test.go`, and the guardian, root-entry, and production-census integration tests.
- **Retired vocabulary** (`internal/repoguard/retired_vocabulary_test.go`):
  - Re-point rows 10, 11, 20, 31, 32, 38b, and 38d, whose replacements are now retired.
  - Add rows retiring `stale-run-id`, `unknown-run-id`, `run-id-mismatch`, `--run-id` on the three commands, `DOCKET_AGENT_GUARDIAN_RUN_ID`, and the `run.start` result's `run_id`. Scope each row so it cannot match the gate supervisor's `run_id`.
  - Update the negative control and the planted lines that contain `--run-id <id>`.
  - Mutation-test every new row.
- **Budgets and the build gate:**
  - Re-measure the `tests/runtime-budgets.tsv` rows for the shrinking test corpus per `tests/README.md`.
  - Read every `BUDGET WATCH` and `SERIAL CONFIRMED OVER BUDGET` line.
  - Run the whole suite through `build.test_command`.

## Decision record

- **A new ADR, recorded through docket-adr:** *The run key is the run tracker's only handle; gate starts carry no run check.* `relates_to: [95, 111, 118, 124, 128, 129, 132]`; it supersedes none.
  - **Context:** Problem facts 1–8.
  - **Decision:** Decisions 1–8.
  - **Consequences:** the accepted losses.
  - **Alternatives:**
    - **Keep a narrow "is this run over?" check keyed on `--run-context`.** Rejected: it keeps refusing gate starts on bookkeeping state, which is the failure class this change removes, and caller identity is not enforceable anyway, because the mutation fence keys on the worktree.
    - **Keep the run id as an optional locator.** Rejected: every consumer can use the key, and threading a second token is where the 0463, 0467, and 0477 bugs came from.
    - **F3: name `run.cancel` in the verdict, or only document it.** Rejected: it records a success as cancelled and needs a human every time.
    - **F3: settle only on the success path.** Rejected: it leaves the run-incomplete path's `continuation-unverified`.
    - **F3: stop counting a never-launched drive without writing anything.** Rejected: with the launch check gone the drive stays launchable, and it remains a takeover candidate.
    - **`agent.enter`: delete the dormant linkage.** Rejected: it removes the Codex automatic Stop that 0375 built and 0490 kept.
    - **`agent.enter`: document `--run-key`.** Rejected: it switches on participant accounting, a new way to block the closeout on the Codex route.
- **Dated Update notes** (an Accepted ADR changes only by these):
  - **ADR-0124:**
    - Rule 3 changes: the keyed verdict's census settles a proven never-launched drive HALTED `launch-abandoned`. It still stops no process and signals nothing.
    - `completing` no longer refuses gate starts, because no launch check exists.
    - Since ADR-0132, finalize's gate no longer depends on the closeout. The closeout still retires the run's worktree ownership for the mutation fence, and makes cancel and resume refuse a completed run.
  - **ADR-0128:** the arm-time "epoch" is the run record named by the run key, and `run.cancel` is keyed by the run key alone. Decisions 1–4 stand.
  - **ADR-0132:** the cancellation and resume rules of ADR-0118 that it left standing until change 0491 are replaced by the new ADR.
- **ADR-0129, amended in place** (authorized at this groom, as for earlier rename families): rows 3, 10, 11, 20, 31, 32, 38b, and 38d name their 0491 successors or are marked retired. Derive any further rows from the grep.
- **Results file:**
  - **Important:** install after the merge only while no run is in flight, then restart every agent session, because the run-tracker block changed.
  - **Optional:**
    - a run left `completing` by a never-launched drive completes on its next keyed `run.verdict`;
    - a cancel stuck `cancellation-pending` on a missing run root finishes when re-run.

## Effect on related changes

- **0422** was killed in the 2026-10-02 backlog review: its retry over-count cannot fire at the default `run.max_attempts` of 2. Nothing here depends on it, and the attempt numbering it targeted (`attempt := 1 + usedBefore` in `RunVerdict`) is untouched.
- **0443** was killed in the same review. Its wording fix is folded into this change's run-tracker block rewrite (see Prose and generated sites).
- **0493** is retargeted to retire finalize's automatic relaunch. This change therefore leaves the reserved-relaunch path (`reconcileReservation`) as it is (Decision 3).
- **0492** (suite teardown): unchanged.
- **The "killed inside `process.Launch`" case** stays fail-closed here: `ResolveReservation` returns `unresolved` for an allocated manifest with no established supervisor. It belongs with 0492 and 0493.
- **0494** (captured at this groom): a `pr.publish` or `workspace.publish` killed mid-flight leaves its journal entry `admitted` forever, wedging cancel and the closeout.

## Out of scope

- The run tracker's attribution and retry model.
- The workflow-mutation fence beyond the rename, and the publish journal wedge (0494).
- Process-tree teardown (0492), finalize's automatic relaunch and its reserved-relaunch accounting (0493), and launches that cannot be resolved either way.
- Deleting the dormant `agent.enter` lifecycle linkage.
- Any migration or storage reset.

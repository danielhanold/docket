<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0497 — Re-running finalize can start a second suite beside an orphaned one](../../changes/archive/2026-10-04-0497-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md)**
<!-- docket:backlink:end -->

# Re-running finalize can start a second suite beside an orphaned one — design

## Problem

A gate's supervisor holds the worktree lock (ADR-0132), and the kernel frees that lock the moment the supervisor dies. When a supervisor dies alone (SIGKILL or a crash), `go run` and the test runner keep running in its process group, and since change 0493 (ADR-0135) the drive halts and a human re-runs the workflow. If the human re-runs right away, the new gate takes the free lock and starts a second suite in the same checkout beside the leftover one. The stub proposed checking 0492's launch census at the next gate start and refusing. Tracing found these facts, several of which change that hypothesis.

1. **The leftover check exists, but only the census calls it.** `process.Service.ProbeLeftover` (change 0492) answers, for a run whose supervisor has exited, `leftover` when the supervisor's group still has members and its leader is provably gone; `none` when the group is empty; and `unclear` for anything it cannot prove. Its only caller is the launch census (`supervisorGone`), which reports `tree-survives:<drive>:<pgid>` from `run.cancel`, the death guardian, and `run.verdict`'s success closeout.
2. **The death halt does not look.** `driveSlice`'s `StateSignaled, StateVanished` branch halts `supervisor-died`, or `uncertain-ownership` when `proveNoTreeSurvives` cannot prove the supervisor gone. Despite its name, that helper never probes the group.
3. **Finalize deletes the evidence in the same call that reports the halt.** `processFinalizeGate.mapDriveOutcome` removes the drive's run root on every terminal outcome unless `haltedRunRootHoldsUnexitedRun` finds a run not proven exited. A dead supervisor is exited, so the root, including the run manifest that records the process group, is removed at once. After that, no later check can find the leftover: `ProbeLeftover` needs the manifest, the census reads a missing run dir as clean absence, and the worktree lock's holder note names a deleted dir.
4. **The census never sees finalize anyway.** Finalize drives carry no run context, so the census never attributes them. 0492's finding cannot reach a finalize gate on any path.
5. **Finalize does not even say the supervisor died.** `mapDriveHaltCause` maps `supervisor-died` to `unavailable`, and the `finalize.rebase` composition reports "the local gate did not reach a decidable pass/fail; retained, no red fabricated".
6. **Build keeps its run root, but nobody tells the human.** The build caller does not remove the run root, so cancel and the verdict closeout can see a build leftover. `run.start --resume` runs the census (`validateResumeQuiescence`) but prints findings only when it refuses. The human learns about a build halt from the halt report that becomes `## Run halted`.
7. **A dead supervisor reads `unclear` while its launching process lives** (ADR-0134 fact 8: it stays an unreaped zombie). `gate.drive.start` and `finalize.rebase` each launch the supervisor, run one slice (`productionSlice`, 30s), and exit; every later slice is a fresh process. So a death after the first slice reads cleanly and a death within it reads `unclear`. `evidence.recertify` drives every slice in one process, so its deaths always read `unclear`.
8. **Harm and frequency.** Two overlapping suites use separate temp work dirs (0492 fact 9), so the harm is a shared checkout and CPU contention: a slower or timed-out run, or a red caused by contention, which in finalize feeds integration repair. None of the 237 gate drives on the development machine since 2026-09-29 had a supervisor death.

Two alternatives were rejected at grooming:

- **Refuse the next start while a leftover runs.** By fact 3 it would also need finalize to keep the halted run root and admission to read the holder note as a decision input, and it adds a new way for a run to be refused. That is the same shape ADR-0134 rejected for cancel ("blocks a resume for a suite runtime").
- **Print the finding when `run.start --resume` admits.** The coordinator agent reads that output just before it dispatches. It would either ignore the line or wait, and waiting is a block. The human has already seen the warning in the halt record.

## Decision

Report a leftover at the one moment docket can still see it: the death halt. The halt carries the same informational `tree-survives:<drive>:<pgid>` finding the census uses, and finalize's halt message tells the human to wait before re-running. Nothing is refused, stopped, or signalled, and every halt keeps its outcome and cause.

### 1. The death halt probes for a leftover (`internal/gatedrive`)

- In `driveSlice`'s death branch, after the cause is chosen (`supervisor-died` or `uncertain-ownership`), call the existing `ProbeLeftover` through the process seam on the drive's run dir. Only a `leftover` answer stamps the finding `tree-survives:<drive>:<pgid>`. `none`, `unclear`, and a probe error stamp nothing.
- The finding never changes the outcome (`HALTED`) or the cause.
- The slice result carries the finding. It is persisted with the halt beside `LastCause` as `last_finding` (`omitempty`). Unknown fields are ignored on read, so old records decode unchanged and there is no schema bump. Re-reading the terminal drive (`recordedDoc`) returns the same finding.
- `DriveDoc` gains `finding` (`omitempty`), set only on a `HALTED` document. `GateDriveResult.HumanText` prints a `finding:` line when it is set.
- Only the death branch probes. Every other halt either reaches its terminal through a `Stop` that waited for the group to empty (deadline expiry, a drifted pass, `stopped-not-initiated`) or has no reason to suspect a leftover.

### 2. Finalize reports it (`internal/app`, finalize gate)

- A `HALTED` drive's finding becomes the local gate result's teardown finding, so the finalize gate report's existing `teardown_finding` field carries `tree-survives:<drive>:<pgid>`. It cannot co-occur with the run-root retention tokens: the root is retained only when a run under it is not proven exited, and a `leftover` answer requires an exited supervisor. If both ever applied, keep the retention token in `teardown_finding`; the leftover still reaches the message below.
- The `finalize.rebase` halt message, for a drive halt (no admission refusal):
  - cause `supervisor-died` with no finding: `the local gate's supervisor died before the suite finished; re-run finalize to re-run the suite`;
  - cause `supervisor-died` with a finding: `the local gate's supervisor died before the suite finished; part of the suite is still running as process group <pgid> — wait until pgrep -lg <pgid> prints nothing, then re-run finalize`;
  - cause `uncertain-ownership` with a finding: today's message, followed by `; part of the suite is still running as process group <pgid> — wait until pgrep -lg <pgid> prints nothing, then re-run finalize`;
  - every other halt: today's message, byte for byte.
- The disposition (`blocked`), reason (`gate-halted`), result, and `halt_cause` (`unavailable`) are unchanged. The finalize skill already relays the message to the human, the PR comment, and `## Finalize blocked`.
- `evidence.recertify` is unchanged. By fact 7 it never sees a finding, and its message stays as it is.

The plan may choose the field and helper names; the message text above is the contract.

### 3. Build relays it (skill prose)

`docket-build`'s gate-caller-loop reference gains one sentence: a `HALTED` document's `finding`, when present, is copied verbatim into the halt report together with the glossary's remedy (wait until the group is gone before re-running). The sentence also says the finding is information only: it never changes the halt, never adds one, and never feeds repair. The plan traces where the build controller's halt report becomes `## Run halted` and adds at most one sentence there if the finding would otherwise be dropped.

### Failure posture

No new halt, block, refusal, or signal. The probe is read-only and runs only on a drive that has already halted. A pinned, mutation-checked test asserts that for every probe answer (`leftover`, `none`, `unclear`, error) and both death states (`signaled`, `vanished`), the drive's outcome and cause and finalize's disposition, reason, result, and halt cause are identical. Only the finding and the message text differ. Two mutations must each turn a test red: removing the probe call, and letting a finding alter the cause. The glossary entry and both skill sentences state that the finding is never a blocker, so no reviewer or controller escalates it into one.

## Accepted losses

- **A death within the first 30s slice** reads `unclear` (fact 7), so the halt carries no finding. Today's behavior.
- **`evidence.recertify`** never reports a leftover (fact 7).
- **Targets in their own process groups** after the runner itself died stay invisible (0492 gap 2).
- **The warning is a snapshot, not a guard.** A human who re-runs anyway still starts a second suite beside the leftover, exactly as today; ADR-0134's consequence "A new gate in the same worktree can still start beside a leftover" stands.
- **A reused group number** could produce a wrong note (ADR-0134 fact 7). At the halt the probe runs moments after the death, so this is even less likely than in the census, and the cost is a note, never a signal.

## Unchanged

- Gate admission, the worktree lock, and the holder note (ADR-0132).
- The launch census and its findings (ADR-0134).
- The output of `run.start`, `run.verdict`, and `run.cancel`.
- Finalize's run-root removal.
- The relaunch retirement (ADR-0135): a supervisor death still halts, and nothing relaunches.

## Prose and generated sites

Derive every site from a whole-repo grep (`tree-survives`, `teardown_finding`, `last_cause`, and the drive document's field names), then sort them into prose and executable. Expected:

- `docs/reference/glossary.md`: the `tree-survives` entry, titled "Cancel finding" today, is retitled to cover both reporters and adds "a halted gate (build or finalize) whose supervisor died" to the list of places it appears. Its check-and-stop commands stay. Fix any anchor that links the old heading.
- `docs/concepts/run-tracker.md`: update its `tree-survives` mention only if it lists the reporters.
- `skills/docket-finalize-change/references/gate-failure.md`: the sentence on a gate whose supervisor dies gains one sentence: when the halt reports a leftover suite, wait until that group is gone before re-running finalize.
- `skills/docket-build/references/gate-caller-loop.md`: the sentence from Decision 3.
- Any operation schema or reference page that lists the drive document's or the finalize gate report's fields.

## Tests

- `internal/gatedrive`: the death branch with the leftover seam answering each of `leftover`, `none`, `unclear`, and error, under both `signaled` and `vanished`. The finding appears only on `leftover`; outcome and cause are identical across all of them. Re-reading the terminal drive returns the finding. A record without `last_finding` decodes and re-reads unchanged. `HumanText` prints the `finding:` line.
- `internal/app`: the finalize mapping. `supervisor-died` with and without a finding produces the exact messages above; `uncertain-ownership` with a finding produces today's message plus the clause; every other halt produces today's message byte for byte. `teardown_finding` carries the token. Disposition, reason, result, and halt cause are identical across all of them.
- The failure-posture test above, with both mutations recorded in the results file.
- The whole suite at the build gate.

## Decision record

ADR-0134 gets a dated Update note: the death halt also reports `tree-survives` (change 0497), because finalize removes a halted run's root in the same call and the census never attributes finalize drives. The rule is unchanged: report, never signal; `unclear` keeps today's behavior. Refusing the next start was considered and rejected for the reason ADR-0134 rejected making cancel wait. No new ADR.

## Out of scope

- Refusing or delaying a gate start while a leftover runs.
- A `tree-survives` line from `run.start --resume`.
- Keeping a halted gate's run root on disk.
- Stopping or signalling a leftover suite.
- 0492's accepted gaps 2 and 4.
- Raw `gate.launch` runs.
- `evidence.recertify`'s reporting.
- Process leaks inside individual tests (`t.Cleanup` hygiene).

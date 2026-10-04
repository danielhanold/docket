---
id: 134
slug: 'a-dead-supervisor-s-suite-counts-as-gone-only-when-its-proce'
title: 'A dead supervisor''s suite counts as gone only when its process group is empty, and docket never signals on that evidence'
status: 'Accepted'
date: '2026-10-03'
supersedes: []
reverses: []
relates_to: [95, 132]
change: 492
---

## Context

A gate's process tree is supervisor → `go run` → test runner → one test target per process group. `RunSupervisorFromEnv` starts the supervisor as a session leader (`Setsid`). Its child (`go run`) and the runner join the supervisor's process group. `ExecuteTarget` starts each target with `Setpgid`, so each target leads its own group inside the same session. The supervisor waits only for its direct child (`cmd.Wait`). Docket reads "the suite is gone" from the supervisor alone: a terminal record, or a free `live.lock` (`vanished`).

The trace found these facts. Some of them correct the stub.

1. **Gap 1 — a supervisor that dies alone leaves its suite running. Confirmed.** The supervisor takes SIGTERM and SIGINT on a channel (`RunSupervisorFromEnv` step 5), so a plain `kill <pid>` does not stop it. Only SIGKILL or a crash does. Afterwards `go run` and the runner keep running in the supervisor's process group, and the runner keeps its targets. `Observe` reports `vanished`. The launch census's `supervisorGone` counts every `SupervisorExited` state as torn down, so `run.cancel` reports `cancelled` while the suite still runs, and the worktree lock (ADR-0132) is already free.
2. **Gap 2 — a KILL escalation leaves test targets running. Real, but narrower than the stub says.** `Service.Stop` sends TERM to the supervisor's group and waits `stopTermWait` (10s) for a terminal record and an empty group before it sends KILL. The runner's own grace (`defaultKillAfter`, 5s in `InstallSignalHandling`) is shorter. On TERM the runner forwards TERM to every registered target group, sends KILL to them after 5s, and exits only after its lanes drain. So `Stop` escalates only when the runner has not exited 10s after TERM, which means the runner itself is stuck. A normal stop never reaches the KILL.
3. **Gap 3 — the single relaunch trusts a dead supervisor. Confirmed, and broader than the stub says; left to 0493.** In `driveSlice`'s `StateSignaled, StateVanished` branch, `proveNoTreeSurvives` returns true for `vanished` without any probe. For `signaled` it calls `Stop`, which finds the terminal record and returns the already-terminal no-op (`terminalNoOp`) without probing the group either. Only finalize's local gate relaunches (`IdempotentSuiteGate`). The 2026-10-02 backlog review retargeted 0493 to retire that relaunch, which removes gap 3's only caller, so this change does not touch it.
4. **Gap 4 — on a graceful stop the worktree frees before teardown ends. Real, but the window is small.** `go run` handles only SIGINT and SIGQUIT, so TERM kills it at once. The supervisor then records `signal` and closes `live.lock` and the worktree lock while the runner is still stopping its targets. Targets that die on TERM drain in milliseconds; 5s is the bound for a target that ignores TERM. `Stop`'s own caller is not exposed: `awaitTeardown` waits for the supervisor's whole group, and the runner is in it. Only a different process starting a gate in that window can overlap.
5. **The right check already exists in one place.** `gate.recover`'s `classifyRun` (`internal/process/recover.go`) writes an abandoned marker for a free-lock, no-terminal run only when the recorded group is provably absent (`recoverGroupProbe`, which is `groupAlive`). A live or unprovable group is reported as `needs-inspection`. The stub's "`ClassifyRun`'s group check" is this unexported `classifyRun`. The census does not use it.
6. **An empty supervisor group means the whole suite is gone, unless the runner was itself killed.** The runner exits only after every target it started has been reaped. While the runner lives, the supervisor's group is non-empty. Targets can outlive the group only when the runner dies abnormally (gap 2, or a second kill).
7. **Process-group ids are reused, but not while the group exists.** Linux and the BSD-derived kernels (macOS included) do not hand out a pid that is still in use as a process-group id. A group whose leader pid is gone but which still has members is therefore the original group. Long after a run, the same number can name an unrelated group whose leader is alive. Docket's own suite starts thousands of processes per run, so on a development machine the numbers wrap within hours.
8. **A dead supervisor can linger as a zombie.** `Service.Launch` starts the supervisor and never waits for it. Normally the launching CLI exits first, and launchd or init reaps the supervisor. When the launching process is still alive at the supervisor's death, the supervisor stays a zombie, and its pid still answers `kill(pid, 0)`.
9. **No production incident is on record.** The evidence is the 0490 groom's experiments on macOS (gaps 1 and 2) and reading the code (3 and 4). Two overlapping suites use separate temporary work dirs (`os.MkdirTemp("", "docket-devtest-*")` in `suiterunner`), so the harm is CPU contention (slower runs and possible timeouts), a false `cancelled`, and leftover processes that finish on their own.

## Decision

A dead supervisor's suite counts as gone only when its process group is empty. Docket reports that evidence, and never signals on it. Wherever the evidence is unclear, behavior is exactly today's, so no path becomes worse than it is now.

### 1. The leftover check (`internal/process`)

A new read-only method on `process.Service` answers, for a run whose supervisor has exited, whether any of its suite is still running. Suggested shape: `ProbeLeftover(runDir) (Leftover, error)`, where `Leftover` carries an answer and the recorded group id. The plan may choose the names.

- **Validation** is `Observe`'s: run path, manifest, and run-id agreement. A validation failure is an error, which callers treat as **unclear**.
- **Supervisor still holding `live.lock`**, or an unprovable lock probe: **unclear**. The check is meaningful only once the supervisor is gone, and callers reach it only from an exited observation.
- **The recorded group** is the manifest's `pgid`. A `pgid` ≤ 1, or one that differs from `supervisor_pid`, is **unclear**.
- **Group probe** (`groupAlive(pgid)`):
  - absent → **none**;
  - unknown → **unclear**;
  - live → probe the leader pid (`processAlive(pgid)`): absent → **leftover**; live (a zombie supervisor, or the number reused by an unrelated process) or unknown → **unclear**.
- It never signals and never writes.
- `internal/process` stays standard-library-only (`TestImportBoundaryStdlibOnly`). The group and leader probes get package-private test seams in the style of `recoverGroupProbe`, so every unclear branch is deterministic on every platform. `classifyRun` and its abandoned-marker rule are unchanged.

Callers act only on **leftover**. **None** and **unclear** both keep today's behavior.

What it sees: `go run`, the runner, and anything else that stayed in the supervisor's group. It does not see targets in their own groups. By fact 6 that is enough while the runner lives. Targets whose runner was itself killed stay invisible (Accepted losses).

### 2. The census reports a leftover without changing any outcome (`internal/gatedrive`)

In `supervisorGone`, the `o.State.SupervisorExited()` branch runs the leftover check:

- **leftover** → still `true` (torn down), with the finding `tree-survives:<drive>:<pgid>`;
- **none** or **unclear** → `true` with `run-terminal:<drive>`, as today.

The branch after a stop this census performed (`replacement-stopped`) needs no check: `Stop` itself waited for the group to empty. In `proveRunDirsGone`, `tree-survives` outranks `replacement-stopped` and `run-terminal`, so a leftover on any of a drive's run dirs is the drive's reported finding. The finding stays credential-free: a drive id and a group id.

Because the drive still counts as settled, `RunLaunchReport.Accounted` is unchanged, and the finding surfaces where accounted findings already surface:

- `run.cancel` (`reconcileRunTeardown`) and the agent death guardian (`guardianFenceAndReap`, same code): the disposition stays `cancelled`, and the finding appears in the result's `findings`;
- `run.verdict`'s success closeout (`accountCompletionLaunches`): the verdict is unchanged; an accounted report's findings are surfaced without blocking;
- `run.start --resume` (`validateResumeQuiescence`): unchanged. It prints census findings only when it refuses, and a leftover never makes it refuse.

PASSED and FAILED drives stay settled by their verdict with no probe. A normal exit means `go run` exited after the runner, and the runner exited after its targets.

### Failure posture

No new halt and no new block. The check is read-only, and its only effect is an informational finding. A pinned, mutation-checked test asserts that a **leftover** never changes a census's settled result, a cancel disposition, or a closeout verdict. The glossary entry states that `tree-survives` is information, never a blocker, so neither a reviewer nor a skill escalates it into one.

## Consequences

- **Gap 2.** A runner stuck for more than 10s after a stop's TERM is killed by `Stop`'s escalation, and its targets keep running in their own groups. The leftover check cannot see them.
- **Gap 3, until 0493 lands.** Finalize's relaunch keeps trusting a dead supervisor exactly as today. 0493 retires the relaunch.
- **Gap 4.** After a graceful stop, the worktree frees while the runner finishes stopping its targets: milliseconds normally, up to about 5s for a target that ignores TERM. `Stop`'s own caller already waits.
- **Docket never stops a leftover.** The finding names the group; a human stops it. The glossary gives the check and the command. A new gate in the same worktree can still start beside a leftover, as today.
- **Unclear means today's behavior.** A supervisor that dies while its launching process is still alive stays a zombie (fact 8), so the check answers **unclear** and the census reports `run-terminal`, as today.
- **A reused number can fool the check.** If a populated group whose leader is dead is not the run's own (fact 7 makes this improbable), the cost is a wrong note, never a signal.

## Alternatives considered

- Stop a leftover group in cancel. Rejected: the group's supervisor is dead, so no lock proves ownership, and a cancel can run hours after the death, when a reused number (fact 7) could make it signal an unrelated group. It would also carve an exception out of ADR-0095.
- Make cancel wait (`cancellation-pending`) until a leftover drains. Rejected: it blocks a resume for a suite runtime, a new block path.
- Make the supervisor wait for its whole tree before it records a terminal (a stdout pipe the runner holds, or the child in its own recorded group). Rejected: it fixes only gap 4, the smallest, and reopens 0490's rejected tree-held lock, where a leaked descendant pins the worktree.
- Have finalize's relaunch wait out a leftover. Dropped: 0493 retires the relaunch.
- Defer. Rejected: cancel reports `cancelled` with no sign of a suite that is still running.

## Update (2026-10-04)

Change 0497 extends where this rule reports, not what it decides. The supervisor-death halt now also reports the informational `tree-survives:<drive>:<pgid>` finding, computed through the existing `ProbeLeftover`, and only when the probe answers a clear `leftover`. The halt needs its own report because finalize removes a halted run's root in the same call that reports the halt, and the launch census never attributes finalize drives, so no later reader would see the surviving group. The rule is unchanged: report, never signal. An `unclear` answer keeps today's behavior and adds no finding.

Refusing the next gate start while a leftover still runs was considered and rejected, for the same reason this ADR rejected making cancel wait: it would add a new block path that holds a resume for the length of a suite run.

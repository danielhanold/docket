---
id: 492
slug: 'suite-teardown-can-outlive-its-supervisor'
title: 'Suite teardown can outlive its supervisor'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [490]
stacked_on:
related: [375, 491]
discovered_from: [490]
adrs: [95]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

Grooming 0490 traced how completely a gate's test suite is torn down and found four places where docket treats a suite as gone too early. None of them comes from the worktree slot, and 0490's worktree lock doesn't fix them; its ADR records them as known limitations.

The gate's process tree is supervisor → `go run` → test runner → one test target per process group (the runner starts each target with `Setpgid`). The supervisor waits only for its direct child (`RunSupervisorFromEnv`, `cmd.Wait`).

1. **A supervisor that dies alone leaves its suite running.** After `kill -9` or a crash of the supervisor process, its lock frees at once and `Observe` reports `vanished`, while the child tree keeps running with ppid 1. After 0490 the worktree reads as free, so a new gate can start next to the orphaned suite.
2. **A KILL escalation leaves test targets running.** `process.Service.Stop` sends TERM, then KILL, to the supervisor's process group. KILL takes down the runner before it can stop its targets. The targets live in their own process groups, so they survive and run to completion, while `Stop` reports verified absence (it checks only the supervisor's group).
3. **The single relaunch trusts "vanished".** Before finalize's automatic relaunch, `proveNoTreeSurvives` returns true for `vanished` without any probe, so the replacement can start beside the first run's still-running runner. The code contradicts itself: `stopProvesTeardown` says a signaled group "may still hold descendants", and `incumbent.go` says "a free lock alone ('vanished') is never sufficient".
4. **On a graceful stop the worktree frees before teardown ends.** `go run` exits as soon as it gets TERM, so the supervisor records a terminal status and lets go while the runner spends up to about 5s forwarding TERM to its targets and then killing them.

Experiments on macOS (recorded in the 0490 groom) reproduced 1 and 2. 3 and 4 come from reading the code.

## What changes

Make "the suite is gone" mean the whole test tree, not just the supervisor. These are hypotheses for grooming to evaluate, not decisions:

- **Relaunch:** before relaunching, `proveNoTreeSurvives` probes the old supervisor's process group, reusing `ClassifyRun`'s group check rather than adding a new predicate. It refuses or stops on a live group.
- **Stop:** the escalation path also reaches the runner's target groups, or the runner reaps its targets before it exits on TERM.
- **Graceful stop:** remove the `go run` indirection from the supervised command, or have the supervisor wait until its group drains before it records a terminal status.
- **Supervisor-only death:** decide whether anything short of a subreaper or a tree-held lock is worth it. 0490's groom rejected having the whole tree inherit the worktree lock, because a leaked or daemonized process would pin the worktree and nothing in docket could free it.

Any new check states its failure posture up front, and prefers making the problem visible over halting a run.

## Out of scope

- The worktree lock and its holder model (0490).
- The run id and its fences (0491).
- Process leaks inside individual tests (`t.Cleanup` hygiene), except where a fix here depends on them.

## Open questions

### Item 3 goes away if 0493 retires the relaunch

The 2026-10-02 backlog review retargeted 0493: instead of fixing cancel's accounting for finalize's single automatic relaunch, retire the relaunch (Daniel's decision). If that lands, item 3 (`proveNoTreeSurvives` trusting `vanished` before a relaunch) has no caller left, so drop it here. Items 1, 2 and 4 are unaffected.

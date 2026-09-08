---
id: 412
slug: 'forked-implement-next-build-agent-still-backgrounds-the-gate'
title: 'Forked implement-next/build agent still backgrounds the gate driver and yields (recurring suite-gate yield-wedge)'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-08'
updated: '2026-09-08'
depends_on: []
stacked_on:
related: [405, 345, 342, 359]
discovered_from: [349]
adrs: [24]
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
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

The forked-agent suite-gate yield-wedge recurred again during the change-0349 implement-next run, despite a long line of merged fixes that each targeted exactly this failure. In the 0349 run the forked docket-implement-next agent, on hitting the full-suite gate, THREE times launched `docket gate drive advance` as a backgrounded shell loop and then STOPPED/YIELDED ('waiting for its completion notification' / 'the advance loop continues in the background') instead of driving the driver inline via blocking synchronous slices. Each time it emitted a task-notification that reads as `completed` while the suite gate was still in flight, forcing the parent session to manually poll the loop's terminal-output file (observed drive-ids 0c2b56a29c5c590367e34e840f27f57a and 213dcc1fada6ac6a10adf66e638d4f16, run_roots under scratchpad/gate-final2 and gate-final3) to recover the true PASSED/FAILED disposition. On the final iteration the agent stopped AFTER the gate reached PASSED but BEFORE pushing the branch and opening the PR, leaving change 0349 at not-implemented / remote-head-mismatch / pr-unverified with a complete-but-unpushed build in the worktree; it was recovered only by arming a fresh `run.gate-before --resume` gate and re-dispatching to finish push -> PR. This is the exact architectural deadlock that ADR-0024 (a forked agent has no channel to receive a completion notification) and the following merged changes already addressed: 0223 (stated the gate execution posture in docket-build), 0232 (propagated it to the docket-build-task workers), 0271 (runner-delegation launch-and-observe posture), 0342 (added the native Go gate driver above the raw supervisor, whose contract is to drive via short synchronous WAITING/PASSED/FAILED/HALTED slices with a durable handoff, NOT a background-and-yield), and 0359 (run-gate gives up too soon). It was previously observed on 0315/0334/0337/0339/0341; 0349 adds another recurrence, which is evidence the current mitigations are still not holding at the forked-agent behavior layer. A likely contributing cause seen in this same run: the `gate.drive.prepare-scope` recovery scopes were non-resolvable throughout, so drives fell back to keyless `--owner task` mode — the same prepare-scope <-> start seam that change 0405 is investigating; if the inline-slice driving path is unavailable under that handshake failure, the agent is nudged back toward the background loop.

## What changes

Root-cause WHY a forked implement-next/build agent still backgrounds the gate driver and yields despite change 0342's synchronous-slice contract, and close the gap so the yield-wedge stops recurring. Determine which of these is operative (they are not mutually exclusive): (a) the agent/wrapper behavior does not actually drive the driver via blocking synchronous `gate drive advance` slices and instead spawns a background loop + yields; (b) the `gate.drive.prepare-scope` -> `gate.drive.start` handshake failure (change 0405) denies the intended inline drive, so the agent falls back to a background keyless loop; (c) a contract gap where, even after the gate reaches PASSED, the terminal push -> PR steps are not reached in the same foreground continuation and the run stops short. Then fix accordingly so a forked run drives the suite gate to a terminal disposition inline and continues through push -> open-PR without yielding, or fails closed with an explicit durable handoff a human/parent can resume — never a `completed` notification over an in-flight or stopped-short run. Add a regression guard that reddens if a forked build/implement path backgrounds the gate loop and yields, and/or if a run reports completion while the gate driver is still WAITING or the branch is unpushed.

## Out of scope

Change 0405's specific prepare-scope <-> start handshake root-cause investigation (related, and a candidate contributing cause, but tracked as its own change). Reworking the run-gate attribution model or the driver's disposition vocabulary beyond what this root cause requires. The pre-existing internal/app wall-clock finalize-timeout blocker and other unrelated suite-timing findings.

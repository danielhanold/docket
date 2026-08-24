---
id: 342
slug: 'harden-autonomous-build-implement-agents-against-the-suite-y'
title: 'Harden autonomous build/implement agents against the suite-yield deadlock (ADR-0024)'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-08-24'
updated: '2026-08-24'
depends_on: []
stacked_on:
related: []
discovered_from: [339]
adrs: [24]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | `docs/adrs/0024-claude-context-fork-skill-dispatch.md` |
<!-- docket:artifacts:end -->

## Why

Change 0339's autonomous build repeatedly wedged, needing ~6 human-driven resumes to make progress that a normal change lands unattended. Postmortem: the failure is the ADR-0024 subagent yield-loop deadlock. A `docket-implement-next` / build agent backgrounds the ~10-minute test suite and *yields* to await a completion notification, but a forked subagent has no channel to receive its own notification, so it re-enters the same wait forever and reports 'still waiting' with no new commits.

What made 0339 uniquely hit it, where other changes do not: (1) the suite is now ~10 min / 122 files, and long suites are precisely what pushes an agent to background-and-yield instead of blocking foreground; (2) an early monitor loop `until ! pgrep -f "run-tests.sh"` matched its OWN command line, so the wait could never terminate even after the suite finished — a transient wait turned into a hard wedge; (3) repeated `/docket-implement-next 339` slash re-launches each spawned another orchestrator + monitor + build worker, amplifying one wedge into a tangle; (4) the run re-wedged separately at the finish step and never pushed the branch or opened the PR.

What broke it loose: verifying against git each cycle (never trusting a 'completed' report), and a direct SendMessage to the file-owning build worker to commit its own green work synchronously, bypassing the parents' yield-await.

## What changes

Two layers. (a) Immediate guard: any liveness/monitor pattern an agent writes must not be able to self-match (anchor as `[r]un-tests.sh`, or watch the runner PID directly) — codify this in the build/gate-caller guidance and, where possible, a lint/guard. (b) Root fix (ADR-0024): make the autonomous build/implement agents block ACTIVELY on a child's return rather than background-a-suite-and-yield — e.g. the build role runs its per-task suite gate synchronously and commits-then-returns per task, and the orchestrator never yields to await a task-notification it cannot receive. Include the finish step (evidence/PR publish) so a completed build reliably reaches an open PR without human nudging.

## Out of scope

Rewriting the native gate supervisor (`internal/process`); the 0339 build itself (already completed and reviewed); changing how the main session receives notifications (only forked subagents have the ADR-0024 limitation).

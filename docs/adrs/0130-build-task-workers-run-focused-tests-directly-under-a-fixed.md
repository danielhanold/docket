---
id: 130
slug: 'build-task-workers-run-focused-tests-directly-under-a-fixed'
title: 'Build-task workers run focused tests directly under a fixed time limit; the gate driver serves only full-suite gates'
status: 'Accepted'
date: '2026-10-02'
supersedes: [117]
reverses: []
relates_to: [107, 24]
change: 488
---

## Context

Change 0359 (ADR-0107) and change 0405 (ADR-0117) put every build-task worker test on the gate driver inside recovery scopes (identity bundle, predecessor receipts, acknowledge, WAITING handoff, parent takeover). That protocol became the main cause of stuck builds: about 25 refusal modes, several permanent (predecessor-not-reusable after a HALTED drive, scope-busy after a failed launch, scope-identity-mismatch on path spelling, stale-run-id). Roughly two-thirds of 27 run halts since 2026-09-09 trace to gate ownership machinery.

Nothing reads a task drive; only skill prose enforces it; the full-suite build gate re-runs everything anyway; and 0359's acceptance probes for worker handoff and takeover were never run. The motivating incident (0333) was a roughly 4-minute package run treated as a focused test, which a time limit addresses directly. Fix-loop workers were already dispatched without a scope.

## Decision

Build-task workers run every test directly in the feature worktree, in the foreground, as `timeout --kill-after=10s 10m <cmd>` (falling back to `gtimeout`; if neither exists the worker returns BLOCKED).

The verdict comes from exit status: 0 is green; 124 or 137 means the limit was hit (not red); 125, 126, or 127 means the test could not run (not red); any other non-zero status is red.

Workers never run the full suite and call no gate operation; their outcomes are COMPLETE, NEEDS_ESCALATION, or BLOCKED. The build controller runs every full-suite attempt itself, including after a repair worker's fix.

A non-blocking time-limit audit records missing wrappers as informational results lines, never a halt.

The run id leaves worker prompts. implement-next keeps it on build-owned starts because run.cancel depends on it (until change 0491).

ADR-0107's recovery scopes and parent takeover stop applying between the build controller and its workers, but still apply between the coordinator and implement-next (precedent: ADR-0126 narrowing ADR-0101/0106 via relates_to). ADR-0024's never-yield rule is unchanged.

## Consequences

A static repoguard pin keeps the rule in the worker contract; the audit reports honest omissions only.

Accepted losses: nothing can take over a still-running focused test after its worker died; there is no durable record of worker test runs (nothing read one); GNU coreutils becomes a macOS prerequisite.

The task-drive Go machinery stays in place but unused until change 0489 deletes it.

## Alternatives considered

- Use the driver for task tests without scopes: still exposed to the worktree slot, the run fence, and the 30s slice collision of 0412.
- 0359's earlier rule of using the driver only when a test looks long: a prediction, replaced here by a fixed limit.
- Have the repair worker drive or hand off the full suite: keeps the driver and WAITING at worker level.
- A configurable limit: nothing enforces it.
- A docket-owned bounded runner: puts docket back in the test path.
- Hard enforcement that halts on a missing wrapper: introduces a new stuck mode.
- Harness hooks: work in one harness only and cannot tell tests from other commands.

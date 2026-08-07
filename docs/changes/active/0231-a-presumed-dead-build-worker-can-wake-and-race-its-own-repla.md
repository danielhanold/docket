---
id: 231
slug: a-presumed-dead-build-worker-can-wake-and-race-its-own-repla
title: A presumed-dead build worker can wake and race its own replacement in one worktree
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [223]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A build worker that stalls is indistinguishable from a build worker that is slow, and docket has no
protocol for resolving the difference. On change 0223 this produced a real double-write.

Task 5's worker backgrounded the full suite and yielded to await a completion event that never
reaches a subagent. After ~15 minutes with no commit, the controller followed the only available
posture — treat it as failed, discard its uncommitted tree, re-dispatch fresh. The original worker
was not actually stopped. It woke, wrote its own citation and its own assert group into the same two
files the replacement worker was editing, and committed (`1be89816`), sweeping the replacement's
appended block in: the test file shipped with assert group (8) duplicated, seven asserts repeated
under identical descriptions. It then amended (`e1e64002`) to de-duplicate. The net state happened
to be correct; nothing in the system made that likely.

Three separate hazards compose here:

- **No liveness check before discard.** `docket-build`'s halting conditions and
  `docket-implement-next`'s fix loop both assume a returned worker is a finished worker. Neither has
  a rule for "presumed dead, actually running."
- **No worktree write lease.** Two workers sharing one worktree is forbidden by prose
  ("never dispatch two workers concurrently") and by nothing else. The prose binds the controller
  that dispatches deliberately; it does not bind a controller that dispatches believing the first
  worker is gone.
- **A resumed worker may amend.** The amend rewrote a commit another agent's work was already
  inside. `docket-build` forbids escalating onto a stray commit but says nothing about a worker
  amending a commit it authored after a rival has written to the same files.

## What changes

Decide the protocol for a non-responsive worker. Candidates, not conclusions: a liveness probe the
controller must pass before discarding; a lease file in the worktree that a second worker refuses to
write over; making discard-and-redispatch illegal outright, so a stalled worker is always a halt for
a human rather than a race the controller opens itself.

## Out of scope

- The yield defect that caused the stall — change 0223 fixed the gate execution posture that
  produced it. This stub is about what the controller does when a worker stalls for any reason.

## Open questions

- Is a liveness probe even available to a controller, or is the honest fix to make presumed-dead a
  halt condition?
- Does the same hazard reach the `docket-rebase-resolver` / `docket-integration-repair` dispatches,
  which also act in a shared feature worktree?

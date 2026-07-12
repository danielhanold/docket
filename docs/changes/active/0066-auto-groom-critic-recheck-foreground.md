---
id: 66
slug: auto-groom-critic-recheck-foreground
title: Auto-groom's critic re-check must be foreground — a forked skill that yields returns a half-done run to its caller
status: proposed
priority: high
created: 2026-07-12
updated: 2026-07-12
depends_on: []
related: [17, 61, 65]
adrs:
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
issue:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Observed live on 2026-07-12 grooming change #0065. `docket-auto-groom` (running as a fork) drafted the spec, dispatched its critic, got back a *wrong but fixable* verdict, and entered the bounded revision round. It then **dispatched the critic's re-check as a background task and yielded**, returning to its caller with:

> `Holding for the critic's re-check verdict — the monitor will notify me the moment it lands.`

To the parent, the Skill tool reported `completed (forked execution)` — the run looked **finished**. It was not: the agent was alive and mid-step-5, with a 22KB spec sitting **uncommitted and unlinked** in the shared `.docket` worktree and the stub still reading needs-brainstorm.

The parent then did the natural thing — inspected the "aftermath", concluded the fork had crashed, and committed and pushed the agent's working-tree files itself. The groom later woke, found its own work already committed under someone else's messages, and reported it as a concurrent loop racing it. Nothing was lost this time (the landed content was byte-for-byte the intended output), but that was luck, not design: two writers were live in one worktree, each believing the other was not.

**Two independent defects, and both are load-bearing for autonomous operation:**

1. **The skill's foreground rule doesn't cover the re-check.** SKILL.md §5 (line 44) explicitly qualifies the *initial* critic dispatch as **foreground**, then describes the revision round — "the critic re-checks only the revised items" — with **no foreground qualifier**. The agent read the gap literally and backgrounded the re-check. The convention's composition rule (dispatches are foreground; the parent suspends until the child returns) was never restated for the second round.

2. **A forked skill has no channel to receive a notification, so "wait for a notification" is a yield.** A fork cannot be resumed by the task-notification it is waiting on the way a main-loop session can — waiting on one hands control back to the caller. This is the same family as the known SDD hazard (an implementer that backgrounds the long test suite stalls the loop), and it generalizes beyond auto-groom: **any forked skill that awaits a background child returns a half-done run to a caller that will believe it finished.**

Left alone, an overnight drain silently hands build-ready-looking stubs (or uncommitted specs) to whatever runs next.

## What changes

- **Close the wording gap in `docket-auto-groom`**: make the critic's **re-check** round explicitly foreground, in the same sentence that bounds the revision round — so no reader (human or agent) can take the qualifier as applying only to the first dispatch.
- **State the general rule where it binds every skill, not just this one**: a forked/dispatched skill must complete its own work before returning — it may never background a child and wait on a notification, because it cannot receive one. Natural home is `docket-convention`'s *Composition* paragraph (which already declares dispatches foreground) and/or the autonomous wrappers' abort-and-report rule. Blocking on a child is fine; **yielding** to wait for one is not.
- **Consider a caller-side guard**: the hazard is only dangerous because a premature return is indistinguishable from a real completion. Whether docket can detect this (e.g. a groom that returns without either a linked spec or an abstain section is an incomplete run, not a success) is a design question for the spec.

## Out of scope

- Changing how the critic gate works, its verdict vocabulary, or the one-bounded-revision-round rule — the gate behaved correctly; only its *dispatch mechanics* are at fault.
- Removing `context: fork` from auto-groom or any other skill (0061's fork-exclusion principle stands; see #0065).
- Concurrency control / locking for the shared `.docket` worktree. The race here was a *consequence* of the premature return, not an independent defect. If the spec concludes locking is the real fix, that is a separate change.

## Open questions

- Does the same wording gap exist in the other composing skills — `docket-implement-next` (dispatches `docket-status` and `docket-adr`) and `docket-finalize-change` (dispatches the rebase-resolver and integration-repair)? Their dispatches are single-shot, so there is no "second round" to under-qualify, but the general never-yield rule should be checked against each.
- Should the caller-side guard be advisory (warn) or hard (treat a spec-less, abstain-less groom return as a failed run)?

## Reconcile log

---
id: 572
slug: 'let-a-finalize-loop-carry-on-past-a-blocked-change'
title: 'Let a finalize loop carry on past a blocked change'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-19'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [570]
discovered_from: [570]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`/loop docket-finalize-change` stops at the first change it cannot merge: a conflict the resolver cannot settle, a suite that stays red after rebase, a failed merge condition. The other changes waiting to merge sit until a person fixes the blocked one or lands them by naming their ids.

Carrying on is not as simple as on the implement side (#570). A finalize block writes `## Finalize blocked`, a visible note that never removes the change from finalize selection — by design, so the next run retries it. Selection always takes the first eligible change, so a loop that carried on would pick the same blocked change again and never reach the next one.

## What changes

Let a finalize loop move past a change it could not merge and keep landing the rest, while still stopping when blocks keep coming.

To settle when grooming:

- How a loop skips a blocked change while today's "the next run retries it" behaviour still holds for a person running finalize by hand.
- Whether to reuse #570's halt count and reset command, or give finalize its own; finalize runs are not bracketed by the run tracker today.
- Which finalize blocks concern only one change and which would hit every change (an unresolved base, a missing gate agent, unsupported config).
- What the stop rule in `docs/guide/landing-changes.md` says afterwards.

## Out of scope

The implement-side loop and its halt count: #570 owns those. Running two finalize loops at once: docket supports one loop per stage.

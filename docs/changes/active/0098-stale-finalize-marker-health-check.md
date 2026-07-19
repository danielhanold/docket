---
id: 98
slug: stale-finalize-marker-health-check
title: Health check for a stale `## Finalize blocked` marker
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87]
discovered_from: [87]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Change 0087 added the `## Finalize blocked` marker and its board cell, but `scripts/board-checks.sh`
gained nothing. A marker whose underlying cause the human fixed — without merging, and without
re-running finalize with the change id named — sits on the board indefinitely with no advisory.

The marker's clearing rule fires on the paths finalize itself drives. It does not fire when the
human resolves the cause out of band, which is exactly the case a health check exists to catch.
`merge-gate-stall` is the obvious precedent: same shape (a state that is legitimate briefly and
suspicious once it persists), same surface.

## What changes

Add a health check that flags a `## Finalize blocked` marker which has outlived its plausible
lifetime, and surface it through the same needs-you channel `merge-gate-stall` uses.

The design question is what "stale" means here. `merge-gate-stall` keys on elapsed time, but a
marker has a stronger signal available: whether the condition that produced it still holds (the PR
is still unmerged, still unapproved, still conflicting). A check that re-probes the cause would be
precise where a timer is only a heuristic — at the cost of doing real work per marked change.

## Out of scope

- Auto-clearing the marker. A health check advises; it does not mutate change files.
- Revisiting the clearing rule itself (tracked separately as the wording follow-up).

## Open questions

- Time-based (mirroring `merge-gate-stall`) or cause-re-probing? If time-based, what threshold?
- Should the check distinguish "cause resolved, marker stale" from "cause still holds, genuinely
  blocked"? The second is not stale and should probably stay quiet.

## Reconcile log

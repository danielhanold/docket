---
id: 115
slug: extend-the-board-row-dropped-invariant-to-archive-files
title: Extend the board-row-dropped invariant to archive/ files
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [104]
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

Change 0104 added `board-row-dropped`, a computed invariant catching an `active/` change file that
`render-board.sh` counts in the board's total but renders in no section. The check is deliberately
bounded to `active/`. The **symmetric archive-side violation is real and currently undetected**, and
0104's whole-branch review reproduced it:

An `archive/` file carrying a **non-terminal** status (e.g. `implemented`) is counted in `total` and
rendered nowhere. The archive block is gated on `ndone + nkilled > 0` and its summary count comes
from `ARC_COUNT`, which such a file never joins. Rendered against one healthy active change, the
board reads `**2 changes**` above a single row, with zero mentions of the dropped id — the exact
count-vs-tables disagreement change 0104 exists to eliminate.

A second flavor: when a real `done` file coexists, the misfiled row *does* print, but under a
`<summary>` whose count excludes it. Count and tables still disagree.

**This is reachable by the same interrupted operation as the active-side case.** `archive-change.sh`
performs its `git mv` (step 2) *before* the status flip (step 3) and the commit (step 4), and each
of those can `die`. A failure in between leaves the file moved but not re-statused — precisely this
state. It is the mirror image of the `sweep-failed <id> archive <reason>` path that motivated
0104's active-side terminal-status trigger.

## What changes

Extend the drop invariant to `archive/`, or generalize it so both directories are covered by one
predicate rather than two.

The natural shape mirrors what 0104 already built: `renders_row` currently answers "does
`render-board.sh` emit a row for this `active/` file", derived from the renderer's real bucketing
(`int_field id` non-empty AND status ∈ `DOCKET_STATUSES_ACTIVE`). The archive side needs the
corresponding predicate over the archive pass — status ∈ `DOCKET_STATUSES_TERMINAL` and whatever
`ARC_COUNT`/the archive table actually gate on. Read the renderer and derive it; do not infer the
predicate from this stub.

Open design question worth settling first: is this one check reporting a directory/status mismatch
in either direction, or two checks? A single "this file is in the wrong directory for its status"
finding may be the more useful diagnostic than a drop-shaped one, since the remedy is the same —
finish the interrupted archive move.

## Out of scope

- Making `archive-change.sh` atomic. Ordering its `git mv` after the status flip, or making the
  sequence transactional, is a separate concern from *detecting* the resulting state. Worth its own
  change if the failure proves common.
- The `active/`-side invariant, which 0104 already ships.
- Repairing any offending file. Like 0104, this makes the failure visible; it does not decide what
  the file's canonical location or status should be.

## Open questions

- Does the archive table have any *other* gate (the `ARCHIVE_RECENT` recency window, the per-month
  digest collapse) that legitimately drops a well-formed row, and would a naive predicate produce
  false positives on it?
- Should this fold into `board-row-dropped` as a widened scope, or land as its own check-id? A
  widened scope keeps one concept; a separate id keeps 0104's finding meaning exactly one thing.

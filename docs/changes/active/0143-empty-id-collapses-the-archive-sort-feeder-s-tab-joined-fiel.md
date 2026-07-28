---
id: 143
slug: empty-id-collapses-the-archive-sort-feeder-s-tab-joined-fiel
title: Empty id collapses the archive sort feeder's TAB-joined fields in render-board.sh
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [115]
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

`render-board.sh`'s archive sort feeder joins fields with TAB and reads them back with `IFS=$'\t'`. Because TAB is IFS-whitespace, an empty `id` field collapses rather than being preserved as empty, shifting every later field left. That defeats the renderer's own id guard and emits a corrupt archive row of the shape `| [0000](archive/) |  | <date> |`.

The active side is unaffected — its guard runs before the join.

Found during change 0115 (extending the `board-row-dropped` invariant to `archive/` files) and deliberately left unfixed there: `render-board.sh` is the oracle that 0115's check is validated against, so fixing both in one commit would have destroyed the independence the backstop depends on.

## Scope

Fix the field-shift in the archive sort feeder so an empty `id` cannot collapse, and pin it with a test. Keep the fix separate from `board-checks.sh` so the check and its oracle stay independent.

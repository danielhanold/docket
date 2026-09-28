---
id: 467
slug: 'document-run-epoch-in-the-docket-build-task-gate-drive-start'
title: 'Document --run-epoch in the docket-build-task gate.drive.start contract'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [461]
discovered_from: [461]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

During change 0461's implement-next run, a build-task worker's `gate.drive.start` was refused as `stale-run-epoch` (naming an incumbent epoch that was not the run's own) until the worker added `--run-epoch` on its own. The `docket-build-task` skill spells out the full `gate.drive.start` argv a worker must use and says the driver rejects a start that omits or alters any identity value, yet that argv has no `--run-epoch`, and `docket-build`'s start-ready scope bundle does not list the run epoch among the values it hands the worker. CLAUDE.md already requires threading the run epoch into every `--run-epoch` dispatch flag including `gate drive start`, so the worker contract is out of step with the coordinator contract: workers either get refused or have to improvise.

## What changes

Bring the build-task worker contract in line with the run-epoch requirement: `docket-build`'s start-ready scope bundle carries the run epoch to the worker, and `docket-build-task`'s `gate.drive.start` invocation passes it through unchanged alongside the other identity values. Add a prose-contract guard so the flag cannot silently drop out again, and regenerate the embedded skill copies.

## Out of scope

Changing the driver's run-epoch fencing or `stale-run-epoch` refusal semantics. Investigating why the refusal named a foreign incumbent epoch, unless grooming finds it is a defect rather than the expected consequence of omitting the flag.

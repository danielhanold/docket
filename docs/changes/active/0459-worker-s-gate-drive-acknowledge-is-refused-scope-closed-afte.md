---
id: 459
slug: 'worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte'
title: 'Worker''s gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: []
discovered_from: [458]
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

During change 0458's implement-next run, Task 2's worker ran a package-test drive that handed off as WAITING. The parent claimed the drive and advanced it to PASSED. The worker then made its planned commit (0c2534e1), but its `gate.drive.acknowledge` was refused with `scope-closed` because the claim had moved authority over the scope to the parent. The worker reported BLOCKED even though its work had landed and the tree was clean. A parent that trusts the worker's status could then treat a completed task as failed, re-dispatch it, or halt the run.

## What changes

Design question to settle at groom time: after a parent claims a WAITING drive, should (a) the worker still be allowed to close its own scope, or (b) the worker recognize `scope-closed` after a claim as a terminal success and return COMPLETE instead of BLOCKED? Either way, the continuation flow (gate drive claim / acknowledge, and the docket-build-task worker contract) must stop producing a BLOCKED status for work that completed. Add a regression test that reproduces the claim-then-acknowledge sequence.

## Out of scope

Other gate-drive handoff redesign beyond the claim/acknowledge interaction; the artifact.backlink absolute-path issue found in the same run (tracked separately).

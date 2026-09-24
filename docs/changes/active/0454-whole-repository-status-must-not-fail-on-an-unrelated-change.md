---
id: 454
slug: 'whole-repository-status-must-not-fail-on-an-unrelated-change'
title: 'Whole-repository status must not fail on an unrelated change''s invalid branch name'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [449]
discovered_from: [449]
adrs: [127]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0127](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0127-scoped-metadata-validation-for-named-operations.md) |
<!-- docket:artifacts:end -->

## Why

Change 0449 stopped one broken change record from blocking named operations on other changes. It left `docket status`, the whole-repository read, strict. `status` probes live branch facts for every change, so when any unrelated change records an invalid branch name the whole read fails with an external error. The one read a human or agent uses to see and repair a broken record then fails because of that same record. This is the last known way a single unrelated record can take down a shared read, and it is confirmed in 0449's tests (see its results file, "Known issues and follow-ups").

## What changes

Make `docket status` report an invalid branch name on one change as a finding against that change, and still render the rest of the backlog, readiness, selection, and health. The branch-fact probe should skip or isolate the defective record instead of aborting the whole read. The design needs to settle how the finding is shaped in status's JSON and human output, whether the affected change is excluded from build-ready selection, and how `repository check` and the health checks report it. This should stay consistent with 0449's repair-notice model (ADR-0127).

## Out of scope

Changing named-operation validation scoping, which 0449 already did. Repairing or auto-correcting the invalid branch name itself. Other whole-repository reads unless the design finds they share the same probe path.

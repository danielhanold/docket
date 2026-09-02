---
id: 398
slug: 'extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd'
title: 'Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: []
discovered_from: [373]
adrs: [108]
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
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

Change 0373 bounded total Go test load and added the internal/testsupport temp-dir fixture plus the repoguard guard (TestRealProcessPackagesUseFixtureTempDir), but deliberately scoped its derived package set to internal/. The cmd/ real-process test packages are unprotected: cmd/docket/gate_cli_test.go uses bare t.TempDir() and carries its own private gateTempDir drain-then-retry helper, duplicating logic the fixture now centralizes. These sites can reintroduce the parallel-load isolation flakes 0373 fixed, and the repoguard will not catch them.

## What changes

Extend the repoguard's real-process package derivation to include cmd/ test packages, convert their bare t.TempDir() call sites (starting with cmd/docket/gate_cli_test.go) to testsupport.TempDir(t), and remove the now-redundant private gateTempDir helper in favor of the shared fixture.

## Out of scope

The internal/ package set already covered by 0373; changing the fixture's semantics or the runner concurrency cap (ADR-0108).

---
id: 398
slug: 'extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd'
title: 'Extend the testsupport temp-dir fixture and repoguard to cmd/ real-process test packages'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-02'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [373]
discovered_from: [373]
adrs: [108]
spec: 'docs/superpowers/specs/2026-09-27-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd-design.md'
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
| Spec | [2026-09-27-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-extend-the-testsupport-temp-dir-fixture-and-repoguard-to-cmd-design.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

Change 0373 bounded total Go test load and added the internal/testsupport temp-dir fixture plus the repoguard guard (TestRealProcessPackagesUseFixtureTempDir), but deliberately scoped its derived package set to internal/. The cmd/ real-process test packages are unprotected: cmd/docket/gate_cli_test.go uses bare t.TempDir() and carries its own private gateTempDir drain-then-retry helper, duplicating logic the fixture now centralizes. These sites can reintroduce the parallel-load isolation flakes 0373 fixed, and the repoguard will not catch them.

## What changes

Widen the repoguard real-process derivation (`TestRealProcessPackagesUseFixtureTempDir`) from the `internal/`-only `scanRoot` to an explicit `internal` + `cmd` root list, with a `cmd/docket` population floor alongside the existing `internal/process` one. Convert every bare `t.TempDir()` in the derived `cmd/` real-process packages (`cmd/docket`, `cmd/releasepkg` — 12 sites across 5 files) to `testsupport.TempDir(t)`, and delete the private `gateTempDir` helper in `cmd/docket/gate_cli_test.go` in favor of the shared fixture. Mutation-test the widened guard. Detailed design in the linked spec.

## Out of scope

The internal/ package set already covered by 0373; changing the fixture's semantics or the runner concurrency cap (ADR-0108).

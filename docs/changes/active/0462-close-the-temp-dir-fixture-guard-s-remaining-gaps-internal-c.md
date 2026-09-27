---
id: 462
slug: 'close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c'
title: 'Close the temp-dir fixture guard''s remaining gaps (internal/cli gateTempDir, scan-root removal)'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-27'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [398]
discovered_from: [398]
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

Change 0398 extended the testsupport temp-dir fixture and the TestRealProcessPackagesUseFixtureTempDir guard to cmd/, and its build reported two gaps it deliberately left out of scope.

1. internal/cli/gate_test.go still defines its own private gateTempDir helper (the same shape 0398 deleted from cmd/docket/gate_cli_test.go). The guard misses it because the helper creates its directory with os.MkdirTemp rather than a bare t.TempDir(), so the guard's syntactic check never sees it.
2. The guard can still be defeated by removing both `cmd` from its scan roots and the cmd/docket non-vacuity check in the same edit. It takes two deliberate edits to one test, so 0398 accepted it as a residual gap, but it means the guard's coverage of cmd/ rests on nothing outside the test itself.

## What changes

- Replace internal/cli/gate_test.go's private gateTempDir helper with the shared testsupport.TempDir fixture and delete the helper.
- Widen the guard so private temp-dir helpers built on os.MkdirTemp (or a similar ad hoc temp-dir spelling) in real-process test packages are caught, keyed on syntactic shape rather than one helper name.
- Harden the guard's scan-root coverage so dropping a root (and its non-vacuity check) cannot silently narrow it — for example by deriving the roots from the repository layout instead of a hand-listed pair.
- Mutation-test each widening: restore the helper / drop a root and watch the guard redden.

## Out of scope

- Converting test temp dirs outside the real-process packages the guard already targets.
- Any change to the testsupport.TempDir fixture's own behavior.
- Re-opening 0398's already-converted cmd/ sites.

---
id: 462
slug: 'close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c'
title: 'Close the temp-dir fixture guard''s remaining gaps (internal/cli gateTempDir, scan-root removal)'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-27'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [398, 373]
discovered_from: [398]
adrs: []
spec: 'docs/superpowers/specs/2026-09-27-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-27T19:56:55Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0398 extended the testsupport temp-dir fixture and the TestRealProcessPackagesUseFixtureTempDir guard to cmd/, and its build reported two gaps it deliberately left out of scope.

1. internal/cli/gate_test.go still defines its own private gateTempDir helper (the same shape 0398 deleted from cmd/docket/gate_cli_test.go). The guard misses it because the helper creates its directory with os.MkdirTemp rather than a bare t.TempDir(), so the guard's syntactic check never sees it.
2. The guard can still be defeated by removing both `cmd` from its scan roots and the cmd/docket non-vacuity check in the same edit. It takes two deliberate edits to one test, so 0398 accepted it as a residual gap, but it means the guard's coverage of cmd/ rests on nothing outside the test itself.

## What changes

- Replace internal/cli/gate_test.go's private gateTempDir helper with the shared testsupport.TempDir fixture and delete the helper.
- Widen the guard to ban any executable `<ident>.MkdirTemp(` call in real-process test packages, unless the call carries an adjacent `// tempdir-exempt: <reason>` marker with a non-empty reason. Mark the legitimate existing sites (TestMain binary builds, process-lifetime sync.Once dirs, failure evidence, the /tmp-alias test, and a MkdirTemp nested in a fixture dir), each with a site-specific reason.
- Replace the guard's hand-listed scanRoots with the shared repoguard.MaintainedFiles whole-repo walk, keeping the realProcFloors population floors, so coverage can't be narrowed by one quiet edit to the guard.
- Mutation-test each widening (restore the helper, strip or blank a marker, add an unmarked helper elsewhere, skip cmd/) and watch the guard redden.

## Out of scope

- Converting test temp dirs outside the real-process packages the guard already targets.
- Any change to the testsupport.TempDir fixture's own behavior, including new lifetime modes for the exempt sites.
- Re-opening 0398's already-converted cmd/ sites.
- os.CreateTemp (temp files) and calls through interface values or shadowing helpers.

## Reconcile log

### 2026-09-27

2026-09-27 — Re-read against current main (86f14149). Both gaps are still present: internal/cli/gate_test.go still defines gateTempDir on os.MkdirTemp, and internal/repoguard/tempdir_fixture_test.go still walks a hand-listed scanRoots {internal, cmd}. No related or archived change has addressed either. Scope unchanged; the build re-derives the MkdirTemp exempt-site list by grep rather than trusting the spec's enumeration.

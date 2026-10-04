---
id: 517
slug: 'make-evidence-record-certify-a-finalize-re-test-with-the-fin'
title: 'Make evidence.record certify a finalize re-test with the finalize gate settings'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502, 360]
discovered_from: [502]
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

After an integration repair, finalize re-tests the repaired head through the gate driver (`--owner finalize`) using `finalize.test_command`. But `EvidenceRecord` in `internal/app/evidence_ops.go` reads only `build.*` settings: its comment says it records `build.test_command`, "never finalize.test_command". So after a passing finalize re-test it can (a) mint skipped evidence when `build.gate` is `off`, even though a real run passed; (b) name `build.test_command` instead of the command that actually ran; or (c) refuse with `unconfigured-gate-command` when `build.gate` is local and `build.test_command` is unset. The evidence then misstates what certified the repaired head. Found while building change 0502.

## What changes

Let `evidence.record` know it is certifying a finalize re-test and, in that case, read the finalize gate settings and record the command that actually ran. The build-gate path keeps its current behavior. Grooming decides how the caller signals the finalize case (for example an owner flag that matches the gate driver's `--owner finalize`) and updates the finalize skill text to pass it. Cover each of the three failure modes (a)-(c) with a test, and mutation-test them.

## Out of scope

The wider coordination-tax and evidence items bundled in change 0360 (session-scoped sync, accepting results-only deltas at `pr publish`, honoring the primary tree's `.docket.local.yml` from a feature worktree, auto-detecting test commands). This change touches only the finalize re-test evidence path.

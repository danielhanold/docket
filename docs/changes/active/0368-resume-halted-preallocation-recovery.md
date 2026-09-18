---
id: 368
slug: resume-halted-preallocation-recovery
title: Recover a run halted before its workspace was allocated
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-29
updated: '2026-09-18'
depends_on: []
stacked_on:
related: [313, 316, 318, 354, 366, 375, 429]
discovered_from: [318]
adrs: [34, 35, 118]
spec: 'docs/superpowers/specs/2026-09-18-resume-halted-preallocation-recovery-design.md'
plan: 'docs/superpowers/plans/2026-09-18-resume-halted-preallocation-recovery.md'
results:
trivial: false
auto_groomable:
branch: 'fix/resume-halted-preallocation-recovery'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-18T21:22:20Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-resume-halted-preallocation-recovery-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-resume-halted-preallocation-recovery-design.md) |
| Plan | [2026-09-18-resume-halted-preallocation-recovery.md](https://github.com/danielhanold/docket/blob/fix/resume-halted-preallocation-recovery/docs/superpowers/plans/2026-09-18-resume-halted-preallocation-recovery.md) |
| ADRs | [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0035](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0035-cleanup-teardown-fail-closed.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

A run halted during reconciliation, before feature-workspace allocation, leaves an in-progress change with a fresh claim and a durable halt marker but no workspace manifest. Workspace inspection currently collapses that absence into the foreign state, so the existing resume operation refuses it. Reclaim serves a different purpose and requires a strictly expired lease; a fresh halt should not have to wait for that expiry or allocate a workspace just to pass the resume check.

Change 0318 exposed this recovery gap. Tracing the current code confirms the lost absence distinction, while the allocation path already supplies branch, path, and registration probes that can be reused. Missing files alone are insufficient proof: leftover resources, foreign ownership, and failed probes must still block recovery.

## What changes

- Extend existing workspace inspection to distinguish a proven absent local workspace from foreign or ambiguous state, reusing the allocation inventory checks.
- Let the existing acknowledged, exact-version resume operation recover that state after proving the recorded remote feature branch is also absent. Preserve the same claim and recorded branch; leave workspace allocation to its ordinary later step.
- Keep reclaim expiry, ownership safeguards, run cancellation, and replacement-run admission intact. Carry the refined inspection result through existing consumers without granting cleanup or adoption authority.
- Add a real claim-to-halt-to-resume regression fixture with no prior workspace allocation, plus refusal coverage for leftover resources and failed probes.

The linked spec records the implementation trace, prior decisions, alternatives, and acceptance criteria.

## Out of scope

- Changing reclaim expiry or its configured default.
- New recovery commands or bypass flags; allocating, adopting, deleting, or resetting workspaces during resume.
- New persistent records or manifest format migrations.
- Changing recovery semantics for already-owned workspaces or the run gate cancellation, attribution, and replacement-admission protocols.
- Unrelated refactoring, implementation, or implementation planning during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-09-18

2026-09-18: Reconciled against current main (60d356ff). The spec examined commit 3ccf9fac; `git log 3ccf9fac..60d356ff` shows no commits touched internal/workspace/inspect.go, manifest.go, prepare.go, internal/app/change_halt.go, or change_reclaim.go, so the recovery trace is intact. Confirmed current code: inspect.go collapses manifestAbsent into StateForeign (line 87-88); manifest.go classifyManifest distinguishes manifestAbsent/Valid/Foreign/Unknown (line 205-214); change_halt.go resumeQuiescenceRefusal refuses StateForeign/StateMismatch (line 420). No StateAbsent exists yet. Scope, relations (related 313,316,318,354,366,375,429; discovered_from 318; adrs 34,35,118; depends_on []), and acceptance criteria remain valid as authored. No design invalidation; proceeding to build.

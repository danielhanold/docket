---
id: 391
slug: 'carry-skipped-build-evidence-through-the-pr-publish-path'
title: 'Carry skipped build-evidence through the PR publish path'
status: 'killed'
priority: 'medium'
type: 'feat'
created: '2026-09-01'
updated: '2026-09-03'
depends_on: [374]
stacked_on:
related: [374]
discovered_from: [374]
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

Change 374 made build and finalize gate configuration independent and taught implement-next verify plus the four publication gates to accept a build-gate-off `skipped` evidence record. But the PR-body build-evidence block and `docket pr publish` remain green-only: a repo running `build.gate: off` cannot publish its truthful `skipped` evidence through the PR path, even though the skipped record is already a first-class, accepted outcome everywhere else in the loop. This leaves a `build.gate: off` repo unable to open a PR whose evidence block honestly reflects that the build gate was intentionally off.

## What changes

Extend the PR-body build-evidence rendering and `docket pr publish` to carry a `skipped` (build-gate-off) evidence record end-to-end, so its marker-bounded block truthfully states the gate was off rather than requiring a green suite. Keep the existing green-evidence path unchanged.

## Out of scope

Any change to how skipped evidence is produced, recorded, or verified (already landed in 374); the four publication gates' acceptance of skipped records (already correct); non-PR surfaces.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go by change 0374 — internal/app/pr_publish.go accepts VerdictSkipped at exact head, covered by TestPRPublishAcceptsSkippedEvidenceAtExactHead.

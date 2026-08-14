---
id: 323
slug: docket-uninstall-and-version-tree-collection-for-the-go-inst
title: 'docket uninstall and version-tree collection for the Go installer'
status: proposed
priority: medium
type: feat
created: 2026-08-14
updated: 2026-08-14
depends_on: []
stacked_on:
related: []
discovered_from: [311]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — change 0311's deep review (important #4 follow-up): every `docket install` of a new asset set publishes another immutable tree under `<data-root>/versions/<asset-set-id>/`, and nothing ever removes superseded ones; there is also no uninstall operation at all.
**Opportunity** — a `docket uninstall` / version-tree collection operation: remove owned targets by ownership proof, then collect version trees no installed state references.
**Independent value** — bounded disk growth and a clean exit path for users; valuable regardless of how 0311's internals evolve, and a natural companion to the release flow (0317).
**Boundary** — collection only of trees unreferenced by `state/install.json`; target removal only under the existing ownership proofs; never touches unknown files; repo-local surfaces stay out of scope.
**Reason for deferral** — 0311's dir-sealing fix made trees removable, but a safe GC needs reference-counting against installed state and its own tests — a distinct deliverable the review explicitly named as follow-up rather than an in-branch fix.

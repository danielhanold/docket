---
id: 379
slug: 'reapply-sha256-source-revision-width-fix-isfullobjectid'
title: 'Re-apply the SHA-256 (64-hex) source-revision width fix to isFullObjectID'
status: 'in-progress'
priority: medium
type: fix
created: '2026-08-30'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [378]
discovered_from: [378]
adrs: []
spec: 'docs/superpowers/specs/2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/reapply-sha256-source-revision-width-fix-isfullobjectid'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-07T01:44:43Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md) |
<!-- docket:artifacts:end -->

## Why

Metadata ownership verification rejects a valid migration receipt when its source revision is a full 64-character SHA-256 Git object ID. The local syntax check accepts only the 40-character SHA-1 width, even though its downstream Git reader accepts both. This produces a false foreign-metadata verdict before the existing ancestry check can run. Change #378's reverted correction is available as implementation reference; the defect remains in current main.

## What changes

Accept exactly 40 or 64 lowercase-hex characters in `isFullObjectID`, preserving strict character validation and all later ownership checks. Update the helper comment and add focused unit coverage for valid widths, malformed inputs, and length boundaries. Prove the width regression and character guard with mutation checks, format edited Go files, and pass the configured full build suite. The linked spec defines the complete behavior and verification contract.

## Out of scope

- Any broader object-id abstraction or hash-algorithm plumbing beyond the width check.
- The other 0378 follow-ups (descendant-receipt fixture; internal/process flake) — separate changes.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

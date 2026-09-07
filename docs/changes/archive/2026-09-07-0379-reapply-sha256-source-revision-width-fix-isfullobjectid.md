---
id: 379
slug: 'reapply-sha256-source-revision-width-fix-isfullobjectid'
title: 'Re-apply the SHA-256 (64-hex) source-revision width fix to isFullObjectID'
status: 'done'
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
plan: 'docs/superpowers/plans/2026-09-06-reapply-sha256-source-revision-width-fix-isfullobjectid.md'
results:
trivial: false
auto_groomable:
branch: 'fix/reapply-sha256-source-revision-width-fix-isfullobjectid'
pr: 'https://github.com/danielhanold/docket/pull/282'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md) |
| Plan | [2026-09-06-reapply-sha256-source-revision-width-fix-isfullobjectid.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-06-reapply-sha256-source-revision-width-fix-isfullobjectid.md) |
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

### 2026-09-07

2026-09-06 — Reconciled against current main (effc9a6d). Confirmed the defect is still present and unfixed: internal/app/metadata_ownership.go isFullObjectID still gates on len==40 only, so a 64-hex SHA-256 source revision yields a false RootForeign verdict before the ancestry check runs. Downstream internal/gitcli validateObjectID already accepts 40 or 64 lowercase hex — the width mismatch is app-side. internal/app/metadata_ownership_test.go does not yet exist, so the focused table-driven coverage will be created new. related/discovered_from [378] is done; no dependencies, no stack parent, no ADR needed. Scope, boundaries, and verification contract from the spec remain accurate and unchanged.

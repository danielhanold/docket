---
id: 379
slug: 'reapply-sha256-source-revision-width-fix-isfullobjectid'
title: 'Re-apply the SHA-256 (64-hex) source-revision width fix to isFullObjectID'
status: proposed
priority: medium
type: fix
created: '2026-08-30'
updated: '2026-08-30'
depends_on: []
stacked_on:
related: [378]
discovered_from: [378]
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

`isFullObjectID` accepts only the SHA-1 40-hex object-id width, so on a SHA-256 repository a
full 64-hex object id is misjudged as not-full. This is a real, fail-safe correctness gap: the
narrow width makes the check reject a legitimate full id rather than accept a bad one, but it
still mis-classifies ids on any SHA-256 repo. The fix was authored during change 0378's review
(the "important" finding) but was reverted along with the other non-blocker fixes when the
fix-loop's post-fix suite gate reddened on an unrelated `gofmt` nit. It must be re-applied
deliberately. The reverted commit remains in change 0378's branch history for cherry-pick, needing
only a one-character `gofmt` fix.

## What changes

Widen `isFullObjectID` to accept both the SHA-1 (40-hex) and SHA-256 (64-hex) full object-id
widths, with a focused test covering the SHA-256 case. Cherry-pick the reverted 0378 commit as the
starting point and correct the `gofmt` alignment that tripped the original gate.

## Out of scope

- Any broader object-id abstraction or hash-algorithm plumbing beyond the width check.
- The other 0378 follow-ups (descendant-receipt fixture; internal/process flake) — separate changes.

## Open questions

<!-- None yet — resolve the exact call sites and test placement during reconcile. -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

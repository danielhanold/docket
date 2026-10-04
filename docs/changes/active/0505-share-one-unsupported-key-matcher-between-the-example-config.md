---
id: 505
slug: 'share-one-unsupported-key-matcher-between-the-example-config'
title: 'Share one unsupported-key matcher between the example-config test and the docs guard'
status: 'proposed'
priority: 'low'
type: 'refactor'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [464]
discovered_from: [464]
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

Change 0464 added two guards that each detect unsupported configuration keys: `internal/config/example_correspondence_test.go` and the living-docs guard in `internal/repoguard/docs_alignment_test.go`. An import cycle kept them from sharing code, so the matcher exists twice and must be kept in sync by hand. Neither copy catches a key inside a comment nested in another comment.

## What changes

Stub, a hypothesis for grooming: move the matcher into a package both tests can import without the cycle, keep one copy, and decide whether the nested-comment case matters. Trace the import cycle first.

## Out of scope

Changing which keys count as unsupported, or what the two guards check.

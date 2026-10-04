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
spec: 'docs/superpowers/specs/2026-10-04-share-one-unsupported-key-matcher-between-the-example-config-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-share-one-unsupported-key-matcher-between-the-example-config-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-share-one-unsupported-key-matcher-between-the-example-config-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0464 added two guards that each detect unsupported configuration keys: `internal/config/example_correspondence_test.go` and the living-docs guard in `internal/repoguard/docs_alignment_test.go`. An import cycle kept them from sharing code, so the matcher exists twice and must be kept in sync by hand. Neither copy catches a key inside a comment nested in another comment.

## What changes

Move the duplicated unsupported-key matcher into one exported function in `internal/config`, next to the key registry it reads, and point both guards at it. No new package. A separate package would hit the same import cycle, because config's own test cannot import a package that imports config.

While there, close the shared gap: a key commented out inside an already-commented block (`#   # terminal_publish: true`) is matched from now on. No file in the example config or living docs newly fails today.

Design: the linked spec.

## Out of scope

Changing which keys count as unsupported, what else the two guards check (citations, the structural extractor, refused values of supported keys), or the schema registry.

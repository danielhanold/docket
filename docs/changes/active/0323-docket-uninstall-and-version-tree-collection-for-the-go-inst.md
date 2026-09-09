---
id: 323
slug: docket-uninstall-and-version-tree-collection-for-the-go-inst
title: 'docket uninstall and version-tree collection for the Go installer'
status: 'in-progress'
priority: medium
type: feat
created: 2026-08-14
updated: '2026-09-09'
depends_on: []
stacked_on:
related: [311, 317, 322, 351]
discovered_from: [311]
adrs: [96, 110]
spec: 'docs/superpowers/specs/2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/docket-uninstall-and-version-tree-collection-for-the-go-inst'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-09T19:30:31Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md) |
| ADRs | [ADR-0096](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md), [ADR-0110](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0110-install-path-configuration-reads-tolerate-unknown-keys-the-s.md) |
<!-- docket:artifacts:end -->

## Why

Release installs accumulate immutable asset versions, and users have no command to remove Docket's user-level harness integrations. Change 0311's deep review identified uninstall and safe version collection as an independent follow-up. Provide a predictable removal path and reclaim verified asset trees that no remaining installation references, including installations whose harnesses use different versions after scoped upgrades.

## What changes

- Add `docket uninstall` for all recorded harness integrations or an explicit harness selection, with ownership checks, dry-run output, and journaled recovery.
- Add `docket install collect` and automatically collect unused versions after successful release installs, development installs, and uninstalls.
- Derive references from the complete installed state and recorded links, verify current and legacy version-tree contents, and preserve unprovable targets.
- Make interrupted collection resumable and report cleanup failures separately from successful installation.
- Preserve the CLI and its ownership records, and support reinstalling from a valid empty-harness state.

## Out of scope

Removing development or release CLI binaries; deleting global configuration or source checkouts; changing repository-local instruction surfaces, metadata, or ownership records; force/purge switches; scanning the user's home or repositories for arbitrary references; and background scheduling. The release downloader's separate binary ownership lifecycle remains unchanged.

## Reconcile log

### 2026-09-09

2026-09-09 — Reconciled against origin/main at 881d7cfb. Changes 0311, 0317, 0322, and 0351 are merged and archived; the Go installer, release/development installation, ownership records, legacy reproducer, and global-dispatch retirement foundations described by the spec are present. Change 0323 remains the focused follow-up for uninstall, reference-derived version-tree collection, resumable collection journaling, and associated CLI/schema/documentation/tests. No dependency, stack base, relation, or scope adjustment is required; no adjacent follow-up work was surfaced beyond already-tracked changes.

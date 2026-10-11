---
id: 548
slug: 'rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se'
title: 'Rename the v0.9.13 frozen fixture tree to the 1.0 release series'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-10-11'
updated: '2026-10-11'
depends_on: [546]
stacked_on:
related: [546]
discovered_from: [546]
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

Change 546 added the frozen fixture tree testdata/repositories/v0.9.13 because frozen fixtures are immutable and the new agents.*.*.cache_ttl registry row needed new inputs. It continued the existing v0.9.x series, but the repository is now at 1.0.0-rc.1, so the name reads as stale. It was deliberately kept out of 546 so that PR stays focused.

## What changes

Rename the v0.9.13 tree to a name matching the 1.0 release series (for example v1.0.0-rc.1). Update the two test paths that read it (deferredActiveRoot in internal/config/fixtures_test.go and sidecarPath in internal/config/defaults_test.go) and the tree's PROVENANCE.md. Decide whether future versioned trees follow release names or keep the numeric series, and say so in testdata/README.md.

## Out of scope

Renaming or editing the older immutable trees (v0.9.2 through v0.9.12). Changing fixture contents or the immutability rule itself.

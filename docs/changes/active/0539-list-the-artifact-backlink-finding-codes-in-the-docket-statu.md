---
id: 539
slug: 'list-the-artifact-backlink-finding-codes-in-the-docket-statu'
title: 'List the artifact-backlink finding codes in the docket-status skill'
status: 'proposed'
priority: 'low'
type: 'docs'
created: '2026-10-07'
updated: '2026-10-07'
depends_on: []
stacked_on:
related: [533]
discovered_from: [533]
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

Change 533 added three repository-check findings — `artifact-backlink-stale`, `artifact-backlink-malformed`, and `artifact-backlink-shared` — reported by `repository.check` and repaired by `repository.repair`. The docket-status skill's findings paragraph (skills/docket-status/SKILL.md, the clause listing derived-view drift reported by `repository.check`) does not mention them, so a status pass has no guidance to surface or route them.

## What changes

Name the three artifact-backlink codes in the docket-status skill alongside the other `repository.check` findings, say they are repaired through `repository.repair`, regenerate embedded assets (`go generate ./internal/assets/`), and run any skill/doc-drift guards.

## Out of scope

Changing the findings themselves, their detection, or the repair behavior; listing every finding code (the `schema` operation owns codes and shapes).

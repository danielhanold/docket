---
id: 456
slug: 'show-finding-remedies-in-docket-status-human-view'
title: 'Show finding remedies in docket status human view'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-25'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: [454]
discovered_from: [454]
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

`docket status` prints each finding's remedy only in `--json` output; the human-readable view shows the finding but not how to fix it. Change 0454 added a `branch-malformed` finding whose remedy (a filled-in `repair-identity` command or a hand-edit + `docket repository migrate`) is the actionable part, yet a human running plain `docket status` never sees it. This was already true for every finding before 0454.

## What changes

Add a way for `docket status`'s human view to show finding remedies (and any other finding detail currently JSON-only). Whether this is a new flag (e.g. `--verbose`/`--remedies`) or a default-on change to the human renderer is to be resolved during grooming.

## Out of scope

Changing the JSON output shape, the set of findings, or remedy text itself.

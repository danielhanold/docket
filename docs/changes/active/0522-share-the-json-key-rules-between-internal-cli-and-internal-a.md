---
id: 522
slug: 'share-the-json-key-rules-between-internal-cli-and-internal-a'
title: 'Share the JSON-key rules between internal/cli and internal/app'
status: 'proposed'
priority: 'low'
type: 'refactor'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [520]
discovered_from: [521]
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

Change 521 consolidated three copies of the JSON-key rules in internal/app into one shared helper, but requestJSONKeys in internal/cli still keeps its own copy. If the rules change in only one package, the CLI's request check drifts from the app-layer one. It would fail loudly rather than pass silently, so this is a maintenance risk, not a live bug.

## What changes

Make internal/cli's requestJSONKeys use the shared JSON-key helper from internal/app (or move the helper somewhere both can import), so the key rules live in one place.

## Out of scope

Changing the JSON-key rules themselves, the schema tags, or validator behavior.

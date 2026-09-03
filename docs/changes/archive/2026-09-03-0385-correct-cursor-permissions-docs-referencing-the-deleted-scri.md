---
id: 385
slug: 'correct-cursor-permissions-docs-referencing-the-deleted-scri'
title: 'Correct cursor permissions docs referencing the deleted scripts/docket.sh'
status: 'killed'
priority: 'medium'
type: 'docs'
created: '2026-08-31'
updated: '2026-09-03'
depends_on: []
stacked_on:
related: []
discovered_from: [370]
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

Change 0370 deleted the frozen Bash facade (scripts/docket.sh), but the user-facing Cursor docs still publish a terminalAllowlist that names it. These docs sit under docs/ outside the facade-consumer seal by design, so 0370 did not touch them, leaving a stale reference that would mislead anyone configuring Cursor permissions.

## What changes

Update docs/cursor/permissions.md and docs/cursor/permissions.example.json to remove or replace the terminalAllowlist entry for scripts/docket.sh with the native docket binary invocation, so the published permissions guidance matches the post-0370 topology.

## Out of scope

Any change to the seal or to non-cursor permissions surfaces; behavior of the docket binary itself.

## Why killed

Absorbed by change 0402 (restructure the technical docs), which removes `docs/cursor/` entirely: its setup prose folds into the `Running on your harness` guide page and the example JSON moves to `docs/reference/harness/`, with the stale `scripts/docket.sh` allowlist entry replaced by the native `docket` binary invocation in the same rewrite. Fixing the page in place first would be work 0402 immediately discards. Decided with the human on 2026-09-03.

---
id: 105
slug: pin-docket-mode-main-coverage-for-docket-status-digest-only
title: Pin DOCKET_MODE=main coverage for docket-status --digest-only
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: []
discovered_from: [94]
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

Change 0094 shipped `docket-status --digest-only` as a write-free selection read, with its own
fail-closed gates because it deliberately skips `docket_preflight` (ADR-0047).

Its `DOCKET_MODE=main` path is **unfixtured**: no test pins it. The resolution path is shared with
`backlog_pass`, so it is likely fine — but "likely fine" is exactly the state the rest of the
digest contract was moved out of during 0094, where two fail-open holes on this same path were
found by review rather than by the suite.

## What changes

Add `main`-mode coverage for `--digest-only`: that it resolves the metadata worktree as the primary
tree, emits a well-formed `ready` line, and keeps the write-free contract (neither `HEAD` nor the
working tree moves).

## Out of scope

Any behavior change to `--digest-only` itself — this is coverage for what already ships.

## Open questions

- Does the existing `main`-mode fixture harness cover enough to reuse directly?

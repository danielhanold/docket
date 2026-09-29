---
id: 477
slug: 'rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex'
title: 'Rename the gate drive''s leftover gate vocabulary (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY)'
status: 'proposed'
priority: 'high'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: [471]
discovered_from: [471]
adrs: [129]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0471 renamed the run gate to the run tracker (ADR-0129 family (a)) as a hard cut, but kept two spellings because neither has an ADR-0129 row: the gate drive's own `--gate-context` flag and the `DOCKET_AGENT_GUARDIAN_GATE_KEY` environment variable. They are the last places the retired "gate" word survives on the run-tracker surface, so the vocabulary is still split until they are renamed.

## What changes

Decide and record the new spelling for each of the two names as new ADR-0129 table rows (a dated Update note), then rename both as one hard cut with no aliases, and extend the repoguard retired-vocabulary check so the old spellings cannot return. Both renames go in one change because they share the same kind of work and the same landing risk: any running coordinator or dispatched run using the old flag or variable breaks until it restarts, so the coordinated rebuild/restart cutover should happen once. Grooming should also confirm whether the `rungate store` error prefix rename (already done in 0471, missing from the ADR table) should be recorded in the same Update note.

## Out of scope

Renaming anything already covered by ADR-0129 family (a) rows 1-38d; family (b) (change 0472) and later families; the unrelated pre-existing `gofmt` failure in internal/githubcli/comment_integration_test.go.

---
id: 521
slug: 'finish-schema-operation-documentation-outcomes-md-flag-only'
title: 'Finish schema operation documentation: outcomes.md flag-only operations and ADRReplaceRequest required fields'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [360, 520]
discovered_from: [520]
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

Change 0520 made `docket schema --operation` publish exactly the JSON file each operation reads, and flag-only operations no longer publish a request. Two loose ends were found and left out of that change's scope. First, `docs/reference/outcomes.md` still says `docket schema --operation` shows a request shape for every operation, which is now false for the 23 flag-only operations. Second, in `ADRReplaceRequest` the nested `target` and `successor` fields are refused when empty but are not marked required, so the published schema does not tell a caller they are mandatory.

## What changes

(1) Update `docs/reference/outcomes.md` so it describes only current behavior: operations that read a JSON file show its request shape, and flag-only operations show no request. (2) Mark the nested `target` and `successor` fields of `ADRReplaceRequest` as required in the published schema, matching what the validator already refuses, using the same declaration helper (`declareJSONFile`) and required-field test conventions 0520 established (ADR-0138).

## Out of scope

Changing validator behavior, adding new schema vocabulary, or auditing other request types beyond the two items listed.

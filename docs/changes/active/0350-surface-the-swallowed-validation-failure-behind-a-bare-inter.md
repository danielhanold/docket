---
id: 350
slug: surface-the-swallowed-validation-failure-behind-a-bare-inter
title: 'Surface the swallowed validation failure behind a bare internal-error in the transaction engine'
status: proposed
priority: medium
type: fix
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: []
discovered_from: [348]
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

During change 348's implementation, `change claim` failed with a bare `internal-error` that carried
no `failure` field. Root-causing it revealed a real defect masked by that opacity: when an early
call-shape validation fails inside the transaction engine, it returns an **empty disposition**, and
`mapOutcome` / `failureStatus` collapse that empty result into a generic `internal-error` — dropping
the underlying validation `*Failure` entirely. The operator sees `internal-error` with nothing
actionable, when a precise validation message ("wrong `--version` token", etc.) was available and
swallowed.

This is a diagnosability defect: a validation failure that the engine actually detected is
indistinguishable, at the surface, from an unexpected internal crash. It cost real debugging time on
348 (the true cause was passing the metadata-commit token instead of the record-blob oid as
`--version`).

## What changes

Surface the swallowed validation `*Failure`: when an early call-shape validation returns an empty
disposition, `mapOutcome` / `failureStatus` should propagate the real validation failure (its kind
and message) into the reported `failure` field instead of collapsing to a bare `internal-error`.

## Out of scope

- Changing the validation rules themselves — only how a detected validation failure is reported.
- The 348 `--version` call-site bug, which was already worked around during that change.

## Open questions

- Exactly where the empty disposition originates (the specific validation branch) and whether the
  fix belongs in the engine's return path or in `mapOutcome` / `failureStatus`.
- Whether other call-shape validations share this empty-disposition path and are all fixed by one
  change.

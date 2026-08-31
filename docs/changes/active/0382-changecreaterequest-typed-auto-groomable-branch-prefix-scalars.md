---
id: 382
slug: 'changecreaterequest-typed-auto-groomable-branch-prefix-scalars'
title: 'ChangeCreateRequest should accept typed auto_groomable / branch_prefix scalars'
status: proposed
priority: medium
type: feat
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [377]
discovered_from: [377]
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

The native `docket change create` operation (`ChangeCreateRequest`) has no typed field for a
change's `auto_groomable` tri-state or its `branch_prefix` scalar. Today `docket-new-change` sets
those two frontmatter fields by editing the change file with plain git after the record is minted,
rather than by passing them through the create op — a bridge that works but leaves the two fields
outside the typed, validated create path (and outside whatever quoting/validation the op applies to
every other frontmatter scalar). Surfaced during change [[0377]]'s Bash-facade → Go cutover, which
routed change creation through the native op.

## What changes

Extend `ChangeCreateRequest` (and the `docket change create` CLI verb) to accept optional typed
`auto_groomable` (tri-state) and `branch_prefix` (single unqualified path component) inputs, apply
the same write-boundary validation/quoting the op already gives other scalars, and retire the
post-mint frontmatter-edit bridge in `docket-new-change` so both fields flow through the one typed
create path.

## Out of scope

- Any change to the *meaning* of `auto_groomable` or `branch_prefix` (the tri-state inheritance and
  the branch-prefix normalization/refusal rules are unchanged) — this is a plumbing fix only.
- Other frontmatter fields not currently accepted by the create op.

## Open questions

- Whether `branch_prefix` validation (strip one trailing slash; refuse slash-embedded / `refs/`
  values) belongs in the op or stays in the calling skill.
- Whether any other current caller relies on the post-mint edit bridge.

## Reconcile log

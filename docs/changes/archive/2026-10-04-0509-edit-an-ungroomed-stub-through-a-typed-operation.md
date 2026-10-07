---
id: 509
slug: 'edit-an-ungroomed-stub-through-a-typed-operation'
title: 'Edit an ungroomed stub through a typed operation'
status: 'done'
priority: 'medium'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [445, 461]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-design.md'
plan: 'docs/superpowers/plans/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation.md'
results: 'docs/results/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/edit-an-ungroomed-stub-through-a-typed-operation'
pr: 'https://github.com/danielhanold/docket/pull/381'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-design.md](../../superpowers/specs/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-design.md) |
| Plan | [2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation.md) |
| Results | [2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-04-edit-an-ungroomed-stub-through-a-typed-operation-results.md) |
<!-- docket:artifacts:end -->

## Why

A needs-grooming stub has no typed way to change its title, owned sections, or relationship fields without also grooming it. `change.groom` `revise` refuses a stub with `not-revisable`, because it only accepts already-groomed changes. The outcomes that do accept edits on a stub (`spec`, `trivial`, `re-enable`) each change the stub's groom state as a side effect. So sharpening a stub's Why, fixing its title, or adding a `related:` link today means a hand-edit in the `.docket` tree, which skips the writer's quoting guarantee (ADR-0071) and leaves `BOARD.md` stale.

## What changes

Docket can edit a `proposed` stub's title, owned proposal sections, and relationship fields through a typed operation, leaving it needs-grooming. The grooming skills say how to reach that path when a human asks to edit a stub rather than groom it.

## Out of scope

Editing changes past `proposed`. Changing a stub's groom state through `revise` (spec, trivial, abstain, re-enable keep their owners). Editing `type`, `priority`, or `auto_groomable`.

## Reconcile log

### 2026-10-04

2026-10-04: origin/main is still at the spec's design baseline 5805127ab. The `revise` gate in `changeGroomOp.Plan`, the `spec-not-linked` refusal, and the re-enable-only `## Auto-groom blocked` heading check in `validateChangeGroomShape` are all as the spec describes. No scope change.

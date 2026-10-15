---
id: 550
slug: 'let-change-groom-set-a-change-s-priority'
title: 'Let change.groom set a change''s priority'
status: 'done'
priority: 'medium'
type: 'feat'
created: '2026-10-14'
updated: '2026-10-15'
depends_on: []
stacked_on:
related: [461]
discovered_from: [523]
adrs: []
spec: 'docs/superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md'
plan: 'docs/superpowers/plans/2026-10-14-let-change-groom-set-a-change-s-priority.md'
results: 'docs/results/2026-10-14-let-change-groom-set-a-change-s-priority-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/let-change-groom-set-a-change-s-priority'
pr: 'https://github.com/danielhanold/docket/pull/412'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-14-let-change-groom-set-a-change-s-priority-design.md](../../superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md) |
| Spec (merged) | [2026-10-14-let-change-groom-set-a-change-s-priority-design.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md) |
| Plan | [2026-10-14-let-change-groom-set-a-change-s-priority.md](../../superpowers/plans/2026-10-14-let-change-groom-set-a-change-s-priority.md) |
| Results | [2026-10-14-let-change-groom-set-a-change-s-priority-results.md](../../results/2026-10-14-let-change-groom-set-a-change-s-priority-results.md) |
<!-- docket:artifacts:end -->

## Why

A change's priority is visible on the board and drives implement-next's selection order, but only `change.create` can set it. Once a change exists, no typed operation edits `priority:`. The only way to re-prioritize is a hand edit committed to the `docket` branch. That leaves BOARD.md stale until the next refresh, and auto mode blocks agents from hand-pushing to `docket`, so every re-prioritization has to go back to the human.

## What changes

`change.groom` accepts an optional `priority` field and writes it to the record's `priority:` line in the same atomic metadata commit. The board and the `## Artifacts` block are re-rendered in that commit. The field rides on the same outcomes as `title` (spec, trivial, revise, re-enable) and is refused on abstain. An unknown value is refused with `unknown-priority` before anything is written. Omitting the field leaves the priority unchanged, and a revise that carries only a priority is a valid edit. Like every groom, it applies only to `proposed` changes; to re-prioritize a deferred change, revive it first. The `docket-groom-next` skill documents the field.

## Out of scope

A dedicated `change.set-priority` command. Changing the priority values themselves or how selection orders them. Bulk re-prioritization across several changes.

## Reconcile log

### 2026-10-14

2026-10-14: Reconciled against main at 50420ee58. change.groom still has no priority field; the title (retitle) precedent, ParsePriority, and the unknown-priority finding code are all present. Related #461 is done. Scope unchanged.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: e2dbe4e7fec4156d0c56c7c9ee8f8c7f085eb926
ran_at:   2026-10-14T23:54:43Z
<!-- docket:build-evidence:end -->

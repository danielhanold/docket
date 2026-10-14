---
id: 550
slug: 'let-change-groom-set-a-change-s-priority'
title: 'Let change.groom set a change''s priority'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-14'
updated: '2026-10-14'
depends_on: []
stacked_on:
related: [461]
discovered_from: [523]
adrs: []
spec: 'docs/superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/let-change-groom-set-a-change-s-priority'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-14T23:33:50Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-14-let-change-groom-set-a-change-s-priority-design.md](../../superpowers/specs/2026-10-14-let-change-groom-set-a-change-s-priority-design.md) |
<!-- docket:artifacts:end -->

## Why

A change's priority is visible on the board and drives implement-next's selection order, but only `change.create` can set it. Once a change exists, no typed operation edits `priority:`. The only way to re-prioritize is a hand edit committed to the `docket` branch. That leaves BOARD.md stale until the next refresh, and auto mode blocks agents from hand-pushing to `docket`, so every re-prioritization has to go back to the human.

## What changes

`change.groom` accepts an optional `priority` field and writes it to the record's `priority:` line in the same atomic metadata commit. The board and the `## Artifacts` block are re-rendered in that commit. The field rides on the same outcomes as `title` (spec, trivial, revise, re-enable) and is refused on abstain. An unknown value is refused with `unknown-priority` before anything is written. Omitting the field leaves the priority unchanged, and a revise that carries only a priority is a valid edit. Like every groom, it applies only to `proposed` changes; to re-prioritize a deferred change, revive it first. The `docket-groom-next` skill documents the field.

## Out of scope

A dedicated `change.set-priority` command. Changing the priority values themselves or how selection orders them. Bulk re-prioritization across several changes.

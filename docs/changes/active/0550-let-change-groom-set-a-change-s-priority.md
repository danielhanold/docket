---
id: 550
slug: 'let-change-groom-set-a-change-s-priority'
title: 'Let change.groom set a change''s priority'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-14'
updated: '2026-10-14'
depends_on: []
stacked_on:
related: []
discovered_from: [523]
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

A change's priority is visible on the board and drives implement-next's selection order, but only `change.create` can set it. Once a change exists, no typed operation edits `priority:`. The only way to re-prioritize is a hand edit committed to the `docket` branch. That leaves BOARD.md stale until the next refresh, and auto mode blocks agents from hand-pushing to `docket`, so every re-prioritization has to go back to the human.

## What changes

`change.groom` accepts an optional `priority` field and writes it to the record's `priority:` frontmatter in the same atomic metadata commit, with the board re-rendered. The value is checked against the same closed priority set `change.create` uses (critical, high, medium, low), and an unknown value is refused before anything is written. Omitting the field leaves the priority unchanged. The schema descriptor and the skills that build groom requests document the new field.

## Out of scope

A dedicated `change.set-priority` command. Changing the priority values themselves or how selection orders them. Bulk re-prioritization across several changes.

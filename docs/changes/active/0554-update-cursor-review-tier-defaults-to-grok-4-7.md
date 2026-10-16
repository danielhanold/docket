---
id: 554
slug: 'update-cursor-review-tier-defaults-to-grok-4-7'
title: 'Refresh the shipped Claude and Cursor model defaults'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-15'
updated: '2026-10-16'
depends_on: [548]
stacked_on:
related: [195, 166]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md'
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
| Spec | [2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md](../../superpowers/specs/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md) |
<!-- docket:artifacts:end -->

## Why

New models are out: Claude Opus 5.5 and Haiku 5.5, plus Grok 4.6 and 4.7 in Cursor. Docket's shipped defaults still pin the previous generation. Claude still uses `claude-opus-5`, `claude-sonnet-5`, and `claude-haiku-4-5-20251001`, and Cursor still uses `cursor-grok-4.5-*` and `claude-opus-5-high`. The built-in defaults should match the roster Daniel now runs.

## What changes

Replace every row of the Claude and Cursor blocks in the built-in model defaults (17 agents each). Claude: Opus 5.5 for every agent except `status` (Haiku 5.5), with effort setting the tier. `implement-next` keeps its 1-hour cache pin. Cursor: Grok 4.6 for most agents at the level in its name, Grok 4.7 for the three review tiers, and Claude models through Cursor for `status`, `plan-writer`, and `build-max`. Every Cursor effort stays `auto`. Apply the same values everywhere the table lives: `agents/harness-defaults.yml`, the Go built-in table, the commented example in `.docket.example.yml`, and the regenerated embedded copies. Update `docs/reference/harness/validation.md`'s list of Cursor wrappers allowed to carry a Claude model ID. Rewrite the defaults-file comments the new values make untrue, and drop the "review-deep equals build-max" rule entirely. The spec has the exact per-agent tables. This change waits for #548, so it needs no new frozen fixture folder.

## Out of scope

The Codex and opencode values (only an opencode comment sentence is removed). The interactive skills' advisory session-model recommendation (#166). Changes to how the Cursor emitter handles effort. Frozen historical data: legacy install fixtures, surviving `testdata/repositories/` trees, release notes, and archived records.

---
id: 554
slug: 'update-cursor-review-tier-defaults-to-grok-4-7'
title: 'Refresh the shipped Cursor model defaults and Claude''s status model'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-15'
updated: '2026-10-16'
depends_on: [548]
stacked_on:
related: [195]
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

New models are out: Grok 4.6 and 4.7 in Cursor, and Claude Opus 5.5 and Haiku 5.5. Docket's shipped Cursor defaults still pin `cursor-grok-4.5-*` and `claude-opus-5-high`, and Claude's `status` agent still pins `claude-haiku-4-5-20251001`. The built-in defaults should match the roster Daniel now runs.

## What changes

Replace all 17 rows of the Cursor block in the built-in model defaults. The roster: Grok 4.6 for most agents at the level in its name, Grok 4.7 for the three review tiers, and Claude models through Cursor for `status`, `plan-writer` and `build-max`. Every effort stays `auto`. Also move Claude's `status` agent to `claude-haiku-5-5` (effort stays `medium`). Apply the same values everywhere the table lives: `agents/harness-defaults.yml`, the Go built-in table, the commented example in `.docket.example.yml`, and the regenerated embedded copies. Update `docs/reference/harness/validation.md`'s list of Cursor wrappers allowed to carry a Claude model ID. Reword the defaults-file comments the new values make untrue. On Cursor, review-deep no longer matches build-max: that is a deliberate choice, so Cursor's review tiers all run on Grok 4.7. The spec has the exact per-agent table. This change waits for #548, so it needs no new frozen fixture folder.

## Out of scope

Claude rows other than `status`, and the Codex and opencode blocks. Changes to how the Cursor emitter handles effort. Frozen historical data (legacy install fixtures, surviving `testdata/repositories/` trees, release notes, archived records).

---
id: 554
slug: 'update-cursor-review-tier-defaults-to-grok-4-7'
title: 'Refresh the shipped Claude and Cursor model defaults; drop the interactive-skill model advice'
status: 'implemented'
priority: 'medium'
type: 'chore'
created: '2026-10-15'
updated: '2026-10-16'
depends_on: [548]
stacked_on:
related: [195, 166]
discovered_from: []
adrs: [8]
spec: 'docs/superpowers/specs/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md'
plan: 'docs/superpowers/plans/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7.md'
results: 'docs/results/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/update-cursor-review-tier-defaults-to-grok-4-7'
pr: 'https://github.com/danielhanold/docket/pull/419'
blocked_by:
reconciled: true
claimed_at: '2026-10-16T12:52:39Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md](../../superpowers/specs/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-design.md) |
| Plan | [2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7.md](../../superpowers/plans/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7.md) |
| Results | [2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-results.md](../../results/2026-10-16-update-cursor-review-tier-defaults-to-grok-4-7-results.md) |
| ADRs | [ADR-0008](../../adrs/0008-agent-layer-generated-subagents.md) |
<!-- docket:artifacts:end -->

## Why

New models are out: Claude Opus 5.5 and Haiku 5.5, plus Grok 4.6 and 4.7 in Cursor. Docket's shipped defaults still pin the previous generation: `claude-opus-5`, `claude-sonnet-5`, and `claude-haiku-4-5-20251001` on Claude, and `cursor-grok-4.5-*` and `claude-opus-5-high` on Cursor. The built-in defaults should match the roster Daniel now runs.

The two interactive skills (`docket-new-change`, `docket-groom-next`) also hard-code a recommended session model (`claude-sonnet-5`), and it goes stale with every model refresh. Docket can't pin those skills' model anyway, because the human's session runs them. The choice belongs to whoever runs the session, based on their preference and how complex the change looks, so the recommendation shouldn't need upkeep.

## What changes

Replace every row of the Claude and Cursor blocks in the built-in model defaults (17 agents each). On Claude, every agent uses Opus 5.5 except `status`, which uses Haiku 5.5, and effort sets the tier. `implement-next` keeps its 1-hour cache pin. On Cursor, most agents use Grok 4.6 at the level in its name, the three review tiers use Grok 4.7, and `status`, `plan-writer` and `build-max` use Claude models through Cursor. Every Cursor effort stays `auto`. Apply the same values everywhere the table lives: `agents/harness-defaults.yml`, the Go built-in table, the commented example in `.docket.example.yml`, and the regenerated embedded copies. Update the validation doc's list of Cursor wrappers allowed to carry a Claude model ID. Rewrite the defaults-file comments that the new values make untrue, and drop the "review-deep equals build-max" rule entirely.

Delete the `Recommended model/effort (advisory)` section from both interactive skills, and drop the convention's mention of it. In the models-and-effort doc, replace it with short, model-agnostic guidance: pick the session model by preference and by how complex the change is expected to be. Keep that doc's intro free of per-skill tiers that would go stale. Record the removal as a dated update note on ADR-0008. The spec has the exact tables and file list. This change waits for #548, so it needs no new frozen fixture folder, and it makes #166 obsolete.

## Out of scope

The Codex and opencode values (only one opencode comment sentence is removed). Changes to how the Cursor emitter handles effort. Frozen historical data: legacy install fixtures, surviving `testdata/repositories/` trees, internal corpus copies, release notes, and archived records.

## Reconcile log

### 2026-10-16

Reconciled against main e99f310. Dependency #548 is done: internal/config/defaults_test.go now reads agents/harness-defaults.yml directly (no versioned fixture tree), so this is a plain data edit as the spec assumes. The example-correspondence mutation probe still keys on `{ model: claude-sonnet-5,` and must be retargeted, as the spec says. No scope change.

### 2026-10-16

Recorded ADR-0008 in adrs: — it gained a dated Update note (the interactive skills no longer carry an advisory model recommendation).

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: 8811c790466745b025a036aeb9ca709beac317e2
ran_at:   2026-10-16T15:24:38Z
<!-- docket:build-evidence:end -->

---
id: 555
slug: 'give-claude-its-own-claude-md-block-when-agents-md-stops-bei'
title: 'Keep Claude''s dispatch block in AGENTS.md while CLAUDE.md links to it'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-16'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [537]
discovered_from: [537]
adrs: [78]
spec: 'docs/superpowers/specs/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-design.md'
plan: 'docs/superpowers/plans/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md'
results: 'docs/results/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/give-claude-its-own-claude-md-block-when-agents-md-stops-bei'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-16T20:22:12Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-design.md](../../superpowers/specs/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-design.md) |
| Plan | [2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md](../../superpowers/plans/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md) |
| Results | [2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-results.md](../../results/2026-10-16-give-claude-its-own-claude-md-block-when-agents-md-stops-bei-results.md) |
| ADRs | [ADR-0078](../../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

In a shared repository, install can make `CLAUDE.md` a link to `AGENTS.md`, so Claude reads the same file Codex and opencode read. Install counts only codex and opencode as owners of the `AGENTS.md` dispatch block. Dropping both while keeping claude retires the block. Claude keeps reading `AGENTS.md` through the link, but without the dispatch and run-tracker rules, and later installs never put them back. Since #537, an `AGENTS.md` left empty is deleted, and the link points at nothing.

The same rule leaves two claude-only setups without Claude's rules. A repository where someone linked `CLAUDE.md` to `AGENTS.md` by hand gets no dispatch block at all. A repository with its own `AGENTS.md` and no `CLAUDE.md` gets a block-only `CLAUDE.md`, so Claude never sees that `AGENTS.md`. ADR-0078 chose the link so Claude sees everything `AGENTS.md` carries, and the Go planner drifted from it. Found while grooming #537.

## What changes

- When `CLAUDE.md` is a relative link to `AGENTS.md` and claude is opted in, Claude co-owns the `AGENTS.md` dispatch block, so the block stays in the file the link points to.
  - Dropping codex and opencode keeps the block.
  - A claude-only repository with the link gets the block.
  - The next install repairs a repository that already lost the block or `AGENTS.md`.
- A claude-only repository with its own `AGENTS.md` and no `CLAUDE.md` gets the `CLAUDE.md` link instead of a block-only `CLAUDE.md`.
- The block's text, and the choice to share, follow every harness the repository opted into, not just the ones a `--harness` run is limited to. A scoped install no longer strips the Codex clause.
- Install never retires a block in the same run that writes it.
- The install docs, the glossary, and the convention's agent-layer reference describe the new rule.

## Out of scope

- A `CLAUDE.md` that links anywhere other than `AGENTS.md`. It stays unsupported and keeps being skipped.
- Turning an existing block-only `CLAUDE.md` into a link.
- Hand-made links being recorded as docket's, so that dropping claude deletes them.
- A scoped run for a harness no longer opted in retiring a block Claude now shares. The next run that includes claude restores it.
- Private repositories, which keep no surfaces in the working tree.

## Reconcile log

### 2026-10-16

2026-10-16: Groomed today; reposeed.Plan, ClaudeMDLinkToAgents handling, PrivatePlanInput.OptIns, and computeRemovals on main (d253a2cff) still match the spec's description. #537 is done and its empty-AGENTS.md deletion is in place. No scope change.

---
id: 395
slug: 'migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap'
title: 'Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: [394]
stacked_on:
related: [394]
discovered_from: []
adrs: [104]
spec: 'docs/superpowers/specs/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md'
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
| Spec | [2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md) |
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

Change 0394 introduced an authoritative compact CLI capability catalog and migrated the maintained workflow surfaces (skills/, agents/, cursor-rules/, scripts/, README, .docket.example.yml) to fetch it and resolve executable spellings from it, but its Task-6 migration inventory and its repoguard enforcement guard were both scoped to those surfaces only. The two repo-root instruction files CLAUDE.md and AGENTS.md were left outside that scope, so their agent-executed blocks still hard-code CLI argv and the resulting asymmetry with the migrated Cursor mirror is unflagged by any guard. Two blocks are affected: the "Run gate" block (hard-codes `docket run gate-before` / `docket run gate-verdict` argv) and the "Rebuild the binary after a merge to main" block (hard-codes `docket development install` argv). These are exactly the agent-executed argv the catalog exists to make authoritative — this session itself executed the un-migrated Run gate block. Governed by ADR-0104 (the capability catalog is the authoritative executable CLI surface).

## What changes

Migrate the "Run gate" block to the run.gate-before / run.gate-verdict catalog idiom and the "Rebuild the binary after a merge to main" block to the development.install catalog idiom, in both CLAUDE.md and AGENTS.md, preserving each block's operational contract while removing hard-coded argv. Extend the repoguard enforcement guard's scope to cover CLAUDE.md and AGENTS.md so this parity cannot silently regress, reconciling the guard's exemption model with the repo-root instruction files (which are human-and-agent-facing prose, distinct from the generated/guarded workflow surfaces the catalog primarily targets). Regenerate any embedded/derived assets affected by the guard-scope extension and add coverage proving the guard reddens on a re-introduced hard-coded argv in these files.

## Out of scope

Introducing new catalog leaves or changing the catalog protocol; migrating any surface not named here; JSON-schema/request-shape discovery (owned by change 0360); rewriting historical changes, specs, plans, results, or Accepted ADR prose.

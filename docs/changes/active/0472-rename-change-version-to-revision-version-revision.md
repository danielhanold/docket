---
id: 472
slug: 'rename-change-version-to-revision-version-revision'
title: 'Rename change version to revision (--version → --revision)'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [468]
discovered_from: []
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

Change 0468 settled a collision-free docket vocabulary. The `--version <blob>` flag on change and finalize operations reads like a semantic version and collides with the `docket version` command. This family applies 0468's rename table rows 39–45.

## What changes

Apply 0468 spec rows 39–45 as a hard cut (no aliases), following the spec's "Family changes" obligations:

- `--version <blob>` → `--revision` on the 22 operations row 40 lists.
- Request/response `version` → `revision` on those operations, `learning.update`, `status` `changes[].version`, and the `context.*` reads; `spec_version` → `spec_revision`; ADR `target.version` → `target.revision`; `pr_version` → `pr_revision`.
- Concept: change version / entity version → revision / record revision.
- Go identifiers, tests, golden `capabilities`/`schema` output, skills, agents, embedded copies, generated dispatch material, docs, glossary entries for these rows, and CLAUDE.md/AGENTS.md (including memory-style remedies such as `workspace prepare --version`).
- Add these rows' retired tokens to the retired-vocabulary table's absence seal (creating the table if this family lands first), mutation-tested.
- `resolver_budget_version` (row 45) and software/format versions (`protocol_version`, `schema_version`, the `version` op, …) stay unchanged.

## Out of scope

- Any alias, deprecation-window, or dual-spelling support: old spellings are hard-cut (0468 Decision 2).
- Renaming persisted storage names, config keys, agent names, or frontmatter fields (0468 Decisions 3, 9).
- Editing point-in-time records (archived changes, results, specs, plans, Accepted ADRs).
- Rows owned by the other 0468 family changes or by change 0469.

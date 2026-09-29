---
id: 471
slug: 'rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run'
title: 'Rename the run gate to the run tracker (epoch → run id, gate-* → run-*)'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [468, 469, 467]
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

Change 0468 settled a collision-free docket vocabulary. The run gate is not a checkpoint but bookkeeping for a launched run, and "epoch" hides that it names a run and its id. This family applies 0468's rename table rows 1–38 to the wire surface: operation ids, CLI verbs, flags, verdict lines, dispositions, error codes, and failure stages of the run tracker.

## What changes

Apply 0468 spec rows 1–38 as a hard cut (no aliases), following the spec's "Family changes" obligations:

- `run.gate-before` / `run.gate-verdict` / `run.gate-claim` → `run.start` / `run.verdict` / `run.continue` (and the `docket run …` verbs).
- `--run-epoch` → `--run-id`; `run cancel --epoch` → `--run-id`; `change claim --gate-context` → `--run-context`.
- `gate-armed` / `gate-unarmed` / verdict lines `gate-*` → `run-started` / `run-untracked` / `run-*`; `gate-claimed` → `run-continued`; `gate-unavailable` → `run-tracker-unavailable`; `gate-context-*` → `run-context-*`.
- The `epoch-*` codes and stages, renamed by meaning (run id / run record / run) exactly as rows 16–37 list them.
- Go identifiers, tests (incl. repoguard wording pins), golden `capabilities`/`schema` output, skills, agents, embedded copies, generated dispatch material, docs, glossary entries for these rows, and CLAUDE.md/AGENTS.md.
- Create the retired-vocabulary table in `internal/repoguard` if no earlier family has, and add these rows' retired tokens to its absence seal (mutation-tested).
- Persisted names (row 38) stay unchanged.

Landing requirement (0468 spec §4): merge with no dispatched run in flight, then run the post-merge binary rebuild immediately.

## Out of scope

- Any alias, deprecation-window, or dual-spelling support: old spellings are hard-cut (0468 Decision 2).
- Renaming persisted storage names, config keys, agent names, or frontmatter fields (0468 Decisions 3, 9).
- Editing point-in-time records (archived changes, results, specs, plans, Accepted ADRs).
- Rows owned by the other 0468 family changes or by change 0469.

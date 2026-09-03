---
id: 402
slug: 'restructure-the-technical-docs-into-goal-organised-guide-con'
title: 'Restructure the technical docs into goal-organised guide, concepts, and reference tiers'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-03'
updated: '2026-09-03'
depends_on: [400]
stacked_on:
related: [385, 283]
discovered_from: [400]
adrs: [53, 54]
spec: 'docs/superpowers/specs/2026-09-03-restructure-the-technical-docs-into-goal-organised-guide-con-design.md'
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
| Spec | [2026-09-03-restructure-the-technical-docs-into-goal-organised-guide-con-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-03-restructure-the-technical-docs-into-goal-organised-guide-con-design.md) |
| ADRs | [ADR-0053](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0053-readme-yaml-fences-guarded-by-default-opt-out-marker-grammar.md), [ADR-0054](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0054-cross-reference-anchor-style.md) |
<!-- docket:artifacts:end -->

## Why

Change 400 relocates the 1,075-line README body verbatim into `docs/guide/README.md` and leaves the split to a follow-on. That relocated page is still organised by mechanism (install, config, docket-mode, tuning, skills, customisation) rather than by what a reader is trying to do, and it is written in docket's internal vocabulary (worktree, claim lease, CAS push, gate facade, dispatch) with no gloss. Three harness directories (`docs/cursor/`, `docs/codex/`, `docs/opencode/`) sit beside it in a different voice, and one of them (change 385) already carries a stale reference.

Two readers are served badly by that shape. A user of docket wants task-shaped pages: capture work, design it, build it unattended, prove it, land it. The maintainer wants a plain-language picture of how the pieces fit (two branches, the lifecycle state machine, dispatch, the run gate, config layers, reconcile) that stays current, which today lives only across 97 immutable ADRs and a 1,000-line convention skill.

Sorting pages by audience would rot: pages with one reader get no pressure to stay true. Sorting by the question a page answers does not. Three tiers cover both readers: a guide (how do I do X), concepts (what is this and why is it built this way), and a reference (what are the exact fields and verbs). The voice for all three is the shipped default `dummy_mode` persona: a mid-level engineer who knows architecture and is told every docket-internal term with a gloss on first use.

## What changes

- `docs/README.md` becomes the docs entry point: the three tiers, one hook line per page, and a start-here path for a new user. The top-level README's documentation map links to it.
- `docs/guide/` gains twelve how-to pages, one per non-landing row of change 400's goal-first docs map (capturing work, designing before building, building without supervision, proving the build, reviewing before the human does, landing changes safely, keeping the backlog honest, remembering why, governing through configuration, running on your harness, where the metadata lives, plus a short index). The relocated body is rewritten for the persona, not moved verbatim; every decision, caveat, config key, and option in it lands on exactly one page, tracked by a coverage table in the spec. `docs/guide/README.md` is removed once split.
- `docs/concepts/` gains nine living explanation pages with a fixed shape (the problem it solves, the moving parts, the invariants, a `decided in` list of ADR links): two branches and the metadata worktree; the change lifecycle as a state machine; skills, agents, and harness dispatch; the run gate and attribution; config layers and the coordination fence; reconcile; build profiles and the test gate; finalize as a sequencer; learnings and ADRs as memory. ADRs are linked, never re-narrated.
- `docs/reference/` gains pointer pages only (CLI by noun and verb, manifest and ADR fields, config keys, dispositions and health codes, skill and agent inventory): each names where the fact is owned (`docket --help`, `.docket.example.yml`, the convention skill, the capability catalog) and quotes nothing that can drift. `docs/reference/harness/` takes the cursor and codex validation runbooks, the example JSON files, and the codex fixtures.
- The `Running on your harness` guide page absorbs the setup prose from `docs/cursor/`, `docs/codex/`, and `docs/opencode/` as one section per harness and carries change 385's correction (the `scripts/docket.sh` allowlist entry becomes the native binary invocation). The three directories are removed.
- The `internal/repoguard` prose-contract rows that pin the relocated body and the harness pages are repointed at the page that now carries each phrase, in the same commit as the move. Every in-repo link into the old paths is retargeted under 400's maintained-source versus point-in-time rule.
- The spec carries a glossary of docket terms with one approved one-clause gloss each and a per-page voice checklist; review verifies by reading. No new guard.

## Out of scope

- Any change to the top-level `README.md` beyond retargeting its documentation-map links (400 owns it).
- A generator for the reference tier, or any Go code beyond the repoguard row repointing.
- Rewording ADRs, archived changes, results files, specs, or `docs/release/`; those are point-in-time records.
- Editing skill bodies or `.docket.example.yml`; documentation drift inside skills is a separate change.
- A mechanical voice or glossary guard.

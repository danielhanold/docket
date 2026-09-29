---
id: 471
slug: 'rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run'
title: 'Rename the run gate to the run tracker (epoch → run id, gate-* → run-*)'
status: 'in-progress'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [467, 468, 469, 472, 473, 474]
discovered_from: []
adrs: [129]
spec: 'docs/superpowers/specs/2026-09-29-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-29T14:01:51Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-29-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-29-rename-the-run-gate-to-the-run-tracker-epoch-run-id-gate-run-design.md) |
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0468 settled a collision-free docket vocabulary. The run gate is not a checkpoint but bookkeeping for a launched run, and "epoch" hides that it names a run and its id. This family applies 0468's rename table rows 1–38 to the wire surface: operation ids, CLI verbs, flags, verdict lines, dispositions, error codes, and failure stages of the run tracker.

## What changes

Apply ADR-0129's family (a) rows 1–38 and 38a–38d (the ADR was amended in place on 2026-09-29) as a hard cut, with no aliases:

- **Operations:** `run.gate-before` / `run.gate-verdict` / `run.gate-claim` become `run.start` / `run.verdict` / `run.continue`, along with their `docket run …` verbs.
- **Flags:** `--run-epoch` becomes `--run-id`, and so does `run cancel --epoch`. `change claim --gate-context` becomes `--run-context`, and `agent enter --run-gate-key` becomes `--run-key`.
- **Report lines:** `gate-armed` / `gate-unarmed` become `run-started` / `run-untracked`, verdict lines `gate-*` become `run-*`, and `gate-claimed` becomes `run-continued`.
- **Codes and stages:** the `epoch-*` codes and failure stages are renamed by meaning (run id, run record, or run). `gate-unavailable` becomes `run-tracker-unavailable`, and `gate-context-*` becomes `run-context-*`.
- **Keys and env:** the result key `epoch` becomes `run_id`, the claim request key `gate_context` becomes `run_context`, and the guardian env var becomes `DOCKET_AGENT_GUARDIAN_RUN_ID`.
- **Local storage is reset to new names, not migrated:** `run-tracker/`, `run-tracker-resume/`, `run.json`, `run.lock`, and `v2` roots for gate admission, gate scopes and gate drives. The new binary starts these stores empty. The claim receipts committed on `docket` keep their `gate_context_hash` key.
- **Every file named after a retired term is renamed:** `internal/app/rungate_*` becomes `runtracker_*`; the gatedrive and repoguard epoch tests are renamed; so are the six run-tracker integration shards (with their test prefixes and budget rows), the concept page and the cursor-rules asset.
- **Everything that names these terms moves with them:** Go identifiers, tests, golden `capabilities`/`schema` output, skills, embedded copies, generated dispatch material, the AGENTS.md dispatch block, docs and the glossary entries for these rows.
- **Retired-vocabulary seal:** create the retired-vocabulary table in `internal/repoguard` and seal the retired tokens at executable sites. The seal is mutation-tested.

**Landing (human procedure):**
1. Merge with no dispatched run in flight.
2. Run the post-merge binary rebuild immediately.
3. Restart coordinator sessions.
4. Re-run `docket install` in consumer repos.
5. On each machine, switch binaries only when no run is in flight. Old storage roots may then be deleted by hand.

## Out of scope

- Aliases, a deprecation window or dual-spelling support: old spellings are hard-cut (ADR-0129 Decision 2).
- Renaming config keys, agent names, frontmatter fields, or committed metadata (the claim receipts' `gate_context_hash` and the claim digest).
- Migrating old run-tracker state, an upgrade-detection guard, a live-run listing operation, or deleting old storage roots automatically.
- Editing point-in-time records: archived changes, results, specs and plans.
- Rows owned by the other ADR-0129 family changes (0472–0474) or by change 0469.

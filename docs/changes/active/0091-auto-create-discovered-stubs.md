---
id: 91
slug: auto-create-discovered-stubs
title: Auto-create discovered stubs — a config flag that turns mid-run findings into proposed changes
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [90]
adrs: [19]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
<!-- docket:artifacts:end -->

## Why

Captured alongside #0090 (2026-07-17). Today, when an autonomous run surfaces follow-up work —
implement-next's reconcile/review notices an adjacent gap, a build discovers a latent bug, a
close-out finding implies a next step — the model *asks* the human whether to file it, or worse,
mentions it in prose that scrolls away. In an unattended run there is no human to ask, so
discovered work is routinely dropped on the floor. Beads' agents are simply told to capture
(`bd create`/`bd q` mid-task, with `discovered-from` provenance); the near-zero-friction capture
path is what makes agent-discovered work durable.

docket should have the same posture behind a flag: when enabled, a skill that identifies genuine
follow-up work mints a needs-brainstorm stub directly (with `discovered_from:` set, per #0090)
instead of asking. Stubs are cheap, reviewable markdown on the metadata branch — the human still
gates everything at groom time, so auto-creation adds no autonomy risk, only capture fidelity.

## What changes

- A new config knob (name settled in brainstorm, e.g. `auto_capture: true|false`, default
  `false`) — **configurable in all layers**: repo `.docket.yml`, user-level global config, and
  `.docket.local.yml`. It gates behavior only and creates re-derivable markdown, so per the
  ADR-0019 fence classification it should be global-able — brainstorm confirms.
- When enabled, the autonomous skills (implement-next, auto-groom, finalize/harvest) create
  `proposed` needs-brainstorm stubs for discovered work via the normal id-allocation CAS path,
  with `discovered_from:` populated; when disabled, today's ask-or-mention behavior stays.
- Guardrails against noise: a materiality bar for what deserves a stub (vs a reconcile-log or
  learnings note), and a per-run cap — brainstorm decides both.
- Stub minting mid-run must not disturb the run's own claim/branch state (metadata-worktree
  writes only, same as any new-change allocation).

## Out of scope

- Auto-grooming or auto-implementing the created stubs (existing `auto_groom` machinery already
  governs what happens next).
- The provenance field itself (#0090; this change consumes it — likely `depends_on` or a merge,
  decided at groom time).
- Deduplication beyond a cheap check against existing active titles/slugs.

## Open questions

- Combine with #0090 into one change, or keep field (90) and behavior (91) separate?
- Per-skill granularity (allow implement-next but not auto-groom to mint?) or one global switch?
- Does an auto-created stub get flagged on the board (e.g. "discovered — unreviewed") so humans
  can sweep new arrivals?

## Reconcile log

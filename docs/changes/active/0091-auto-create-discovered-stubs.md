---
id: 91
slug: auto-create-discovered-stubs
title: Auto-create discovered stubs — a config flag that turns mid-run findings into proposed changes
status: in-progress
priority: medium
created: 2026-07-17
updated: 2026-07-18
depends_on: [90]
related: [90]
adrs: [19]
spec: docs/superpowers/specs/2026-07-17-auto-create-discovered-stubs-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/auto-create-discovered-stubs
claimed_at: 2026-07-18T19:58:05Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-auto-create-discovered-stubs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-auto-create-discovered-stubs-design.md) |
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

- A new **boolean** config knob `auto_capture: true | false`, default `false` — **global-able**
  across all layers (repo `.docket.yml`, user-level global config, `.docket.local.yml`), classified
  by direct analogy to `auto_groom` under ADR-0019 (gates a local-run behavior producing ordinary
  backlog commits, never coordination state). Resolved with the same layered read as `auto_groom`
  and recorded in the authoritative fence table in `scripts/docket-config.md`.
- When enabled, the **autonomous single-change** skills — `docket-implement-next` (reconcile/review
  discoveries) and the `docket-finalize-change` / `docket-status` harvest (close-out findings) —
  mint `proposed` needs-brainstorm stubs for discovered work with `discovered_from:` populated (per
  #0090), instead of asking or mentioning. When disabled, today's ask-or-mention behavior is
  unchanged. `docket-auto-groom` is deliberately **not** a mint site (it would break its own
  provable-termination invariant and create an `auto_groom` × `auto_capture` growth loop);
  interactive skills already mint with a human present.
- The mint reuses `docket-new-change`'s id-allocation + CAS routine via a deterministic helper: the
  model decides *what* is material (a stub = distinct follow-up work that would be its own PR; not a
  learnings lesson, not current-change drift), the helper does the mechanical mint (ADR-0012).
- Guardrails against noise: the materiality bar above, a cheap active-slug dedup check, and a small
  hardcoded per-invocation cap (overflow surfaced in the run report, not dropped).
- Minting is a metadata-worktree write only — it never disturbs the running change's own
  claim/branch/PR state.
- Shipped end-to-end: the knob in `config.yml.example` + the `.docket.yml` schema block, README, and
  the relaxed convention prose.

## Out of scope

- Auto-grooming or auto-implementing the created stubs (existing `auto_groom` machinery already
  governs what happens next).
- The provenance field itself (#0090; this change consumes it via `depends_on: [90]`, kept a
  separate change — not merged).
- Deduplication beyond a cheap check against existing active titles/slugs.
- Making the per-invocation cap configurable (deferred follow-up).

## Open questions

Resolved at grooming (2026-07-17; rationale + rejected alternatives in the spec's `## Assumptions`):

- **Combine with #0090 or keep separate?** → Keep separate; #0091 consumes #0090's field via
  `depends_on: [90]`. (A cross-change *merge* was out of scope for this groom.)
- **Per-skill granularity or one switch?** → One global boolean `auto_capture`; granularity is a
  reversible follow-up if a need appears.
- **New board flag for auto-created stubs?** → No new board state; they surface as ordinary
  needs-brainstorm and provenance rendering is #0090's territory.

## Reconcile log

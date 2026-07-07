---
id: 42
slug: retune-agent-model-defaults
title: Re-tune default agent models for the Claude 5 lineup (pin explicit versions)
status: proposed
priority: high
created: 2026-07-07
updated: 2026-07-07
depends_on: []
related: [16, 43]
adrs: []
spec: docs/superpowers/specs/2026-07-07-retune-agent-model-defaults-design.md
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
| Spec | [2026-07-07-retune-agent-model-defaults-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-07-retune-agent-model-defaults-design.md) |
<!-- docket:artifacts:end -->

## Why

Every docket subagent wrapper (`agents/docket-*.md`) and the two interactive-skill advisories
pick their model with a **bare alias** — `opus` or `sonnet` — chosen back when Sonnet 4.6 was
current. Sonnet 4.6 is being retired. Because the wrappers use aliases, nothing breaks: `sonnet`
silently becomes **Sonnet 5** and `opus` is now **Opus 4.8**. But for a system whose whole promise
is clone-identical reproducibility, that silent resolution is the problem — the model a pinned
change runs on can shift with no commit recording it — and the tier assignments have never been
re-examined against the new lineup (Opus 4.8, Sonnet 5, Haiku 4.5, Fable 5).

This is the **urgent, standalone re-tune** on today's mechanism, deliberately kept simple so it
lands before the 4.6 sunset. Change #0043 later folds the concrete versions this pins into a single
tier map so the *next* sunset is a one-line edit.

## What changes

Pin **explicit model IDs** in all eight built-in wrappers and both advisories, and re-evaluate the
tiers. Two owner judgments (see the spec for the full table + rationale):

- **Explicit versions, not aliases** — the built-ins become `claude-opus-4-8` / `claude-sonnet-5` /
  `claude-haiku-4-5-20251001`, so a clone runs the exact model the commit records and the 4.6→5 jump
  is a reviewed diff. The per-sunset edit cost is what #0043 removes.
- **`status` demotes to Haiku 4.5** — its dominant work (rendering `BOARD.md`) is mechanical and
  Haiku 4.5 is cheap on the most-frequently-run agent; the residual risk on its git-mutating sweep
  is the accepted trade.

The five no-backstop / adversarial / conflict-&-repair agents stay flagship (Opus 4.8, `xhigh`);
`adr` + `finalize-change` stay one tier down (Sonnet 5, `medium`); only `status` changes tier.
Touch-points: the 8 agent frontmatters, 2 advisory lines, and `tests/test_sync_agents.sh` (relax
the "known alias" assertion to accept full IDs; update the hardcoded per-agent expectations,
incl. `status`).

**Build-time gate:** confirm the agent `model:` field accepts full model IDs (not just aliases);
if the harness rejects them, surface it and stop rather than silently reverting to aliases.

## Out of scope

- Any new mechanism (tier map, manifest, `sync-agents.sh` changes) — that is #0043.
- The TDD build model (SDD implementer/reviewer dispatches) — that is #0044.
- The `docket-convention` `agents:` example, which demonstrates user-facing config syntax with
  short aliases (still valid input) — left unchanged.

## Open questions

- Whether the agent `model:` frontmatter resolves full IDs like `claude-sonnet-5` — verified at
  build (the environment advertises these exact IDs; expected to work).

## Reconcile log

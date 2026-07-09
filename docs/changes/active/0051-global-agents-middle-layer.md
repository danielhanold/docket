---
id: 51
slug: global-agents-middle-layer
title: Make the global agents: block a real middle layer for opted-in repos (restore per-agent fall-through, or seed)
status: proposed
priority: high
created: 2026-07-09
updated: 2026-07-09
depends_on: [50]
related: [45, 46, 48, 50]
adrs: [8, 15, 16, 19]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
<!-- docket:artifacts:end -->

## Why

Live testing of change 0050 (2026-07-09, Daniel) surfaced that the global `agents:` block is
**dead in any repo that opts into per-repo generation**. Expected: a repo's `.docket.yml`
overrides one agent (`status: composer-2.5`) and every unlisted agent falls back to the
global `config.yml` cursor models. Actual: the per-repo pass (change 0048, always-full-set)
commits the FULL agent set resolved from `.docket.yml` + built-ins only, so unlisted cursor
agents were pinned to the built-in **Claude** IDs, and the committed files shadow the
user-level wrappers that do carry the global models.

Two compounding causes:

- Pre-0048, an agent without a per-repo override got **no committed file** and fell through
  to the user-level wrapper (built-in ⊕ global) via harness project-over-user precedence —
  an effective per-repo > global > built-in merge. 0048's always-full-set generation
  removed that fall-through for its self-contained-clone + clone-identical-model goals.
- The 0050 docs (README "Global config", convention Configuration paragraph) state that
  `agents:` "merges field-by-field" across per-repo > global > built-in — true for
  non-opted-in repos and for `skills:`, **false** for `agents:` in opted-in repos. The
  documented promise and the shipped behavior diverge exactly in the tested case.

A stopgap shadowing warning (loud, causal, at `sync-agents.sh` generation time) was added
to PR #59; this change decides and ships the real semantics.

## What changes

To be decided in the brainstorm. Candidate directions, with their trade-offs:

1. **Restore the fall-through** — the per-repo pass commits files ONLY for agents the repo's
   `agents:` block actually overrides; unlisted agents resolve from the user-level wrappers
   (built-in ⊕ global), per machine. Delivers the documented semantics and the per-machine
   expectation (cursor models on laptop A, different models on laptop B, same repo). Costs
   0048's self-contained-clone guarantee (mitigation: `install.sh` is already a prerequisite
   for docket to run at all) and unlisted agents' clone-identical models. Must re-examine the
   Cursor dispatch rule's assumption that every dispatch target exists per-repo, and what
   `--check` covers (only the overridden agents' files).
2. **Seed command** — an explicit `sync-agents.sh` action copies the global harness block
   into the repo's `.docket.yml` `agents:` (committed, diff-visible). Reproducibility fully
   intact; global edits stop propagating to opted-in repos until re-seeded.
3. **Docs-only** — keep 0048 behavior; correct the README/convention `agents:` merge claim
   and rely on the PR #59 shadowing warning.

Whatever wins: update README + convention so the documented `agents:` semantics match the
shipped behavior; align `sync-agents.sh --check` with the chosen generation scope; keep the
ADR-0019 fence intact (no global value may shape committed bytes); record the decision as an
ADR if it revisits 0048 (supersede/update ADR-0008's always-full-set consequence).

## Out of scope

- Any change to `skills:` resolution (already a true runtime three-layer merge).
- Weakening the coordination-key fence (ADR-0019) — global values entering committed files
  remains ruled out.
- New config keys.

## Open questions

- Solo-operator vs. team weighting: is per-machine model preference (fall-through) or
  clone-identical models (0048 status quo / seed) the default docket should pick?
- Under fall-through, does the per-repo Cursor dispatch rule still resolve every
  `subagent_type` (user-level wrappers provide the unlisted agents — verify Cursor merges
  project + user agent registries the way Claude Code does)?
- What does `--check` mean under fall-through — only overridden agents' committed files, plus
  an orphan check for files that should no longer be committed?
- Migration: repos that already committed the full set under 0048 — does the next sync prune
  the no-longer-generated unlisted wrappers (orphan pass), and is that safe on CI?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

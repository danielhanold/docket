---
id: 166
slug: retune-the-interactive-skills-advisory-session-model-recomme
title: Retune the interactive skills' advisory session-model recommendation
status: proposed
priority: low
type: chore
created: 2026-07-28
updated: 2026-08-09
depends_on: []
related: [168, 227]
discovered_from: [164]
adrs: []
spec: docs/superpowers/specs/2026-08-07-retune-the-interactive-skills-advisory-session-model-recomme-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-retune-the-interactive-skills-advisory-session-model-recomme-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-retune-the-interactive-skills-advisory-session-model-recomme-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0164 retunes the nine `agents/docket-*.md` wrapper defaults and the `.docket.example.yml`
mirror, but deliberately leaves one surface untouched: the **advisory** recommended model pinned
in the two interactive skills. `docket-new-change` and `docket-groom-next` get no generated
wrapper (a skill cannot force the session model), so each instead surfaces an advisory
recommendation at startup — and both still name `claude-sonnet-5`, asserted by string at
`tests/test_sync_agents.sh:494-495`.

After 0164 lands, every autonomous wrapper sits on `claude-opus-5` while the two interactive
skills keep recommending a model no other docket surface names. That is the same drift class 0164
exists to fix, one layer over — a user following the advisory gets a session model inconsistent
with the agents the same repo dispatches.

It was excluded from 0164 on purpose: an advisory session-model recommendation is a different
judgment (what a human should run interactively) than a wrapper default (what a machine runs
autonomously), and folding it in would have made a values-only retune also decide that question.

## What changes

Settled by the linked spec (2026-08-07). Both advisories anchor to the shipped claude
`brainstorm-consultant` default in `agents/harness-defaults.yml` — today `claude-opus-5`:
`docket-new-change` recommends `claude-opus-5` with effort left at model default;
`docket-groom-next` mirrors the consultant pair, `claude-opus-5` / `medium`. Each advisory names
its anchor and adds one sentence pointing non-claude harnesses at their harness's
`brainstorm-consultant` row.

The advisory keeps a literal model ID (actionable as a `/model` command), and the drift class is
closed on the test side: the assertions — which have moved to the Task 6 block of
`tests/test_sync_agents_drift_docs.sh` (change 0227 shard), not `tests/test_sync_agents.sh` as
originally cited — stop hardcoding the value and instead resolve it from
`agents/harness-defaults.yml` via `hd_field` at test time, the mirror rule ADR-0039 → ADR-0048 →
ADR-0064 applied to the advisory surface. A future retune that misses the SKILL.md advisories
fails the suite instead of drifting.

Surfaces: the two advisory paragraphs, plus the Task 6 test block. Nothing else moves.

## Out of scope

- The wrapper defaults themselves — that is change 0164 (done).
- Any mechanism for a skill to actually set the session model. The advisory is advisory.
- Per-harness advisory literals for cursor/codex/opencode — a prose row-pointer only, so no new
  drift surfaces are minted.

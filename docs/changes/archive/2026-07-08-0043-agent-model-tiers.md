---
id: 43
slug: agent-model-tiers
title: Model-tier indirection for agent model selection + config-driven advisories
status: killed
priority: medium
created: 2026-07-07
updated: 2026-07-08
depends_on: [42]
related: [16, 17, 42]
adrs: []
spec: docs/superpowers/specs/2026-07-07-agent-model-tiers-design.md
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
| Spec | [2026-07-07-agent-model-tiers-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-07-agent-model-tiers-design.md) |
<!-- docket:artifacts:end -->

## Why

After #0042, a concrete model ID is stamped into eight `agents/docket-*.md` frontmatters and two
advisory lines — but those values cluster into just three groups (Opus 4.8/`xhigh`,
Sonnet 5/`medium`, Haiku 4.5/`medium`), repeated across files. So the **next** model sunset repeats
#0042's ~10-file churn, and a repo that wants "everything one tier cheaper" must override every
agent by hand. Naming the three clusters as **tiers** — with each tier's `{model, effort}` defined
once — makes a sunset (or a whole-repo cost policy) a three-line edit.

## What changes

Introduce a **tier layer** as the source of truth for agent model/effort, resolved into concrete
frontmatter by the existing `sync-agents.sh` (full detail in the linked spec):

- **Built-in tier map + agent→tier manifest** — three tiers (`critical` / `standard` / `economy`)
  and each agent's tier assignment become docket-shipped data. `sync-agents.sh` resolves tier →
  concrete `model:/effort:` and **regenerates the shipped `agents/docket-*.md`** from it (they stay
  committed and directly usable, but are now generated). Editing the 3-line tier→model map + a
  re-run rewrites all eight at once.
- **Config layers** — a new `tiers:` block remaps a tier's model, and `agents: { x: { tier: T } }`
  reassigns an agent; the existing explicit `model:/effort:` override still wins. Precedence stays
  per-repo > global > built-in (reuses `sync-agents.sh`'s layering).
- **Config-driven advisories** — `docket-new-change` / `docket-groom-next` (no agent file) resolve
  their advisory at startup via a `docket-config.sh` tier lookup instead of a hardcoded string.
- **Drift gate** — `sync-agents.sh --check` extends to the now-generated shipped agent files so a
  hand-edit or un-synced manifest change fails CI.

Backward-compatible: a repo that never touches tiers gets byte-identical agent files to #0042, and
existing `agents:` overrides resolve unchanged. Likely warrants a small ADR (shipped agent files
become generated artifacts) — decided at build.

## Out of scope

- The TDD build model (`build.implementer` / `build.reviewer`) — that is #0044, which consumes this
  tier map.
- Re-deciding which agent belongs to which tier beyond #0042's assignments (this makes them
  indirect, not different).
- Per-change / per-run model selection, and user-defined *new* tiers (remap-only in the first cut).

## Open questions

- Manifest format/location (standalone `agents/tiers.yaml` vs a block `sync-agents.sh` reads).
- Final tier names (`critical/standard/economy` proposed).
- The advisory-lookup surface on `docket-config.sh`, incl. graceful offline behavior.

## Reconcile log

## Why killed

Killed in favor of a simpler, harness-neutral direction. Per-agent model configuration already flows end-to-end without a tier layer: the .docket.yml `agents:` block feeds sync-agents.sh, which passes any model-ID string through verbatim (no allowlist — the only alias assertion is a docket-repo test over the shipped built-ins), into agent frontmatter, which the running harness honors (observed reaching Cursor's model selection). The tier indirection this change proposed (critical/standard/economy → concrete Claude models) is Claude-lineage-specific and works against the goal that actually surfaced — pointing docket's agents at an arbitrary, harness-provided model roster, i.e. running docket through Cursor where the models are not Claude's. Chosen mechanism: direct per-agent model IDs in the `agents:` block; no tier abstraction is added. The SDD build-model follow-up (#0044) is reshaped to take direct model IDs rather than this tier map. Decided 2026-07-08, owner.

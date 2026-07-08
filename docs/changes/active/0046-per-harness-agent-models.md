---
id: 46
slug: per-harness-agent-models
title: Per-harness model overrides for docket agents
status: in-progress
priority: medium
created: 2026-07-08
updated: 2026-07-08
depends_on: [45]
related: [16, 42, 43, 44, 45]
adrs: [15]
spec: docs/superpowers/specs/2026-07-08-per-harness-agent-models-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/per-harness-agent-models
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-08-per-harness-agent-models-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-per-harness-agent-models-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

Change #0045 fans the per-repo agent wrappers out to every harness in `agent_harnesses`
(`.claude/agents/`, `.cursor/agents/`, …), but resolves them all from the **one** agent-keyed
`agents:` block — so every harness gets the **same** model string. On a non-Claude harness that
string is wrong: set `implement-next: { model: claude-opus-4-8 }` and the generated Cursor wrapper
carries `claude-opus-4-8`, which Cursor silently ignores (running its house default). The per-repo
config is present but ineffective on the non-Claude harness — the exact reproducibility failure
ADR-0015 set out to fix, just moved one layer down.

Running docket across Cursor / Claude / Codex is the whole motivation, and each exposes a different
model roster. The operator needs to specify **different model IDs per harness**, with a global
(user-level) default per harness and a per-repo override, falling back to a shared neutral default
when a harness isn't called out.

## What changes

Make the `agents:` block **harness-first** (full design in the linked spec):

- Top-level keys become a reserved **`default:`** (neutral fallback) plus **harness names**; each
  holds the familiar agent → `{model, effort}` map. Same shape at both config layers — user/global
  (`~/.config/docket/agents.yaml`) and per-repo (`.docket.yml`).
- Each generated wrapper resolves **field by field**: `agents.<harness>.<agent>` →
  `agents.default.<agent>` → the shipped built-in default. Override only what differs per harness.
- `sync-agents.sh` (readers + both passes + `--check`) moves to this resolution; a **non-Claude**
  harness whose model fell through to `default`/built-in gets a non-fatal warning (likely-wrong ID).
- `docket-convention` and this repo's commented `.docket.yml` example move to the new shape.

## Out of scope

- Any **tier** abstraction — killed with #0043; values stay direct model IDs (ADR-0015).
- A **per-harness catch-all default model** — considered and rejected for layer symmetry + explicit
  per-agent entries.
- **Validating** model IDs against a roster — docket stays passthrough; the fallback warning is the
  only signal and never blocks.
- The **`build:`** SDD-dispatch surface (#0044) and narrowing the **user-level** fan-out (#0045).

## Open questions

- Whether each **non-Claude** harness applies project-over-user precedence natively (as Claude Code
  does); if not, docket must merge global into that harness's project-level file — build-time live
  verification, per the spec.
- Whether the harness-first shape lands as a **new ADR** or a dated `## Update` note on ADR-0015 —
  decided at build.

## Reconcile log

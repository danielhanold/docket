---
id: 45
slug: multi-harness-agent-generation
title: Per-repo agent model config reaches Cursor via multi-harness generation
status: proposed
priority: medium
created: 2026-07-08
updated: 2026-07-08
depends_on: []
related: [16, 42, 43, 44]
adrs: [15]
spec: docs/superpowers/specs/2026-07-08-multi-harness-agent-generation-design.md
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
| Spec | [2026-07-08-multi-harness-agent-generation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-multi-harness-agent-generation-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

docket's per-agent model config (#0016) generates committed `<repo>/.claude/agents/docket-*.md`
from the `.docket.yml` `agents:` block — but the per-repo pass writes `.claude/agents/` **only**.
So running docket through a non-Claude harness (the motivating case: **Cursor** at work) means a
committed `agents:` block reaches nothing the harness reads; ADR-0008's reproducibility guarantee
silently fails there. Verified 2026-07-08: Cursor honors an arbitrary project-level `model:`
(`gpt-5.5-medium-fast`), so the only missing piece is generating the committed wrappers where
Cursor looks.

## What changes

Add a `.docket.yml` `agent_harnesses:` list (default `[claude]`) that per-repo generation fans out
over (full detail in the linked spec; decision in ADR-0015):

- **`sync-agents.sh`** project-level pass generates committed `<repo>/.<H>/agents/docket-*.md` for
  each listed harness (`claude`→`.claude`, `cursor`→`.cursor`, …); default `[claude]` is
  byte-identical to today.
- **`sync-agents.sh --check`** extends its drift diff to every generated per-harness file.
- **`docket-config.sh`** parses/exports `agent_harnesses` (unknown token warned + dropped).
- Model IDs stay **direct, harness-neutral passthrough** — no tier layer (killed #0043).
- Docs (`docket-convention`) cover `agent_harnesses` + the direct-model-ID contract.

## Out of scope

- The `build:` SDD-dispatch surface (#0044) — separate change; shares only the direct-model-ID
  vocabulary.
- Any tier abstraction — killed with #0043.
- Changing the **user-level** pass (it keeps "every present harness"; `agent_harnesses` is
  per-repo / committed only — see spec).
- Validating model IDs — docket stays passthrough (ADR-0008/0015).

## Open questions

- Where `agent_harnesses` is read (docket-config.sh vs direct parse) — lean docket-config.sh.
- Confirm the generated `.cursor/agents/docket-*.md` is read by Cursor identically to the hand-made
  probe file.

## Reconcile log

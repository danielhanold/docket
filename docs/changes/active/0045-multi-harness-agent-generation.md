---
id: 45
slug: multi-harness-agent-generation
title: Per-repo agent model config reaches Cursor via multi-harness generation
status: in-progress
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
branch: feat/multi-harness-agent-generation
pr:
blocked_by:
reconciled: true
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
- **`sync-agents.sh`** parses `agent_harnesses` directly (unknown token warned + dropped) — it is a
  self-contained `.docket.yml` parser and does not use `docket-config.sh`.
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

- Build-time **live verification** (not an automated test): confirm the *generated* wrapper — richer
  than the hand-made probe (`effort:` + `skills:`) — is honored by Cursor for `model:` **and** still
  loads the skill via `skills:` (else the agent runs on the right model but with no docket behavior).

_Resolved: `agent_harnesses` read via direct parse in `sync-agents.sh` (not `docket-config.sh`);
dir creation via sync's `mkdir -p` per harness._

## Reconcile log

### 2026-07-08 — reconcile before build (no drift)

Spec was authored today; verified current reality is unchanged from the snapshot it was drafted
against, so nothing dropped or rescoped:

- **Code base is current.** Live `sync-agents.sh` (177 lines) is byte-identical to `origin/main`.
  Confirmed the spec's edit targets exist as described: the single hardcoded
  `PROJECT_AGENT_DIR="$REPO/.claude/agents"` (line 32), the `project_level_pass` writing only
  there (lines 126–144), and `check_project_level` diffing only `$PROJECT_AGENT_DIR` files
  (lines 146–169). `sync-agents.sh` lives at the **repo root**, not `scripts/`.
- **Related changes settle as expected:** 0016 done (the ADR-0008 agent-layer base), 0042 done
  (recent model-default retune), 0043 **killed** (tier layer — its rejection is load-bearing here),
  0044 proposed (separate `build:` surface, out of scope). No new constraints folded in.
- **ADR-0015 is Accepted** and records the decision this change implements (direct model IDs +
  explicit per-repo harness fan-out).
- **Parser style confirmed:** `docket-config.sh` parses the `board_surfaces` flow-list
  (`[inline]`) by strip-brackets / commas→spaces / trim; the new self-contained `agent_harnesses`
  reader in `sync-agents.sh` mirrors that shape (flow form `[claude, cursor]`, default `[claude]`,
  unknown-token warn-and-drop). `docket-config.sh` stays untouched.
- **Token→dir mapping:** derive valid harness tokens from the existing `HARNESS_AGENT_DIRS`
  vocabulary; project-level dir for token `H` is `$REPO/.<H>/agents` (uniform, incl. `.agents/agents`).
- The Open-questions **live Cursor verification** is a build-time manual check, not an automated
  test — carried forward to the build. This repo dogfoods Claude Code, so its `.docket.yml` stays
  at the default `[claude]` (byte-identical output).

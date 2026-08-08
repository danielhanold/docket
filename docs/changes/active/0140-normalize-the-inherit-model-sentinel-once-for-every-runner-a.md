---
id: 140
slug: normalize-the-inherit-model-sentinel-once-for-every-runner-a
title: Normalize the inherit model sentinel once for every runner adapter
status: in-progress
priority: medium
type: fix
created: 2026-07-27
updated: 2026-08-08
depends_on: []
related: [135, 205]
discovered_from: [135]
adrs: []
spec: docs/superpowers/specs/2026-08-07-normalize-the-inherit-model-sentinel-once-for-every-runner-a-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/normalize-the-inherit-model-sentinel-once-for-every-runner-a
claimed_at: 2026-08-08T07:53:57Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-normalize-the-inherit-model-sentinel-once-for-every-runner-a-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-normalize-the-inherit-model-sentinel-once-for-every-runner-a-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0135 fixed the `inherit` model sentinel in `scripts/runners/cursor.sh` — the adapter was
receiving the literal string and passing `--model inherit[effort=xhigh]` to `cursor-agent`, silently
destroying both the model pin and the effort pin and bypassing the adapter's own WARN branch.

Grooming (2026-08-07) re-verified the stub against today's tree; the defect is smaller than the stub
claimed but still real. The stub's upstream root cause is **already fixed**: since changes 0168 and
0205 (ADR-0067), a generated shim can never carry `--model inherit` — `sync-agents.sh` rejects an
empty-or-`inherit` model for any `runner:`-bearing agent at generation time. What remains is the
**hand-invocation path**, which every adapter contract documents: `cursor.sh` and the newer
`opencode.sh` each normalize `inherit` → no-pin locally, while **`codex.sh` still forwards
`-m inherit` verbatim** to the child (effort survives via its separate
`-c model_reasoning_effort=` flag, but the child gets a non-existent model ID). And
`runner-dispatch.sh` — the layer every adapter is dispatched through — does not normalize at all, so
nothing owns the sentinel: three adapters, two behaviors.

## What changes

Per the linked spec:

- **Single owner:** `scripts/runner-dispatch.sh` normalizes `inherit` → empty right after argument
  parsing, so no adapter re-decides it. Normalize, not reject — ADR-0067 already rejects the
  sentinel at generation time, so a dispatch-time `inherit` is a hand invocation, and the adapters'
  documented model-less hand contract is tolerant.
- **Defensive twins:** `codex.sh` gains the same one-line normalization the other two adapters
  already carry; `cursor.sh`/`opencode.sh` keep theirs, with comments retargeted to point at the
  single owner. Adapters are hand-invocable and directly tested, so removal would regress the 0135
  defect on that path.
- **Docs:** `runner-dispatch.md` records the sentinel rule and the ADR-0015 boundary (`inherit` is
  docket's own sentinel — normalizing it is not model-ID validation, no vendor allowlist);
  `cursor.md` and `codex.md` `--model` bullets record the normalization (folding in 0135's doc
  drift; `opencode.md` already has it).
- **Tests:** dispatch-level and codex-adapter inherit asserts in `tests/test_runner_dispatch.sh`,
  mirroring the existing cursor/opencode adapter inherit tests, which stay untouched.
- **No new ADR** — ADR-0015 and ADR-0067 already carry the decisions this leans on.

## Out of scope

- Changing what `inherit` means, or adding any other sentinel.
- Validating or rewriting real model IDs (ADR-0015 passthrough stands).

## Reconcile log

### 2026-08-08 — build-time reconcile (claim → plan)

Re-read the spec against `origin/main` at `17ff6eed`. **The design holds unchanged; scope is
unchanged.** Every defect the spec asserts was re-verified in today's tree:

- `scripts/runner-dispatch.sh` still does **not** normalize the sentinel — it forwards
  `--model "$MODEL"` verbatim into the adapter argv (now the `args=()` assembly at lines
  131–133, not line 122 as the spec cites; change 0237's run gate was inserted **below** the
  handoff assembly, so the "immediately after argument parsing" insertion point the spec names is
  unaffected).
- `scripts/runners/codex.sh` still forwards `-m inherit` verbatim (line 87). Its `auto` effort
  sentinel **is** normalized (line 80), which makes the model-sentinel asymmetry starker, not
  smaller — the same file already carries the "docket's own sentinel, normalize before mapping"
  pattern for the sibling knob.
- `cursor.sh:82` and `opencode.sh:94` still carry their local normalization with comments claiming
  local ownership.
- Assumption 3 re-verified: the generation-time gate is intact, at
  `runner_config_error()` in **`sync-agents.sh` at the repo root** (not `scripts/` — the spec's
  path is loose; the function is at line ~1038). It rejects an empty-or-`inherit` model for any
  `runner:`-bearing claude agent. Its comment at line 1089 already asserts *"every adapter
  normalizes it to 'no flag'"* — a claim that is **false today for codex.sh** and that this change
  makes true, which strengthens the case for the codex twin rather than weakening it.
- Test homes confirmed: `tests/test_runner_dispatch.sh` is both the facade's and the codex
  adapter's test home (`ADAPTER=scripts/runners/codex.sh`, line 22); the cursor/opencode inherit
  tests are at `test_runner_cursor.sh:75–88` and `test_runner_opencode.sh:104–114` and stay
  untouched.

No scope adjustment, no folded-in work, no new constraints. Nothing dropped as done-elsewhere.

**Auto-capture:** nothing minted — no discovery cleared the six admission gates (the one adjacent
observation, sync-agents.sh's now-true-by-this-change comment, is inside this change's own scope).

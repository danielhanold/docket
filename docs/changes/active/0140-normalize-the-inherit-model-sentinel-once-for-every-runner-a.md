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
claimed_at: 2026-08-08T07:52:04Z
pr:
blocked_by:
reconciled: false
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

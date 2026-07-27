---
id: 140
slug: normalize-the-inherit-model-sentinel-once-for-every-runner-a
title: Normalize the inherit model sentinel once for every runner adapter
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [135]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

Change 0135 fixed the `inherit` model sentinel in `scripts/runners/cursor.sh` — the adapter was
receiving the literal string and passing `--model inherit[effort=xhigh]` to `cursor-agent`, silently
destroying both the model pin and the effort pin and bypassing the adapter's own WARN branch (which
keys on `-z "$MODEL"`). That was I-1 of 0135's final whole-branch review: the change's own thesis
turned on itself, since 0135 exists because a silently-dropped pin was reported as honored.

The fix was made in the Cursor adapter only. **The twin is still live in the Codex adapter**, and
the root cause is upstream of both: `sync-agents.sh`'s `emit_shim` bakes `--model $2` whenever the
resolved override is non-empty, and `model: inherit` is a legal config value (it is exercised in
`tests/test_sync_agents_cursor.sh`). `scripts/runners/codex.sh` then forwards `-m inherit` verbatim.
Codex degrades less badly than Cursor did — it applies effort through its own separate
`-c model_reasoning_effort=` flag, so the effort pin survives — but it still hands a non-existent
model ID to the child.

The asymmetry is the real defect: `emit_cursor_md` normalizes the sentinel, `emit_shim` does not,
and now one adapter normalizes while the other does not. Four sites, two behaviors, one sentinel.

## What changes

Normalize docket's own `inherit` sentinel **once**, at the layer that serves every adapter — most
likely `scripts/runner-dispatch.sh`, which already resolves config and exports the
`DOCKET_RUNNER_CFG_*` environment — rather than per adapter. Then remove the now-redundant
per-adapter normalization, or keep it as a defensive no-op with a comment pointing at the single
owner.

Fold in the small contract drift 0135 left behind: `scripts/runners/cursor.md`'s `--model` bullet
still describes only "verbatim passthrough / omitted implies child default" and does not record that
`inherit` is normalized to no-pin. In a repo where the co-located `.md` is the contract, behavior
that lives only in the script is drift.

Note the ADR-0015 boundary explicitly in whatever lands: `inherit` is **docket's own sentinel**, not
a vendor value, so normalizing it is not model-ID validation and introduces no vendor allowlist.

## Out of scope

- Changing what `inherit` means, or adding any other sentinel.
- Validating or rewriting real model IDs (ADR-0015 passthrough stands).

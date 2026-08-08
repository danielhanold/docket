<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0140 — Normalize the inherit model sentinel once for every runner adapter](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0140-normalize-the-inherit-model-sentinel-once-for-every-runner-a.md)**
<!-- docket:backlink:end -->

# Normalize the `inherit` model sentinel once for every runner adapter — design

**Change:** 0140 · **Date:** 2026-08-07 · **Groomed:** autonomously (auto-groom; assumptions below are the audit trail)

## Problem, re-verified against today's tree

The stub (2026-07-27) predates changes 0168, 0173, 0205/ADR-0067, 0206, and the opencode
adapter. The landscape has shifted; the defect is smaller but still real:

- **The generated path is closed.** `sync-agents.sh` no longer bakes `--model inherit` into a
  shim: `runner_config_error()` (~line 949) rejects an empty **or `inherit`** model for any
  `runner:`-bearing agent at generation time (ADR-0067, change 0205), and provenance filtering
  (change 0168) bakes only user-configured values. The stub's "root cause is upstream in
  `emit_shim`" claim is stale — that root cause is fixed.
- **The hand-invocation path is still asymmetric.** Every adapter `.md` documents direct
  invocation ("the model-less case is reachable only by invoking this adapter by hand").
  `scripts/runners/cursor.sh:82` and `scripts/runners/opencode.sh:94` normalize
  `inherit` → no-pin; **`scripts/runners/codex.sh` does not** — line 87 forwards
  `-m inherit` verbatim to `codex exec`, handing the child a non-existent model ID. Effort
  survives on Codex (separate `-c model_reasoning_effort=` flag), so it degrades less badly
  than the 0135 Cursor defect, but the sentinel still leaks.
- **`scripts/runner-dispatch.sh` — the layer that serves every adapter — does not normalize**
  (it forwards `--model "$MODEL"` untouched at line 122), so nothing owns the sentinel; each
  adapter re-decides it, and codex decided differently. Three adapters, two behaviors.
- **Doc drift.** `scripts/runners/cursor.md`'s `--model` bullet (line 21–22) still says only
  "verbatim passthrough"; `codex.md` likewise. Only `opencode.md` records the sentinel. In this
  repo the co-located `.md` is the contract, so behavior living only in the script is drift.

## Design

### 1. Single owner: `scripts/runner-dispatch.sh`

Immediately after argument parsing (before the handoff at the bottom), add:

```bash
# `inherit` is DOCKET'S OWN no-pin sentinel (never a vendor model ID) — normalized to "no
# model" HERE, the one layer every adapter is dispatched through, so no adapter re-decides
# it. Not model-ID validation: real IDs still pass verbatim (ADR-0015).
[ "$MODEL" = "inherit" ] && MODEL=""
```

Because `[ -n "$MODEL" ]` already gates the `--model` flag in the handoff, normalization makes
the sentinel indistinguishable from "no model supplied" — exactly what the adapters' documented
model-less hand path already handles (cursor/opencode WARN-and-drop effort; codex keeps effort
via its separate flag).

### 2. Adapters keep/gain a one-line defensive twin

- `codex.sh`: **add** `if [ "$MODEL" = "inherit" ]; then MODEL=""; fi` beside its flag
  mapping, with the same comment style as cursor.sh:76–82, pointing at runner-dispatch.sh as
  the owner.
- `cursor.sh` / `opencode.sh`: **keep** their existing normalization; trim the comment to
  point at the single owner ("defensive twin — runner-dispatch.sh owns this; kept because
  adapters are documented as directly invocable").

Rationale for keep-not-remove: every adapter `.md` documents direct hand invocation that
bypasses runner-dispatch, and `tests/test_runner_cursor.sh` / `test_runner_opencode.sh`
exercise the adapters directly. Removing the lines would regress the hand path back to the
0135 defect and force rewriting passing tests. Cost of keeping: one line + comment each.

### 3. Docs

- `runner-dispatch.md`: add the sentinel rule to the argument contract — `--model inherit`
  is docket's own no-pin sentinel, normalized here to "no model" for every adapter; explicitly
  note the ADR-0015 boundary (this is sentinel normalization, not model-ID validation; no
  vendor allowlist is introduced).
- `cursor.md` + `codex.md` `--model` bullets: append the sentinel sentence (mirroring
  `opencode.md:37–42`'s existing wording). This discharges the 0135 doc-drift item the stub
  folds in.

### 4. Tests

- `tests/test_runner_dispatch.sh`: new asserts — dispatch with `--model inherit` invokes the
  adapter with **no** `--model` flag and the literal `inherit` never appears in the adapter's
  argv (mirror the mock-argv pattern of the existing adapter tests).
- New codex-adapter inherit coverage mirroring `test_runner_cursor.sh:75–…` /
  `test_runner_opencode.sh:104–114`: no `-m` flag, literal never reaches the child, effort
  **survives** via `-c model_reasoning_effort=` (codex's asymmetric-but-correct behavior),
  child still runs. Home: **`tests/test_runner_dispatch.sh`** — it is already the codex
  adapter's test home (it sets `ADAPTER=scripts/runners/codex.sh` and carries the adapter's
  flag-mapping and no-model asserts) as well as the facade's, so both new assert groups land
  beside their existing siblings there.
- Existing cursor/opencode inherit tests stay untouched (they now double as the
  defensive-twin regression net).

### 5. No new ADR

ADR-0015 (opaque passthrough) and ADR-0067 (runner-bearing agent requires a user model)
already carry the decisions this leans on; this change is defect symmetry-restoration, not a
new decision. The ADR-0015 boundary is recorded in comments and the `.md` contracts.

## Assumptions

1. **Normalize at runner-dispatch.sh, and keep/add per-adapter defensive twins** (chosen)
   vs. dispatch-only (rejected: adapters are documented hand-invocable; removal regresses the
   hand path to the 0135 defect and rewrites passing tests) vs. adapter-only in all three
   (rejected: no single owner — the exact shape that produced "three adapters, two
   behaviors"). The dispatch line is the owner; the adapter lines are commented as twins.
2. **Normalize `inherit` to no-pin rather than reject it at dispatch/adapter time** (chosen)
   vs. loud abort (rejected: ADR-0067 already rejects it at generation time for shims, so a
   dispatch-time `inherit` is a hand invocation, and the adapters' documented hand contract is
   tolerant of the model-less case; aborting would put a generation-time policy at the wrong
   layer and contradict cursor/opencode's shipped behavior and tests).
3. **The stub's upstream-root-cause scope (`emit_shim` bakes the sentinel) is treated as
   already fixed and dropped from scope** — verified in today's `sync-agents.sh`
   (`runner_config_error` line ~949 rejects `inherit`; provenance filter change 0168). The
   spec re-scopes to: dispatch-layer owner, codex.sh twin, doc drift, tests. If a build-time
   reconcile finds the gate moved, the dispatch-layer normalization still stands alone.
4. **Scope includes opencode.sh** (comment retarget only) even though the stub predates it —
   the stub's title is "for every runner adapter" and leaving opencode's comment claiming
   local ownership would recreate the drift this change exists to end.
5. **No new ADR** (chosen) vs. minting one for "sentinel normalization is owned by the
   dispatch facade" (rejected: refinement of ADR-0015/0067 at implementation altitude, below
   ADR threshold; the `.md` contracts are the right ledger).
6. **Couplings:** `related: [135, 205]` — 135 is the discovering change whose doc-drift this
   folds in; 205/ADR-0067 is the generation-time gate this design leans on. No `depends_on:`
   — both are merged and nothing gates the build.

## Out of scope (unchanged from stub)

- Changing what `inherit` means, or adding any other sentinel.
- Validating or rewriting real model IDs (ADR-0015 passthrough stands).
- Reopening ADR-0067's generation-time gate.

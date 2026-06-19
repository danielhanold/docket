---
id: 26
slug: config-resolution-script
title: Extract config resolution + bootstrap guard into a deterministic script
status: proposed
priority: medium
created: 2026-06-19
updated: 2026-06-19
depends_on: []
related: [11, 22, 25]
adrs: [2, 7]
spec: docs/superpowers/specs/2026-06-19-config-resolution-script-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Every docket skill opens with the **same startup boilerplate** — repair `origin/HEAD`,
read `.docket.yml` authoritatively and resolve every knob, then (docket-mode) evaluate
the `DOCKET`/`LIVE` bootstrap 2×2. It is pure, judgment-free, deterministic work, and it
runs at the front of **all 7 skills** on **every** invocation — so its model-token cost
is paid more often than any other block in docket. It is also **subtle**: the
convention itself flags the `LIVE`-probe traps (probe only the pruned surface; use
`git ls-tree`, never bare `<ref>:<path>`; an unreachable `origin/HEAD` is a hard error,
not `¬LIVE`; abort keys on the `set-head`/fetch return code, never on `git show`).
Re-deriving that from prose every session is both costly and a genuine failure surface.

This is the same extraction the project has already done for the GitHub mirror (0011),
the board render (0022), and the close-out mechanics (0025): lift a deterministic block
out of the model into a tested script. The startup resolver is the highest-frequency
remaining block, so it is the next one to lift.

## What changes

To be built per [the spec](../../superpowers/specs/2026-06-19-config-resolution-script-design.md):

- A single **`scripts/docket-config.sh`** that resolves the config **and** evaluates the
  bootstrap guard, emitting eval-able `KEY=value` output the skill consumes in one turn
  (`eval "$(docket-config.sh --export)"`) — `DOCKET_MODE`, `METADATA_BRANCH`,
  `INTEGRATION_BRANCH` (with the `auto → origin/HEAD → main` resolution), the dirs,
  `FINALIZE_GATE`, `BOARD_SURFACES`, the metadata-worktree path, and a `BOOTSTRAP=`
  verdict (`PROCEED | STOP_MIGRATE | CREATE_ORPHAN`).
- **Read-only by default.** The default invocation only reads; the lone write (create +
  push the empty orphan `docket` on a fresh repo) is **opt-in** (`--bootstrap`), guarded
  to the `¬DOCKET ∧ ¬LIVE` cell. **Fail-closed:** non-zero + diagnostic on a hard error
  (unreachable origin, unresolvable `origin/HEAD`, unparseable yaml); `STOP_MIGRATE` /
  `CREATE_ORPHAN` are valid verdicts at exit 0 for the skill to act on.
- **Rewire the 7 skills'** blocking Step 0 to invoke the resolver instead of restating
  the resolution + 2×2 prose; the skill keeps acting on `STOP_MIGRATE` (refuse-and-point
  at `migrate-to-docket.sh`) and deciding when to opt into the bootstrap write.
- `tests/test_docket_config.sh` with **hermetic local fixtures** (temp repos + bare
  origins, no network): every config permutation, all four bootstrap cells, and the
  fail-closed error paths (unreachable origin, stale-`origin/HEAD`).

## Out of scope

- **The close-out scripts** (0025), the **board / sweep / health checks** (0022 / 0023),
  and the **`github` surface** (already scripted) — separate extractions.
- **`migrate-to-docket.sh`** — unchanged; this script only *detects* the states that
  point at it, it never migrates.
- **Any config or bootstrap *semantics*** — the defaults, the `auto` resolution, the 2×2
  cells, and the error distinctions are reproduced exactly from ADR-0002 + the
  convention, not redesigned.
- **The `agents:` block** — consumed by `sync-agents.sh` at install time, not at skill
  startup; out of the runtime resolver's scope.

## Open questions

Resolved at brainstorm 2026-06-19 — see the spec. None blocking; build-ready. (The
read-only-default + opt-in-`--bootstrap`-write boundary and the `KEY=value` output
contract were settled there.)

## Reconcile log

---
id: 59
slug: board-refresh-surface-gate
title: board-refresh honors board_surfaces — gate BOARD.md regeneration on the resolved surface set
status: in-progress
priority: medium
created: 2026-07-10
updated: 2026-07-10
depends_on: []
related: [58, 11]
adrs: []
spec: docs/superpowers/specs/2026-07-10-board-refresh-surface-gate-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/board-refresh-surface-gate
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-board-refresh-surface-gate-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-board-refresh-surface-gate-design.md) |
<!-- docket:artifacts:end -->

## Why

`board_surfaces: []` is documented to disable the board entirely ("no `BOARD.md`, no mirror"), and
`docket-status`'s Board pass restates that `[]` makes the whole pass a no-op. In practice the
opt-out is **ignored**: multiple skills keep regenerating `BOARD.md` and pushing it to
`origin/docket` even when the resolved surface set is empty.

Root cause is that the no-op is documented centrally but **not enforced at each board-refresh call
site**. `docket-config.sh` resolves `[]` to an empty `BOARD_SURFACES` correctly, and
`render-board.sh` is a pure renderer with no surface gate — but every skill's board step is a raw
`render-board.sh > BOARD.md` redirect followed by an unconditional commit + push. The empty-surfaces
guard lives only in prose inside `docket-status`'s section; the other skills delegate to it as
"refresh via `docket-status`'s Board pass," and an executing agent regenerates and pushes without
ever re-loading the cross-referenced gate.

## What changes

Introduce a deterministic gate so the opt-out is enforced in code, not prose (per the spec):

- **New `scripts/board-refresh.sh` (+ `board-refresh.md` contract)** — the single gated entry point
  for the inline board surface. Takes the caller's resolved `--surfaces "$BOARD_SURFACES"`; writes
  `BOARD.md` only when the `inline` token is present, otherwise touches nothing (no create, no
  write, no delete). It owns the file write so a disabled run never hits the
  `> BOARD.md` truncation trap; it does no git writes.
- **Rewire every board-refresh call site** (`docket-status`, `docket-new-change`,
  `docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next`, and
  two `docket-convention` references) to call `board-refresh.sh` and commit + push **only if
  `BOARD.md` actually changed**. Each skill keeps its existing must-land / best-effort discipline —
  the gate just wraps it. No site names `render-board.sh > BOARD.md` directly afterward.
- **General fix:** keys on the `inline` token, so it is correct for `[]`, `[inline]`, `[github]`
  (github-only ⇒ no `BOARD.md`), and `[inline, github]` alike — the empty-list bug is one instance.
- **New `tests/test_board_refresh.sh`**, including a truncation-trap regression (pre-existing
  `BOARD.md` + empty surfaces ⇒ file left byte-identical).

## Out of scope

- Deleting or cleaning up a stale `BOARD.md` when a repo switches from `[inline]` to `[]` — the
  existing file is left untouched (non-destructive; keeps the helper git-write-free).
- The `github` mirror surface and its existing conditional invocation (already gated separately).
- Any change to `render-board.sh`'s rendering logic or its stdout contract.

## Reconcile log

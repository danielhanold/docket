---
id: 59
slug: board-refresh-surface-gate
title: board-refresh honors board_surfaces — gate BOARD.md regeneration on the resolved surface set
status: implemented
priority: medium
created: 2026-07-10
updated: 2026-07-11
depends_on: []
related: [58, 11]
adrs: []
spec: docs/superpowers/specs/2026-07-10-board-refresh-surface-gate-design.md
plan: docs/superpowers/plans/2026-07-11-board-refresh-surface-gate.md
results: docs/results/2026-07-11-board-refresh-surface-gate-results.md
trivial: false
auto_groomable:
branch: feat/board-refresh-surface-gate
pr: https://github.com/danielhanold/docket/pull/64
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-board-refresh-surface-gate-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-board-refresh-surface-gate-design.md) |
| Plan | [2026-07-11-board-refresh-surface-gate.md](https://github.com/danielhanold/docket/blob/feat/board-refresh-surface-gate/docs/superpowers/plans/2026-07-11-board-refresh-surface-gate.md) |
| Results | [2026-07-11-board-refresh-surface-gate-results.md](https://github.com/danielhanold/docket/blob/feat/board-refresh-surface-gate/docs/results/2026-07-11-board-refresh-surface-gate-results.md) |
| PR | [#64](https://github.com/danielhanold/docket/pull/64) |
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

- **New `scripts/board-refresh.sh` (+ `board-refresh.md` contract)** — the single **git-write-free**
  gated writer for the inline board surface. Takes the caller's resolved `--surfaces
  "$BOARD_SURFACES"`; writes `BOARD.md` only when the `inline` token is present, otherwise touches
  nothing (no create, no write, no delete). It owns the file write so a disabled run never hits the
  `> BOARD.md` truncation trap; it does **no git writes** — the caller keeps its own
  add/commit/push discipline.
- **Rewire the sibling board-refresh call sites** (`docket-new-change`, `docket-groom-next`,
  `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next`, and two
  `docket-convention` references) to call `board-refresh.sh` and commit + push **only if
  `BOARD.md` actually changed**. Each skill keeps its existing must-land / best-effort discipline —
  the gate just wraps it. No site names `render-board.sh > BOARD.md` directly afterward.
- **Compose the helper into change 0058's orchestrator** — refactor `scripts/docket-status.sh`'s
  `board_pass_inline` (which 0058 shipped with its own inline copy of the render-to-tmp + gate)
  to call `board-refresh.sh` for the gated write, then do its own commit/push. This is the
  composition the earlier reconcile anticipated; it leaves **one** gated-write primitive instead of
  two divergent copies. `docket-status/SKILL.md` is **not** edited — since 0058 it merely invokes
  the orchestrator and trusts its exit code, so the gate lives in the script, not the skill prose.
- **General fix:** keys on the `inline` token, so it is correct for `[]`, `[inline]`, `[github]`
  (github-only ⇒ no `BOARD.md`), and `[inline, github]` alike — the empty-list bug is one instance.
- **New `tests/test_board_refresh.sh`**, including a truncation-trap regression (pre-existing
  `BOARD.md` + empty surfaces ⇒ file left byte-identical); update `test_docket_status.sh` to assert
  `board_pass_inline` routes through `board-refresh.sh`.

## Out of scope

- Deleting or cleaning up a stale `BOARD.md` when a repo switches from `[inline]` to `[]` — the
  existing file is left untouched (non-destructive; keeps the helper git-write-free).
- The `github` mirror surface and its existing conditional invocation (already gated separately).
- Any change to `render-board.sh`'s rendering logic or its stdout contract.

## Reconcile log

- 2026-07-11 — **Resumed and rebuilt after #67 (0055) and #68 (0056) merged.** All overlapping slim
  changes are now on `main` (0054/0055/0056; `main` at `5ce5b49`, which also added ADR-0022 + a new
  `docket-brainstorm` skill via 0056). Re-reconciled and rebuilt the branch **last**: reset
  `feat/board-refresh-surface-gate` onto current `main`, carried the 9 non-overlapping files (the
  `board-refresh.sh` primitive, the `docket-status.sh` composition, tests, plan), and **re-applied the
  board-refresh gating to the slimmed skill prose** — the original 0059 wording still matched most
  target lines; `finalize` step 5 (reworded by 0054) and `docket-new-change`'s proposed-kill line
  (reworded by 0055) were adapted, preserving finalize's `is **never** published` assertion. Full suite
  **32/32 green** (was 31 pre-0056). Independent code review: no material findings (the two minor
  observations were verified non-issues — the empty-render guard is unreachable, and the porcelain
  path's mode-agnostic form is correct as written). Force-pushed PR #64 (3 clean commits on `5ce5b49`,
  MERGEABLE/CLEAN); wrote a results file. Status → `implemented`. Ready for the human merge gate.
- 2026-07-11 — **HELD pending PRs #67 (0055) and #68 (0056).** After the 0058 rescope was built and
  the full suite passed green (on a branch rebased onto main@`2a3e20a`), `origin/main` advanced twice
  mid-work: 0058 (#65) then **0054/#66 merged** (slimmed `finalize/SKILL.md`), and a concurrent
  session/loop is actively finalizing. **0059 is the most conflict-prone change in the backlog** — it
  touches ~18 files (every skill), and the still-open slim PRs overlap heavily: **#67 (0055)** shares
  5 files (`docket-auto-groom`, `docket-groom-next`, `docket-implement-next`, `docket-new-change`,
  `references/terminal-close-out.md`) and **#68 (0056)** shares 3 (`docket-convention`,
  `docket-groom-next`, `docket-new-change`). Decision (human, 2026-07-11): **rebase 0059 LAST** — land
  #67 and #68 first, then rebase 0059 once onto the fully-slimmed skills (one clean reconcile instead
  of three). **State of the held work:** the rescoped rework is committed on the local feature branch
  `feat/board-refresh-surface-gate` (tip `1f0b4b0`, rebased onto `2a3e20a` + the docket-status
  composition) but **NOT force-pushed** — PR #64's remote branch stays at `b83bf0c` (pre-rebase).
  **To resume once #67/#68 are done:** (1) rebase `feat/board-refresh-surface-gate` onto the new
  `origin/main`; (2) resolve conflicts in the overlapping skill files by re-applying 0059's
  board-refresh rewiring to the *slimmed* prose — notably `finalize/SKILL.md` step 5's board line
  became a one-liner after 0054, and 0055 will similarly slim the other siblings + `terminal-close-out.md`;
  the docket-status composition commit (`1f0b4b0`) touches `scripts/docket-status.sh`,
  `tests/test_docket_status.sh`, `tests/test_render_board.sh` — none overlap #67/#68, so it replays
  cleanly; (3) re-run the full suite foreground; (4) force-push PR #64; (5) finalize. `reconciled` is
  left `true` for the 0058 pass, but this rebase-onto-final-main is a fresh reconcile the resumer must
  perform (per the implementer's "re-reconcile if origin advanced" rule).
- 2026-07-11 — **Re-reconciled after change 0058 merged (PR #65).** The prior entry's assumption
  ("0058 is still proposed and can later compose the new helper") inverted: 0058 landed first and
  independently built the same gate inside its new `scripts/docket-status.sh` orchestrator —
  `board_pass_inline` already does empty-surfaces short-circuit (`[ -n "$surfaces" ] || return 0`),
  keys on the `inline` token, renders to `BOARD.md.tmp` + `cmp -s` diff (no truncation trap), and
  commits/pushes only on change. 0058 also **deleted** the raw-redirect prose section that PR #64's
  `docket-status/SKILL.md` edit rewrites, and did not touch the sibling skills. Rescope: **drop**
  the `docket-status/SKILL.md` edit (obsolete + conflicting — the skill now just invokes the
  orchestrator); **keep** `board-refresh.sh` as the git-write-free primitive and the sibling-skill
  rewiring (real residual gap — those skills still push `BOARD.md` under `board_surfaces: []`);
  **add** folding `docket-status.sh board_pass_inline` into `board-refresh.sh` to dedup the gate.
  Chose `board-refresh.sh` over routing siblings through 0058's `docket-status.sh --board-only`
  because `--board-only` is a heavyweight self-syncing/self-committing pass that would fight each
  sibling's own commit discipline. PR #64 (branched at `3fad316`, pre-0058) needs a rebase-onto-main
  + rework before it can merge. Status returned to `in-progress`; `reconciled` cleared.
- 2026-07-11 — Reconciled against `origin/main` at `3fad316`, related changes 0058/0011,
  ADR-0012, recent archived change 0053, and current scripts/tests. No design invalidation:
  0058 is still proposed and can later compose the new helper into its orchestrator. Scope was
  sharpened for current reality: only `docket-status` contains the literal raw redirect after
  0053's slimming, while sibling skills delegate to its Board pass; each delegated caller still
  needs an explicit gated/diff-only contract. Added the required update to
  `test_render_board.sh`'s existing wiring sentinel so the full suite accepts the new entry point.

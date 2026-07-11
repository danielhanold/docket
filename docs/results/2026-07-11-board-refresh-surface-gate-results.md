# board-refresh honors board_surfaces — results
Change: #59 · Branch: feat/board-refresh-surface-gate · PR: https://github.com/danielhanold/docket/pull/64 · Plan: docs/superpowers/plans/2026-07-11-board-refresh-surface-gate.md · ADRs: none

## Verify (human)

Automated tests cover the behavior (32/32 suite green); no interactive checks are required to merge.
Optional spot-check if desired:

- [ ] With `board_surfaces: []` set, run a status-writing skill (or `docket-status.sh --board-only`) and confirm **no** `BOARD.md` commit lands on `origin/docket`.
- [ ] With `[inline]` (default), confirm the board still refreshes and pushes as before (behavior-neutral for the default).

## Findings

- **Scope inverted twice against concurrent merges.** 0059 was drafted/reconciled while 0058 was still
  `proposed`, assuming 0058 would later compose this helper. 0058 merged **first** and independently built
  the same gate inside `scripts/docket-status.sh` (`board_pass_inline`), and deleted the raw-redirect prose
  in `docket-status/SKILL.md` that 0059 originally edited. Rescope: **dropped** the `docket-status/SKILL.md`
  edit (0058's orchestrator owns that gate); **kept** `board-refresh.sh` + the sibling-skill rewiring (the
  real residual bug — those skills still pushed `BOARD.md` under `board_surfaces: []`); **added** composing
  `board-refresh.sh` into `board_pass_inline` so `render-board.sh` is reached only via the gated helper.
- **Held, then rebuilt last.** While 0059 was in flight, 0054/0055/0056 (PRs #66/#67/#68) merged and slimmed
  the same skill files 0059 rewires (5-file and 3-file overlap). Per human decision, 0059 was **held** until
  those landed, then rebased **last**: the branch was reset onto current `main` and 0059's board-refresh
  gating was **re-applied to the slimmed prose** (the exact 0059 wording still matched most target lines;
  `finalize` step 5 and `new-change`'s proposed-kill line were adapted to their 0054/0055 rewrites).
- **`board-refresh.sh` vs. `docket-status.sh --board-only`.** Sibling skills call the git-write-free
  `board-refresh.sh` (each keeps its own must-land/best-effort commit discipline) rather than 0058's
  self-syncing/self-committing `--board-only` pass, which would fight their commit sequencing.
- **git pathspec correctness.** `board_pass_inline` detects change via `git -C "$mw" status --porcelain --
  "$CHANGES_DIR/BOARD.md"` — the tree-relative form; a full `$mw/…/BOARD.md` pathspec fatals under `git -C`.
  Independent code review confirmed the refactor (porcelain detection, atomic temp+mv, exit-code
  propagation, preserved push/rebase-retry loop) is correct with no material findings.

## Follow-ups

- None. The `github`-mirror surface and stale-`BOARD.md` cleanup on an `[inline]`→`[]` switch remain
  explicitly out of scope (see the spec's *Out of scope*).

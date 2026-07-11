# Board-render truncation guard — design

**Change:** #0060 · **Slug:** board-render-truncation-guard · **Status:** in-progress (reconciled)
**Related:** #0059 (board-refresh-surface-gate, **merged** PR #64) · **Depends on:** none

> **Reconciled 2026-07-11 against merged #0059.** #0059 already built the atomic-write guard this
> spec originally proposed for `render-board.sh`, but located it in a new `scripts/board-refresh.sh`
> and migrated every Board-pass call site there. This spec is rewritten to its residual: the one
> sub-case #0059 left unclosed — the **non-empty** half of the success test. The original
> `render-board.sh --out` mode and call-site migration are dropped as obsolete. History of the
> original design is in git.

## Problem

The Board pass must never overwrite `BOARD.md` with an empty or partial render — a wiped board
committed to `origin/docket` is a success-shaped silent failure, and an autonomous loop that hit it
would publish it with no human to catch it. Two real, hand-recovered incidents motivated the
original guard (#0052 `/dev/null` misdirection; #0055 unknown-flag → exit 2 → truncated redirect,
146-deletion board wipe on `origin/docket`).

**Current state after #0059 (merged).** `scripts/board-refresh.sh` is now the sole gated writer of
the `inline` `BOARD.md`. It renders `render-board.sh` into a `mktemp` temp file **inside the changes
dir** (same filesystem → atomic `mv`), captures the renderer's exit code, and:

- exit ≠ 0 → prints `render-board.sh failed …; BOARD.md left untouched`, leaves the prior board
  byte-identical, and propagates the code (test `test_board_refresh.sh` #9 pins this — partial
  output + exit 7);
- exit 0 → `chmod 644` the temp file and `mv` it onto `BOARD.md`.

Every call site (docket-status.sh's `board_pass_inline`, and the skills that point at it) routes
through `board-refresh.sh`; the `> BOARD.md` shell redirect is **gone from the codebase**. Both
motivating incidents are therefore structurally prevented already.

**The residual gap.** `board-refresh.sh` gates the `mv` on the exit code **only** — it never checks
that the temp file is **non-empty**. The original design's success test was `exit 0 **AND**
[ -s tmp ]` (below); #0059 implemented only the exit-code half. `render-board.sh` unconditionally
emits a `# Backlog\n\n` header on any clean run, so an exit-0-but-empty render cannot happen today
with the real renderer — but the guard's completeness rests on that implicit coupling. A future
render-board regression (an early `exit 0`, swallowed output) or the `RENDER_BOARD` mock seam could
still `mv` an empty temp file over a good board. This change closes that hole so board-refresh.sh's
no-truncate guarantee is self-contained by construction.

## Decision

Add the missing **non-empty guard** to `scripts/board-refresh.sh`. `render-board.sh` stays an
unchanged pure stdout renderer; all write ownership already lives in `board-refresh.sh` (per #0059).

Between the existing exit-code check and the `mv`, add a second gate: the temp file must be
non-empty (`[ -s "$tmp_board" ]`).

- **Success test** becomes: `render-board.sh` exited 0 **AND** the temp file is non-empty.
  - Success → `chmod 644` + `mv` onto `BOARD.md` (unchanged).
  - Empty (exit 0, zero bytes) → print a distinct `board-refresh: render produced empty output;
    BOARD.md left untouched` to stderr, let the `EXIT` trap remove the temp file, leave `BOARD.md`
    **byte-identical**, and **exit non-zero** so the caller skips its `git add`/commit — mirroring
    the existing non-zero-exit branch.

Exit code for the empty case: a fixed non-zero code (e.g. `1`) distinct from the propagated
renderer code and from the usage `exit 2`, so callers and tests can tell "renderer failed" from
"renderer produced nothing." No structural/format validation beyond non-empty (a `# Backlog` H1
check remains rejected as YAGNI — no incident has hit a non-empty-but-truncated render, and it
would couple the guard to the output format).

`render-board.sh` always emitting the `# Backlog` header means a *legitimately* empty board is
impossible, so the non-empty gate has no false-positive risk — the empty branch is pure
defense-in-depth.

Update `scripts/board-refresh.md` (the contract) to state the two-part success condition
(exit 0 **and** non-empty) and the new empty-render failure row/exit code.

## Testing

Extend `tests/test_board_refresh.sh`, reusing its `RENDER_BOARD` mock seam and hermetic temp-repo
fixtures (the same pattern as its existing test #9):

1. **Empty render leaves target intact** — a `RENDER_BOARD` stub that prints nothing and exits 0;
   seed a pre-existing `BOARD.md` with known bytes; assert after the run that `BOARD.md` is
   byte-identical, board-refresh.sh exits non-zero (the chosen empty-render code), the stderr names
   the empty-output failure, and no temp file leaks in the changes dir.
2. **Existing coverage stands** — the current tests (#9 non-zero-exit/partial-output leaves board
   untouched; happy-path byte-identical write; `chmod 644`; disabled/empty/github-only surfaces;
   arg-validation exit 2) remain as regression guards for the additive change.

## Out of scope

- Board *content* or layout, and any dependency-resolution/readiness logic (render-board's and
  #0059's domain).
- The `github` board surface / mirror path — this is specifically the `inline` `BOARD.md` write.
- `render-board.sh` itself — unchanged pure stdout renderer.
- The `render-board.sh --out` mode and the Board-pass call-site migration from the original design —
  dropped as obsolete (superseded by #0059's `board-refresh.sh` write ownership + call-site
  migration).

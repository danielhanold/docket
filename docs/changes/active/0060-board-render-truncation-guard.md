---
id: 60
slug: board-render-truncation-guard
title: board-refresh.sh must not write an empty BOARD.md — non-empty guard on the atomic write
status: in-progress
priority: medium
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: [59]
adrs: []
spec: docs/superpowers/specs/2026-07-11-board-render-truncation-guard-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/board-render-truncation-guard
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-11-board-render-truncation-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-board-render-truncation-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

The Board pass must never overwrite `BOARD.md` with an empty or partial render — a wiped board
committed to `origin/docket` is a success-shaped silent failure, and an autonomous loop that hit
it would publish it with no human to catch it. Two real close-out incidents motivated this:

- **#0052 finalize** — the render was piped to `/dev/null` (memory:
  `docket-render-board-stdout-redirect`); the staged stale board reported "board unchanged", a
  success-shaped silent no-op, and origin kept showing the change as in-progress.
- **#0055 finalize (2026-07-11)** — an unknown flag (`--adrs-dir`, which render-board does not
  accept, unlike its sibling renderers `render-change-links.sh` / `terminal-publish.sh`) made the
  script exit 2 with empty stdout; the `> BOARD.md` redirect had already truncated the file, and a
  commit wiping the board (146 deletions) landed on `origin/docket` before it was caught by hand.

**Reconcile update (2026-07-11): #0059 landed the bulk of this fix first.** #0059
(board-refresh-surface-gate, now merged, PR #64) introduced `scripts/board-refresh.sh` as the sole
gated writer of the `inline` `BOARD.md`: it renders through a `mktemp` temp file and only `mv`s it
onto `BOARD.md` **after `render-board.sh` exits 0**, otherwise leaving the prior board
byte-identical. Every Board-pass call site (docket-status.sh, and the skills that point at it) now
routes through `board-refresh.sh` — the `> BOARD.md` shell redirect is **gone from the codebase**.
That structurally eliminates both motivating incidents: the `/dev/null`-misdirection class (the
script owns the write; there is no redirect to misaim) and the unknown-flag non-zero-exit class
(caught by both board-refresh.sh's own arg validation and its post-render exit-code gate; test
`test_board_refresh.sh` #9 pins it).

**What #0059 left unclosed** is the one remaining sub-case this change now targets: `board-refresh.sh`
gates the atomic `mv` on `render-board.sh`'s **exit code only**, never on the temp file being
**non-empty**. The design spec (§1.3) committed to `exit 0 **AND** [ -s tmp ]`; #0059 implemented
only the exit-code half. render-board.sh always emits a `# Backlog` header on a clean run, so an
exit-0-but-empty render cannot happen today with the real renderer — but the guard's completeness
rests on that implicit coupling, and a future render-board regression (or the `RENDER_BOARD` mock
seam) could still `mv` an empty temp file over a good board. Closing this makes board-refresh.sh's
no-truncate guarantee self-contained by construction.

## What changes

Add the missing **non-empty guard** to the writer #0059 built (design: linked spec, rescoped):

- In `scripts/board-refresh.sh`, after the `render-board.sh` exit-0 check and before the atomic
  `mv`, additionally require the temp file be non-empty (`[ -s "$tmp_board" ]`). On a zero-exit but
  empty render, leave `BOARD.md` **byte-identical** and exit non-zero (a distinct "empty render"
  failure) so the caller skips its `git add`/commit — mirroring the existing non-zero-exit branch.
- Update `scripts/board-refresh.md` (its contract) to document the non-empty condition alongside
  the existing exit-code condition.
- Add a regression test to `tests/test_board_refresh.sh`: a `RENDER_BOARD` stub that exits 0 with
  empty stdout must leave a pre-existing `BOARD.md` untouched and make board-refresh.sh exit
  non-zero (the belt-and-suspenders companion to the existing test #9 non-zero-exit case).

The `render-board.sh --out` mode and the call-site migration from the original design are **dropped
as obsolete** — #0059 relocated the write ownership into `board-refresh.sh` and migrated every call
site there, so adding `--out` to the pure stdout renderer would be dead code.

## Out of scope

- Changing what the board *contains* or how it is laid out (that is #0059's and render-board's
  domain).
- The `github` board surface / mirror path (this is specifically the `inline` BOARD.md write).
- `render-board.sh` itself — it stays an unchanged pure stdout renderer; the guard lives entirely
  in `board-refresh.sh`, which now owns the write decision.
- Any structural/format validation of the rendered content beyond non-empty (a `# Backlog` H1
  check was considered and rejected as YAGNI in the original spec).

## Open questions

<!-- Resolved during grooming (see spec) and reconcile: fix locus moved from `render-board.sh --out`
     to `board-refresh.sh`'s non-empty guard (0059 relocated the write ownership there); failure
     condition = zero-exit-but-empty render; #0059 is now merged, not in-flight — the reconcile
     folded this change down to the single sub-case 0059 left unclosed. -->

## Reconcile log

### 2026-07-11 — reconciled against merged #0059 (board-refresh-surface-gate)

Re-read change #0060, its spec, related #0059, and current code (`scripts/board-refresh.sh`,
`scripts/render-board.sh`, `scripts/docket-status.sh`, `tests/test_board_refresh.sh`).

**Finding:** #0059 (merged, PR #64) already delivered the bulk of #0060's original design. Its
`board-refresh.sh` is the sole gated writer of `inline` `BOARD.md`, using a `mktemp`+atomic-`mv`
guard that only overwrites the board after `render-board.sh` exits 0 — and every Board-pass call
site now routes through it, so the `> BOARD.md` redirect that caused the #0052/#0055 incidents no
longer exists anywhere. Both motivating incidents are structurally prevented.

**Scope adjustment (not a kill):** the change is *not* obsolete — #0059 implemented only the
exit-code half of the spec's §1.3 success test (`exit 0 AND [ -s tmp ]`). The non-empty half is
genuinely absent from `board-refresh.sh`. Rescoped #0060 down to: add the `[ -s ]` non-empty guard
to `board-refresh.sh` + its contract note + a `test_board_refresh.sh` regression (zero-exit-empty
render leaves BOARD.md untouched). Dropped as obsolete: the `render-board.sh --out` mode and the
call-site migration (#0059 relocated write ownership into board-refresh.sh and migrated the sites).
Not a fundamental invalidation — the change's core intent (the board write is fail-safe by
construction) stands; only the fix locus moved to where #0059 put the write.

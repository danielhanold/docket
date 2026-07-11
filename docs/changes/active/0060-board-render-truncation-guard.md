---
id: 60
slug: board-render-truncation-guard
title: Board pass must not truncate BOARD.md when render-board.sh exits non-zero
status: proposed
priority: medium
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: [59]
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

The Board pass is universally written as `render-board.sh --changes-dir <dir> > docs/changes/BOARD.md`.
The `>` redirect is opened (and truncates the target to zero bytes) by the shell **before**
`render-board.sh` runs — so any non-zero exit or empty-stdout run leaves BOARD.md **emptied**, and
the follow-on `git add && commit` then commits a wiped board. `render-board.sh` is faithful to its
contract (stdout-only, caller redirects), so the fragility lives in the **call pattern**, not the
script.

This has now bitten twice in real close-outs:

- **#0052 finalize** — the render was piped to `/dev/null` (memory:
  `docket-render-board-stdout-redirect`); the staged stale board reported "board unchanged", a
  success-shaped silent no-op, and origin kept showing the change as in-progress.
- **#0055 finalize (2026-07-11)** — an unknown flag (`--adrs-dir`, which render-board does not
  accept, unlike its sibling renderers `render-change-links.sh` / `terminal-publish.sh`) made the
  script exit 2 with empty stdout; the redirect had already truncated BOARD.md, and a commit
  wiping the board (146 deletions) landed on `origin/docket` before it was caught and reverted by
  hand.

Both were recovered manually, but the pattern is a latent footgun on every Board pass in
`docket-status`, `references/terminal-close-out.md`, `docket-new-change`, and the two kill paths —
and an autonomous loop that hit it would silently publish an empty board with no human to catch it.

## What changes

Make the Board pass **fail-safe**: BOARD.md is only ever overwritten by a *successful, non-empty*
render. Candidate shapes for the brainstorm to decide between (not yet chosen):

- Have the Board pass render to a temp file, gate on exit code **and** non-empty output, then move
  into place — codified once so every call site inherits it (a tiny wrapper script, or a
  `render-board.sh --out <file>` mode that writes atomically only on success and never truncates on
  failure).
- Whichever shape wins, update the single-source Board-pass prose (`docket-status`'s *Board* step,
  which the other skills point at) so the `> BOARD.md` redirect pattern is retired everywhere, and
  add a regression test asserting a failed/empty render leaves the prior BOARD.md intact.

## Out of scope

- Changing what the board *contains* or how it is laid out (that is #0059's and render-board's
  domain).
- The `github` board surface / mirror path (this is specifically the `inline` BOARD.md write).
- Reworking `render-board.sh`'s stdout contract for its other callers, beyond adding an opt-in
  safe-write path if that is the chosen shape.

## Open questions

- Fix locus: a shared wrapper the Board pass calls, a new `--out` atomic-write mode on
  `render-board.sh`, or prose-only guidance mandating temp-file-then-move? (Prose-only repeats the
  human-discipline failure that caused both incidents, so lean toward a mechanical guard.)
- Should the guard also treat a *structurally degenerate but non-empty* render (e.g. missing the
  count line) as failure, or only zero-byte / non-zero-exit?
- Does this interact with #0059's surface-gating — i.e. when `inline` is disabled the Board pass
  must legitimately NOT write BOARD.md; the guard must not confuse "intentionally skipped" with
  "failed"?

## Reconcile log

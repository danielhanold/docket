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
spec: docs/superpowers/specs/2026-07-11-board-render-truncation-guard-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-07-11-board-render-truncation-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-board-render-truncation-guard-design.md) |
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

Make the Board pass **fail-safe** by construction (design: linked spec):

- Add an optional `render-board.sh --out <file>` mode: validate args first (so a bad invocation
  writes nothing), render to a `mktemp` file in the target's directory, and atomically `mv` into
  place **only** when the render exits 0 and is non-empty — otherwise leave the prior `BOARD.md`
  byte-identical and exit non-zero. Default stdout behavior is unchanged (purely additive). This
  also removes the `/dev/null`-misdirection class, since the script owns the write.
- Retire the `> BOARD.md` redirect at every Board-pass call site: make docket-status's *Board*
  step (the single source the other skills point at) invoke `--out` explicitly and gate its
  commit on the exit status; re-point any literal redirect examples in the references/kill paths.
- Add regression tests (in `test_render_board.sh`) asserting a failed/empty render — including the
  #0055 unknown-flag case — leaves a pre-existing `BOARD.md` untouched.

## Out of scope

- Changing what the board *contains* or how it is laid out (that is #0059's and render-board's
  domain).
- The `github` board surface / mirror path (this is specifically the `inline` BOARD.md write).
- Reworking `render-board.sh`'s stdout contract for its other callers, beyond adding an opt-in
  safe-write path if that is the chosen shape.

## Open questions

<!-- Resolved during grooming (see spec): fix locus = `render-board.sh --out` atomic-write mode;
     failure test = non-zero exit OR empty output (no structural check); #0059 interaction is
     orthogonal (it gates whether the pass runs; --out gates how it writes) — a reconcile note,
     not a dependency. -->

## Reconcile log

# Board-render truncation guard — design

**Change:** #0060 · **Slug:** board-render-truncation-guard · **Status:** proposed (build-ready)
**Related:** #0059 (board-refresh-surface-gate, in progress) · **Depends on:** none

## Problem

Every docket Board pass is invoked as:

```
render-board.sh --changes-dir <dir> > docs/changes/BOARD.md
```

The shell opens — and **truncates to zero bytes** — the redirect target *before*
`render-board.sh` runs. So any run that exits non-zero or emits nothing to stdout leaves
`BOARD.md` emptied, and the follow-on `git add && commit` publishes a wiped board.
`render-board.sh` is faithful to its stdout-only contract; the fragility lives entirely in the
**call pattern**.

This has caused two real, hand-recovered incidents:

- **#0052 finalize** — the render was piped to `/dev/null`; the staged stale board reported
  "board unchanged" (a success-shaped silent no-op) and origin kept showing the change
  in-progress. (Memory: `docket-render-board-stdout-redirect`.)
- **#0055 finalize (2026-07-11)** — a Board pass passed an unknown flag (`--adrs-dir`, which
  render-board does not accept, unlike its sibling renderers `render-change-links.sh` /
  `terminal-publish.sh`); the script exited 2 with empty stdout, the redirect had already
  truncated `BOARD.md`, and a 146-deletion "wipe the board" commit landed on `origin/docket`
  before it was caught and reverted by hand.

An autonomous loop that hits this would publish an empty board with no human to catch it.

**Scope boundary (from a call-site scan):** `render-board.sh` is the *only* derived-view script
that writes via a shell redirect. `render-change-links.sh` edits its target in place
(`--change-file`, sole writer of the marker block) and `terminal-publish.sh` commits directly.
So the blast radius is exactly the `inline` `BOARD.md` write, across the Board-pass call sites:
the docket-status **Board** step (single source) and the sites that point at it
(`references/terminal-close-out.md` §5, `docket-new-change` §5, `docket-groom-next` §5, and the
two kill paths).

## Decision

Add an optional atomic-write mode to `render-board.sh` and route every Board pass through it, so
`BOARD.md` is only ever overwritten by a *successful, non-empty* render — the guard holds by
construction, not by author discipline (which is what failed twice).

### 1. `render-board.sh --out <file>`

Default behavior (no `--out`) is **unchanged**: render the board to stdout. Existing callers and
tests that redirect stdout keep working; the mode is purely additive.

With `--out <file>`:

1. **Validate arguments first, before touching any file.** Missing/invalid `--changes-dir`, an
   unknown flag, or any other usage error exits non-zero (the existing exit 2) and writes
   nothing — this is the property the shell `>` redirect cannot provide, and the exact class the
   #0055 bad-flag incident hit.
2. Render into a temp file created with `mktemp` **in the same directory as `<file>`**, so the
   final replace is an atomic same-filesystem `mv` (a cross-filesystem `/tmp` temp would make the
   replace non-atomic).
3. **Success test:** the render's exit status is `0` **and** the temp file is non-empty (`[ -s ]`).
   - Success → `mv` the temp file over `<file>` (atomic replace) and exit 0.
   - Failure → remove the temp file, leave `<file>` **byte-identical to its prior contents**, and
     exit non-zero so the caller skips its `git add`/commit.

No structural/format validation of the rendered content beyond non-empty — `render-board.sh` is
deterministic and unit-tested, so a zero-exit, non-empty render is trusted well-formed. (A
`# Backlog` H1 sanity check was considered and rejected as YAGNI: no incident has hit a
truncated-but-non-empty render, and it would couple the guard to the output format.)

This also eliminates the `/dev/null`-misdirection class outright: the script owns the write, so
there is no separate shell redirect a caller can aim at the wrong target.

### 2. Call-site migration

`render-board.sh` and `render-board.md` (its contract) gain the `--out` flag and the
no-truncate-on-failure invariant.

The **single source** is docket-status's **Board** step. Make it explicitly invoke:

```
render-board.sh --changes-dir <metadata-tree>/<changes_dir> --out <metadata-tree>/<changes_dir>/BOARD.md
```

— no `>` redirect — and gate the subsequent `git add`/commit on the script's exit status. The
other Board-pass sites already point at this prose; the implementer greps the skills, references,
and tests for any literal `> …BOARD.md` example and re-points it to the `--out` form. Where the
prior redirect was the *only* thing a call site said about writing the board, the substitution is
mechanical.

### 3. Interaction with #0059 (orthogonal)

#0059 (board-refresh-surface-gate) decides **whether** the Board pass runs at all, gating on the
resolved `board_surfaces` (when `inline` is disabled, `BOARD.md` is legitimately not written).
`--out` governs **how** the render writes *when it does run*. The two are orthogonal: the guard
must never treat "inline intentionally disabled → skipped" as a failure, and it does not — a
skipped pass simply never calls `render-board.sh`.

Both changes edit the docket-status Board prose, so this is a **reconcile note, not a
dependency**. #0059 is in flight (PR #64). Whichever merges second rebases; the implementer's
reconcile pass re-reads the current Board prose at build time and folds `--out` into whatever
#0059 left. Kept as `related`, not `depends_on` — designing ahead of an in-flight sibling is
expected.

## Testing

Extend `tests/test_render_board.sh`, reusing its existing `GIT="${GIT:-git}"` mock seam and temp-repo
fixtures:

1. **`--out` success** — writes `<file>`, exits 0, and the written content is byte-identical to
   the stdout render of the same fixture.
2. **`--out` failure leaves target intact** — seed a pre-existing `<file>` with known bytes, force
   a non-zero render (e.g. bad `--changes-dir`), assert `<file>` is byte-identical afterward and
   the script exits non-zero.
3. **`--out` empty render leaves target intact** — same as (2) for an empty-stdout render.
4. **Arg-validation does not truncate** — a pre-existing `--out` target plus an unknown flag
   (the #0055 regression): exits non-zero and the target is untouched.
5. **Default stdout path unchanged** — existing stdout assertions stand (regression guard for the
   additive change).

## Out of scope

- Board *content* or layout, and any dependency-resolution/readiness logic (render-board's and
  #0059's domain).
- The `github` board surface / mirror path — this is specifically the `inline` `BOARD.md` write.
- Reworking `render-board.sh`'s stdout contract for its other consumers beyond adding the opt-in
  `--out` path.

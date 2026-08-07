<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0259 — Harden render-board: sanitize feeder values and settle the failure contract](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0259-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md)**
<!-- docket:backlink:end -->

# Harden render-board: sanitize feeder values and settle the failure contract — design

**Change:** 0259 · **Date:** 2026-08-07 · **Mode:** auto-groom (defaults committed autonomously; audit trail in `## Assumptions`)

## Problem

`scripts/render-board.sh` has two residual hazards after 0143's fixes (landed via 0157):

1. **Interior control characters in feeder-interpolated values.** The archive sort feeder
   (`render-board.sh:400-405`) interpolates the raw `status:` value into a TAB-joined tuple read
   back with `IFS=$'\t'` (`:388`); a TAB inside the value splits into extra fields and shifts every
   later field right. The same raw value is an `ARC_COUNT` associative-array subscript (`:154`) and
   a `SECTION` subscript (`:141`). 0143 guarded emptiness only; interior TAB/CR survive `field()`.
2. **Exit 0 on malformed input.** The only non-zero exits are CLI-argument errors (exit 2). Files
   with unusable id/status are silently skipped row-by-row; the digest path ends `exit 0` (`:247`).
   A corrupt-but-non-empty render passes `board-refresh.sh`'s gates (exit code + non-emptiness,
   `:110-123`) and `docket-status.sh:407`'s trust of the digest exit status, so silent corruption
   can empty the `ready` queue line and starve the autonomous build loop unremarked.

## Design

### Failure contract: render-complete, then fail loud ("warn AND fail")

The renderer keeps its row-level skip behavior — stdout stays complete-modulo-skipped-rows, in both
`markdown` and `digest` formats — but every skipped/invalid file is **counted and diagnosed**, and a
non-zero count makes the whole run exit non-zero:

- Per malformed file, one stderr line:
  `render-board: malformed change file: <path>: <reason>` (embedded values passed through
  `sanitize()` — see below — so the diagnostic itself cannot smuggle a control character).
- After emission (markdown fall-off-the-end, and the digest block's `exit 0` at `:247` made
  conditional): `exit 3` when any file was malformed, `exit 0` otherwise.
- Exit-code vocabulary, documented in the header comment: `0` clean, `2` CLI-argument error
  (unchanged), `3` malformed change file(s) detected.

Callers need **no new branching** (verified): `board-refresh.sh` already propagates a non-zero
renderer exit and leaves BOARD.md untouched; `docket-status.sh`'s `backlog_pass` already reports
"backlog digest failed; continuing without it" on non-zero, and `digest_only_pass` fails closed on
empty captured output — so `docket-implement-next` can no longer select from a corrupt queue. The
board goes *stale-with-a-named-cause* instead of *corrupt-and-committed*.

### What "malformed" means (closed enumeration, one validation pass)

A single upfront validation pass walks `AFILES` + `ARCFILES` once (before the section/tally loops,
which keep their existing guards untouched) and counts a file malformed when:

- **M1 — unusable id:** `int_field FILE id` is empty (absent, empty, or non-integer).
- **M2 — empty status:** `field FILE status` is empty.
- **M3 — impossible status:** the status is not a member of `DOCKET_STATUSES` (this subsumes any
  status carrying an interior TAB/CR — a control-char value can never match the closed vocabulary,
  so rejection-by-vocabulary IS the sanitization for `status`, applied before the value ever
  reaches a TAB join or an array subscript).

Additionally, **M4 — feeder read-back mismatch** (belt-and-suspenders, archive consumer `:388`):
after the `IFS=$'\t' read -r date id st f` split, a tuple whose `f` is empty/nonexistent or whose
`st` fails the vocabulary check is counted malformed and the row skipped — this catches any future
control-character path into the join (e.g. a TAB in a filename) that M1–M3 cannot see.

Explicitly **not** malformed:

- A vocabulary-valid status in the "wrong" directory (`done` in `active/`, `proposed` in
  `archive/`): legitimate mid-sweep state, and the pre-existing divergence 0143 deliberately left
  to `board-checks.sh`'s `board-row-dropped` (placement is that check's territory; the renderer
  validates vocabulary only).
- Unknown/absent `type:` (change 0127 pinned: a type problem must never affect a row).
- Malformed `created:` (change 0094's `9999-99-99` sentinel already handles it in the ready sort).
- Archive basenames without a parseable date prefix (sort-order oddity, not corruption; out of
  scope).

### Sanitize helper

Copy `board-checks.sh:142`'s three-expansion `sanitize()` (`\t`/`\r` → visible two-character
escapes) into `render-board.sh`, used **only in diagnostics** — never to launder a value into the
feeders, because validation (M3) rejects before interpolation. `board-checks.sh` is not edited.

### Tally subscripts

With the upfront pass rejecting non-vocabulary statuses, `ARC_COUNT["$st"]` (`:154`) and
`SECTION["$st"]` (`:141`) only ever see closed-vocabulary keys. The malformed file is excluded from
the tallies (its existing `continue` guards extended by the membership check), so the 0143-era
"header counts what the table drops" mismatch narrows: it can now arise only from placement
divergence, which stays `board-row-dropped`'s report.

## Tests (`tests/test_render_board.sh`)

- **New fixture — interior-TAB status** (the stub's required regression): an archive file with
  `status: done<TAB>x`. Assert: no field-shifted archive row (0143's corrupt-row ERE), the
  well-formed sibling row still renders, one `malformed change file` stderr line naming the file
  with the TAB rendered as `\t`, exit 3 (markdown and digest), and the digest's `ready` line still
  emitted.
- **New fixture — impossible status** (`status: bogus` in active/): row skipped, diagnostic, exit 3.
- **Contract flips on existing asserts:** the `:1447` block ("render-board exits 0 with a
  malformed-id file present") becomes exit 3 with the same row-skip assertions plus a diagnostic
  assert; the 0143 block's renders now exit 3 (fixtures contain empty id/status), its stdout/digest
  content assertions preserved verbatim except "stderr is clean" becomes "stderr contains only
  `malformed change file` diagnostics", and — a deliberate override of a deliberately-pinned
  assert — the tally asserts narrow: the empty-**id** archive file (`status: done`) is today
  counted by `ARC_COUNT`, which keeps no id guard (pinned at `test_render_board.sh:2156-2162`
  precisely so no "fix" lands silently; this spec is that fix landing loudly). Under M1 exclusion
  `Archive — done (2)` becomes `(1)` and `backlog done 2` becomes `backlog done 1`. The
  empty-status files were already uncounted (guards at `:140`, `:154`) and do not move any tally.
- **Clean-path guard (new assertions):** the golden byte-compare fixture renders with exit 0 and
  empty stderr — currently the golden render (`test_render_board.sh:227-229`) discards stderr and
  never captures the exit code, so both pins are written fresh, not claimed as existing.
- One caller-integration assert in `tests/test_board_refresh.sh` (if not already present): a
  malformed changes dir → board-refresh exits non-zero and BOARD.md is untouched.

## Out of scope

From the stub: replacing the TAB-join protocol structurally; new board surfaces or digest fields.

This spec's own scope-narrowing (not stub content): any edit to `board-checks.sh` (assumption 5);
slug/title content hygiene in digest lines (tail-position, no consumer splits on them).

## Assumptions

1. **Warn-and-skip vs exit-nonzero (the stub's open question): both — render complete output,
   skip bad rows, exit 3.** Grounded on the stub's own declared default: "the renderer detects
   malformed input and exits non-zero with a diagnostic (default posture — callers already gate
   on exit code)". Pure warn-and-skip (exit 0) preserves the exact silent-corruption channel the
   stub exists to close; abort-on-first-error destroys the diagnostics and the partial render a
   human needs to fix the file. The honest cost, stated plainly: today a malformed *archive* file
   leaves the `ready` queue intact (it reads only proposed active files), so under this design one
   malformed file anywhere makes the digest exit 3, trips `digest_only_pass`'s fail-closed
   capture, halts all autonomous selection, and freezes BOARD.md until a human fixes the named
   file — a real availability regression for that case, accepted deliberately: corruption-adjacent
   state should stop the loop loudly rather than let it run beside an unrenderable backlog.
   Rejected: exit-0-warn (status quo silence), fail-fast abort (worse diagnostics, no gain —
   callers discard on any non-zero), per-directory severity (archive-lenient/active-strict splits
   the contract and leaves the board freeze in place anyway via board-refresh).
2. **Exit code 3, not 1 or 2.** 2 is the established CLI-argument-error code (asserted in tests);
   reusing it would conflate operator error with data corruption. 1 is the generic bash failure
   code and would be indistinguishable from an unexpected crash. Rejected: 1, 2.
3. **"Malformed" = M1–M4 only; placement stays board-checks' territory.** A `done` in `active/` is
   legitimate mid-sweep state — failing the renderer on it would make `docket-status`'s own sweep
   window a failure. Rejected: validating directory-appropriate status in the renderer
   (re-litigates 0143's deliberate out-of-scope and races the sweep).
4. **Reject-by-vocabulary instead of sanitize-at-feeder for `status`.** `status` is a closed
   vocabulary; a sanitized `done\tx` is still not a status, so laundering it into the join merely
   converts field-shift corruption into a phantom tally key. Sanitization is reserved for the
   diagnostic surface (board-checks:142 precedent). Rejected: sanitize-then-interpolate,
   write-time-only rejection (the gap is hand-edited manifests, which no write path guards).
5. **Duplicate the 3-line `sanitize()` rather than hoist it to lib/docket-frontmatter.sh.**
   Hoisting edits `board-checks.sh` (explicitly out of scope, 0143 precedent) for a one-line
   dedupe. Rejected: shared-lib hoist.
6. **Existing exit-0-on-malformed test assertions are updated, not preserved.** They pin the old
   contract; the stub's whole second half is that the old contract is the bug. Content-level
   assertions (which rows render, digest completeness) are preserved.
7. **Coupling: `related: [244]`; `depends_on` stays empty.** All stub-referenced changes (0143,
   0155, 0156, 0157, 0104, 0115, 0127) are terminal, but live proposed change 0244 ("one selection
   rule for the four frontmatter read shapes") migrates call sites of the read helpers
   (`field`/`int_field`/`fm_field`) this change's M1–M3 validation reads through, and its census
   names `render-board.sh` as a consumer — a file-collision coupling, recorded as `related: [244]`
   (subject overlap, no
   ordering constraint; reciprocal edit skipped per house practice). Whichever lands second
   reconciles mechanically.
8. **Dependency state note:** `depends_on:` is empty and stays empty; 0143's fixes this builds on
   are merged to main (verified in the working tree at `render-board.sh:139-140,:402-403`).

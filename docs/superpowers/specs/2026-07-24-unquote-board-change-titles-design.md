<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0138 — Board generator wraps each change title in literal double quotes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-24-0138-unquote-board-change-titles.md)**
<!-- docket:backlink:end -->

# Design — Unquote board change titles (change 0138)

## Problem

The rendered board (`BOARD.md`) shows some change titles wrapped in literal double
quotes. Change 0135 renders as:

```
"Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort"
```

instead of the bare title. The quotes are noise in the human-facing view and read as
if they were part of the title.

## Root cause (established from code, not hypothesis)

The board printf uses a bare `%s` for the title (`scripts/render-board.sh` lines 303,
310–321, 399) — it adds **no** quotes of its own. The quotes come from the value the
shared frontmatter reader returns:

- `field()` in `scripts/lib/docket-frontmatter.sh` (lines 32–37) does `sed -n
  "s/^KEY:[[:space:]]*//p"` and trims trailing whitespace only. It returns the raw YAML
  scalar **verbatim**, including any surrounding quote characters.
- The stored files legitimately carry quoted titles. `active/0135-*.md` and
  `active/0137-*.md` have `title: "…"` because the titles contain commas (and an
  apostrophe) — a title with those characters is YAML-serialized with surrounding double
  quotes by the interactive capture path (`0135` was written by `docket-new-change`, per
  its commit `docket(0135): capture Cursor wrapper contract mismatch`). This is **valid
  YAML quoting of a value that needs it**, not garbage in the file.

So the bug is a **read/render defect**: `field()` hands the raw YAML token (with quotes)
to a display context, and the board prints it as-is. It is **not** a change-127
regression (127 anchored `type:` reads to the first frontmatter block and did not touch
title storage or the `field()` quote handling) and **not** an unconditional wrap (only
titles whose value forced YAML quoting are affected). The latent defect became visible
only once titles containing special characters (commas) entered the backlog.

## Shared-path finding (resolves the stub's open question)

Every title consumer reads through the shared frontmatter readers, so a single fix there
covers all of them. Enumerated from code (not three — **six** call sites across five
scripts, via both readers):

- `scripts/render-board.sh:303` (active rows) and `:399` (archive rows) — `field()`.
- `scripts/github-mirror.sh:156` and `:208` — `field()`, passed to `gh issue
  create/edit --title "$title"`, so a quoted title is pushed to the mirror's issue title
  too.
- `scripts/board-checks.sh:169` — `field()`, the pipe-injection check.
- `scripts/render-adr-index.sh:40` — `field()`, **ADR** titles in the ADR index.
- `scripts/render-artifact-backlink.sh:78` — `fm_field()`, the change title stamped into
  each artifact's `docket:backlink` block.
- `scripts/mint-stub.sh:158` (`dup_of`) — `field()` then `slugify` (the strip is a no-op
  there; slugify already collapses surrounding quotes).

Every one is affected in the beneficial direction (bare titles everywhere). This directly
answers the stub's "whether the GitHub mirror shares the same quoting path" — it does, and
so do the ADR index and the artifact backlinks. The correct single-source fix lives in the
shared readers.

## Decision — fix the shared reader

Make the shared frontmatter readers return the **logical scalar value** — strip a single
matched pair of surrounding quotes — so every `field()`/`fm_field()` consumer sees the
value, not its YAML serialization token.

Strip rule (conservative, applied after the existing trailing-whitespace trim):

- Strip **only** when the trimmed value is at least two characters long AND its first and
  last characters are the **same** quote character, either `"` or `'`.
- Strip exactly **one** layer (one leading + one trailing quote).
- Leave the interior bytes byte-for-byte. Do **not** attempt YAML escape processing
  (`\"`, `\\`) — see Assumptions.

Apply the rule to **both** `field()` and `fm_field()` (the anchored sibling reader). This
is not merely twin-hygiene: `fm_field()` has a **live** title consumer today —
`scripts/render-artifact-backlink.sh:78` reads the change title via `fm_field` to stamp
the `docket:backlink` block — so the strip is an active fix on that surface, and the
`field()`/`fm_field()` symmetry (fixing both) additionally forecloses the latent-twin
hazard. Implement once as a small shared unwrap helper the two readers call, so there is a
single definition rather than a duplicated snippet.

**Preserve the `field()` output contract.** `field()` deliberately terminates its output
with a trailing newline (`printf '%s\n'`, `docket-frontmatter.sh:35-37`) because callers
that pipe it directly (e.g. the mermaid done-id list) rely on that separator. The unwrap
must operate on the value **before** that terminator is emitted (or otherwise re-emit it),
never via a `$(...)`-style round-trip that would strip the newline. `fm_field()` prints a
single line without a trailing newline via `print`; keep its shape too.

`list_field()` and `int_field()` call `field()` internally: `list_field` strips `[`…`]`
(list values are never quote-wrapped) and `int_field` gates on `^[0-9]+$` (integers are
never quoted). On the values that actually occur the unwrap is a no-op for both; the only
change is on the never-occurring quoted-list / quoted-int shapes (e.g. a hypothetical
`id: "7"` would now parse as `7` rather than being rejected) — immaterial in practice, and
called out so the reader knows the base reader's output shifted, not just the board.

## What changes

- `scripts/lib/docket-frontmatter.sh` — add the matched-quote unwrap to the value
  returned by `field()` and by `fm_field()` (via one shared helper). Update the header
  contract comment for both to state that a single matched surrounding quote pair is
  stripped (logical scalar returned).
- `scripts/render-board.md` (and any co-located contract for the frontmatter lib, if the
  lib gains one) — note that titles render bare because the reader returns the logical
  value.
- Tests:
  - `tests/test_docket_frontmatter.sh` — unit assertions on the unwrap: double-quoted
    value → bare; single-quoted value → bare; unquoted value → unchanged; value with an
    **interior** quote but no surrounding pair (`Say "hi"`) → unchanged; mismatched /
    unterminated (`"foo`) → unchanged; empty and single-character values → unchanged.
    Add the mirror-image cases for `fm_field()`.
  - `tests/test_render_board.sh` — a change file whose `title:` is YAML double-quoted
    renders **without** the surrounding quotes in the produced `BOARD.md` row (regression
    guard so the quoting cannot silently return).

## Out of scope

- Any other board column formatting or layout change.
- Changing how titles are **written / stored** — quoting at write time is legitimate YAML
  and stays; the fix is purely read-side.
- Full YAML string decoding (escape sequences, block scalars, folded scalars). Titles are
  single-line plain-or-quoted scalars; interior-escape handling is deliberately deferred
  (see Assumptions).

## Assumptions

1. **Fix location: read-side, not write-side.**
   - Chosen: normalize on read (the board is a *derived view*; the logical string is what
     should render).
   - Rejected: force titles unquoted in storage — some titles genuinely require YAML
     quoting to remain valid (commas in flow context, leading indicators, colons), so
     stripping at write could produce invalid/ambiguous YAML. Rejected as incorrect.

2. **Scope: the shared reader, not a board-local unwrap.**
   - Chosen: fix `field()`/`fm_field()` in `scripts/lib/docket-frontmatter.sh` — one
     change fixes board, GitHub mirror, and board-checks, all of which read title through
     the same reader (verified: github-mirror.sh:156/208, board-checks.sh:169). This is
     the single-source fix and directly answers the stub's mirror open question.
   - Rejected: unwrap only inside `render-board.sh` — would leave the mirror still pushing
     quoted issue titles, reintroducing the same defect on a second surface and violating
     DRY across sibling readers.

3. **Quote rule: strip exactly one matched surrounding pair of `"` or `'`, interior bytes
   untouched.**
   - Chosen: strip only when first==last and is a quote and length≥2 — safe on every
     shape (bare values, interior-only quotes, unterminated/mismatched, empty) which all
     pass through unchanged.
   - Rejected: full YAML unescaping (decode `\"`, `\\` inside a double-quoted scalar) —
     correct in the general YAML case but unnecessary for titles (no observed title
     carries an escaped inner quote) and materially more complex/riskier in shell. If a
     title ever needs it, that is a separate, later change. Documented as out of scope so
     the omission is a conscious choice, not an oversight.

4. **Apply to both `field()` and `fm_field()` twins.**
   - Chosen: both, via one shared unwrap helper. `fm_field()` is not a hypothetical case —
     `render-artifact-backlink.sh:78` reads the change title through it today, so fixing
     `fm_field` is an active fix on a live surface; fixing both readers also keeps the twin
     contract consistent and forecloses the latent-twin hazard.
   - Rejected: fix only `field()` (the reader the board title currently uses) — would leave
     the artifact-backlink title (a live `fm_field` consumer) still quoted and leave the
     twins inconsistent.
   - The shared helper must preserve `field()`'s trailing-newline output contract
     (`docket-frontmatter.sh:35-37`) — a `$(...)` round-trip would strip it and break the
     mermaid done-id separator. Pinned in "What changes".

5. **Origin: not a change-127 regression.** The stub hypothesized change 127 introduced
   the quoting. Investigation shows 127 did not touch title storage or `field()` quote
   handling; the defect is latent in `field()` and was merely *exposed* when titles with
   special characters appeared. The fix and its test do not depend on this attribution —
   noted so the human's "confirm the regression origin" question is answered.

## Dependency state

`depends_on: []` — none. No gating dependencies; build-ready once groomed.

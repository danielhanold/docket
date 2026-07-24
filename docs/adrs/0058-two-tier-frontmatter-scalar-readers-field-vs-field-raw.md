---
id: 58
slug: two-tier-frontmatter-scalar-readers-field-vs-field-raw
title: Two-tier frontmatter scalar readers — field() (logical value) vs field_raw() (raw token)
status: Accepted
date: 2026-07-24
supersedes: []
reverses: []
relates_to: []
change: 138
---

## Context

`scripts/lib/docket-frontmatter.sh` provides the shared frontmatter accessors every docket
board/mirror/render script reads through. Change 0138 fixed a defect where a change title that
YAML legitimately double-quotes (because it contains a comma/apostrophe) leaked its literal
surrounding quotes into the rendered board and other title surfaces. The fix made `field()` return
the logical scalar — stripping a single matched pair of surrounding quotes. The change's spec
asserted every `field()` consumer benefits from this. That premise was incomplete:
`scripts/render-learnings-index.sh` reads the finding `hook` via `field()` and then runs its own
full YAML unescaper `dequote()` (handles `\"`→`"`, `\\`→`\` for double-quoted scalars, `''`→`'` for
single-quoted, plus an escaped-closer guard). `dequote` REQUIRES the raw quoted scalar — outer
quotes intact — to detect the quote style and run its escaped-closer check. Stripping the quotes in
`field()` broke it (a full suite run caught the regression: `tests/test_render_learnings_index.sh`
went red).

## Decision

Provide two reader tiers in the shared lib:

- `field()` returns the LOGICAL scalar: a single matched pair of surrounding quotes (`"` or `'`)
  is stripped. Use it for display/comparison of ordinary values (titles, statuses, slugs, paths).
  It is defined as `field_raw()` piped through the `_docket_unwrap_quotes` helper, so the raw read
  has one definition.
- `field_raw()` returns the RAW token with surrounding quotes intact (the pre-0138 `field()`
  behavior). Use it when the caller does its own richer YAML decoding that needs the quote style
  preserved.

Rule: a consumer that performs its own quote/escape decoding must read via `field_raw()`, never
`field()`. The sole such consumer today is `render-learnings-index.sh`'s `dequote()` on the `hook`
field; it now reads via `field_raw()`.

## Consequences

- The board, GitHub mirror, ADR index, board-checks, and artifact backlinks render bare titles
  from a single-source fix in `field()`.
- The learnings index keeps its full `hook` YAML dequote intact via `field_raw()`.
- Future reader-consumers must choose the correct tier; the near-identical `field()`/`field_raw()`
  pair is a deliberate two-tier contract, not accidental duplication — a "simplify these two
  readers into one" refactor would re-introduce the 0138 learnings-hook regression. Unit tests
  (`tests/test_docket_frontmatter.sh`) pin both tiers, and `tests/test_render_learnings_index.sh`
  guards the hook behavior.
- Cost: two readers to understand instead of one; mitigated by the DRY definition (field =
  field_raw + unwrap) and this record.

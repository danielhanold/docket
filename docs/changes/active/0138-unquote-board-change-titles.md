---
id: 138
slug: unquote-board-change-titles
title: Board generator wraps each change title in literal double quotes
status: in-progress
priority: medium
type: fix
created: 2026-07-24
updated: 2026-07-24
depends_on: []
related: []
discovered_from: [127]
adrs: [58]
spec: docs/superpowers/specs/2026-07-24-unquote-board-change-titles-design.md
plan: docs/superpowers/plans/2026-07-24-unquote-board-change-titles.md
results:
trivial: false
auto_groomable: true
branch: feat/unquote-board-change-titles
claimed_at: 2026-07-24T15:14:26Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-24-unquote-board-change-titles-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-24-unquote-board-change-titles-design.md) |
| Plan | [2026-07-24-unquote-board-change-titles.md](https://github.com/danielhanold/docket/blob/feat/unquote-board-change-titles/docs/superpowers/plans/2026-07-24-unquote-board-change-titles.md) |
| ADRs | [ADR-0058](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0058-two-tier-frontmatter-scalar-readers-field-vs-field-raw.md) |
<!-- docket:artifacts:end -->

## Why

The rendered board (`BOARD.md`) shows each change's title wrapped in literal
double quotes. For change 135 the board reads:

```
"Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort"
```

when it should read:

```
Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort
```

The quotes are noise in the human-facing view and read as if they were part of
the title.

Root cause (confirmed from code): the shared frontmatter reader
`field()` in `scripts/lib/docket-frontmatter.sh` returns the raw YAML scalar
**verbatim**, including any surrounding quotes. The board printf uses a bare
`%s` and adds no quotes of its own. Titles with special characters (commas, an
apostrophe) are legitimately YAML-double-quoted at write time, so the reader
hands the quotes straight through to the rendered view. This is a latent
read-side defect exposed once such titles entered the backlog — **not** a change
127 regression (127 did not touch title storage or the reader's quote handling)
and **not** an unconditional wrap.

## What changes

Fix the shared frontmatter reader so titles render as their logical value —
strip a single matched pair of surrounding quotes in `field()` and its anchored
twin `fm_field()`. This is a single-source fix: the board, the GitHub mirror,
the ADR index, and the artifact backlinks all read titles through these readers,
so all render bare from one change. Cover the fix with a reader-level unit test
and a board-render regression test so the quoting cannot silently return. Design,
scope, and the safe-strip rule are in the linked spec.

## Out of scope

- Any other board-column formatting or layout change.
- Changing how titles are written / stored — YAML quoting at write time is valid
  and stays; the fix is purely read-side.
- Full YAML string decoding (escape sequences, block/folded scalars); interior
  escaped-quote handling is deferred.

## Open questions

Resolved during grooming (see spec Assumptions):

- The quoting is a YAML-serialization artifact — titles containing commas / an
  apostrophe are double-quoted at write time, not an unconditional wrap.
- The GitHub mirror **does** share the code path (reads title via the same
  `field()`), as do the ADR index and artifact backlinks — the shared-reader fix
  covers them all.

## Reconcile log

### 2026-07-24 — reconcile pass (implement-next)

Verified the spec against current code (`origin/main` and `origin/docket`); no
scope change.

- `field()` (`scripts/lib/docket-frontmatter.sh:32-37`) and its anchored twin
  `fm_field()` (`:56-70`) are byte-for-byte as the spec's root-cause analysis
  describes — last touched by change 0127 (`298f382e`), which anchored `type:`
  reads and did **not** alter title storage or quote handling, confirming the
  spec's "not a 127 regression" attribution.
- All six enumerated title consumers still read through the shared readers at the
  cited call sites: `render-board.sh:303` (active) / `:399` (archive),
  `github-mirror.sh:156`/`:208`, `board-checks.sh:169`, `render-adr-index.sh:40`
  (`field()`), and `render-artifact-backlink.sh:78` (`fm_field()`). The
  `mint-stub.sh:158` `dup_of` no-op site also holds.
- Both target test files exist: `tests/test_docket_frontmatter.sh`,
  `tests/test_render_board.sh`.
- Auto-capture: the only adjacent item surfaced is the spec's deliberately
  deferred full-YAML-unescaping (assumption 3), framed conditionally ("if a title
  ever needs it") with no current instance — below the materiality bar, so
  reported here rather than minted as a stub.

The design is true as written; proceeding to plan + build.

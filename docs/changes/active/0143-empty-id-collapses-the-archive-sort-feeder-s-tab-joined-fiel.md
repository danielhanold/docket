---
id: 143
slug: empty-id-collapses-the-archive-sort-feeder-s-tab-joined-fiel
title: Empty id collapses the archive sort feeder's TAB-joined fields in render-board.sh
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-28
depends_on: []
related: [115, 144]
discovered_from: [115]
adrs: []
spec: docs/superpowers/specs/2026-07-28-archive-sort-feeder-empty-field-collapse-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-archive-sort-feeder-empty-field-collapse-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-archive-sort-feeder-empty-field-collapse-design.md) |
<!-- docket:artifacts:end -->

## Why

`render-board.sh`'s archive sort feeder joins fields with TAB and reads them back with `IFS=$'\t'`.
Because TAB is IFS-whitespace, an empty `id` field collapses rather than being preserved as empty,
shifting every later field left. That defeats the renderer's own id guard (which sits downstream of
the lossy join) and emits a corrupt archive row of the shape `| [0000](archive/) |  | <date> |`,
plus `printf: … invalid number` and `sed: : No such file or directory` on stderr. The corrupt row
also escapes the `done_seen` counter, widening the `ARCHIVE_RECENT` verbatim window by one per
corrupt file.

An empty `status:` is the same defect twice over: it collapses the same join, and it makes the
per-status tally loops fail with `bad array subscript` — an error that **aborts the whole loop**, so
every file sorted after it is silently dropped from the tally. That bites on both sides: the archive
header undercounts (`Archive — done (1)` for two `done` files), and the active-side `SECTION` loop
drops later changes from the board **and from the `--format digest` `ready` queue line** that
`docket-implement-next` reads. The script still exits 0 throughout, so a corrupt board commits
silently.

The active side's field-shift is genuinely unaffected — its id guard precedes the join.

Found during change 0115 (extending the `board-row-dropped` invariant to `archive/` files) and
deliberately left unfixed there, to keep the check independent of the renderer it is validated
against.

## What changes

- Hoist the emptiness guard above the archive feeder's join, so a file with no usable `id` or no
  `status` is dropped from the archive table instead of shifted into a corrupt row. The delimiter
  stays TAB.
- Guard both per-status tally loops (`ARC_COUNT`, archive; `SECTION`, active) against an empty
  subscript, ending the collateral loop abort. Neither guard changes which well-formed files are
  accounted for.
- Pin all of it with a focused fixture in `tests/test_render_board.sh` (corrupt-row ERE, empty
  stderr, untruncated header count, untruncated digest), leaving the golden byte-compare untouched.

## Out of scope

- The non-empty but non-terminal archive status (e.g. `status: proposed` in `archive/`): a
  pre-existing divergence from `board-checks.sh`'s `renders_row()`, reported by `board-row-dropped`.
- Guarding `ARC_COUNT` on `id`. The resulting header/table mismatch is the state 0115's check exists
  to report, and is preserved deliberately.
- Interior TABs in a frontmatter value (already handled in `board-checks.sh` by `sanitize()`,
  change 0104) — a separate hazard worth its own stub.
- Making `render-board.sh` exit non-zero on a malformed input file — a contract change for every
  caller; its own stub.
- Any edit to `board-checks.sh`, and any delimiter change.

## Open questions

None — resolved at design time; see the spec's `## Assumptions`. 0115 is `done`, so no `depends_on`
is recorded; the stub's original "must land after 0115" rationale was found inverted and dropped.
`related: [144]` records subject overlap on the board/health surface (different file, no ordering
constraint).

---
id: 116
slug: single-source-the-remaining-duplicated-board-vocabularies
title: Single-source the remaining duplicated board vocabularies
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [104]
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

Change 0104 single-sourced the seven-name **status** vocabulary into
`scripts/lib/docket-frontmatter.sh` (`DOCKET_STATUSES_ACTIVE` / `DOCKET_STATUSES_TERMINAL` /
`DOCKET_STATUSES`), on the argument that duplicating a vocabulary makes the checker and the renderer
drift in two directions while only one direction is detectable. That argument was accepted — and it
applies unchanged to **three more vocabularies 0104 left duplicated**. Its whole-branch review found
them:

- **Terminal statuses.** `render-board.sh:125` and `:195` hard-code `case "$st" in done|killed)`
  rather than deriving from `DOCKET_STATUSES_TERMINAL`. A third terminal status added to the array
  would fall through to `count_of "$st"` — which reads the active-only `SECTION` map — yielding `0`,
  so the section is silently skipped while `ARC_COUNT` is ignored. That is exactly the
  silent-emptiness class 0104 exists to kill.
- **The section call list.** `render-board.sh:265-269` hard-codes five `print_section` invocations,
  plus table-shape `case`s at `:235-241` and `:246-261`. A sixth active status renders no section.
  This list is now *load-bearing for correctness*, not just display: 0104's `renders_row` predicate
  claims to mirror it, and that correspondence is asserted only by comment and by fixtures — no
  mechanical guard.
- **The priority vocabulary.** `board-checks.sh` validates `low|medium|high|critical` while
  `render-board.sh:166-171` independently ranks `critical/high/low/*`. Duplicated across the checker
  and the renderer, no shared array, no correspondence test — and 0104 made the checker an active
  consumer of that vocabulary, so the drift now has two directions and a false-finding mode.

0104's array-arity asserts (`= 7`, `= 5`) act as a tripwire that reddens on any vocabulary change, so
a human is forced to look. That is a mitigation, not a guard — and it re-hardcodes the counts a third
time, which is the shape 0104's own design argued against.

## What changes

Finish the job 0104 started: derive each of the three from a single source, and pin the
correspondences that cannot be unified.

- Terminal-status `case` arms → derive from `DOCKET_STATUSES_TERMINAL`.
- The `print_section` call list → drive it from `DOCKET_STATUSES_ACTIVE`, so adding an active status
  cannot silently render nothing. If the per-section table shapes genuinely differ (they do — the
  `case`s at `:235-241` and `:246-261` are a mapping, not a list), keep them as `case` statements and
  pin them by set-equality against the array, the way 0104 pinned `emoji_for` / `label_for_title`.
- The priority vocabulary → one shared array, consumed by both the checker's domain test and the
  renderer's sort rank.

Reuse 0104's guard pattern rather than inventing one: extract the case arms, assert the COUNT found
(a tokenizer that parses nothing passes everything), then assert set equality against the array —
which binds both directions at once — and mutation-test each direction.

Once the `print_section` list is single-sourced, `board-checks.sh`'s `renders_row` predicate can be
tied to it mechanically instead of by comment.

## Out of scope

- Re-litigating 0104's decision to keep `emoji_for` / `label_for_title` as `case` statements. A case
  statement is the right shape for a mapping; they stay pinned by test, not unified.
- The `board-row-dropped` archive-side extension, which is its own follow-up.
- A standing check-id registration guard, tracked separately.

## Open questions

- Does the priority vocabulary want an ordered array (rank is meaningful: `critical > high > medium
  > low`) plus a derived membership test, or two separate constants? The status vocabulary got away
  with one ordered array because display order and membership coincided.
- Are there consumers of these vocabularies outside `render-board.sh` and `board-checks.sh`
  (`github-mirror.sh` renders status labels; `docket-status.sh` maps them)? Derive the site list
  from a whole-repo, case-insensitive grep before scoping — a hand-written list of sites is a floor,
  not the set.

---
id: 116
slug: single-source-the-remaining-duplicated-board-vocabularies
title: Single-source the remaining duplicated board vocabularies
status: in-progress
priority: medium
created: 2026-07-20
updated: 2026-07-22
depends_on: []
related: [111, 115]
discovered_from: [104]
adrs: [55]
spec: docs/superpowers/specs/2026-07-20-single-source-the-remaining-duplicated-board-vocabularies-design.md
plan: docs/superpowers/plans/2026-07-22-single-source-board-vocabularies.md
results:
trivial: false
auto_groomable: true
branch: feat/single-source-the-remaining-duplicated-board-vocabularies
claimed_at: 2026-07-22T11:22:18Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-single-source-the-remaining-duplicated-board-vocabularies-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-single-source-the-remaining-duplicated-board-vocabularies-design.md) |
| Plan | [2026-07-22-single-source-board-vocabularies.md](https://github.com/danielhanold/docket/blob/feat/single-source-the-remaining-duplicated-board-vocabularies/docs/superpowers/plans/2026-07-22-single-source-board-vocabularies.md) |
| ADRs | [ADR-0055](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0055-exhaustive-vocabulary-mappings-require-array-pinned-set-equality.md) |
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

Finish the job 0104 started: derive each vocabulary from a single source, and pin the
correspondences that cannot be unified. Scope settled by a whole-repo derivation at **thirteen**
executable sites across **six** scripts — all of which already source
`scripts/lib/docket-frontmatter.sh`, so the comprehensive scope adds no new plumbing. Full site
table, dispositions and rationale in the spec.

- **Priority** gets the single source it has never had: one ordered `DOCKET_PRIORITIES` array
  (rank order — the index IS the sort rank) plus a named `DOCKET_PRIORITY_DEFAULT`, consumed by
  both `board-checks.sh`'s domain test and `render-board.sh`'s sort ladder.
- **Terminal-status** arms derive from `DOCKET_STATUSES_TERMINAL` — including the archive
  `<details>` block, which spells the vocabulary as `ARC_COUNT[done]`/`ARC_COUNT[killed]` and which
  a literal `done|killed` sweep cannot see.
- **The `print_section` call list** is driven from `DOCKET_STATUSES_ACTIVE`; the per-section table
  and row-format `case`s stay `case`s (they are mappings) and get set-equality guards.
- **Beyond the renderer:** `github-mirror.sh`'s `STATUS_OPTIONS` (a hand-written copy of
  `DOCKET_STATUSES_ACTIVE`, already drifted in order), its close-reason and column-skip cases, and
  the `done|killed` validators in `archive-change.sh`, `terminal-publish.sh` and `docket-status.sh`.

Reuse 0104's guard pattern rather than inventing one: extract the case arms, assert the COUNT found
(a tokenizer that parses nothing passes everything), then assert set equality against the array —
which binds both directions at once — and mutation-test each direction.

Once the `print_section` list is single-sourced, `board-checks.sh`'s `renders_row` predicate is tied
to it structurally (both read the same array) instead of by comment.

An **ADR** records the general rule the guards rest on: a `case` mapping is pinned iff it is
exhaustive over a named array; a vocabulary that is exhaustive but *un-arrayed* gets an array first,
then a pin; a sparse mapping with a correct default is left alone.

**Sequencing.** `related: [111, 115]` remains a build-order note, not a gate. Change 0111 landed
first and added `BOARD_CHECK_IDS` beside the status arrays in `docket-frontmatter.sh`; this change
preserves that vocabulary while extending the same shared block. Change 0115 remains proposed and
independently implementable. Its planned `done|killed`-to-`DOCKET_STATUSES_TERMINAL`
correspondence assert overlaps this change's scope, so if it lands before this PR merges, reconcile
the guard by intent rather than choosing one version.

## Out of scope

- Re-litigating 0104's decision to keep `emoji_for` / `label_for_title` as `case` statements. A case
  statement is the right shape for a mapping; they stay pinned by test, not unified.
- The `board-row-dropped` archive-side extension, which is its own follow-up.
- A standing check-id registration guard, tracked separately.

## Open questions

Both of the stub's questions are **resolved** in the spec (2026-07-20, autonomous groom):

- ~~Ordered array plus derived membership, or two constants?~~ **One ordered
  `DOCKET_PRIORITIES=(critical high medium low)`**, membership derived from it, with
  `DOCKET_PRIORITY_DEFAULT` as a separate named constant. Unlike statuses — where the shared order
  was a coincidence of two independent facts — priority has exactly one ordering and it is
  normative, so a membership-only second constant would protect nothing while reproducing the
  drift being removed. The index doubles as the sort rank, deleting the magic `0/1/3/2` ladder.
- ~~Consumers outside the two scripts?~~ **Yes — six scripts, thirteen sites**, derived by
  whole-repo case-insensitive sweep (see the spec's inventory). The sweep also proved its own
  lesson: a first pass anchored on the literal `done|killed` missed the archive block's
  `ARC_COUNT[done]` spelling, caught only by the critic's semantic read.

One question remains open **for the human, non-blocking** (the spec commits to a default and names
what would reverse it):

- **Is the GitHub Projects v2 column order intentional?** Deriving `STATUS_OPTIONS` from
  `DOCKET_STATUSES_ACTIVE` reorders the options to put `in-progress` before `proposed`. Verified
  safe to change — the options are written only on the auto-create path, so existing boards are
  untouched, and option lookup is by name, not position — and the new order matches `BOARD.md`'s
  section order. If the current order *is* deliberate, say so and the change keeps an explicitly
  ordered constant with a comment instead.

## Reconcile log

### 2026-07-22 — reconciled at claim (docket-implement-next)

Re-derived the inventory semantically against `origin/main` at `c3ad10fb` and re-read related
changes 0111/0115 plus ADR-0049/0050. The thirteen executable sites are still present and the
design remains valid. Three current-reality adjustments were folded in:

1. **0111 is done, not concurrent.** It added `BOARD_CHECK_IDS` to the shared library and expanded
   the correspondence tests. This change must preserve that array and extend the existing
   vocabulary block; no scope is obsolete.
2. **The inventory spans six affected scripts, not seven.** The groomed spec counted
   `render-change-links.sh` because it already sources the shared library, but it owns none of the
   thirteen in-scope enumerations. The no-new-plumbing conclusion still holds for all six actual
   consumers.
3. **0115 remains proposed.** Its planned terminal-status correspondence guard still overlaps this
   work, but there is no branch to compose today.

No adjacent follow-up met the auto-capture materiality bar; these are corrections inside 0116's
existing scope.

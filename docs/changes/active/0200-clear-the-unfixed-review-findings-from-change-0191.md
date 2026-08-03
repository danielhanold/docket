---
id: 200
slug: clear-the-unfixed-review-findings-from-change-0191
title: Clear the unfixed review findings from change 0191
status: proposed
priority: medium
type: chore
created: 2026-08-03
updated: 2026-08-03
depends_on: []
related: []
discovered_from: [191]
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

Change 0191's whole-branch review returned 3 minor findings that were consciously left for
merge-time judgment and never fixed; PR #151 merged with all three outstanding. They are cosmetic
or documentation-level, but each is a real small defect in shipped code and would otherwise be lost
once the results artifact stops being read.

## What changes

1. `scripts/board-checks.sh` — hoist `scalar_form_check(){...}` out of the per-file walk loop (it is
   redefined per file) to sit alongside `renders_row` with the file's other top-level helpers.
2. `scripts/board-checks.sh` + `scripts/board-checks.md` — document the comment-strip asymmetry:
   `blocked_by` is read via `fm_field_raw` (strips a whitespace-preceded inline `#...` comment) while
   `title` is read via `field_raw` (keeps it), so `title: Fix thing # see: notes` flags while the same
   shape on `blocked_by` does not. Add a sentence to the block comment and the doc section, or pin it
   with a fixture.
3. `tests/test_board_checks.sh` — the count-pin provenance comment above the `BOARD_CHECK_IDS` assert
   still reads "13 since change 0117..." while the assert now says 14; reword for the next bumper.

## Out of scope

Any behavior change to the `scalar-form` check itself, and fixing change 0121's flagged title.

## Open questions

Whether finding 2 warrants a pinning fixture in addition to the prose, or prose alone.

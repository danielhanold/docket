---
id: 111
slug: guard-the-board-checks-check-id-enumerations-against-drift
title: Guard the board-checks check-id enumerations against drift
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: [116]
discovered_from: [104]
adrs: []
spec: docs/superpowers/specs/2026-07-20-check-id-vocabulary-drift-guard-design.md
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
| Spec | [2026-07-20-check-id-vocabulary-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-check-id-vocabulary-drift-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

The `board-checks.sh` check-id vocabulary is written down in **three** places and nothing keeps them
in correspondence. Change 0104's reconcile found **both** documentation enumerations already
drifted, in opposite directions, each undetected since the change that introduced the gap:

- `scripts/board-checks.sh:11-12` (the header block) omits `malformed-id`.
- `scripts/docket-status.md:344` (the closed `check <check-id>` enumeration) omits
  `stale-finalize-blocked` — change 0098 shipped that check-id without registering it there.

0104 repairs both instances as a by-product of adding its own check-ids, but it adds no guard, so
the drift recurs on the next check-id anyone ships. The failure is quiet by construction: a missing
registration costs nothing at runtime, the suite stays green, and the enumeration that callers are
told is *closed* silently is not.

This is the same defect class as changes 0107 and 0108 (README config-snippet / config-fence drift
guards), one layer over: a documented vocabulary asserted to be complete, with no test tying it to
the code that emits it.

## What changes

A correspondence guard over the check-id vocabulary, resting on a single **declared, sourceable**
array and a **total** derivation from the emitting code — the two together, not either alone.

- **Declare `BOARD_CHECK_IDS`** in `scripts/lib/docket-frontmatter.sh`, beside `DOCKET_STATUSES`.
  It goes in the lib because `board-checks.sh` is not sourceable, so declaring it there would force
  the guard to *parse source text* for its own expected set; the lib lets the test `source` the real
  runtime array, exactly as the precedent (`tests/test_render_board.sh:1881`) does.
- **Pin all three registration surfaces against it**, plus a fourth set derived from the `emit` call
  sites themselves:
  - `scripts/board-checks.sh`'s header `check-id ∈ {…}` block (retained **verbatim** — `--help`
    reprints it, so it is user-facing output, not an internal comment)
  - `scripts/board-checks.md`'s per-check sections
  - `scripts/docket-status.md`'s `check <check-id> <change-id> <message>` report-line row
  - the `emit` call-site literals in executable code

The correspondence is a **mirror, not a subset**, so per the `correspondence-guard-runs-one-way`
learning it needs both directions and mutation proof in both: a check-id emitted but undocumented
must redden, and a documented check-id nothing emits (a phantom / removed check) must redden too.
Set-equality asserts pin both directions at once. Non-vacuity is asserted at **two** granularities —
distinct ids *and* `emit` call-site count — because an unregistered id added beside an existing
`case` arm leaves the distinct count untouched; that mutant defeated the design's first draft.

Baseline at `HEAD`: 16 `emit` call sites → 11 distinct check-ids; all three surfaces hold exactly
those 11 today.

## Out of scope

- Changing the check-ids themselves, or the findings format.
- The `docket-status` report-line vocabulary beyond the `check` row.
- Any change to `docket-status.sh`'s pipeline or exit-code handling.
- Repairing the two drift instances 0104 already fixes — this is the guard that keeps them fixed.
- Consolidating any *other* duplicated board vocabulary — that is change 0116.

## Notes for the implementer

**Expect a rebase collision with change 0116** (`single-source-the-remaining-duplicated-board-vocabularies`,
in flight, same two scripts). Both changes append a final correspondence section to the tail of
`tests/test_board_checks.sh`, which is the most likely textual conflict. Reconcile by **intent** —
compose both blocks, never choose (`concurrent-edits-compose-at-rebase`). If 0116 relocates the
vocabulary arrays, `BOARD_CHECK_IDS` travels with them and this guard needs only its `LIB=` path
updated; the mirror asserts are unaffected.

## Resolved questions

Both of the stub's open questions were settled at grooming (full reasoning in the spec's
`## Assumptions`):

- **`emit <id>` as the anchor** — insufficient *alone*, and a declared array *alone* is equally
  insufficient. The design takes both: the array is the declared source the docs are pinned against;
  the call-site derivation proves the array still describes the code. The naive `emit <id>` grep was
  mutation-proven **vacuous** for this file's most common shape (it matches no `case`-arm site and
  catches two more only by accident), so the widened, mutation-verified anchor set is normative.
- **`malformed-id`'s framing** — it becomes an ordinary member of the enumeration, since it is a
  first-class emitted check-id already registered on both surfaces. The *semantic* carve-out
  (it reports a malformed **file**, not an unhealthy **change**) survives as a one-sentence reword
  in `board-checks.md`. No taxonomy change, no extractor change.

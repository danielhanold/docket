---
id: 111
slug: guard-the-board-checks-check-id-enumerations-against-drift
title: Guard the board-checks check-id enumerations against drift
status: done
priority: medium
created: 2026-07-20
updated: 2026-07-21
depends_on: []
related: [116]
discovered_from: [104]
adrs: []
spec: docs/superpowers/specs/2026-07-20-check-id-vocabulary-drift-guard-design.md
plan: docs/superpowers/plans/2026-07-21-check-id-vocabulary-drift-guard.md
results: docs/results/2026-07-21-guard-the-board-checks-check-id-enumerations-against-drift-results.md
trivial: false
auto_groomable: true
branch: feat/guard-the-board-checks-check-id-enumerations-against-drift
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/117
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-check-id-vocabulary-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-check-id-vocabulary-drift-guard-design.md) |
| Plan | [2026-07-21-check-id-vocabulary-drift-guard.md](https://github.com/danielhanold/docket/blob/feat/guard-the-board-checks-check-id-enumerations-against-drift/docs/superpowers/plans/2026-07-21-check-id-vocabulary-drift-guard.md) |
| Results | [2026-07-21-guard-the-board-checks-check-id-enumerations-against-drift-results.md](https://github.com/danielhanold/docket/blob/feat/guard-the-board-checks-check-id-enumerations-against-drift/docs/results/2026-07-21-guard-the-board-checks-check-id-enumerations-against-drift-results.md) |
| PR | [#117](https://github.com/danielhanold/docket/pull/117) |
<!-- docket:artifacts:end -->

## Why

The `board-checks.sh` check-id vocabulary is written down in **three** places. Change 0104's
reconcile found **both** documentation enumerations already drifted, in opposite directions, each
undetected since the change that introduced the gap:

- `scripts/board-checks.sh`'s header block omitted `malformed-id`.
- `scripts/docket-status.md`'s closed `check <check-id>` enumeration omitted
  `stale-finalize-blocked` — change 0098 shipped that check-id without registering it there.

**0104 repaired both instances AND shipped a partial guard** (`tests/test_board_checks.sh:941-999`,
labelled in its own comment "tracked structurally as change 0111"). What that guard already covers
is no longer this change's work — see *What changes*. What it does **not** cover is:

**The correspondence runs one way.** Every check-id the script emits must appear in
`board-checks.md` and `docket-status.md` — but nothing asserts the converse. A check-id **retired
from the code and left behind in the docs** — a phantom — passes silently, and so does a typo'd
*extra* entry in either document. Both documents assert their enumeration is *closed*
(`∈ {…}`, and `board-checks.md`'s `### Check enumeration` heading), and a closed set that can
quietly over-claim is exactly the failure `correspondence-guard-runs-one-way` names. The one surface
that *is* pinned in both directions is the script's own header, via 0104's `comm -3` set compare —
which is the shape the two documentation surfaces still need.

This is the same defect class as changes 0107 and 0108 (README config-snippet / config-fence drift
guards), one layer over: a documented vocabulary asserted to be complete, with no test tying it to
the code that emits it.

## What changes

**Reconciled scope (2026-07-21): this change now completes 0104's guard rather than authoring one
from scratch.** 0104 already derives the emitted set from the script and pins the script's own
header against it in both directions. The residual work is the two *documentation* surfaces, the
declared array, and the extractor's own integrity lint.

- **Close the loop on the two documentation surfaces** — the core of what remains. Extract the
  enumeration from each and assert **set equality** with the emitted set, replacing 0104's
  subset-only per-id membership loop:
  - `scripts/board-checks.md`'s per-check sections (`^**\`<id>\`**` heads)
  - `scripts/docket-status.md`'s `check <check-id> <change-id> <message>` report-line row
  A phantom documented id must redden; an emitted-but-undocumented id must keep reddening.
- **Declare `BOARD_CHECK_IDS`** in `scripts/lib/docket-frontmatter.sh`, beside `DOCKET_STATUSES`.
  It goes in the lib because `board-checks.sh` is not sourceable, so declaring it there would force
  the guard to *parse source text* for its own expected set; the lib lets the test `source` the real
  runtime array, exactly as the precedent (`tests/test_render_board.sh:1883-1885`) does. It is the
  declared object the surfaces are pinned against, and it carries the vocabulary into change 0116's
  consolidation already single-sourced.
- **`scripts/board-checks.sh`** — header enumeration retained **verbatim** (`--help` reprints it, so
  it is user-facing output, not an internal comment), gaining only a pointer line naming
  `BOARD_CHECK_IDS`.
- **A no-dynamic-check-id lint** — 0104's extractor keys on the literal shape `emit <id> "`, so an
  `emit "$var"` site would go invisible to the whole guard without reddening anything. The lint
  makes that structurally impossible.
- **`malformed-id`'s framing** — reword `board-checks.md`'s "not counted among the named checks
  above", which contradicts its membership in the closed enumeration this guard now pins.

The correspondence is a **mirror, not a subset**, so per the `correspondence-guard-runs-one-way`
learning it needs both directions and mutation proof in both.

**Already shipped by 0104 — do NOT rebuild:** the emitted-set derivation, the header-enumeration
extraction, their `comm -3` set equality, and the cross-checked non-vacuity asserts. 0104's
`grep -oE 'emit [a-z][a-z-]*[[:space:]]+"'` extractor **supersedes** the widened line-anchor
alternation this change's spec designed; it is shape-anchored rather than position-anchored, so it
catches the `cond || emit …` idiom without an anchor list to maintain. Build on it; do not
re-litigate it.

Baseline at `HEAD` (re-measured 2026-07-21): **17 `emit` call sites → 12 distinct check-ids**; all
four surfaces hold exactly those 12 today.

## Out of scope

- Changing the check-ids themselves, or the findings format.
- The `docket-status` report-line vocabulary beyond the `check` row.
- Any change to `docket-status.sh`'s pipeline or exit-code handling.
- Repairing the two drift instances 0104 already fixed — this is the guard that keeps them fixed.
- **Rebuilding or re-litigating 0104's emitted-set derivation and header set-compare** — this change
  extends that block, it does not replace it.
- Consolidating any *other* duplicated board vocabulary — that is change 0116.

## Notes for the implementer

**The work lands inside 0104's existing correspondence block** (`tests/test_board_checks.sh:941-999`),
not in a new section appended after it. Extending that block keeps one guard with one derivation;
appending a second block beside it would create two extractors of the same set — the very
duplication this change exists to close.

**Change 0116 is `proposed`/build-ready, NOT in flight** (the spec's "in flight" is stale as of
2026-07-21 — nothing is claimed and no branch exists). The rebase-collision warning is therefore
future-facing only: if 0116 is claimed while this builds, reconcile by **intent** — compose both
blocks, never choose (`concurrent-edits-compose-at-rebase`). If 0116 relocates the vocabulary arrays,
`BOARD_CHECK_IDS` travels with them and this guard needs only its `LIB=` path updated; the mirror
asserts are unaffected.

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

## Reconcile log

### 2026-07-21 — reconciled at claim (docket-implement-next)

Verified every factual claim in the change and spec against `origin/main` at `c660535`. Four
material deltas; the change stays valid and build-ready, at **reduced scope**.

1. **⭐ Scope reduction — 0104 already shipped a partial guard.**
   `tests/test_board_checks.sh:941-999` exists at `HEAD`, its own comment naming this change
   ("tracked structurally as change 0111"). It already provides: the emitted-set derivation from
   `board-checks.sh`, extraction of the script header's `check-id ∈ {…}` span, **set equality**
   between those two via `comm -3`, cross-checked non-vacuity asserts, and a whole-word
   **subset-only** registration loop over `board-checks.md` + `docket-status.md`. The spec was
   written as if the guard were zero-way; it is now one-way on the two doc surfaces and two-way on
   the header. Body rewritten to build on that block rather than author a new one.
2. **The spec's `S4` extractor design is superseded.** 0104's
   `grep -oE 'emit [a-z][a-z-]*[[:space:]]+"'` anchors on the call's *syntactic shape*; the spec's
   A1 widened line-position alternation solves the same problem more fragilely (an anchor list to
   maintain). 0104's comment documents the same `cond || emit` blind spot the spec's mutation
   analysis found, and fixes it more durably. The spec's separate **call-site-count** assert was
   load-bearing only for the position-anchored extractor and loses its rationale with a shape-
   anchored one; the distinct-id arity assert on `BOARD_CHECK_IDS` survives on the precedent.
   Assumption A1 amended in the spec.
3. **Baseline moved: 11 ids / 16 call sites → 12 / 17.** Change 0083's `publish-deferred` check
   merged to `main` on 2026-07-20, after the spec's baseline was taken (verified: 11 distinct ids at
   `73895a7`, the pre-0083 tip). The spec was correct when written. 0083 registered its check-id on
   all three surfaces correctly, so all four sets agree at 12 today — the guard inherits a clean
   baseline. Counts updated throughout; `BOARD_CHECK_IDS` must carry 12 members, not 11.
4. **Stale line references repaired.** `docket-status.md:344` → **:352**; the header brace span is
   lines **11-13**; the precedent `source "$LIB"` block is `test_render_board.sh:1883-1885` (spec
   said 1881) and the arity assert is `:1903`.

Also corrected: change 0116 is described as "in flight" in both documents — it is `proposed` and
unclaimed, with no branch. Downgraded to a future-facing caution.

**Residual value confirmed — the change is not obsolete.** Its core deliverable, the reverse
direction on `board-checks.md` and `docket-status.md`, is exactly what 0104 left unbuilt: a check-id
retired from the code but left in either document passes silently today.

No follow-up work met the auto-capture materiality bar at this pass — every delta above is drift
*inside* this change's own scope, which belongs in this log rather than in a new stub.

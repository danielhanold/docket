# Guard the board-checks check-id enumerations against drift — results
Change: #111 · Branch: feat/guard-the-board-checks-check-id-enumerations-against-drift · PR: <url> · Plan: docs/superpowers/plans/2026-07-21-check-id-vocabulary-drift-guard.md · ADRs: none

## Verify (human)

No interactive or manual checks are required — the guard is a static text guard and the full
suite (53/53) is green. The one thing worth an eyeball at the merge gate:

- [ ] `bash scripts/board-checks.sh --help` renders the `check-id ∈ {…}` enumeration followed by
      the new two-line pointer at `BOARD_CHECK_IDS`. (Verified during the build; listed because the
      header block is user-facing output and a brace in the wrong place would corrupt both the help
      text and the guard's own extraction of it.)

## Findings

### Scope moved substantially at reconcile — 0104 had already shipped half this guard

The change was groomed against a codebase where the check-id correspondence was guarded **zero**
ways. By build time, change 0104 had shipped `tests/test_board_checks.sh:941-999`, whose own comment
names this change ("tracked structurally as change 0111"). It already provided the emitted-set
derivation, the script-header extraction, their `comm -3` set equality, and cross-checked
non-vacuity asserts. The change was rewritten at reconcile to *complete* that block rather than
author a new one; a second block beside it would have created two derivations of one set — the exact
duplication this change exists to close. Full detail in the change file's `## Reconcile log` and the
spec's `## Reconcile amendments`.

### The residual value was real, and is what shipped

0104 guarded the two documentation surfaces **subset-only**: every emitted check-id must be
documented, nothing asserted the converse. A check-id retired from the code and left behind in
`board-checks.md` or `docket-status.md` — or a typo'd extra entry — passed green. Both documents
claim a *closed* enumeration, so this is the mirror case of `correspondence-guard-runs-one-way`, not
its `#107` subset exception. Both surfaces are now compared as sets.

### Two deliberate deviations from the spec's design

1. **The spec's `S4` extractor was not built.** Spec §2 mandated a widened line-anchor alternation
   (`(^|;|\)|&&|\|\||then|else|do)[[:space:]]*emit …`), mutation-proven at grooming against the
   naive form. 0104's `grep -oE 'emit [a-z][a-z-]*[[:space:]]+"'` is *shape*-anchored instead — it
   keys on the call's syntax (identifier followed by the quoted change-id argument) rather than on
   a list of tokens that may precede `emit`. It catches the `cond || emit …` idiom by construction
   with no anchor list to maintain. Keeping it was made an explicit build constraint.
2. **The spec's `emit` call-site-count assert (`= 16`) was dropped.** It was load-bearing *only*
   against a position-anchored extractor, where an unregistered `case`-arm site could hide. A
   shape-anchored extractor cannot miss such a site, so the assert lost its rationale. Its real
   concern — a site going invisible to the extractor — is covered instead by the new
   no-dynamic-check-id lint (see below), which catches a strictly harder case.

Both deviations are recorded as normative amendments in the spec so the design record and the code
agree.

### Baseline moved between grooming and build

The spec's baseline (11 distinct check-ids / 16 call sites) was correct when taken. Change 0083's
`publish-deferred` check merged to `main` on 2026-07-20, after that measurement — verified at
`73895a7`, the pre-0083 tip, which shows 11. The build baseline is **12 ids / 17 call sites**.
0083 had registered its check-id on all three surfaces correctly, so the guard inherited a clean
baseline rather than a drift to repair. A textbook `moving-base` instance.

### The no-dynamic-check-id lint is not redundant — proven, not assumed

The whole guard derives its set by matching a *literal* check-id. A site written `emit "$var" …`
matches nothing and leaves the guard's view without reddening anything. Verified by mutation:
changing `board-checks.sh:181` from `emit field-domain` to `emit "$dyn"` leaves the distinct emitted
set at **12** — unchanged, because `field-domain` is emitted at other sites too — so every set
compare stays green while a real emit site has gone dark. Only the call-site count sees it
(17 → 16). This is the single assert in the file that can.

### Whole-branch review: mutation matrix

The reviewer ran nine mutants independently and confirmed each target assert fires **alone** where
that is the claim:

| Mutant | Result |
|---|---|
| retire `merged-orphan` from all emit sites | 5 asserts redden (header, both docs, array, lint) |
| phantom section head in `board-checks.md` | doc set assert reddens — *phantom direction closed* |
| phantom entry in `docket-status.md`'s `{…}` | ds set assert reddens — *phantom direction closed* |
| rename a `board-checks.md` head (count unchanged) | doc set assert reddens |
| rename inside `BOARD_CHECK_IDS` only | array set assert reddens, alone |
| **duplicate** entry in `BOARD_CHECK_IDS` (set still 12) | arity assert reddens **alone** |
| duplicate the `docket-status.md` anchor row | `ds_row_count = 1` reddens **alone** |
| `emit field-domain` → `emit "$dyn"` | lint reddens **alone**; all set compares stay green |
| retitle the `docket-status.md` anchor row | anchor assert fires first, as designed |

Verdict: no way to retire a check-id from the code, or add one to either document's enumeration,
and still get a green suite.

### Three review findings fixed before the PR

1. **The change's own remedy prose was wrong** (blocking). Adding a check-id now requires editing
   five places, but `board-checks.md`'s `## Invariants` bullet and the `BOARD_CHECK_IDS` comment
   both said "all four" — omitting the array this change had just added. Reworded to "the array plus
   the four surfaces it is pinned against". Notable because a drift-guard change had drifted in its
   own procedure documentation on first write.
2. **The `ds_ids` extractor was unanchored.** `sed -E 's/.*\{([^}]*)\}.*/\1/'` binds the *last*
   brace group on the line; a second `{…}` on that row would silently shift extraction. Now anchored
   on `∈ {`, matching its sibling `header_ids` extractor. Verified against a decoy brace group.
3. **The emit counters' line-orientation is now documented** in their own comment: a backslash-
   continued `emit` call, or an inline trailing `# … emit …`, produces a false red. Both fail loud,
   so this was a comment fix, not a logic change.

## Follow-ups

Two known limitations, both surfaced by the whole-branch review, both **deliberately not filed as
changes** — each is either a settled design rejection or fail-loud polish below the bar for its own
PR. Recorded here so a future reader finds the reasoning rather than re-deriving it:

- **The guard is syntactic, not behavioral.** A check-id emitted only from dead or unreachable code
  stays in `$emitted` and keeps every surface green. Spec A6 rejected a behavioral guard
  deliberately: reaching all 12 checks through fixtures needs 12 hand-built fixture trees, and that
  corpus would itself be an enumerated floor. The limitation is inherited from 0104's extractor,
  which this change was constrained not to touch. Revisit only if a check-id is ever actually
  retired to dead code.
- **`doc_ids` scans `board-checks.md` file-wide**, not just under `### Check enumeration`. All 12
  matches sit under that heading today, but a future non-check-id token written as a line-start
  `**\`…\`**` (e.g. `**\`--strict\`**`) would spuriously redden the set compare. Fail-loud, and
  cheaply fixed in place when it happens. Change **0116**
  (`single-source-the-remaining-duplicated-board-vocabularies`) is the natural home if it is ever
  worth scoping the extractor to its section.

**Composition note for change 0116:** the check-id vocabulary now arrives already single-sourced in
`lib/docket-frontmatter.sh`, in the same shape as `DOCKET_STATUSES` — a done deal rather than a
decision. If 0116 splits the lifecycle/board vocabularies out into a dedicated lib,
`BOARD_CHECK_IDS` travels with them and this guard needs only its `LIB=` path updated; the mirror
asserts are unaffected. The accepted impurity (a check-id living in a lib named "frontmatter") is
documented at the array's declaration and is squarely inside 0116's charter.

**No ADR was minted.** The two decisions a future reader might question — the array's placement in
the frontmatter lib, and the supersession of the spec's extractor design — are recorded at the point
of use (a comment at the array's declaration) and in the spec's normative `## Reconcile amendments`
respectively. Neither establishes a new repo-wide rule that a decision record would carry better
than the code comment already does.

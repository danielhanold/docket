# Guard the board-checks check-id vocabulary against drift — design

Change: 0111 · `guard-the-board-checks-check-id-enumerations-against-drift`
Date: 2026-07-20 · Groomed autonomously (`docket-auto-groom`; revised once after the critic pass)

## Problem

`board-checks.sh`'s check-id vocabulary is written down in three places and nothing binds any of
them to the set the script actually emits. Change 0104's reconcile found drift in **both**
directions simultaneously — the script header omitted `malformed-id`, and `scripts/docket-status.md`
omitted `stale-finalize-blocked` (shipped by change 0098 and never registered). 0104 repaired both
instances; it added no guard, so the next check-id anyone ships re-opens the gap. The failure is
silent by construction: a missing registration costs nothing at runtime and the suite stays green,
while the enumeration callers are told is *closed* is not.

Today the correspondence is guarded **zero**-way. The learning `correspondence-guard-runs-one-way`
records this exact case (#104 war story) and prescribes the remedy used here: *derive the true set
from the emitting code before trusting any mirror.*

**Baseline, verified at `HEAD` (independently re-verified by the critic pass):** 16 `emit` call
sites in `board-checks.sh` → **11 distinct check-ids**. All three registration surfaces hold exactly
those 11 today (orders differ; `stale-finalize-blocked`/`merge-gate-stall` are transposed between
the two enumerations, which sorted-set comparison absorbs).

## The design

One declared, **sourceable** source of truth; four extracted surfaces; mirror equality in both
directions on every edge.

### 1. `BOARD_CHECK_IDS` — the declared vocabulary, in the sourceable lib

Declare the array in `scripts/lib/docket-frontmatter.sh`, alongside `DOCKET_STATUSES`:

```sh
# The CLOSED board-checks check-id vocabulary. Sole source of truth for the set board-checks.sh
# emits; the guard in tests/test_board_checks.sh pins it against all three registration surfaces
# (board-checks.sh's --help header, board-checks.md, docket-status.md) in both directions.
BOARD_CHECK_IDS=(board-row-dropped broken-plan-results broken-spec dep-cycle field-domain
                 malformed-id merge-gate-stall merged-orphan stale-finalize-blocked
                 stale-in-progress unknown-commit-ref)
```

`board-checks.sh` already sources this lib at line 52, well before `emit()` (line 71) and its first
call (line 138) — the array is in scope with no new sourcing cost and no ordering hazard.

**Why the lib and not `board-checks.sh` itself:** `board-checks.sh` is *not sourceable* (it parses
argv, validates, and runs the whole walk on source), so declaring the array there would force the
guard to obtain its expected set by **parsing source text** — introducing a tokenizer that can
itself diverge from what bash actually assigns. The lib is sourceable, so the guard does
`source "$LIB"` and reads the **real runtime array**, exactly as the precedent
(`tests/test_render_board.sh:1881-1883`) does for `DOCKET_STATUSES`. This deletes a whole class of
extractor fragility rather than relocating it. Accepted impurity: the lib's name says "frontmatter"
and a check-id is not a frontmatter field — noted in a comment, and squarely inside change 0116's
charter to rationalise (see A2).

### 2. The correspondence guard — `tests/test_board_checks.sh`

`source` the lib for `S0`, extract four surfaces, assert each equals `S0` as sorted-set string
equality (which pins both directions at once, per the `test_render_board.sh:1925` precedent):

| Set | Surface | Extractor |
|---|---|---|
| `S0` | `BOARD_CHECK_IDS` | `source "$LIB"` — the runtime array, no parsing |
| `S1` | `board-checks.sh`'s header `check-id ∈ {…}` block | awk the brace span (**3 physical lines**), strip `^#[[:space:]]*` from each line *inside* the span, join to one line, split on `,`, trim |
| `S2` | `board-checks.md` per-check sections | `grep -oE '^\*\*`[a-z-]+`\*\*'`, then strip `**` and backticks |
| `S3` | `docket-status.md`'s `check <check-id> …` table row | anchor `^\| \`check <check-id>` (exactly one such row, line 344 — the whole `{…}` set is on one physical line), take the brace span, split on `,`, strip the trailing `}` |
| `S4` | the `emit` call sites in executable code | comment-strip, then the anchored `grep -oE` given in the fenced block below (**not** inline — see the note) |

`S4`'s extractor, given verbatim in a fenced block because a markdown table cell mangles its
alternation (pipe-escaping turns it into an empty subexpression, and `grep` then hard-errors to
rc 2 / zero matches — a dead guard on first run):

```sh
grep -vE '^[[:space:]]*#' scripts/board-checks.sh |
  grep -oE '(^|;|\)|&&|\|\||then|else|do)[[:space:]]*emit [a-z][a-z-]*'
```

**`S4`'s anchor set is load-bearing and was proven by mutation, not assumed.** The naive form
(`^`/`;`/`| ` only) is **vacuous for this file's most common emit shape**: it matches none of the
`case`-arm sites (lines 181, 188, 192), and it matches `broken-spec` (197) and
`broken-plan-results` (206) only by the accident that `||` contains a pipe-then-space. A mutant
adding an unregistered `) emit encoding-domain …` case arm passes the naive extractor with all
asserts green — the exact drift this change exists to catch, shipping undetected. The widened anchor
set above measures 11 distinct ids / 16 call sites on the real file and **12 / 17 on that mutant**.
The `emit(){` definition line is excluded by requiring the trailing space after `emit`; this must
stay explicit, since a looser pattern injects a phantom token.

Asserts:

- `S1 == S0`, `S2 == S0`, `S3 == S0`, `S4 == S0` — four mirror equalities.
- **Non-vacuity, at two granularities.** Each of `S0..S4` yielded exactly 11 **distinct** ids — five
  asserts, including the array-arity assert on `S0` itself, matching the precedent's
  `[ "${#DOCKET_STATUSES[@]}" = 7 ]` (`test_render_board.sh:1901`) — *and*
  `S4` independently yielded exactly 16 **call sites**. The call-site count is not redundant: it is
  precisely what the mutant above defeats, because an unregistered id added next to an existing arm
  leaves the distinct-id count untouched. A broken tokenizer (renamed heading, reflowed fence,
  retitled table row) must redden, never pass with an empty loop (`guards-are-code`).
- **No dynamic check-id**: every call site in `S4`'s own site list has a bare lowercase-literal first
  argument. Deriving the lint from the *same* list is what makes a site invisible to `S4`
  structurally impossible — a site the extractor cannot see is a site the lint cannot clear.

### 3. Mutation proof — required in both directions on every edge

The completion bar (`correspondence-guard-runs-one-way`): for each of `S1..S4`, delete a real entry
→ the assert reddens; add a phantom entry → the assert reddens. Plus the two extractor-integrity
mutants: (a) the unregistered `case`-arm emit above → `S4` reddens on both counts; (b) an `emit
"$var"` site → the no-dynamic lint reddens. Record the full matrix in the results file.

### 4. The header enumeration is retained **verbatim**

`board-checks.sh:34` is `-h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0` — the header
block is **user-facing help output**, not an internal comment. The enumeration therefore stays
exactly where it is and keeps listing all 11 ids; it only *gains* a pointer line naming
`BOARD_CHECK_IDS` as the source. Replacing it with a pointer would both delete surface `S1` and stop
`--help` from listing the vocabulary.

### 5. `malformed-id`'s framing (stub open question 2)

`malformed-id` is a first-class emitted check-id already registered in both enumerations, so the
guard **does** carry it in `BOARD_CHECK_IDS` as an ordinary member. The *semantic* distinction
stays: reword `board-checks.md`'s "Guard/carve-out, not counted among the named checks above" to
"Guard/carve-out — it reports a malformed *file* rather than an unhealthy *change* — but a
first-class emitted check-id like the rest." Its section head already carries the uniform
`**\`malformed-id\`**` shape, so no extractor change is needed. One-sentence prose edit, no taxonomy
change.

## Assumptions

Every decision below was defaulted autonomously. Each names the alternatives rejected and why.
Assumptions marked **(revised)** were changed after the critic pass.

**A1 — Anchor: a declared array PLUS a total `emit`-call-site cross-check. (revised)**
*Chosen:* `BOARD_CHECK_IDS` as the declared source (`S0`), with the call-site derivation (`S4`) as a
co-equal producer anchor, using the mutation-proven widened anchor set and the dual-granularity
non-vacuity counts of §2.
*Rejected:* (a) grep-only — no declared object for the docs to be pinned against, and the derivation
degrades silently the moment anyone writes `emit "$x"`; (b) a hand-maintained list in the test file
— the `enumerated-floor` anti-pattern the learning names explicitly.
*The revision:* the first draft asserted S4 made the guard "total" while specifying an extractor
that was demonstrably not total. The claim is only earned by the widened anchors **and** the
call-site count; both are now normative, not illustrative. `backstop-must-compute-not-reenumerate`
does not bite here — `S0` is read by five asserts and by no hand-written condition list.

**A2 — `BOARD_CHECK_IDS` lives in `lib/docket-frontmatter.sh`, sourced by the guard. (revised)**
*Rejected:* declaring it in `board-checks.sh`. The first draft chose locality, arguing a
sourcing-order cost that **does not exist** (the lib is already sourced at :52, before `emit`) and
counting *script* consumers while overlooking that **the test is a consumer** — and the precedent's
entire mechanism is `source`. Since `board-checks.sh` is unsourceable, keeping the array there
converts `S0` from a runtime read into a text parse, i.e. it manufactures the one tokenizer the
whole design is trying to avoid.
**Composition with change 0116** (`single-source-the-remaining-duplicated-board-vocabularies`, in
flight, same two files): this hands 0116 a **done deal rather than a decision** — the check-id
vocabulary arrives already single-sourced in the lib, in the same shape as `DOCKET_STATUSES`, so
0116 can consolidate the *remaining* vocabularies against an established pattern instead of
adjudicating this one. Cross-check the semantic-purity question at 0116's altitude: if 0116 splits
lifecycle/board vocabularies out of `docket-frontmatter.sh` into a dedicated lib, `BOARD_CHECK_IDS`
travels with them and this guard needs only its `LIB=` path updated — the mirror asserts are
untouched either way. If 0116 lands first, this change rebases onto whatever home it established.

**A3 — No runtime validation in `emit()`. (revised — the draft's `exit 2` is DELETED.)**
The draft had `emit()` hard-fail on an unregistered id. The critic proved this **strictly worse than
the drift it guards against**, with a reproduction. `docket-status.sh:623-632` runs
`board-checks.sh` as the **LHS of a pipeline** and `health_checks` unconditionally `return 0`, so
the exit code is unreachable; `FINDINGS` is printed only after the walk completes, so a mid-walk
`exit 2` **discards every genuine finding already accumulated**. Observed: stderr carries the
diagnostic, stdout carries zero `check` lines, rc 0 — a report indistinguishable from a clean tree.
A developer's typo would silently blank the backlog's health report.
*Rejected repairs:* hoisting the validation to a pre-walk startup self-check would work, but only
paired with capturing the exit code out of the pipeline in `docket-status.sh` **and** a new
report-line row in `docket-status.md` — both of which A8 puts out of scope and Deliverable 3
disclaims. That internal contradiction is itself the argument for dropping it.
*What replaces it:* nothing needs to. `specified-but-unreachable`'s rule is "anchor one assert on
the **producer**" — `S4` **is** that assert. With `S4` total (A1), runtime validation detects
nothing `S4` does not already catch statically, at build time, at zero runtime risk.

**A4 — Four surfaces, and the test file is deliberately NOT one of them.** The #104 war story counts
`tests/test_board_checks.sh` as a third registration surface. Inspection (confirmed by the critic
across all 833 lines) shows it holds **no** closed enumeration — check-ids appear only as scattered
per-check literals in individual asserts, which is correct and must stay that way. Registering it as
a mirror would force a fifth enumeration into existence.

**A5 — Set equality, not subset, on all four edges.** The correspondence is a **mirror** — every
registration surface claims completeness (`∈ {…}` twice, plus `board-checks.md`'s `### Check
enumeration` heading, plus `--help` reprinting the header) — so the `#107` subset exception recorded
in `correspondence-guard-runs-one-way` does not apply. Cost accepted: a legitimate new check-id
reddens **five** asserts at once until all surfaces are updated. That is the feature; the failure
messages must name the three files to edit.

**A6 — The guard is a static/text guard, not a behavioral one.** It reads files; it does not run
`board-checks.sh` against fixtures to observe emitted ids. Rejected because reaching all 11 checks
behaviorally needs 11 hand-built fixture trees, and that corpus would itself be an enumerated floor.
*(The draft's claim that A3's runtime validation supplied "the behavioral half" was false — it would
fire only on emit sites some fixture actually executes, so a new check-id on an untriggered path
would never be validated. With A3 deleted the sentence is gone and A6 stands on its own.)*

**A7 — Landing home: `tests/test_board_checks.sh`, appended as a final section — with a flagged
collision. (revised)** Matches the `test_render_board.sh` precedent. **Change 0116 will very likely
also append a final section to this same file** (a priority-vocabulary correspondence block); two
changes appending "a final section" to one file tail is the most likely textual rebase conflict
between them. The draft's "additive to different regions" is wrong for *this* file. The change body
records the collision so the implementer expects it and reconciles by intent — compose both blocks,
never choose (`concurrent-edits-compose-at-rebase`).

**A8 — Scope stays the check-id vocabulary only.** `docket-status.md`'s wider report-line vocabulary
(the ~25 other rows) is explicitly out of scope per the stub, and is 0116's territory if anyone's.
With A3 deleted, this boundary no longer conflicts with anything in the design.

**Dependency state:** `depends_on: []`. Change 0116 is related by file overlap only, never a
readiness gate; nothing here blocks on it.

## Out of scope

- Changing any check-id, adding a check, or altering the findings format.
- The `docket-status` report-line vocabulary beyond the `check` row.
- Any change to `docket-status.sh`'s pipeline or exit-code handling (follows from A3's deletion).
- Re-repairing the two drift instances 0104 already fixed.
- Consolidating any *other* duplicated board vocabulary — that is change 0116.

## Deliverables

1. `scripts/lib/docket-frontmatter.sh` — the `BOARD_CHECK_IDS` array + its comment.
2. `scripts/board-checks.sh` — header enumeration **retained verbatim**, gaining only a pointer line
   naming `BOARD_CHECK_IDS` as the source. No `emit()` change.
3. `scripts/board-checks.md` — the `malformed-id` framing reword; one sentence in `## Invariants`
   noting the vocabulary is closed, that `## Behavior`'s `### Check enumeration` *is* this file's
   completeness claim, and that it is guarded both ways.
4. `scripts/docket-status.md` — no content change expected (its enumeration is correct at `HEAD`);
   it becomes a guarded surface.
5. `tests/test_board_checks.sh` — the correspondence block: 4 mirror asserts, 5 distinct-id
   non-vacuity asserts, 1 call-site-count assert, 1 no-dynamic-check-id lint.
6. Results file recording the both-directions mutation matrix, including the two extractor-integrity
   mutants of §3.

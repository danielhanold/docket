# Extend the board-row-dropped invariant to `archive/` — design

**Change:** 0115 · **Status:** design settled (auto-groomed) · **Date:** 2026-07-20

## Problem

Change 0104 shipped `board-row-dropped` in `scripts/board-checks.sh`: a *computed* backstop asserting
that every change file counted in `render-board.sh`'s `total` is actually accounted for by a rendered
section. Its predicate `renders_row` is deliberately derived from the renderer's own bucketing rather
than from an enumeration of causes (ADR-0050). It is bounded to `active/` — `board-checks.md` states
that bound is a deliberate scope, not a safety claim, and names the archive-side gap as follow-up.

This change closes that gap.

## What the renderer actually does with `archive/` (derived, not assumed)

Read from `scripts/render-board.sh` at the current tip. Line references are to that file.

| # | Line | Behavior |
|---|---|---|
| 1 | :86–88 | `total` = every `*.md` under `active/` **plus** every `*.md` under `archive/`. No id filter, no status filter. |
| 2 | :91 | `ARC_COUNT[<status>]` is incremented from each archive file's **raw `status:`**, whatever it is — including non-terminal ones. |
| 3 | :193–200 | The count-line segments iterate `DOCKET_STATUSES`; the `done\|killed` arm reads `ARC_COUNT`, every other arm reads the **active-only** `SECTION` map. So `ARC_COUNT[implemented]` is written and never read — unreachable by any segment. |
| 4 | :310 | The whole archive block is gated on `ndone + nkilled > 0`, both taken from `ARC_COUNT`. |
| 5 | :314 | The `<summary>` count is `ndone + nkilled` — a non-terminal archive file never joins it. |
| 6 | :324–341 | The archive table loop iterates **all** `ARCFILES` unfiltered by status, and skips any file whose `int_field id` is empty (`[ -n "$id" ] || continue`, :325; id read at :338). |
| 7 | :326–333 | `done` rows past `ARCHIVE_RECENT=15` are **redirected** into the per-month "Older done (collapsed)" table (:342–348), not discarded. `killed` never collapses. |

Checked and found **not** to be a gate: the sort feeder derives the displayed date from the
**basename** (`d="${base:0:10}"`, :338), not from frontmatter. An archive file lacking the date prefix
yields a junk date and a junk month key — but its row still prints, or still tallies into the digest.
No drop. Named here because it is the one place a reader will suspect a fourth gate.

Truth table for one `archive/` file:

| `int_field id` | `status:` | in `total` | in the summary count | in a table |
|---|---|---|---|---|
| valid | `done` / `killed` | yes | yes | yes — verbatim, or as a month-digest tally |
| valid | non-terminal | yes | **no** | prints only if some other file opens the block, under a summary that excludes it |
| empty | `done` / `killed` | yes | **yes** | **no** — dropped at :325 |
| empty | non-terminal | yes | no | no |

So there are **two** archive-side accounting failures, not one:

- **(A) non-terminal status in `archive/`.** The case the stub names. Counted in `total`, absent from
  the summary. With no `done`/`killed` sibling the entire archive block is skipped and the row appears
  nowhere; with one, the row prints under a `<summary>` whose count excludes it. Either way the count
  line and the tables disagree — 0104's exact symptom, mirrored. Reachable by the same interrupted
  operation: `archive-change.sh` does its `git mv` before the status flip and the commit.
- **(B) a terminal-status archive file with no usable integer id.** *Not* named in the stub, found by
  reading the renderer. `ARC_COUNT` keys on status alone, so the file **is** counted in the summary,
  while :325 drops its row. The summary promises a row that never renders. This is the archive-side
  mirror of the active side's live "no `id:` field at all" trigger.

## Decision

**Widen `board-row-dropped` to cover both directories with one generalized predicate.** No new
check-id.

### The predicate

`renders_row` gains the directory as its first argument and selects the status set the renderer
actually iterates for that directory:

```
renders_row DIR_KIND ID STATUS   # DIR_KIND ∈ {active, archive}
  [ -n "$ID" ] || return 1                       # render-board.sh:76 (active) / :325 (archive)
  active  → STATUS ∈ DOCKET_STATUSES_ACTIVE      # :265-269, the print_section call list
  archive → STATUS ∈ DOCKET_STATUSES_TERMINAL    # :91 → :310 gate → :314 summary
  else return 1
```

Both arrays already exist in `scripts/lib/docket-frontmatter.sh` (:126–127), and `board-checks.sh`
already sources that lib (:52), so the archive arm needs no new plumbing. Nothing is restated. The
`[ -n "$ID" ]` clause is one condition anchored to two renderer lines, so it stays hoisted above the
directory switch.

**Neither arm is a mirror by construction today. Both rest on comment-asserted correspondence, and
the difference between them is degree, not kind** — this must be stated exactly that way, because
ADR-0050's own Consequences (:64–67) already records it as shipped fact: the active arm's
correspondence is asserted "by comment and fixtures rather than mechanically, because the renderer's
`print_section` call list is not yet single-sourced (tracked as follow-up change 116)."

- **Active arm.** Anchored to the `print_section` call list at `render-board.sh:265–269` — five
  literal calls, no array read. Deleting `print_section deferred` while leaving
  `DOCKET_STATUSES_ACTIVE` intact makes `renders_row` claim `deferred` renders and the backstop go
  quiet. It does have *corroboration*: two adjacent surfaces do iterate the array (the digest
  projection at :137, which exits at :188, and the mermaid node loop at :290), and
  `tests/test_render_board.sh:1927` pins `label_for_title` to the array in both directions.
- **Archive arm.** The renderer hard-codes `done|killed` (:125, :195) and the block gate at :309–310
  reads `ARC_COUNT[done]` / `ARC_COUNT[killed]` as literals. `DOCKET_STATUSES_TERMINAL` has **zero**
  readers in `render-board.sh` and zero pinned correspondence anywhere.

Reading the shared arrays is still right — hard-coding `done|killed` here would be strictly worse, a
fourth restatement. Two consequences for the build:

1. The `renders_row` comment block must record the caveat **for both arms**, in ADR-0050's own terms,
   and must not claim either arm is a mirror by construction. Name change 0116 as what single-sources
   the renderer's literals and upgrades both. If 0116 has landed by build time, re-read
   `render-board.sh` and correct the comment rather than shipping it stale.
2. Close the archive arm's corroboration gap mechanically — see **T9**.

The population site in the file walk drops its `fd_active = 1` guard and passes the directory it
already computes:

```
if ! renders_row "$dir_kind" "$id" "$status"; then DROPPED["$cid"]=1; fi
```

`dir_kind` is derived from the same `*/active/*` case the walk already performs.

### Explicitly NOT derived from `ARCHIVE_RECENT`

The recency window and the per-month digest **redirect** a row; they do not discard it. A collapsed
`done` file is still in the summary count and still represented in the "Older done (collapsed)" table.
A predicate written as "does a verbatim row print" would false-positive on every `done` file past the
16th — the failure mode open question 1 asks about. The predicate above is written against
*accounting*, not against verbatim row emission, so collapse is invisible to it and must stay that
way. A regression test pins this (T3 below); a future reader must not "tighten" the predicate toward
row emission.

### Suppression

Unchanged in mechanism and unchanged in code. Both suppressing arms already run over `archive/` files
today — neither `malformed-id` (:136–143) nor the `field-domain` `status` arm (:168–177) is
directory-gated — and both are genuine archive-side drop causes:

- `malformed-id` — a non-empty non-integer id is dropped at :325, exactly as at :76.
- `field-domain` on `status` — a status outside the seven-name vocabulary is outside
  `DOCKET_STATUSES_TERMINAL` too, so it explains the archive drop.

`slug` / `priority` / `title` still do not suppress, for the reasons 0104 records.

Load-bearing detail for case (B): the whole `field-domain` block is **unreachable** for a file with no
usable id — the walk `continue`s at :143 before reaching it. That is exactly why case (B) has no
suppressor and fires cleanly, and it is what T5 pins. Do not "fix" the `continue` without re-checking
this.

The remaining unsuppressed archive triggers are therefore exactly the two that no enumerated check can
see: a **legal status in the wrong directory** (case A) and a file with **no `id:` field at all**
(case B) — the precise mirror of the active side's two live triggers.

### Message

The active-side message stays **byte-identical**. Not because anything depends on it — verified: the
string occurs once in shipped code (`board-checks.sh:294`), no test asserts it (`has_finding` matches
only the `check-id\tchange-id\t` prefix), and no golden captures it. It is a free choice, taken toward
zero churn on 0104's shipped text so the diff stays about the archive side.

Archive files get a parallel, directional message. It must (a) name the archive pass so the reader
knows which way the file is misfiled, (b) be honest about both sub-cases — case A can print a row
under a wrong count, case B prints no row at all — and (c) describe the invariant, never enumerate
causes. Proposed text:

```
counted in the board total but not accounted for by the archive pass (no row rendered, or a
summary count that excludes it); no malformed-id or field-domain status finding accounts for the drop
```

The trailing suppression clause matches the active message's, for the reason 0104 gives: naming the
two suppressing arms specifically, not `field-domain` wholesale.

### Rejected: a "wrong directory for its status" check

The stub floats a single directory/status-mismatch finding as possibly the more useful diagnostic.
Rejected as the *predicate*, on a concrete counterexample rather than on authority: a no-id `done` file
in `archive/` is in the **correct** directory for its status, so a mismatch predicate draws nothing on
it — while :91 counts it in the summary and :325 drops its row. Case (B) would have gone unseen a
second time. That argument stands alone; ADR-0050 is the family it belongs to (a predicate not derived
from the consumer inherits a blind spot), though the ADR's literal rule is about re-enumerating the
*sibling checks*, and a convention restatement is a cousin of that shape rather than the same one.
Directory-vs-status phrasing survives where it belongs: in the message's remedy hint, not in the
trigger.

## Test plan

In `tests/test_board_checks.sh`, extending the change-0104 block. Each is a `--changes-dir` fixture
repo; assertions key on `has_finding <out> board-row-dropped <id>`.

- **T1** — `archive/` file at `status: implemented` beside a healthy `done` file ⇒ fires for the
  misfiled id, not for the `done` sibling. (Case A, block open.)
- **T2** — the same file with **no** terminal sibling, so the archive block never opens ⇒ still fires.
  (Case A, block closed — the flavor that renders nowhere at all.)
- **T3** — 16+ well-formed `done` files so at least one collapses into the month digest ⇒ **no**
  finding for the collapsed id. This is the open-question-1 false-positive guard and the reason the
  predicate is written against accounting rather than row emission. Generate the fixtures in a loop
  with deterministic date prefixes (the sort is date-desc then id-desc), and assert on the *specific*
  id the window pushes past `ARCHIVE_RECENT` — not on "no findings at all", which would pass vacuously.
  Not hypothetical: this repo's own archive already exercises the collapse path.
- **T4** — a `killed` archive file ⇒ no finding. A second healthy terminal negative; its value is
  covering the other member of `DOCKET_STATUSES_TERMINAL`, not anything about collapse (collapse is
  invisible to an accounting predicate, which is T3's point).
- **T5** — archive file with **no `id:` field** and `status: done` ⇒ fires. (Case B.)
- **T6** — archive file with a non-integer id ⇒ `malformed-id` fires and `board-row-dropped` is
  suppressed.
- **T7** — archive file with an out-of-vocabulary status ⇒ `field-domain` fires on `status` and
  `board-row-dropped` is suppressed.
- **T8 (mutation, per ADR-0050)** — one mutation per *independent clause*, because the predicate has
  three and a single blanket mutation would let two of them pass vacuously:

  | Mutation | Must redden |
  |---|---|
  | archive status arm accepts any status | T1, T2 — **not** T5 |
  | the hoisted `[ -n "$ID" ]` clause always passes | T5, and the active-side no-id case (a) at `tests/test_board_checks.sh:552` |
  | restore the `fd_active = 1` guard at the population site | T1, T2, **and** T5 together — this is the population-deletion mutation ADR-0050's corollary demands |
  | delete the `EXPLAINED` marker at `board-checks.sh:141` (malformed-id) | T6 gains a second finding |
  | delete the `EXPLAINED` marker at `:176` (field-domain status) | T7 gains a second finding — **and** the pre-existing active-side `n71 = 1` pair at `tests/test_board_checks.sh:565` |

  The two `EXPLAINED` sites are independent; mutating them separately is what makes each a real
  suppression decision. The last row's second entry is expected collateral, not a defect — a builder
  checking "only the listed tests went red" would otherwise trip on it.

  The first row is the trap: T5 is a **no-id** file, killed by the hoisted id clause before the status
  switch runs, so mutating the archive status arm leaves it green. A builder who expects T5 to redden
  there will misread a working harness as broken — or, worse, "fix" the predicate until it does.

- **T9 (correspondence assert)** — convert the archive arm's caveat from prose into a tripwire.
  `tests/test_render_board.sh:1918–1929` already implements the pattern for `label_for_title`:
  tokenize the function's `case` arms and assert set-equality against the array in **both**
  directions. Copy it for the renderer's terminal literals — extract the `done|killed` arms
  (`render-board.sh:125`, :195) and assert equality with `"${DOCKET_STATUSES_TERMINAL[*]}"`. Touches
  no renderer code and needs no 0116. Without it, the comment is the only thing standing between a
  third terminal status and a silently quiet backstop.

### Existing tests that change

Swept exhaustively rather than assumed: every archive fixture in the suite is `test_board_checks.sh`
:174, :185, :469, :536, :582, :622, :677. All are well-formed `done` files except :469 (non-integer id
⇒ suppressed by `malformed-id`, unchanged) and :582. `new_repo` seeds an **empty** archive dir, so the
clean-tree and exact-count assertions are untouched. `test_docket_status.sh` mocks `board-checks.sh` at
every invocation site, and `test_render_board.sh` is renderer-only. Exactly one assertion flips:

- `tests/test_board_checks.sh` case **(d)** (:579–585) is an `archive/` fixture with **no `id:`
  field** and `status: done`, asserting `board-row-dropped` does **not** fire, under the comment
  "archive/ is NOT subject to the invariant". That fixture is exactly case (B). The assertion
  **inverts**: it becomes T5, and its comment is rewritten from an exemption to the archive-side
  invariant. Flagged because it is the one place where this change deliberately reverses a shipped
  assertion — a build that leaves it untouched will go red, and the correct response is to invert it,
  never to re-exempt `archive/`.
- Case **(f)**'s `L` fixture (well-formed `done` in `archive/`, :620–625) stays green unchanged and
  becomes the archive-side healthy negative.

## Documentation

- `scripts/board-checks.sh` — rewrite the `renders_row` comment block and the `board-row-dropped`
  emission comment to cover both directions, keeping each clause anchored to its renderer line number.
- `scripts/board-checks.md` — the **`board-row-dropped`** section: replace the "Scope: the check covers
  `active/` only" paragraph (which currently documents this gap as follow-up work) with the two-sided
  predicate, the two-sided live-trigger list, and an explicit note that `ARCHIVE_RECENT` collapse is
  not a drop.
- No new check-id ⇒ **no** edit to the check-id enumerations at `scripts/board-checks.sh:13` or
  `scripts/docket-status.md:344`. This is a deliberate benefit of the widen-vs-new-id choice.

## Out of scope

- Making `archive-change.sh` atomic (ordering the `git mv` after the status flip). Detecting the
  resulting state is this change; preventing it is a separate one.
- Repairing any offending file. The check stays warn-only and never mutates.
- The `active/` side, which 0104 ships and which this change leaves behaviorally identical.
- Any change to `render-board.sh`. The renderer is the oracle here, not the subject.

## Assumptions

Every decision an interactive brainstorm would have raised, the default taken, and why. Written for a
deferred human audit.

1. **Widen `board-row-dropped` rather than mint a second check-id.** *Rejected:* a separate
   `archive-row-dropped` id (keeps 0104's finding meaning exactly one thing); a "wrong directory for
   its status" id (the stub's own suggestion). *Why:* the invariant is genuinely singular — the board
   has one `total` and one set of tables, and "is every counted file accounted for" is one question.
   Splitting it by directory yields two half-invariants, duplicates the `DROPPED`/`EXPLAINED`
   machinery, and adds a check-id to two enumerations that change 0111 is concurrently hardening
   against drift. Directionality is preserved in the message instead of in the id. The standing
   preference for the comprehensive fix over the bolted-on one points the same way.
2. **Predicate derived from accounting, not from verbatim row emission** — which is what makes
   `ARCHIVE_RECENT` collapse a non-event and settles open question 1 as "no false positives, by
   construction." *Rejected:* a row-emission predicate, which would fire on every collapsed `done`
   file. Pinned by T3 so the cheaper-looking formulation cannot creep back in.
3. **Case (B) is in scope.** Reading the renderer surfaced a drop the stub did not describe: a
   terminal archive file with no usable id is counted in the summary but never rendered. Including it
   is what makes the archive arm a real mirror of the active arm rather than a status check. *Cost:*
   it inverts a shipped assertion (test case (d)), which is called out above so a builder does not
   read the red as a regression.
4. **The active-side message stays byte-identical; archive gets its own text.** *Rejected:* one
   unified message for both directions (tidier, but rewrites shipped finding text and loses the
   direction the reader needs to act). *Risk accepted:* two message strings to keep in step; both live
   in the same emission block.
5. **No new suppression code.** Both suppressing arms already run over `archive/` files and both are
   genuine archive drop causes, so widening the population is sufficient. *Verified by reading:*
   neither the `malformed-id` block nor the `field-domain` `status` arm carries a directory gate.
6. **Composition with change 0116 (concurrent).** `DOCKET_STATUSES_TERMINAL` **already exists**
   (`lib/docket-frontmatter.sh:127`) and `board-checks.sh` already sources the lib (:52), so this
   design consumes it today with **no ordering dependency** on 0116 in either direction. If 0116 lands
   first, this predicate is unchanged — it already reads the shared array rather than a hard-coded
   `done|killed`. If this lands first, 0116 gains a second consumer of both arrays.
   **But the independence is not symmetric in value.** The renderer currently hard-codes its terminal
   literals (`render-board.sh:125`, :195, :309–310) and reads `DOCKET_STATUSES_TERMINAL` nowhere, so
   until 0116 single-sources them the archive arm is a mirror by *convention*, not by construction —
   see the caveat in "The predicate". 0116 is what makes it real. Two things 0116 must not do: collapse
   `DOCKET_STATUSES_ACTIVE` and `DOCKET_STATUSES_TERMINAL` into a single `DOCKET_STATUSES` read (the
   difference between those sets is this check's entire signal), or single-source the renderer's
   literals to some *third* list this predicate does not read.
7. **Overlap with change 0111 (concurrent, same files).** 0111 hardens the check-id enumerations
   against drift; this change adds no check-id, so the two touch `board-checks.sh` in disjoint regions
   (0111 in the header/doc enumerations, this in `renders_row` and the walk). Expect a mechanical
   rebase, not a semantic one; reconcile by composing both, per the `concurrent-edits-compose-at-rebase`
   learning.
8. **`--strict` becomes stricter.** A repo carrying a pre-existing archive-side violation will newly
   fail `board-checks.sh --strict`. *Accepted:* that is the point of the check. Swept at design time —
   this repo's `archive/` is clean (every file terminal, every id a valid integer), so nothing goes red
   on landing. Re-sweep at build time; a hit is a real misfiled file, not a test failure.
9. **Two pre-existing weaknesses noted, deliberately not fixed here.** (i) `DROPPED` / `EXPLAINED` are
   keyed by `cid` (`board-checks.sh:126`), not by path, so a cross-directory id collision lets one
   file's `EXPLAINED` suppress another's genuine drop. `EXPLAINED` already crossed directories before
   this change; widening makes the exposure symmetric rather than creating it. (ii) The `dir_kind`
   derivation (`case "$f" in */active/*`) misclassifies if `$CHANGES_DIR` itself contains an `active`
   path component; anchoring the glob on `"$CHANGES_DIR"/active/*` hardens it. Both are cheap to fix
   in passing if the builder is already in those lines; neither is a reason to widen this change's
   scope, and neither is newly introduced by it.
10. **`depends_on` state:** none. The stub has no dependencies and none was inferred.

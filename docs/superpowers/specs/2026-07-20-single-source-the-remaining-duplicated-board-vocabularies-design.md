# Single-source the remaining duplicated board vocabularies — design

Change: 0116 · Date: 2026-07-20 · Authored by `docket-auto-groom` (autonomous; critic-gated)

## Problem

Change 0104 single-sourced the seven-name **status** vocabulary into
`scripts/lib/docket-frontmatter.sh` (`DOCKET_STATUSES_ACTIVE` / `DOCKET_STATUSES_TERMINAL` /
`DOCKET_STATUSES`) and converted four iteration sites to read it. Its own whole-branch review
recorded that the conversion was partial, and `tests/test_render_board.sh:1894-1897` says so in the
suite itself:

> Hand-written status lists demonstrably survive elsewhere in the renderer (the `done|killed` count
> arms, `label_for_title`, the per-status table-header and row-format `case`s, and the
> `print_section` call list). Widening this check to cover those sites is deliberately DEFERRED
> follow-up work.

This change is that follow-up. It finishes the conversion, and extends it to the sites outside
`render-board.sh` that a whole-repo derivation found — plus the **priority** vocabulary, which has
no single source anywhere in the repo at all.

## Site inventory (derived, not hand-listed)

Derived 2026-07-20 from a whole-repo, case-insensitive sweep across every file type (not
`--include="*.sh"`), per [[enumerated-floor]]. **This table is a floor, not the set** — the build's
reconcile pass re-derives it and reconciles differences into the reconcile log rather than trusting
this list.

### In scope — set enumerations that must equal an array

| Site | Enumerates | Disposition |
|---|---|---|
| `render-board.sh:125`, `:195` — `case "$st" in done\|killed)` | TERMINAL | **derive** (helper) |
| `render-board.sh:265-269` — the five `print_section` calls | ACTIVE | **derive** (loop) |
| `render-board.sh:236-240` — per-status table-header `case` | ACTIVE | **pin** (set-equality) |
| `render-board.sh:247-260` — per-status row-format `case` | ACTIVE | **pin** (set-equality) |
| `render-board.sh:167-170` — priority sort ladder | PRIORITIES | **derive** (rank helper) |
| `board-checks.sh:187-188` — priority field-domain `case` | PRIORITIES | **derive** (membership) |
| `github-mirror.sh:54` — `STATUS_OPTIONS="proposed,in-progress,…"` | ACTIVE | **derive** (join) |
| `github-mirror.sh:241-242` — `done)` / `killed)` close-reason `case` | TERMINAL | **pin** (set-equality) |
| `github-mirror.sh:316` — `case "$st" in done\|killed\|"") continue` | TERMINAL | **derive** (helper) |
| `archive-change.sh:48` — `--outcome` validation | TERMINAL | **derive** (helper) |
| `terminal-publish.sh:67` — `--outcome` validation | TERMINAL | **derive** (helper) |
| `docket-status.sh:557` — merge-sweep idempotence guard | TERMINAL | **derive** (helper) |
| `render-board.sh:308-314` — archive `<details>` gate, count and label composition | TERMINAL | **derive** (helper) |

The archive site is the one a literal `done|killed` sweep cannot see — it spells the vocabulary as
`ARC_COUNT[done]` / `ARC_COUNT[killed]` across a gate, a summary count, and a label composition
(`em`/`lbl`). A third terminal status would be counted in `total` (`:87-88`) and in the count line
(`:193-197`, which this change converts), and its rows would still print from the `while` loop at
`:324` — but the `<details>` count would exclude it, and if it were the archive's only content the
whole section would silently vanish. That is the ADR-0050 "count line and tables disagree" failure
verbatim. It is **not** covered by 0115, which is checker-side and names `ndone + nkilled` only as
the symptom it models, never as a renderer edit.

All thirteen sites live in six scripts that **already source
`scripts/lib/docket-frontmatter.sh`** (verified: `render-board.sh:45`, `board-checks.sh:52`,
`github-mirror.sh:80`, `archive-change.sh:22`, `terminal-publish.sh:28`, and `docket-status.sh:31`).
`render-change-links.sh` also sources the library but owns no in-scope enumeration and is not an
affected script. The comprehensive scope therefore adds **no new sourcing plumbing** —
which is what collapses the usual narrow-vs-comprehensive trade-off here.

### Explicitly OUT of scope — single-status predicates

`render-board.sh:298` (the mermaid done-node filter) and `:326` (the archive collapse-window
predicate), `board-checks.sh:202/214/245/260`, `docket-status.sh:447/696`,
`render-change-links.sh:92/103`, `reclaim-claims.sh:66`, `archive-change.sh:102/105`,
`github-mirror.sh:130/140`. **This list is a floor too** — same discipline as the in-scope table.

These test **one named status** because that status specifically means something to that code path
(`done` changes publish, `in-progress` changes hold a claim). They are not restatements of a set and
have no array to be derived from. Rewriting `[ "$status" = "done" ]` as an array lookup would be
obfuscation, not single-sourcing, and would make the code lie about its own specificity. **The
in-scope test is "does this site enumerate a set that is supposed to equal one of the arrays?"** —
not "does this line mention a status name?"

Also out: `emoji_for` / `label_for` / `label_for_title` (already pinned by 0104; not re-litigated per
the stub's own Out of scope), all prose/doc restatements (see *Deferred*), and the check-id and
archive-side board-row-dropped work owned by 0111 and 0115.

## Design

### 1. Extend the vocabulary block in `scripts/lib/docket-frontmatter.sh`

The status arrays already live here; priorities join them rather than starting a second home. The
file header comment is updated from "frontmatter + dependency-resolution helper" to name the
vocabulary block as a third responsibility.

```bash
# --- priority vocabulary (change 0116) ---
# Ordered by RANK, descending — the order IS the semantics: the convention's deterministic
# selection order is critical > high > medium > low, and the array index IS the sort rank
# render-board.sh's `ready` line uses (critical=0 … low=3, unchanged from the hand-written ladder).
DOCKET_PRIORITIES=(critical high medium low)
DOCKET_PRIORITY_DEFAULT=medium   # the convention's documented default for an unset priority
```

Plus four pure membership/rank helpers (no side effects on source, matching the file's contract):

- `docket_status_is_terminal ST` — exit 0 iff `ST` ∈ `DOCKET_STATUSES_TERMINAL`.
- `docket_status_is_active ST` — exit 0 iff `ST` ∈ `DOCKET_STATUSES_ACTIVE` (empty ⇒ non-zero).
- `docket_priority_is_member P` — exit 0 iff `P` ∈ `DOCKET_PRIORITIES`. **Strict: empty is NOT a
  member.** Empty-is-legal is a separate documented fact and stays an explicit arm at the one site
  that cares (`board-checks.sh`), so the helper never silently blesses a missing field.
- `docket_priority_rank P` — prints the 0-based index of `P`, or the index of
  `DOCKET_PRIORITY_DEFAULT` when `P` is empty **or unrecognized**. This is exactly the current
  `*) prank=2` arm's behavior, preserved by construction rather than by coincidence.

### 2. Resolve the stub's open question — one ordered array, not two constants

**Decision: a single ordered `DOCKET_PRIORITIES`, with membership derived from it.**

The stub asks whether priority wants an ordered array plus a derived membership test, or two
separate constants, noting that statuses "got away with one ordered array because display order and
membership coincided." For priority the case is *stronger*, not weaker:

- For statuses, the shared order was a **coincidence** of two independent facts (the board's section
  order, and the set of legal values) — 0104's comment block has to warn that reordering breaks a
  golden.
- For priority there is only **one** ordering in the whole system, and it is normative: the
  convention states `critical > high > medium > low` as the selection order. There is no second
  ordering for a membership-only constant to protect. Two constants would be two things to keep in
  sync — reproducing, at smaller scale, the exact defect this change exists to remove.

Deriving rank from the index also deletes a fourth restatement nobody listed: the magic numbers
`0/1/3/2` in the ladder.

`DOCKET_PRIORITY_DEFAULT` is a **separate named constant** rather than "index 2", because "unset
means medium" is an independent documented fact, not a consequence of the ordering. A guard asserts
it is a member of `DOCKET_PRIORITIES`.

### 3. Derive where derivable; pin where the shape is genuinely a mapping

**Derived (the enumeration disappears):**

- Both `done|killed` count arms → `if docket_status_is_terminal "$st"; then n=${ARC_COUNT[$st]:-0}; else n="$(count_of "$st")"; fi`.
- The `print_section` call list → `for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do print_section "$st" "$(suffix_for "$st")"; done`.
- The priority ladder → `prank="$(docket_priority_rank "$(field "$f" priority)")"`.
- `board-checks.sh`'s priority arm → `[ -z "$fd_priority" ] || docket_priority_is_member "$fd_priority" || emit …`, with the finding message's value list rendered as `${DOCKET_PRIORITIES[*]}`.
- `github-mirror.sh`'s `STATUS_OPTIONS` → joined from `DOCKET_STATUSES_ACTIVE`.
- `github-mirror.sh:316` and the three `--outcome` / idempotence validators → `docket_status_is_terminal`.
- The archive `<details>` block → the gate and count become a sum over `DOCKET_STATUSES_TERMINAL`,
  and `em`/`lbl` are composed by iterating it (reusing `emoji_for`, which is already pinned to the
  full vocabulary, instead of the third hand-written `✅`/`🗑️` pair).

**Source-order hazard at `github-mirror.sh`.** `STATUS_OPTIONS` is assigned at `:54`; the lib is
sourced at `:80`. A derivation written in place expands to the **empty string**, and
`--single-select-options ""` degrades into a best-effort `log` at `:299` rather than failing loud —
a silent, board-shaped failure. The assignment must move below the `source` line. This is the one
site where "the script sources the lib" is not sufficient: *sourced* is not *sourced before use*,
and it is true at every other site and false at this one.

**Pinned (kept as `case`, guarded by set-equality against the array):** the table-header `case`, the
row-format `case`, and `github-mirror.sh`'s close-reason `case`. These are **mappings**, not lists —
a `case` is the right shape, exactly as 0104 concluded for `emoji_for`. All three fail the same way
today: they fall through silently, so a sixth active status would render a section header with no
table and no rows.

### 4. The total-vs-sparse distinction (a rule the guards depend on)

Not every `case` over a vocabulary should be pinned to it. The discriminator is **semantic, not
syntactic**: *which vocabulary, if any, is this mapping intended to be exhaustive over?*

- **Exhaustive over a named array** ⇒ pin by set-equality against **that** array. `emoji_for`
  (`DOCKET_STATUSES`), `label_for_title` / `table_header_for` / the row-format `case`
  (`DOCKET_STATUSES_ACTIVE`), the close-reason `case` (`DOCKET_STATUSES_TERMINAL`).
- **Exhaustive over nothing** ⇒ do not pin. `label_for` (`*)` passthrough), the new `suffix_for`
  (only `implemented` carries a suffix), `docket_priority_rank`'s default. Forcing one arm per
  member would add meaningless arms and convert a correct default into a maintenance obligation.

**The tempting syntactic shortcut — "a total mapping is one with no `*)` arm" — is wrong, and the
repo already contains its counterexample.** `github-mirror.sh:127-144` (`readiness_label`) has no
`*)` arm and is nonetheless correctly sparse: two arms over the seven-name vocabulary, with a
comment at `:122-126` documenting that *every other status has no readiness label*. Pinned against
`DOCKET_STATUSES` it would redden permanently. The close-reason `case` at `:241-242` has the same
shape — no `*)`, two arms — and *is* pinned, because it is exhaustive over
`DOCKET_STATUSES_TERMINAL`. Identical syntax, opposite dispositions; only intent separates them.
`readiness_label` is named in the test file as the worked counterexample.

Writing this rule down is load-bearing, because a later reader who applies "pin every case over the
vocabulary" uniformly will pin `suffix_for` and make the codebase worse. It goes in the test file as
a comment, next to the guards.

### 5. Reuse 0104's proven tokenizer rather than writing new fragile ones

`tests/test_render_board.sh`'s `case_labels()` extractor only matches a **one-line function header**
(`fn(){ case`). To reuse it unchanged for the table headers, extract that `case` out of
`print_section` into `table_header_for(){ case … esac; }` in that same style. The row-format `case`
cannot become a format-string function (each arm's `printf` takes a different argument list), so it
stays inline in `print_section` and gets **one** new extractor anchored on its own `case` line.

Per [[plan-supplied-test-code-is-unverified]] and [[backstop-must-compute-not-reenumerate]], **every
extractor carries its own exact-count assert before any comparison** — a tokenizer that parses
nothing passes everything — and each new guard is mutation-tested in **both** directions
([[correspondence-guard-runs-one-way]]): delete a real arm → red; add a phantom arm for a retired
status → red.

### 6. Retire the deferral comment; widen the producer-anchored assert

`tests/test_render_board.sh:1894-1897`'s "deliberately DEFERRED" comment is deleted (the deferral is
discharged), and the `n_all` / `n_active` counts are re-blessed for the new iteration sites. The
comment must not simply be reworded to imply coverage the asserts do not have — the same discipline
0104 applied when it wrote the comment in the first place.

### 7. `renders_row` becomes structurally correct rather than comment-asserted

`board-checks.sh`'s `renders_row` claims to mirror `render-board.sh:265-269`; the correspondence is
asserted today only by a comment and by fixtures. Once `print_section` is driven by
`DOCKET_STATUSES_ACTIVE`, both sides read the **same array** and the correspondence is structural.
The comment is rewritten to point at the shared array rather than at line numbers, and `renders_row`'s
body becomes a `docket_status_is_active` call.

## Behavior preservation

This change is intended to be **behavior-neutral except for two deliberate, golden-visible changes**:

1. **`github-mirror.sh`'s `STATUS_OPTIONS` order changes** from
   `proposed,in-progress,blocked,deferred,implemented` to
   `in-progress,proposed,blocked,deferred,implemented` (the array's order). See *Assumptions* A4.
2. **`board-checks.sh`'s priority finding message** lists values in rank order
   (`critical high medium low`) instead of `low medium high critical`.

Both require re-blessing a golden (`tests/test_github_mirror.sh:211`, `tests/test_board_checks.sh:504`).
Everything else — `BOARD.md` bytes, the digest, the `ready` line's ordering, every check's findings —
must be byte-identical, and the existing goldens are the proof.

## Testing

1. Unit tests for the four new helpers in `tests/test_docket_frontmatter.sh`, including
   `docket_priority_rank` on empty **and** on an unrecognized value (both ⇒ the default's rank).
2. Set-equality + exact-count guards for `table_header_for`, the row-format `case`, and
   `github-mirror.sh`'s close-reason `case`; each mutation-tested in both directions.
3. `DOCKET_PRIORITIES` composition guards: exact count (4), the rank order, and
   `DOCKET_PRIORITY_DEFAULT` ∈ `DOCKET_PRIORITIES`.
4. A **producer-anchored** assert that no hand-written `done|killed` set survives in the converted
   scripts (patterned on the existing `n_literal` assert; scoped honestly in its name to what the
   pattern proves — it does not see single-status predicates, which are out of scope, nor the
   `ARC_COUNT[done]` spelling, which is why item 4 is a floor and not the guarantee).
   **Scoping rule, stated rather than silently applied:** the exemption covers exactly the
   **column-0 comment headers that contain the `done|killed` literal as a documented CLI contract** —
   `archive-change.sh:16` and `terminal-publish.sh:11` (`--outcome done|killed`), and nothing else in
   the repo has that shape. The tempting reason "it prints its header via `-h|--help`" is **false as
   a discriminator**: fifteen scripts share the `grep '^#' "$0"` idiom, including three converted
   ones (`render-board.sh:32`, `board-checks.sh:34`, `github-mirror.sh:70`), so it would silently
   exempt them too. The exemption and this reasoning go in the assert as a comment. It is
   deliberately not the general rule "ignore all comments," which would blind the assert to exactly
   the stale-comment drift this change exists to end.
   **Related trap:** `github-mirror.sh:52` is a column-0 comment reading
   `# (terminal done/killed are expressed by closing the issue, not a column)` — it escapes a literal
   `done|killed` pattern only via the slash spelling, and sits directly above the line A12 relocates.
   Broadening the pattern past the pipe (which A1's own `ARC_COUNT` lesson argues for) trips it; the
   comment must move with the assignment and stay accurate.
5. Re-run the full suite, not only the enumerated tests ([[enumerated-floor]]), because the blast
   radius of retiring a literal is every guard keyed on that literal repo-wide.

**Row-format extractor hazard.** The new extractor must anchor arms at line start, **not** reuse
`case_labels`' body-wide `grep -oE '[a-z][a-z-]*\)'`. Run over `render-board.sh:247-260` that regex
yields **14 raw / 9 distinct** tokens for 5 arms: the five real arms plus `spec)`, `branch)`, `by)`
(from `blocked_by)`), and — the one that appears in *every* arm, so the builder hits it first — `s)`,
six times, from the `(active/%s)` link in each `printf` format string. Since `case_labels` pipes
through `sort -u`, the count assert sees **9**. It reddens, which is the guard working as designed —
but the builder should expect it rather than debug it.

## Expected ADR

**Record one ADR: the total-vs-sparse mapping rule (§4).** Argued rather than omitted, because A6's
own reasoning demands it: A6 rejects "leave the rule implicit" on the evidence that 0104's test-file
comment decayed into the deferral this change is discharging — so adopting *another test-file
comment* as the sole remedy would repeat the mistake at one remove. The rule is also general (it
governs every future `case` over any docket vocabulary, not just the board's), it has a
non-obvious discriminator with a live counterexample in the repo, and it sits directly alongside
ADR-0049/ADR-0050, both of which 0104 extracted from problems of exactly this shape. Neither
existing ADR covers it: ADR-0049 is the findings-channel rule, ADR-0050 the backstop rule. The
test-file comment stays as the local pointer; the ADR is the durable record. `adrs:` is updated when
it lands.

**The ADR must carry a third case the §4 binary does not name: the un-arrayed vocabulary.** §4 asks
"exhaustive over which named array?", which silently assumes an array exists. The four readiness
tokens are a real vocabulary with no array — `render-board.sh:211-215` and `github-mirror.sh:132-139`
are both intended to be exhaustive over them (documented at `github-mirror.sh:122-126`) — and they
would land in "exhaustive over nothing," a mislabel that happens to give the right disposition today.
The correct rule is three-way: exhaustive over a named array ⇒ pin; exhaustive over a vocabulary that
has **no** array ⇒ **give it an array first, then pin**; exhaustive over nothing ⇒ leave it.
This is not academic: **0111 guarded the formerly un-arrayed check-id vocabulary by first adding
`BOARD_CHECK_IDS`**, then pinning its mirrors. Its landed implementation is direct evidence for the
middle case rather than future work.

## Deferred (explicitly not this change)

- **Prose restatements.** ~20 documentation sites restate one of the three vocabularies
  (`skills/docket-convention/SKILL.md:139-140,198-215,230`, `scripts/board-checks.md:127`,
  `scripts/render-board.md:115`, `README.md:55,227,355`, and others). They cannot be derived from a
  bash array, so single-sourcing does not reach them; guarding them needs a structural sentinel,
  which is a different change with a different shape. Named here so the next reader knows the
  omission is a decision, not an oversight.
- **A prose-count sentinel** over vocabulary cardinality ("seven statuses", "five active"), per the
  0098 war story in [[enumerated-floor]].

## Assumptions

Every decision this autonomous groom made, the alternatives rejected, and why. This is the human's
deferred audit trail.

**A1 — Scope is all thirteen derived sites, not the stub's three named files.**
*Revised after critic round 1: the first derivation found twelve and missed the archive `<details>`
block (`render-board.sh:308-314`) because the sweep anchored on the literal `done|killed`, which
cannot see the `ARC_COUNT[done]` / `ARC_COUNT[killed]` spelling. This is [[enumerated-floor]]
landing on the very inventory that cites it — recorded rather than quietly patched, because it is
direct evidence for the change's own thesis, and because it means **the build's reconcile pass must
re-derive the inventory by semantics, not by re-running the same keyword grep.***
Chosen because the stub itself directs deriving the site list from a whole-repo grep, and because
the standing preference on a "finish the job 0104 started" change is the comprehensive fix.
Decisive supporting fact: all six affected scripts **already source the lib**, so the wider scope
costs no new plumbing and no new dependency edges. *Rejected:* limiting to `render-board.sh` +
`board-checks.sh` — that would leave `github-mirror.sh:54` as a hand-written copy of
`DOCKET_STATUSES_ACTIVE` **already drifted in order**, i.e. shipping the exact defect the change
exists to remove, in the file the grep was requested to find.

**A2 — Single-status predicates are out of scope.**
The in-scope test is *"does this site enumerate a set that should equal an array?"*, not *"does this
line mention a status name?"*. `[ "$status" = "done" ]` is a specific predicate about `done`, not a
restatement of `DOCKET_STATUSES_TERMINAL`. *Rejected:* a maximal "every status literal becomes an
array reference" reading of "comprehensive" — it would obfuscate ~12 correct predicates and, worse,
make a future reader think a `done`-specific code path is vocabulary-general. **Risk accepted:** a
human who meant the maximal reading gets less than they asked for; the boundary is stated here and
in the change body so it is visible rather than silent.

**A3 — One ordered `DOCKET_PRIORITIES` array, not two constants (the stub's open question).**
Settled in §2: priority has exactly one ordering and it is normative, so a membership-only second
constant would protect nothing and would itself need syncing. *Rejected:* two constants (adds the
defect being removed); *rejected:* deriving the default as "index 2" (makes a documented independent
fact into a positional accident).

**A4 — `STATUS_OPTIONS`' Projects v2 column order is allowed to change. ⚠ Most consequential.**
Deriving from `DOCKET_STATUSES_ACTIVE` reorders the GitHub Projects v2 single-select options,
putting `in-progress` before `proposed`. Judged acceptable because: the options are written only on
the **create** path, so **existing** project boards are untouched and only a newly-minted board
differs; and the new order matches `BOARD.md`'s section order, making the two surfaces consistent
rather than divergent. Per [[consolidation-flattens-caller-variance]] this *is* real per-caller
variance and was deliberately checked rather than assumed to be duplication — the conclusion is that
the variance is unintentional (nothing documents a reason for `proposed`-first), not load-bearing.
*Rejected:* preserving the current order via a separate explicitly-ordered constant — that is a
second list to keep in sync, i.e. the defect again. **This is the one assumption whose reversal a
human might reasonably want**: if the GitHub board's column order is intentional, the remedy is to
say so in a comment and keep an ordered constant. The change body's Open questions records it.
Two verified facts this rests on, recorded so a future reader knows what would invalidate it:
`STATUS_OPTIONS` has exactly **one** consumer, `github-mirror.sh:298`, inside the
`elif [ "$AUTOCREATE" = 1 ]` branch of `sync_projects` (`:281-300`) — when `--project` is set that
branch is never reached, so existing boards are provably untouched; and `proj_option_id` (`:268-271`)
resolves options **by name**, so reordering cannot break item-status assignment even on a freshly
minted board. **If either changes — options written on an update path, or a positional option
lookup — A4 collapses and the reorder must be revisited.**

**A5 — The vocabulary lives in `docket-frontmatter.sh`, not a new `docket-vocabulary.sh` lib.**
The status arrays already live there and all consumers already source it; a new file would mean
six new `source` lines for zero behavioral gain. *Rejected:* a dedicated vocabulary lib — cleaner
on paper, but it is a refactor of the lib layout wearing this change's clothes. Revisit if the
vocabulary block outgrows the file.

**A6 — A mapping is pinned iff it is exhaustive over a named array; the test is semantic, not
syntactic.**
*Revised after critic round 1.* The first draft stated the rule syntactically — "a total mapping has
no `*)` arm" — and that discriminator is **wrong**: `github-mirror.sh:127-144` (`readiness_label`)
has no `*)` arm and is correctly sparse (2 arms over 7 statuses, intent documented at `:122-126`),
so the syntactic rule would pin it and redden the suite permanently. Its structural twin, the
close-reason `case` at `:241-242`, is also `*)`-less and 2-armed and *is* pinned — because it is
exhaustive over `DOCKET_STATUSES_TERMINAL`. Identical syntax, opposite dispositions. The corrected
rule asks which array, if any, the mapping is meant to cover. *Rejected:* pinning every `case` over
a vocabulary; *rejected:* the syntactic shortcut (disproven above); *rejected:* leaving the rule
implicit in which guards exist — 0104's decayed deferral comment is the evidence against that, which
is also why A11 records an ADR rather than trusting a comment again.

**A7 — `table_header_for` is extracted so the existing `case_labels` tokenizer works unchanged.**
Reusing a proven, already-mutation-tested extractor beats authoring a second fragile awk range
([[plan-supplied-test-code-is-unverified]] records two range-anchoring defects of exactly this kind).
The row-format `case` cannot be extracted the same way (per-arm argument lists differ), so it gets
one new extractor — with its own count assert. *Rejected:* writing two new extractors; *rejected:*
forcing the row-format arms into a uniform signature purely to fit the tokenizer (that would bend
production code around a test's convenience).

**A8 — Behavior neutrality is proven by the existing goldens, with exactly two blessed diffs.**
`BOARD.md`, the digest, and the `ready` line must not move. The two intended diffs are named in
*Behavior preservation* so a reviewer can tell a blessed re-bless from a regression. **Risk
accepted:** a golden re-bless is the moment a real regression hides; the mitigation is that the two
diffs are named up front and everything else must be byte-identical.

**A9 — Dependency state: `depends_on` is empty and stays empty; related changes compose by intent.**
Change 0111 has now landed and added `BOARD_CHECK_IDS` beside `DOCKET_STATUSES`; this change keeps
that array intact while extending the same vocabulary block. Change 0115 (archive-side
`board-row-dropped`) remains proposed and would consume the helpers established here, but is not
blocked on 0116. The `related:` links remain accurate without manufacturing a dependency gate.

**A10 — Prose restatements are deferred, not forgotten.**
They cannot be derived from a bash array, so they are outside "single-source"; guarding them is a
structural-sentinel change of a different shape. *Rejected:* folding a prose sentinel in here — it
would double the change's surface and mix a mechanical refactor with a new guard subsystem.
**Risk accepted:** the prose stays driftable after this change ships; §Deferred names it so the gap
is a recorded decision.

**A11 — This change records one ADR (the §4 mapping rule). — added after critic round 1.**
Argued in *Expected ADR*. The critic correctly flagged that "no ADR" was smuggled in as an omission
(`adrs: []`, never discussed) while A6 simultaneously argued that test-file comments decay. Deciding
it explicitly: the rule is general, its discriminator is non-obvious with a live counterexample, and
it sits beside ADR-0049/0050. *Rejected:* no ADR (repeats 0104's decayed-comment failure);
*rejected:* an ADR per vocabulary (three ADRs restating one rule — the defect, in ADR form).

**A12 — `STATUS_OPTIONS`' assignment moves below the `source` line. — added after critic round 1.**
A mechanical prerequisite, not a preference: at `github-mirror.sh` the assignment (`:54`) precedes
the `source` (`:80`), so an in-place derivation silently expands to empty and degrades to a
best-effort `log` at `:299`. Recorded as an assumption because A1's evidence ("all six scripts
source the lib") proves *sourced*, not *sourced before use* — a gap that holds at one of thirteen
sites and would have shipped a silently broken GitHub board. **Risk accepted:** other order-of-
definition hazards may exist at sites not yet examined; the build must check source-order at each
converted site rather than trusting A1's table.

## Reconcile update — 2026-07-22

Re-derived all thirteen executable enumerations against `origin/main` at `c3ad10fb`; none has been
removed or fundamentally reshaped. Change 0111 landed first and added `BOARD_CHECK_IDS` plus its
four-way correspondence guard, so the shared library and test extensions must preserve that work.
Change 0115 remains proposed with no feature branch. Corrected the affected-script count from seven
to six: `render-change-links.sh` sources the library but contains no in-scope enumeration. No other
scope change or follow-up was warranted.

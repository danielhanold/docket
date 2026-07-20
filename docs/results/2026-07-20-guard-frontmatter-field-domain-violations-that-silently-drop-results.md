# Guard frontmatter field-domain violations that silently drop board rows — results

Change: #104 · Branch: feat/guard-frontmatter-field-domain-violations-that-silently-drop · PR: <url> · Plan: docs/superpowers/plans/2026-07-20-frontmatter-field-domain-guard-plan.md · ADRs: 49, 50

## Verify (human)

Nothing interactive is required — the whole suite is green (52/52 `tests/test_*.sh`) and the live
backlog was verified at build time (see *Findings*). Two judgment calls are worth your eye at the
merge gate:

- [ ] **The change-id column is now mixed-format.** Findings key on the frontmatter id when it is
      valid (`field-domain 71`) and on the filename-derived padded id when it is not
      (`board-row-dropped 0070`). This deviates from the spec's blanket "use the padded id"
      wording — deliberately, because applying it everywhere would have silently renumbered every
      existing check's output. Reasoning and the general rule are in **ADR-0049**. Confirm you
      accept the mixed format over a one-time break of the report format.
- [ ] **`board-row-dropped` is bounded to `active/` and the symmetric archive-side gap is real,
      known, and shipped undetected.** An `archive/` file carrying a non-terminal status is counted
      in the board's total and rendered nowhere; nothing reports it. It is documented honestly in
      `scripts/board-checks.md` and tracked as change **#115**. Confirm you are content to ship the
      active-side half now rather than hold the change for both.

## Findings

**The whole-branch review found 2 Critical + 4 Important issues in work that had already passed
five per-task reviews with a green suite.** All were fixed on the branch. The class is instructive:
every one was a way for a fully green guard to be proving less than it claimed.

**Critical — the backstop did not compute its invariant, and a live drop path went undetected.**
`board-row-dropped` was specified as a computed count-vs-rows invariant. The first implementation
populated it from two hand-written conditions that were *the same two conditions* `malformed-id` and
`field-domain` already enumerate — a fourth restatement wearing the word "invariant". Consequence:
an `active/` file carrying a **terminal status** (`done`/`killed`) is counted in `render-board.sh`'s
`total` but rendered in no section (`print_section` runs only for the five active statuses; the
count line's `done|killed` arm reads the archive-only `ARC_COUNT`). `field-domain` was silent
(`done` IS in the vocabulary), `malformed-id` was silent (the id is valid), and nothing set
`DROPPED`. The board rendered `**2 changes**` above a single row with every check passing — the
exact symptom this change exists to eliminate, on a state the toolchain documents as reachable
(`sweep-failed <id> archive <reason>`). Fixed by deriving a `renders_row` predicate from the
renderer's real bucketing. Recorded as **ADR-0050**.

**Critical — half the backstop's population was dead code, and its test was a tautology.** The
status path set `EXPLAINED` unconditionally in the same block that set `DROPPED`, so every entry it
created was guaranteed suppressed. Deleting that code left the suite green, and the "yields exactly
ONE finding (suppression works)" assert was checking a self-cancelling pair rather than a
suppression decision. The lesson for review: **mutation-test a backstop by deleting its population,
not only its suppression** — a suppression assert passes vacuously when the invariant never
computes. Folded into ADR-0050.

**Important — the guard's own reporting channel stayed forgeable in a second dimension.** Part 3
hardened the *field separator*, but `docket-status.sh`'s `reclaim_pass` keyed a **mutating** code
path on an unscoped `grep -qF "[reclaimable]"` over the whole findings blob. Since `field-domain`
messages echo untrusted frontmatter by design, `title: Sneaky | thing [reclaimable]` forged the
marker — verified live. Pre-0104 only `malformed-id`'s `id:` echo could reach this; this change
widened the surface to `status`, `slug` and free-form `title`. Fixed by scoping the consumer to the
check-id column with the marker anchored at end-of-line, for both the gate and its count. The
contract's new "the findings channel is not injectable" invariant was correspondingly split into
*columns are not forgeable* vs *message text is untrusted* — the original sentence was false.

**Important — forward-defensive code that would have caused the failure it defended against.**
`EXPLAINED` was marked from all four `field-domain` arms. But a bad `priority` or a piped `title`
does not drop a row (title *injects columns*; priority renders raw), so had a future drop path
populated `DROPPED`, an unrelated pipe in a change's title would have silenced the backstop — the
exact false-suppression mode the spec warns about. Restricted to the arms that genuinely explain a
drop.

**Three defects in the plan's own supplied test code, each caught by the implementer running the
tests as code rather than as an oracle:**

- `has_finding "$out" malformed-id "?"` was **vacuous**: the helper built an unescaped ERE and `?`
  is a quantifier, collapsing the pattern to `^malformed-id\t` and matching any line of that
  check-id. It was green even against the pre-implementation baseline. Fixed at the helper's
  definition (literal `case` match + here-string, which also removed a `printf | grep -q` pipefail
  hazard) rather than only at the call site — `cid` can legitimately be `?`, so later tasks would
  have re-hit it.
- Two asserts used a literal `\t` inside `grep -E`, which **BSD grep does not interpret**. Rewritten
  to the repo's portable `grep -E "$(printf '^x\ty\t')"` idiom.
- The plan's Step-2 registration-verification command was itself broken — anchored to line start, it
  missed `emit` calls following `||` guards and found only 9 of 11 check-ids. A corrected
  unanchored derivation confirmed zero registration gaps.

**Live-data verification (the hermetic suite cannot see this).** Per the repo's
`metadata-branch-invisible-to-suite` lesson, the checks were run against the real metadata branch,
not only fixtures:

- `render-board.sh` against the live `.docket/docs/changes` renders **byte-identical** to the
  committed `BOARD.md` — the part-4 refactor moved no bytes.
- `board-checks.sh` against the live backlog emits **zero findings of any check-id**, exit 0.
- Detection proven non-vacuous against real data: poisoning change 0083's `status:` in a throwaway
  copy fired exactly one `field-domain` finding and zero `board-row-dropped` (suppression held).
- Filtering note for anyone repeating this: filter by **column**
  (`awk -F'\t' '$1=="field-domain"'`), not substring — change 0104's own spec *path* contains the
  text "field-domain" and a naive `grep` miscounts it.

**Registration drift was found in both directions and both were repaired.** The spec knew
`board-checks.sh`'s header omitted `malformed-id`. Reconcile found the converse: `docket-status.md`'s
*closed* check-id enumeration omitted `stale-finalize-blocked`, which change 0098 shipped without
ever registering. Nothing in the suite binds the emitted check-id set to any of the three
enumerations, so the drift recurs on the next check-id — tracked as change **#111**.

## Follow-ups

- **#111 — Guard the board-checks check-id enumerations against drift.** The correspondence between
  the emitted set and its three registration surfaces runs *zero* ways today. Minted at reconcile,
  before the review independently reached the same conclusion.
- **#115 — Extend the board-row-dropped invariant to `archive/` files.** The symmetric drop path
  described in *Verify (human)* above. Reachable from the same interrupted `archive-change.sh`
  sequence (its `git mv` precedes the status flip and the commit).
- **#116 — Single-source the remaining duplicated board vocabularies.** Part 4 single-sourced the
  status vocabulary and left three more duplicated at identical drift risk: the terminal-status
  `case` arms, the `print_section` call list, and the priority vocabulary. The `print_section` list
  matters most now — `renders_row` claims to mirror it, and that correspondence is asserted only by
  comment and fixtures.

**Carried Minors, deliberately not fixed:** the walk-loop `cid` shares its name with the dep-cycle
DFS loop's `cid` (both bare globals; verified harmless because every emit precedes the reassignment,
but it is a trap for anyone reordering blocks); several pre-existing `printf | grep -q` call sites in
`tests/test_board_checks.sh` retain the pipefail early-exit hazard the `has_finding` rewrite closed;
and `docket-status.sh`'s `reclaim_pass` scoping now couples to `health_checks`'s space-joined
`check <id> …` output shape rather than the raw TSV — load-bearing and only tested end-to-end.

**Notable plan deviation.** The plan prescribed the spec's blanket "change-id column uses the
filename-derived padded id". Implemented narrowly instead — padding only where a raw frontmatter
value would otherwise appear — because the blanket form would have silently renumbered every
existing check's output and broken ~15 asserts the spec never argued for. The spec's *rationale*
(the column must never carry a field-shifting value) is fully honored. See ADR-0049.

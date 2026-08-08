<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0200 — Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0200-clear-the-unfixed-review-findings-from-change-0191.md)**
<!-- docket:backlink:end -->

# Board-checks hardening — sanitize LF escape, capture-shape mutation, minor-finding clearance (change 0200)

Design for the consolidated fix change: 0191's surviving minor finding, 0215's sanitize LF gap,
0216's capture-shape mutation guard, and 0217's three minor findings from change 0202. Files
touched: `scripts/board-checks.sh`, `tests/test_board_checks.sh`, `skills/docket-convention/SKILL.md`,
and `tests/test_skill_size_budgets.sh` (the convention file's budget row — see (d)).

## (a) Hoist `scalar_form_check` out of the per-file walk

`scalar_form_check(){…}` is defined at `board-checks.sh:355`, inside the `for f in "${FILES[@]}"`
walk that opens at line 226 — the function is redefined once per change file. Move the definition
with its full comment block — which starts at the `# --- scalar-form:` marker, line **320**, not
at the function line — to top level beside `renders_row` (line 214), keeping the two call sites
(`scalar_form_check title/blocked_by`) and the `sf_title`/`sf_blocked_by` reads inside the walk.
The body references the loop variable `$cid` unqualified; bash resolves it dynamically at call
time, so the hoist is behavior-neutral.

**Mutation 4 MUST be redesigned in the same task, not merely re-run.** Its extraction (test line
1035) is an awk range-delete from the `# --- scalar-form:` marker to `# --- broken-spec:`. After
the hoist that start marker sits at top level: the range-delete would gut `renders_row`, the FILES
`mapfile`, and the walk's `for` opening, leaving an orphaned `done` — a syntactically dead script
whose landed assert (count 3 → 0) still passes and whose "goes GREEN" asserts pass vacuously
(`mrun` discards stderr). Rework it as a two-region delete — the hoisted definition by its own
marker range at top level, the two call-site lines (plus their `sf_*` reads) by a separate
marker or line match inside the walk — and add the `bash -n "$MUTSCRIPT"` landed assert mutation
4 currently lacks, mirroring siblings B/D2/E. Verify mutations 1, 1b, 2, 3, 3b still land (their
seds hit lines inside the function body and are location-independent).

## (b) `sanitize` escapes a raw LF

`sanitize` (line 142) gains a third substitution: `v="${v//$'\n'/\\n}"`. The three substitutions
each target a distinct literal character, so ordering is immaterial. Rewrite the comment above it
(lines 135–141): keep the TAB/CR rationale, drop the implicit premise that every embedded value
passes through `field`/`fm_field` newline truncation, and state the new reason — since change 0202
`branch_only_artifact` delivers **git paths** NUL-delimited (`ls-tree -r -z`), so a path with an
embedded newline reaches `emit` raw; escaping it keeps one finding = one TSV record and preserves
downstream `sort`/`IFS=$'\t' read` determinism.

Two coverage caveats the rewritten comment must respect: it must not imply full LF coverage —
the call sites capture `branch_only_artifact` via `$(…)`, which strips a *trailing* newline before
`sanitize` ever runs, so only interior LFs are escaped; and `sanitize` remains a record-shape
guard, not a completeness guarantee.

Test: a new ARQ-family fixture (modeled on ARQ1, `test_board_checks.sh:1328`, and placed with the
ARQ block — after `AR_FRESH_CLAIM`'s definition, which it consumes under `set -u`): a repo whose
feature branch carries a branch-only plan file whose path embeds an *interior* literal LF
(`printf`-built name), change file `status: in-progress`, `plan:` unset → leg A fires with
`$ar_hit` holding the LF path. The discriminating assert is that the finding's message contains
the two-character escape `\n` **with the path's post-LF tail on the same line** (e.g. grep the
single `^aborted-run\t<id>\t` line for `…\n<tail>`): on the unfixed script the tail lands on a
continuation line, so a bare "exactly one line matches the prefix" count passes either way and
must not be the RED. Fixture id: next free per the suite's allocation comment (line 1526); the
file is created with `printf`/`touch` (an LF filename is legal on APFS/ext4, the suite's only
platforms).

## (c) Mutation O — the capture-shape constraint becomes executable

Letters A–N are taken (F = C-quoting revert, G = idle floor); the new arm is **mutation O**,
placed with F in the branch_only_artifact group. Following the F/G pattern
(`test_board_checks.sh:2473`): `armreseed`, then sed-rewrite `ARMSCRIPT`'s consumption from
`done < <("$GIT" … ls-tree -r -z …)` to the forbidden capture shape —
`boa_list="$("$GIT" … ls-tree -r -z …)"` before the loop, `done <<<"$boa_list"` after — keeping
`-z` and `read -r -d ''` exactly as the "do not simplify this back" comment (line 110) forbids.
Asserts: (1) landed — the process-substitution form is gone (count 1 → 0) and the here-string form
present; (2) `bash -n "$ARMSCRIPT"` still passes (the trap this arm exists to expose: the broken
shape is valid bash); (3) `armrun_at "$ARQ1"` → fixture 230 goes **GREEN** (`! has_finding …
aborted-run 230`) — command substitution strips the NULs, `read -d ''` hits EOF, the loop never
runs, leg A goes silently false-negative. 230's baseline firing is already pinned (line 1348), so
the GREEN assert is not vacuous. `rm -rf "$armcopy"` at the end, as siblings do.

## (d) 0202's minor findings

- **Dead guard** (line 125): delete `[ -n "$boa_p" ] || continue`. Under `-z`, `ls-tree` never
  emits an empty record, and at EOF `read -d ''` returns nonzero with an empty accumulator, ending
  the loop — the guard is unreachable. Removal over keep-with-comment: by the repo's own rule
  unguarded prose is decoration, and dead code invites "what does this protect?" archaeology.
  Existing fixtures 230/231 plus mutations F and O cover the function; no new test.
- **Mutation-baseline comment** (line 2305): reword "fires exactly the three expected findings" to
  drop the count (e.g. "fires the expected baseline findings, pinned one by one below") — the
  asserts beneath it are the guard; a re-pinned number would drift again with no enforcement.
- **Frozen merged plans** (0217's policy question): merged plan files are **frozen build records**
  — never edited after their branch merges. Record this as one short paragraph in
  `skills/docket-convention/SKILL.md`, in the artifacts/plan-field area (near line 169/196): every
  workflow skill loads docket-convention as blocking Step 0, so this is the surface a future build
  reads before touching a plan. **This edit forces a size-budget raise**:
  `tests/test_skill_size_budgets.sh` pins the convention file at 345 lines / 5900 words (line 783)
  and it currently sits at ~342 / 5865. Raise the row minimally, and satisfy the table's header
  rule (change 0201) in-diff: name the `references/` file considered and argue why the prose
  cannot live there — the argument is the rule's own rationale: it must be on the always-loaded
  Step-0 surface, because a build consults references/ only on demand and the rule must fire
  *before* anyone decides to touch a plan. No ADR: it is a docs-lifecycle convention, not an
  architecture decision, and the convention skill is the designated single source. The 0113 plan
  file
  (`docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md`) —
  whose Task-5 grep matches its own explanatory comment — stays untouched.

## Resolved open questions

1. *Other `emit` callers passing non-frontmatter values?* Yes — messages embed branch names,
  basenames, computed counts, and `$ar_hit` git paths. That is exactly why the fix lands in
  `sanitize` (which wraps both the id and message columns of every `emit`) rather than at the
  `$ar_hit` call sites: every current and future caller is covered without auditing them.
2. *Capture-shape hazard elsewhere?* No — a repo-wide grep shows `branch_only_artifact` is the
  only NUL-delimited read in `scripts/` (the only other `done < <(…)` in board-checks.sh, line
  843, is LF-delimited `git log`). One site → one mutation arm; no helper-level guard.

## Assumptions

- **depends_on 224 is `implemented`, not merged** (PR #174 open as of 2026-08-07). There IS one
  real file collision: 0224's branch also edits `tests/test_skill_size_budgets.sh` (it raises
  docket-build's budget row; this change raises docket-convention's). The dependency sequences
  that collision away — build-ready selection waits for 224 to reach done, so this change's
  budget-table edit lands on top of 224's — and no design decision here depends on 224's content.
  Designed ahead per contract. Rejected: treating the dep as a design input (the shared file is a
  different table row; ordering, not content, is the coupling).
- **LF escape lives in `sanitize`, not at the `$ar_hit` call sites.** Covers every emit caller,
  current and future; resolves open question 1 by construction. Rejected: call-site escaping
  (repeats per caller, misses the next non-frontmatter value) and rejecting LF paths outright
  (the check's job is to report reality, not police filenames).
- **Mutation letter O** — verified A–N all taken in `tests/test_board_checks.sh`; the stub's
  "G is taken" note confirmed (G = leg-C idle floor, line 2502). Mechanical.
- **Mutation O asserts GREEN-on-230 via `armrun_at "$ARQ1"`,** reusing the existing ARQ1 repo and
  the F/G arm pattern (reseed, sed, landed-count asserts, `bash -n`, cleanup). Rejected: a
  dedicated fixture repo (ARQ1 already discriminates — its 230 fires at baseline and can only go
  green through the stripped-NUL failure).
- **One mutation arm, not a helper-level guard** for the capture-shape hazard — grep shows exactly
  one NUL-delimited read in `scripts/`. Rejected: a generic "never capture -z output" lint (guards
  a class with zero other members; the learnings ledger already carries
  `git-path-output-is-quoted` for future authors).
- **Delete the dead `[ -n "$boa_p" ]` guard** rather than keep-and-explain. Unreachable under
  `-z`; keeping it would be exactly the unguarded-decoration pattern this change exists to close.
  Rejected: keep with comment (adds prose with no enforcement).
- **Baseline comment drops the count** instead of re-pinning it — the per-fixture asserts are the
  guard; a number in a comment re-drifts silently. Matches the stub's own recommendation.
- **Hoisted `scalar_form_check` keeps dynamic-scope reads of `$cid`** — behavior-neutral in bash;
  the scalar-form fixtures and mutations 1–3b are the regression net. **Mutation 4 is redesigned,
  not re-run**: its marker-to-marker range-delete would span top-level code after the hoist,
  producing a syntactically dead copy whose landed and GREEN asserts all pass vacuously (it has no
  `bash -n` assert today). The spec directs a two-region delete plus the missing `bash -n` landed
  assert. Rejected: "re-verify and adjust if needed" (a broken mutation satisfies that check —
  green for the wrong reason), and leaving the definition in the loop (the finding this item
  exists to clear).
- **Frozen-plans rule lands in `skills/docket-convention/SKILL.md`, no ADR, 0113 plan untouched —
  and pays its size-budget toll.** The convention skill is the single source every workflow skill
  loads at Step 0 — "where a future build will read it". The paragraph exceeds the file's pinned
  345/5900 budget (currently ~342/5865), so the change raises that row of
  `tests/test_skill_size_budgets.sh` minimally with the in-diff references/-considered argument
  the table's header rule requires. Rejected: an ADR (docs-lifecycle rule, not architecture);
  editing the 0113 plan (would violate the very rule being recorded); putting the rule in a
  `references/` file to dodge the budget (on-demand surfaces fire too late for a rule that must
  precede touching a plan).
- **Coupling: add 222 to `related:`** — the re-scope leans on 0222's bash-4.4 floor ruling (it is
  what dropped leg (e)); a reader auditing the dropped scope needs the pointer. 0235 stays
  prose-only (it bears only on already-dropped items).

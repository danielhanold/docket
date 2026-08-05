<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0202 — Clear the unfixed review findings from change 0113](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0202-clear-the-unfixed-review-findings-from-change-0113.md)**
<!-- docket:backlink:end -->

# Clear the unfixed review findings from change 0113 — results

Change: #0202 · Branch: feat/clear-the-unfixed-review-findings-from-change-0113 · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md · ADRs: none

All five of change 0113's unfixed review findings are closed. One was a real production
false-positive (`branch_only_artifact`); three were coverage holes; the fifth was verified as
already satisfied by an earlier merge and deliberately left unedited.

Build: 5 tasks, 4 commits (Task 5 is verify-only by design), no escalations. Full suite green
across 78 test files at `fca49838`.

## Verify (human)

- [ ] **Decide the two `important` review findings below.** Neither is auto-fixed — docket records
      non-blocking findings for merge-time judgment rather than repairing them in-branch. Both are
      real; both are arguably their own small change (which is exactly how *this* change was born
      from 0113). Either fold them in before merge or file them as a follow-up.
- [ ] Optional spot-check of the non-ASCII fixture on a second machine. `core.quotePath` is set
      per-fixture-repo so a developer's global `core.quotePath=false` cannot silently disarm
      mutation F, but the fixture was only ever exercised on macOS/APFS.

## Findings

Reviewed at the **deep** rung (highest task profile was `premium`; 892-line diff, under the
1500-line bump threshold). **0 blockers, 2 important, 3 minor.**

### Important — not fixed, for merge-time judgment

1. **`sanitize` does not escape newlines, and `-z` can now deliver one.** `sanitize` renders only
   TAB and CR, justified by a comment asserting every emitted value arrives via `field`/`fm_field`,
   which truncate at the first newline. `$ar_hit` is the one emitted value that is a **git path**,
   not frontmatter. Previously `--name-only`'s C-quoting guaranteed an embedded newline arrived as
   the two characters `\n`; under `-z` the raw LF reaches `emit`, splitting one finding across two
   TSV records and breaking the `sort` determinism. Suggested fix: add
   `v="${v//$'\n'/\\n}"` to `sanitize`. Note the trigger is a pathological path (git permits
   embedded newlines; nothing in this repo has one), and the check is warn-only — which is why the
   reviewer graded it important rather than blocker.
2. **Mutation F's dropped half leaves the "cannot capture NUL" constraint unguarded.** The spec
   asked mutation F to also revert the capture-then-here-string shape; the build reverted only `-z`
   and the `read -r -d ''` form, reasoning that C-quoting comes from `ls-tree` and not from the
   consumption shape. That reasoning holds for reproducing finding 4's defect, and ARQ2 does
   discriminate it. But it drops the only coverage of the *other* property the new code depends on:
   a refactor to `boa_list="$(… ls-tree -r -z …)"` + `done <<<"$boa_list"` keeps `-z`, keeps
   `read -r -d ''`, passes `bash -n`, and makes `branch_only_artifact` return 1 for **every** input
   (command substitution strips NULs, so `read -d ''` hits EOF and never enters the loop). Leg A
   would go permanently, silently false-negative with a green suite. `board-checks.sh` carries an
   explicit "do not simplify this back" instruction with nothing enforcing it — decoration, by this
   repo's own guard rule. Suggested fix: a mutation G that rewrites to the capture shape (keeping
   `-z`) and asserts fixture 230 goes GREEN.

### Minor — not fixed

3. `[ -n "$boa_p" ] || continue` in `branch_only_artifact` is unreachable under `-z` (`ls-tree -z`
   emits no empty records) and is now unguarded dead code beside a carefully-argued comment block.
4. The mutation-baseline comment "fires exactly the three expected findings" was already stale
   (four asserts) and this change widened the gap to six findings across five fixtures. Better
   reworded to drop the number than to re-pin it.
5. The plan's Task 5 Step 2 verification pattern (`grep -nE '4013|4050|147 for 143'`) matches the
   comment line that *explains* 4050 is stale, so following it literally would halt on a false
   positive. The build reached the right outcome anyway, but the plan ships on the branch as an
   inaccurate instruction.

### Deviations from the spec and plan (both deliberate, both recorded in-code)

- **ARQ fixture placement.** The spec put the new non-ASCII fixtures after the `ar8_custom` block;
  `AR_FRESH_CLAIM` is defined below that point, so under `set -u` a heredoc referencing it there
  aborts the whole test file with an unbound-variable error. The block moved below the
  `AR_STALE_CLAIM`/`AR_FRESH_CLAIM` definitions, every fixture line verbatim, with an in-place
  comment recording why.
- **Mutation F's shape** — see important finding 2 above.
- **Task 1 RED-first was broader than the plan predicted, in the safe direction.** The plan expected
  only the id-231 inherited assert to fail against the unfixed script; the id-230 "reports the
  unquoted path" assert also went red, because the finding reported the C-quoted rendering. Same
  defect, second symptom. Both green after the fix.

### Verification notes

- **Finding 5 confirmed already satisfied, no edit made.** `tests/test_skill_size_budgets.sh`
  records `3728 words -> 3800 (72 words of margin)` and `139 actual, 145 budget`. The stale
  pre-rebase figures from the original finding (`4013 -> 4050`, `147 for 143`) are absent as
  assertion forms under both PATH `grep` (ugrep) and `/usr/bin/grep`.
- **Task 3's mutation was restored byte-exact.** `scripts/docket-status.sh` does not appear in
  `git diff origin/main...HEAD` at all — verified independently of the worker's own report.
- Every new grep pattern was re-checked under `/usr/bin/grep`, since PATH `grep` is ugrep 7.5.0 and
  accepts constructs BSD grep rejects.

## Follow-ups

- **Change 0213** (auto-captured during reconcile) — settle the bash 4.4 `mapfile -d` floor
  inconsistency: `tests/test_grep_portability.sh` uses `mapfile -d` while the shipped-script floor
  is bash 4.0 and `ensure-docket-env.sh` validates only major >= 4. Surfaced by this change's spec
  when it rejected `mapfile -d` for the `branch_only_artifact` rewrite.
- The two `important` findings above, if not folded in before merge.
- **Change 0211** adds a third leg to the same `aborted-run` block and extends the same test file.
  It was `waiting-on-202-unbuilt` at claim time and now builds on hardened predicates.

<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0228 — finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0228-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator.md)**
<!-- docket:backlink:end -->

# Design — finalize's auto-detect suite loop needs a failure accumulator

Change: 0228 · Drafted autonomously by `docket-auto-groom` (2026-08-07).

## Problem

`skills/docket-finalize-change/SKILL.md` publishes the suite command boundary in its
`configured-bash-finalize` marker block. The auto-detect branch is a bare loop:

```bash
for test in tests/test_*.sh; do
  "$DOCKET_BASH_PATH" "$test"
done
```

A `for` loop's exit status is its **last** command's. A mid-suite red therefore leaves the block
green, and both consumers of the block — `docket-finalize-change`'s merge gate (step 5: non-zero ⇒
dispatch `docket-integration-repair`) and `docket-build`'s build gate (non-zero ⇒ the
`premium → max → halt` repair ladder) — read that green and proceed. The gate that exists to keep a
red suite off the integration branch only reliably catches a failure in the alphabetically last
file.

The block is the **single** published source: `skills/docket-build/SKILL.md` (§ build gate)
deliberately does not copy the fragment and names finalize's block as the sole source, so one edit
repairs both consumers. Copies elsewhere in the tree are historical plan/results artifacts, not
live sources, and are not edited.

**Correction to the stub's premise.** The stub says
`tests/test_configured_bash_finalize.sh`'s assertions "pin the command's *text*". They do not — the
guard extracts the fragment and **executes** it, asserting on execution/runtime/argv logs. The gap
is narrower than the stub states: the guard already runs the fragment, it simply never inspects the
fragment's **exit status**. That is the one assert to add.

## Design

Replace the auto-detect branch with a keep-going accumulator; leave the
`FINALIZE_TEST_COMMAND` branch byte-identical (user-authored text is executed unchanged by
contract — the stub's stated non-goal).

```bash
if [ -n "${FINALIZE_TEST_COMMAND:-}" ]; then
  eval "$FINALIZE_TEST_COMMAND"
else
  suite_status=0
  for test in tests/test_*.sh; do
    "$DOCKET_BASH_PATH" "$test" || suite_status=1
  done
  [ "$suite_status" -eq 0 ]
fi
```

Properties this preserves deliberately:

- **Every test still runs** under the configured runtime, in the same order — the two existing
  execution asserts in `tests/test_configured_bash_finalize.sh` stay valid unchanged.
- **The empty-suite signal is unchanged.** No `nullglob`, no `[ -e "$test" ]` guard: with no
  matching files the glob stays literal, the invocation fails, and the block reports non-zero.
  `nullglob` would instead exit **0 with zero tests run** — a green gate certifying nothing.
  Note that neither caller keys its "no detectable suite" decision on this exit status:
  finalize's step 3 pre-checks whether a suite exists, and docket-build's § build gate explicitly
  warns that reading the literal-glob non-zero as RED "would manufacture a repair task". So the
  argument for keeping the literal-glob failure is the zero-tests-run one, not an existing
  exit-status contract.
- **The fragment stays free of gate logic.** `tests/test_docket_review.sh` asserts the fragment
  contains no `evidence|skip|head_sha`; the new text introduces none.

## What changes

1. `skills/docket-finalize-change/SKILL.md` — the `configured-bash-finalize` marker block's
   auto-detect branch, edited as a whole-marker-block replacement.
2. `tests/test_configured_bash_finalize.sh` — a third fixture repo in which a **non-final** test
   exits 1, asserting (a) the extracted fragment's exit status is non-zero and (b) all three tests
   still executed (the keep-going property). Written as an assert that **detects the removed
   state**: reverting the accumulator must redden it, and that mutation is to be run and recorded,
   not assumed. Add a **nullglob-absence** assert too, since the empty-suite property above is
   load-bearing and currently unguarded — either a fragment-text assert or an empty-fixture
   execution asserting non-zero with an empty execution log.
   **Fixture-isolation constraint (must not be discovered at build time):** the existing asserts at
   lines ~85–90 pin the *shared* `RUNTIME_LOG` at exactly 2 lines and reuse one `$fixture` dir. The
   new case must use its own fixture dir and its own `RUNTIME_LOG`/`EXECUTION_LOG` paths, or be
   appended after the existing `wc -l` assert — otherwise it reddens a passing guard. Same class:
   line 77 exports a **non-empty** `FINALIZE_TEST_COMMAND`, so a case appended after it must reset
   that variable or it never reaches the auto-detect branch at all; and an empty-fixture nullglob
   case still writes one line to whatever `RUNTIME_LOG` it is handed.
3. No prose edit elsewhere. **Four** sites describe the block in load-bearing argumentation —
   `scripts/run-tests.md:192`, `scripts/run-tests.sh:70-77`, `tests/test_run_tests.sh:114-118`, and
   `skills/docket-build/SKILL.md:183-187` (the consumer's own literal-glob exit-status claim, in
   the section change 0223 is rewriting). All four were checked; all four ("a bare `eval` of the
   configured test command"; "read any non-zero exit as the suite is red"; "reading that as RED
   would manufacture a repair task") stay true after the fix.
4. **Skill size budget.** `tests/test_skill_size_budgets.sh` pins
   `skills/docket-finalize-change/SKILL.md` at `180 3450`; it is currently 174 lines. The edit adds
   ~2 lines and fits, but the budget must be re-verified at build time, not assumed — change 0190
   is also editing this file.

## Out of scope

- The `FINALIZE_TEST_COMMAND` branch's exit semantics (stub's non-goal).
- Rebinding auto-detect onto `scripts/run-tests.sh`. This repo already sets
  `finalize.test_command: scripts/run-tests.sh`, so the auto-detect branch is the *generic*
  fallback for repos that have no runner; making the fallback depend on a docket-supplied script
  is a separate design question with its own consumers, not a bug fix.
- Distinguishing failure counts, and preserving the failing command's own exit code (127 on the
  literal-glob path collapses to 1). The accumulator is a green/red predicate. The repo already
  argues this explicitly: `scripts/run-tests.md` records that of the block's consumers, "none of
  them can tell 4 from 1."
- Stating the exit-code rule normatively in the build gate contract, and its guard — that is
  change **0224**'s scope, which names "a suite run as a loop over per-file commands" as the very
  surface where the defect became possible. 0228 fixes the fragment; 0224 states the rule. The two
  must not land contradictory edits to the same contract.

## Assumptions

Every decision below was defaulted autonomously; the rejected alternatives and the reason are the
audit trail.

1. **Keep-going vs. fail-fast.** Chosen: keep-going (`|| suite_status=1`, no `break`).
   Rejected: `|| { suite_status=1; break; }` (fails fast, hides sibling reds) and `set -e` around
   the loop (same, plus it changes the semantics of the override branch). Rationale: a repair agent
   handed *all* the reds root-causes once instead of iterating, and the stub names keep-going as
   the more useful shape. **Precedent is an established idiom** (counted, second pass): at least six
   hand-written accumulators of this shape exist — `tests/test_closeout.sh:448,461` (`dangle=1`,
   live in-tree) plus five in `docs/superpowers/plans/` (`rc=1`, `fail=1`, `suite_rc=1`,
   `suite_status=1`); the `rc`/`fail` spellings outnumber `suite_status`. An earlier draft of this
   spec claimed n=1; that was a miscount.
   **Wall-clock cost — checked and found not to apply.** An earlier draft imported change 0223's
   "suite runs at the harness's maximum foreground timeout" measurement as a cost of keep-going.
   That measurement predates change **0227** (parallel runner, `done`, PR #165), after which 0223's
   own spec records the suite "well under the foreground ceiling". It is also the wrong repo's
   number: this repo sets `finalize.test_command: scripts/run-tests.sh`, so the auto-detect branch
   never executes here. The residual risk — a red run in a *consuming* repo burning wall clock
   before reporting — is real but unmeasurable from here, and an abort still blocks the merge
   (fails safe), whereas fail-fast would trade that for hiding sibling reds.
2. **How the status leaves the block.** Chosen: a trailing `[ "$suite_status" -eq 0 ]` as the
   else branch's last command. **Reversed during the critic round** — the first draft chose
   `exit "$suite_status"` on the rationale that both consumers run the fragment as one shell
   invocation; neither `skills/docket-finalize-change/SKILL.md` nor `skills/docket-build/SKILL.md`
   states any invocation shape, so that was a contract asserted into existence rather than read.
   **What was actually verified by executing the fragment** (three shapes, second critic pass):
   under `/bin/bash -c "$contract"` — the guard's shape — it returns rc=1 on a mid-suite red, and
   the guard's own `set -uo pipefail` does not cross the `bash -c` boundary. Inlined into a `set -e`
   wrapper with a trailing sentinel: rc=1 but the sentinel does not print. Inlined without `set -e`
   with a trailing sentinel: **rc=0** — the status is discarded by the next command. So the fragment
   communicates only through `$?` at the immediate point of use, and **any wrapper that appends a
   sentinel must capture `$?` itself** — including change 0223's background-to-a-log posture. That
   obligation is 0223's contract, not this fragment's, and it applies to `exit` equally. The
   trailing test is nonetheless the right default for a published fragment: it is correct in the
   one shape both the guard and a plain gate run use, and unlike `exit` it cannot terminate a
   wrapper early. Rejected: `exit` (strictly worse under inlining) and `( ... )` subshell wrapping
   (a scope with no benefit). Consequence accepted: the red status is normalized to 1 — see
   *Out of scope*.
3. **No `nullglob` / no empty-glob guard.** Chosen: leave the literal-glob failure in place, and
   add a guard for it (see *What changes* item 2). Rejected: `shopt -s nullglob`. Rationale: it
   would exit **0 with zero tests run** — a green gate certifying nothing. The first draft justified
   this by claiming both callers key "no detectable suite" on this exit status; they do not
   (finalize step 3 pre-checks existence; docket-build warns against reading it as RED), so that
   rationale is withdrawn and the zero-tests-run argument stands alone.
4. **Fixture scope — one caller or both.** Chosen: exercise the fragment once, in
   `tests/test_configured_bash_finalize.sh`, and rely on the existing structural asserts in
   `tests/test_docket_build.sh` (docket-build names the block and opens no second marker block) for
   the second consumer. Rejected: a duplicated execution fixture under the build suite. Rationale:
   there is one source and one executable text; a second execution fixture would guard a copy that
   does not exist, and the shared-resource hazard here is a *drifting restatement*, which the
   existing structural asserts already cover.
5. **Variable name `suite_status`.** Chosen: match the single existing hand-written precedent
   (n=1, per assumption 1). Rejected: `fail`, `rc`. Rationale: cosmetic; it keeps the published
   fragment and the hand-run form recognizably the same shape. The name leaks into the caller's
   shell (no `local` outside a function) — acceptable, and now more relevant given assumption 2's
   inlined-execution posture; a caller reusing `suite_status` would collide, which is why the name
   is deliberate rather than generic.
6. **Scope of the marker-block edit.** Chosen: replace the whole marker-bounded block in one edit,
   validating **marker order and balance first** and refusing on a dangling/out-of-order/nested
   marker (`AGENTS.md`, marker-delimited managed blocks), then re-run the existing guard's
   marker-pair and non-vacuity asserts. Rejected: a surgical single-line edit inside the block.
7. **Coupling to other active changes.** `depends_on` stays empty — nothing here needs another
   change merged first — but three couplings are recorded and `related: [190, 223, 224]` is set.
   (0227 is `done`/archived; it is already in `discovered_from:` and matters here only as the
   reason assumption 1's wall-clock number is stale, so it is not carried in `related:`.)
   - **0224** (`proposed`, high) states the green/red-is-the-exit-code rule normatively and names
     the per-file loop as the surface. 0228 is the code half, 0224 the prose half; the boundary is
     stated in *Out of scope*.
   - **0223** (`in-progress`, branch + plan) is rewriting `docket-build`'s § build gate — the
     section that binds this fragment — and its execution posture drives assumptions 1 and 2.
     Re-read it at reconcile.
   - **0190** (`in-progress`) edits step 4 of the same file, *outside* the marker block, and
     re-measures the same skill size budget — a rebase composition, not a conflict. Caveat: 0190's
     plan records the caps as "193 ln / 4350 w" while the tree says `180 3450`, so its budget task
     is working from a stale number; re-measure against the tree, not against either plan.
8. **No ADR.** `adrs: []` carried forward. The first draft's `exit`-form choice would have created
   a new cross-skill "execute this fragment as its own shell invocation" contract, which is
   ADR-shaped; assumption 2's reversal removes that, leaving a bug fix inside an existing contract.
   If a future implementer reinstates the `exit` form, an ADR (or a deferral to 0224) is required.

## Verification

- New fixture: a non-final failing test ⇒ fragment exits non-zero, and all fixture tests appear in
  the execution log.
- Mutation: restoring the accumulator-free loop must redden the new assert, and only it.
- Existing asserts in `tests/test_configured_bash_finalize.sh`, `tests/test_docket_review.sh`
  (fragment purity + non-vacuity anchor) and `tests/test_docket_build.sh` (single source) stay
  green.
- Full suite via `scripts/run-tests.sh`.

<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0228 — finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0228-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator.md)**
<!-- docket:backlink:end -->

# finalize auto-detect suite failure accumulator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the `configured-bash-finalize` marker block's auto-detect branch a keep-going failure accumulator so a mid-suite red reports non-zero, and guard both that behavior and the empty-suite behavior it must not break.

**Architecture:** One published shell fragment, one edit site. `skills/docket-finalize-change/SKILL.md` carries a marker-bounded `configured-bash-finalize` block that is the single source for both consumers of the suite command — `docket-finalize-change`'s merge gate and `docket-build`'s build gate (`skills/docket-build/SKILL.md` deliberately names the block rather than copying it). The block's auto-detect branch is a bare `for` loop whose exit status is its **last** command's, so a red in any non-final test file is invisible. The fix adds a `suite_status` accumulator and a trailing `[ "$suite_status" -eq 0 ]`. The guard `tests/test_configured_bash_finalize.sh` already **executes** the extracted fragment; it simply never inspects the fragment's exit status. Two new fixture cases close that.

**Tech Stack:** Bash (the repo's `DOCKET_BASH_PATH`-configured runtime), the hand-rolled `assert` harness in `tests/*.sh`, `scripts/run-tests.sh` as the full suite.

## Global Constraints

- **Marker-block discipline.** `<!-- configured-bash-finalize:start -->` / `<!-- configured-bash-finalize:end -->` must stay a single balanced, ordered pair. Validate order and balance **before** editing, and refuse on a dangling, duplicated, out-of-order, or nested marker. Replace the whole block, never a surgical single line inside it.
- **The `FINALIZE_TEST_COMMAND` branch stays byte-identical.** User-authored shell text is executed unchanged by contract; its exit semantics are out of scope.
- **No `nullglob`, no `[ -e "$test" ]` guard.** With no matching files the glob must stay literal, the invocation must fail, and the block must report non-zero. `nullglob` would exit **0 with zero tests run** — a green gate certifying nothing.
- **The fragment stays free of gate logic.** `tests/test_docket_review.sh:483` asserts the extracted fragment matches none of `evidence|skip|head_sha`. Introduce none of those words inside the fence.
- **Fixture isolation is mandatory, not optional.** `tests/test_configured_bash_finalize.sh:89-90` pins the **shared** `runtime_log` at exactly 2 lines and line 77 exports a **non-empty** `FINALIZE_TEST_COMMAND`. Every new case must (a) be appended after line 90's assert, (b) use its own fixture directory, and (c) use its own `RUNTIME_LOG` / `EXECUTION_LOG` paths and reset `FINALIZE_TEST_COMMAND` to empty — otherwise it reddens a currently-passing guard or never reaches the auto-detect branch at all.
- **Skill size budget.** `tests/test_skill_size_budgets.sh:463` pins `skills/docket-finalize-change/SKILL.md` at `180 3450`. Measured on the branch base: **174 lines / 3395 words**. The edit adds 2 lines and ~8 words. Re-measure with `wc -lw` after the edit; do not assume. (Change 0190's plan records the caps as "193 / 4350" — that number is stale; measure against the tree.)
- **Guards are code.** Every new assert must be shown to go **red** against the state it exists to detect before it is believed. Run the mutation and record its output; never assume it.
- **The exact accumulator text** (this is the whole diff to the fenced block):

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

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `skills/docket-finalize-change/SKILL.md` | Publishes the `configured-bash-finalize` fragment — the single source both gates execute | Modify, lines 126–136 (the marker block only) |
| `tests/test_configured_bash_finalize.sh` | Hermetic executable guard for that fragment: extracts it and runs it against fixture repos | Modify, append after line 90 |

No other file changes. Four sites describe the block in load-bearing argumentation — `scripts/run-tests.md:192`, `scripts/run-tests.sh:70-77`, `tests/test_run_tests.sh:114-118`, `skills/docket-build/SKILL.md:185-192` — and all four stay true after the fix ("a bare `eval` of the configured test command"; "read any non-zero exit as the suite is red"; "reading that as RED would manufacture a repair task"). Do not edit them. Copies of the fragment under `docs/superpowers/plans/` are historical artifacts, not live sources; do not edit them either.

---

### Task 1: The keep-going accumulator and its mid-suite-red guard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:126-136`
- Test: `tests/test_configured_bash_finalize.sh` (append after line 90)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the shell variable name `suite_status` inside the published fragment, and the fixture-naming convention `acc_*` in the guard. Task 2 adds a sibling case using `empty_*` and must not disturb either.

- [ ] **Step 1: Validate the marker pair before touching anything**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/finalize-s-auto-detect-suite-loop-has-no-failure-accumulator
grep -nF -- '<!-- configured-bash-finalize:start -->' skills/docket-finalize-change/SKILL.md
grep -nF -- '<!-- configured-bash-finalize:end -->' skills/docket-finalize-change/SKILL.md
```

Expected: exactly one hit each, start line **126**, end line **136**, start < end. If the counts are not 1 and 1, or the order is wrong, **stop and report** — do not edit a dangling or duplicated marker range.

- [ ] **Step 2: Write the failing test**

Append to `tests/test_configured_bash_finalize.sh`, **after** the existing `assert "explicit command does not traverse the configured runtime"` block (line 90) and **before** the final `exit $fail`:

```bash
# --- keep-going accumulator: a NON-FINAL red must still report non-zero (change 0228) ---------
# Own fixture dir and own log paths on purpose: the asserts above pin the SHARED runtime_log at
# exactly 2 lines and line 77 exports a non-empty FINALIZE_TEST_COMMAND, so reusing either
# resource would redden a passing guard or skip the auto-detect branch entirely.
acc_fixture="$TMP/repo-accumulator"
mkdir -p "$acc_fixture/tests"
acc_runtime_log="$TMP/runtime-accumulator.log"
acc_execution_log="$TMP/execution-accumulator.log"

# test_bravo.sh is 2nd of 3 and exits 1; test_charlie.sh is last and passes. Without an
# accumulator the loop's status is the LAST test's, so the block reads green on a red suite.
for name in test_alpha.sh test_bravo.sh test_charlie.sh; do
  cat > "$acc_fixture/tests/$name" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$(basename "$0")" >> "$EXECUTION_LOG"
[ "$(basename "$0")" != "test_bravo.sh" ]
SH
  chmod +x "$acc_fixture/tests/$name"
done

acc_status=0
(
  cd "$acc_fixture" || exit 1
  RUNTIME_LOG="$acc_runtime_log" EXECUTION_LOG="$acc_execution_log" FINALIZE_TEST_COMMAND= \
    /bin/bash -c "$contract"
) || acc_status=$?

assert "auto-detect reports non-zero when a NON-FINAL test fails" \
  '[ "$acc_status" -ne 0 ]'
assert "auto-detect keeps going past the failure so every test still runs" \
  '[ "$(sort "$acc_execution_log")" = "test_alpha.sh
test_bravo.sh
test_charlie.sh" ]'
```

- [ ] **Step 3: Run the test to verify the new assert fails**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/finalize-s-auto-detect-suite-loop-has-no-failure-accumulator
bash tests/test_configured_bash_finalize.sh; echo "rc=$?"
```

Expected: `NOT OK - auto-detect reports non-zero when a NON-FINAL test fails`, `rc=1`. The keep-going assert should already say `ok` (the accumulator-free loop also runs every test) — that is correct; it guards the property the fix must not lose, not the fix itself. Every pre-existing assert must still say `ok`. If any pre-existing assert flipped to `NOT OK`, the fixture-isolation constraint was violated — fix that before continuing.

- [ ] **Step 4: Replace the marker block**

Replace `skills/docket-finalize-change/SKILL.md` lines 126–136 in a single whole-block edit. Old:

```
<!-- configured-bash-finalize:start -->
```bash
if [ -n "${FINALIZE_TEST_COMMAND:-}" ]; then
  eval "$FINALIZE_TEST_COMMAND"
else
  for test in tests/test_*.sh; do
    "$DOCKET_BASH_PATH" "$test"
  done
fi
```
<!-- configured-bash-finalize:end -->
```

New:

```
<!-- configured-bash-finalize:start -->
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
<!-- configured-bash-finalize:end -->
```

The `if`/`eval` branch is byte-identical. Only the `else` branch changes.

- [ ] **Step 5: Run the test to verify it passes**

Run:

```bash
bash tests/test_configured_bash_finalize.sh; echo "rc=$?"
```

Expected: every line `ok - …`, `rc=0`. Confirm both of these appear:

```
ok - auto-detect reports non-zero when a NON-FINAL test fails
ok - auto-detect keeps going past the failure so every test still runs
```

- [ ] **Step 6: Mutation-check the new assert**

Temporarily restore the accumulator-free loop — delete the `suite_status=0` line, the `|| suite_status=1`, and the `[ "$suite_status" -eq 0 ]` line — then run:

```bash
bash tests/test_configured_bash_finalize.sh; echo "rc=$?"
```

Expected: `NOT OK - auto-detect reports non-zero when a NON-FINAL test fails` and `rc=1`, with **only** that assert red (the keep-going assert stays `ok`). Paste the actual output into the task report; do not assert the mutation reddened without having run it. Then restore the accumulator and re-run to confirm `rc=0` again.

- [ ] **Step 7: Verify the neighbouring guards are still green**

Run:

```bash
bash tests/test_docket_review.sh; echo "review rc=$?"
bash tests/test_docket_build.sh; echo "build rc=$?"
```

Expected: `rc=0` from both. `test_docket_review.sh` re-checks the fragment's non-vacuity anchor and its purity (`! grep -qiE "evidence|skip|head_sha"`); `test_docket_build.sh` re-checks that docket-build names the block and opens no second marker block.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_configured_bash_finalize.sh
git commit -m "fix(0228): accumulate suite failures in finalize's auto-detect branch

A for loop's exit status is its last command's, so a red in any non-final
test file left the configured-bash-finalize block green and both gates that
execute it — finalize's merge gate and docket-build's build gate — proceeded
on a red suite. Add a keep-going suite_status accumulator and a trailing
[ \"\$suite_status\" -eq 0 ], and guard it with a fixture whose 2nd of 3 tests
fails. The FINALIZE_TEST_COMMAND branch is byte-identical."
```

---

### Task 2: Guard the empty-suite property and re-verify the size budget

**Files:**
- Modify: `tests/test_configured_bash_finalize.sh` (append after Task 1's block)
- Verify only: `skills/docket-finalize-change/SKILL.md`, `tests/test_skill_size_budgets.sh`

**Interfaces:**
- Consumes: Task 1's committed fragment (the `suite_status` accumulator) and its `acc_*` fixture block, which this task appends after.
- Produces: nothing later tasks depend on. This is the last task.

**Why this is a separate task:** Task 1's accumulator is only correct *given* that the empty-suite case still reports non-zero. That property is currently **unguarded** — nothing stops a future edit from adding `shopt -s nullglob` and turning a suiteless repo into a green gate that ran zero tests. It is a distinct property with a distinct fixture, and a reviewer could reasonably accept Task 1 and reject this.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_configured_bash_finalize.sh`, after Task 1's block and before the final `exit $fail`:

```bash
# --- empty suite: the literal glob must FAIL, never exit 0 having run zero tests (change 0228) -
# No nullglob and no [ -e "$test" ] guard, deliberately: with no matching files the glob stays
# literal, the invocation fails, and the block reports non-zero. nullglob would instead exit 0
# with zero tests run — a green gate certifying nothing.
assert "contract does not enable nullglob" \
  '! grep -qi -- "nullglob" <<<"$contract"'

empty_fixture="$TMP/repo-empty"
mkdir -p "$empty_fixture/tests"
empty_runtime_log="$TMP/runtime-empty.log"
empty_execution_log="$TMP/execution-empty.log"
: > "$empty_execution_log"

empty_status=0
(
  cd "$empty_fixture" || exit 1
  RUNTIME_LOG="$empty_runtime_log" EXECUTION_LOG="$empty_execution_log" FINALIZE_TEST_COMMAND= \
    /bin/bash -c "$contract"
) >/dev/null 2>&1 || empty_status=$?

assert "empty suite reports non-zero rather than certifying nothing" \
  '[ "$empty_status" -ne 0 ]'
assert "empty suite runs zero tests" \
  '[ ! -s "$empty_execution_log" ]'
```

- [ ] **Step 2: Run the test to verify the asserts pass against the fixed fragment**

Run:

```bash
bash tests/test_configured_bash_finalize.sh; echo "rc=$?"
```

Expected: all `ok - …`, `rc=0`. These three asserts are green against Task 1's fragment by construction — they exist to detect a *future* `nullglob` regression, so their proof is Step 3's mutation, not this run.

- [ ] **Step 3: Mutation-check all three new asserts**

Temporarily insert `shopt -s nullglob` as the first line of the `else` branch in the marker block, then run:

```bash
bash tests/test_configured_bash_finalize.sh; echo "rc=$?"
```

Expected: **all three** go red — `NOT OK - contract does not enable nullglob`, `NOT OK - empty suite reports non-zero rather than certifying nothing` (with nullglob the loop body never runs, `suite_status` stays 0, and the block exits 0), and `rc=1`. The "runs zero tests" assert stays `ok` under this mutation — it is the companion that proves the non-zero came from an empty suite and not from tests that ran; note that in the report rather than claiming three reds if you observe two. Paste the actual output. Remove `shopt -s nullglob` and re-run to confirm `rc=0`.

- [ ] **Step 4: Re-measure the skill size budget against the tree**

Run:

```bash
wc -lw skills/docket-finalize-change/SKILL.md
grep -n "docket-finalize-change/SKILL.md " tests/test_skill_size_budgets.sh
bash tests/test_skill_size_budgets.sh; echo "rc=$?"
```

Expected: line count **176** and word count **~3403**, both under the pinned `180 3450`, and `rc=0`. If either cap is exceeded, **stop and report** rather than raising the budget row — the budget is not this change's to move.

- [ ] **Step 5: Run the full suite**

Run:

```bash
scripts/run-tests.sh; echo "rc=$?"
```

Expected: `rc=0`. This is the branch's build gate evidence; capture the exact command and the resulting `git rev-parse HEAD`.

- [ ] **Step 6: Commit**

```bash
git add tests/test_configured_bash_finalize.sh
git commit -m "test(0228): guard the empty-suite non-zero property against nullglob

The accumulator is only correct given that a suiteless repo still reports
non-zero. Pin it three ways — the fragment contains no nullglob, an empty
fixture exits non-zero, and it runs zero tests — so a future nullglob cannot
turn the gate green while certifying nothing."
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Replace the auto-detect branch with a keep-going accumulator (`suite_status`, trailing `[ … -eq 0 ]`) | Task 1 Step 4 |
| `FINALIZE_TEST_COMMAND` branch byte-identical | Task 1 Step 4 (stated explicitly); Global Constraints |
| Regression case: non-final test fails ⇒ non-zero, and every test still ran | Task 1 Steps 2, 5 |
| Mutation-check it (restore the accumulator-free loop; it must redden) | Task 1 Step 6 |
| Keep the literal-glob failure; no `nullglob`; add a guard for it | Task 2 Steps 1, 3; Global Constraints |
| Fixture-isolation constraint (own dir, own logs, reset `FINALIZE_TEST_COMMAND`) | Global Constraints; Task 1 Step 2 comment; Task 1 Step 3 check |
| Whole-marker-block edit, markers validated first | Task 1 Steps 1, 4 |
| Existing asserts in `test_configured_bash_finalize.sh` / `test_docket_review.sh` / `test_docket_build.sh` stay green | Task 1 Steps 3, 5, 7 |
| Skill size budget re-verified, not assumed | Task 2 Step 4 |
| No prose edits at the four described sites | File Structure (explicit do-not-edit) |
| Full suite via `scripts/run-tests.sh` | Task 2 Step 5 |
| No ADR (`adrs: []` carried forward) | No task — the `exit` form that would have been ADR-shaped was rejected at grooming |

No gaps.

**2. Placeholder scan.** No TBD/TODO, no "add appropriate error handling", no "similar to Task N". Every code step carries the literal text to write; both the old and new marker-block bodies are quoted in full.

**3. Type consistency.** Shell identifiers are consistent across tasks: `suite_status` (fragment), `contract` / `TMP` / `assert` / `fail` (pre-existing harness), `acc_fixture` / `acc_runtime_log` / `acc_execution_log` / `acc_status` (Task 1), `empty_fixture` / `empty_runtime_log` / `empty_execution_log` / `empty_status` (Task 2). No collision with the existing `fixture`, `runtime_log`, `execution_log`, `argv_log`, `env_log`, `recorder`.

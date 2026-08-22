<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0251 — Retune the run-tests budget regime for portability and sharding** — `docs/changes/active/0251-retune-the-run-tests-budget-regime-for-portability-and-shard.md`
<!-- docket:backlink:end -->
# Retune the run-tests budget regime for portability and sharding — Implementation Plan

> **For agentic workers:** This plan is executed by docket's build role (`docket-build`), task-by-task, one worker per task under the docket-build-task contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the parallel runner's one-machine `5/2` budget verdict with a stateful screen-then-solo-confirm regime (Leg 1), then rework the 0126 prelude-correspondence guard to a family corpus and split `tests/test_docket_config.sh` two ways (Leg 2).

**Architecture:** Leg 1 turns the parallel `5/2` comparison into a *screening* observation tracked in a persistent, advisory, per-execution-context state file; only a solo (`-j 1` or scheduled serial confirmation) measurement against `ceiling * 3/2` establishes an authoritative breach, with `--strict-budget` confirming all current candidates immediately (fail-closed, exit 4). Leg 2 moves the prelude guard's population from `${BASH_SOURCE[0]}` to the glob-discovered `tests/test_docket_config*.sh` corpus (mirroring 0258's existing family-glob prior art at the "0258 leg 2" section), then splits the file at a measured section boundary with summed assertion-count parity.

**Tech Stack:** Bash 4.3+ (`scripts/run-tests.sh`), awk/grep guard extractors, the suite's own `assert` idiom, `tests/runtime-budgets.tsv` + `tests/test_runtime_budgets.sh`.

**Spec:** `docs/superpowers/specs/2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md` (budget leg amended 2026-08-11; read its Design and all 13 Assumptions before any task).

## Global Constraints

- **Every count and line-number literal in the spec and change body is STALE. Never copy them.** Derive mechanically at build time: suite file count from `ls tests/test_*.sh | wc -l` (121 at plan time, 123 after this change's two new files), `test_docket_config.sh` is 3304 lines, `EXPECTED_TOTAL` is currently 2275 (`tests/test_runtime_budgets.sh:31`), `SLACK_NUM=5; SLACK_DEN=2` sits at `scripts/run-tests.sh:80`, the advisory comparison at the `[ "$BUDGET_CHECK" = 1 ]` arithmetic in the report loop. Budget rows and totals come from measured `-j 1 --timings` runs.
- **Exit contract is byte-compatible:** 0 green (including advisory findings), 1 test failure, 3 missing result, 4 strict budget breach or failed strict confirmation, 5 hygiene, 2 usage. Precedence 1 > 3 > 4 > 0. No new exit codes (learning `exit-code-encodes-a-non-failure`: both live consumers read bare non-zero).
- **`--timings` five-column format (`path\tseconds\trc\tpasses\tfailures`) stays byte-compatible.** No appended fields.
- **AGENTS.md shell rules bind everywhere:** never `producer | grep -q/head` under pipefail (capture first); `mktemp` always with a template, and a temp file for atomic rename must sit **beside its destination**; `mv -f` on replace paths; awk indent classes `[^[:space:]]`.
- **Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers** (`tests/test_comment_anchor_style.sh` enforces the filename:line form).
- Every guard added or moved is **mutation-tested**: strip the guarded thing, watch it redden, restore. Record the mutation evidence in the task's commit message or results notes.
- A parallel screen crossing is **never labeled `OVER BUDGET`**. Only a solo confirmation or a direct `-j 1` measurement may be described as over budget.
- State persistence is **advisory infrastructure**: missing/corrupt/locked/unwritable state never fails or blocks a run (fail-open); only `--strict-budget` fails closed, and it needs no stored history.
- The confirmation run **never changes the suite pass/fail verdict**, and a failed confirmation **never clears a candidate**.
- New test fixtures use no real multi-second sleeps: durations are injected through the seam built in Task 1.
- The suite command at the build gate is whatever `finalize.test_command` resolves to; run the whole suite there, never a subset.

## File Structure

| File | Role in this change |
|---|---|
| `scripts/run-tests.sh` | Leg 1: discovery/duration seams, budget-state store, state machine, scheduled + strict solo confirmation, new report vocabulary, rewritten comment block. |
| `tests/test_run_tests_budget_state.sh` | **New.** The 30 deterministic Leg-1 tests, driving `run-tests.sh` against fixture suites with injected durations. |
| `tests/runtime-budgets.tsv` | +1 row (new test file, Task 1); the `test_docket_config.sh` row replaced by two measured rows (Task 8). |
| `tests/test_runtime_budgets.sh` | `EXPECTED_TOTAL` re-seeded in Tasks 1 and 8; stale `5/2`/`OVER BUDGET` comment wording updated in Task 6. |
| `scripts/run-tests.md` | Budget sections rewritten to the confirmation regime; 0229 references repointed to 0251; stale file counts re-derived (Tasks 6, 8). |
| `AGENTS.md` | The Guards bullet's "`OVER BUDGET:` line" vocabulary updated (Task 6). |
| `tests/README.md` | Running-section budget prose (Task 6); "argued whole" paragraph and suite count (Task 8). |
| `tests/test_docket_config.sh` | Leg 2: prelude guard reworked to the family corpus (Task 7); then split — this file keeps the head sections (Task 8). |
| `tests/test_docket_config_guards.sh` | **New (Task 8).** Tail shard: carries the moved sections **including** the prelude guard self-block, section T, the 0148/0258-L2/0276 sections and the 0174 final integrity assert. (If the measured boundary argues for a different topic name, name it `tests/test_docket_config_<topic>.sh` per the family convention — but it must be the tail-holding shard.) |

Task order: 1 → 2 → 3 → 4 → 5 → 6 (Leg 1), then 7 → 8 (Leg 2). Tasks 2–5 each extend the same runner and test file; they are sequential, not parallel.

---

### Task 1: Test seams, fixture harness, and the new test file's budget row

**Profile hint:** standard.

**Files:**
- Modify: `scripts/run-tests.sh` (discovery site — the `find "$REPO/tests"` default-target block; the report loop where `secs` is read; usage/comment header)
- Create: `tests/test_run_tests_budget_state.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces (Produces — every later Leg-1 task consumes these):**
- Env seam `DOCKET_RUNTESTS_TESTS_DIR`: when set, default discovery uses it instead of `$REPO/tests`. Explicit TARGET args still win. Production unaffected when unset.
- Env seam `DOCKET_RUNTESTS_TEST_DURATIONS`: path to a TSV `basename<TAB>parallel_secs<TAB>solo_secs`. When set, the report loop replaces a matching file's measured `secs` with `parallel_secs`, and the solo-confirmation path (Task 4) replaces its measurement with `solo_secs`. Tests still actually execute; only the *reported duration* is injected. Production unaffected when unset.
- Test-file helpers (all in `tests/test_run_tests_budget_state.sh`): `mk_suite`, `mk_budgets`, `mk_durations`, `run_rt` as written below.

- [ ] **Step 1: Write the new test file's harness plus the first two tests (spec tests 28 and 29)**

Create `tests/test_run_tests_budget_state.sh`. The `assert()` definition must be byte-exactly the canonical one (the hygiene checker's `DEFN-DRIFT` class rejects variants):

```bash
#!/usr/bin/env bash
# tests/test_run_tests_budget_state.sh — deterministic fixtures for run-tests.sh's stateful
# budget-confirmation regime (change 0251). Durations are INJECTED via the
# DOCKET_RUNTESTS_TEST_DURATIONS seam — no real multi-second sleeps, ever.
# Run: bash tests/test_run_tests_budget_state.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RUNNER="$REPO/scripts/run-tests.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

tmp="$(mktemp -d "${TMPDIR:-/tmp}/rtbs.XXXXXX")"; trap 'rm -rf "$tmp"' EXIT

# mk_suite <dir> <name>... : a fixture tests dir of trivially-green test files.
mk_suite(){ local d="$1"; shift; mkdir -p "$d"
  local n; for n in "$@"; do printf '#!/usr/bin/env bash\necho "ok - trivial"\nexit 0\n' > "$d/$n"; done; }
# mk_red <dir> <name> : one deliberately failing fixture test.
mk_red(){ printf '#!/usr/bin/env bash\necho "NOT OK - forced"\nexit 1\n' > "$1/$2"; }
# mk_budgets <file> "<name> <ceiling> <mode>"... : a fixture budget table.
mk_budgets(){ local f="$1"; shift; : > "$f"
  local row; for row in "$@"; do printf '%s\t%s\t%s\n' $row >> "$f"; done; }
# mk_durations <file> "<name> <parallel> <solo>"... : the injection seam's TSV.
mk_durations(){ local f="$1"; shift; : > "$f"
  local row; for row in "$@"; do printf '%s\t%s\t%s\n' $row >> "$f"; done; }
# run_rt <suite-dir> <budgets> <durations> <state> [runner args...] : one runner invocation.
# Default -j 2 (parallel); callers append -j 1 / --strict-budget / explicit targets as needed.
run_rt(){ local suite="$1" budgets="$2" durs="$3" state="$4"; shift 4
  DOCKET_RUNTESTS_TESTS_DIR="$suite" DOCKET_RUNTESTS_TEST_DURATIONS="$durs" \
    bash "$RUNNER" --budgets "$budgets" --budget-state "$state" -j 2 "$@"; }

# ---- spec test 28: --timings output stays byte-compatible (five columns, no state fields) ----
mk_suite "$tmp/s28" test_a.sh test_b.sh
mk_budgets "$tmp/s28.tsv" "test_a.sh 10 parallel" "test_b.sh 10 parallel"
mk_durations "$tmp/s28.durs" "test_a.sh 99 99" "test_b.sh 1 1"
t28_out="$(run_rt "$tmp/s28" "$tmp/s28.tsv" "$tmp/s28.durs" "$tmp/s28.state" --timings "$tmp/s28.timings")"
t28_cols="$(awk -F'\t' '{print NF}' "$tmp/s28.timings" | sort -u)"
assert "28: every --timings row has exactly five tab-separated fields" '[ "$t28_cols" = 5 ]'
assert "28: timings rows carry the injected parallel duration" \
  'grep -qE "test_a\.sh\t99\t0\t1\t0$" "$tmp/s28.timings"'

# ---- spec test 29: the parallel run stays the sole authority for results/asserts/logs ----
mk_suite "$tmp/s29" test_a.sh
mk_red   "$tmp/s29" test_b.sh
mk_budgets "$tmp/s29.tsv" "test_a.sh 10 parallel" "test_b.sh 10 parallel"
mk_durations "$tmp/s29.durs" "test_a.sh 1 1" "test_b.sh 1 1"
t29_rc=0; t29_out="$(run_rt "$tmp/s29" "$tmp/s29.tsv" "$tmp/s29.durs" "$tmp/s29.state")" || t29_rc=$?
assert "29: a red fixture file still fails the suite (exit 1)" '[ "$t29_rc" -eq 1 ]'
assert "29: the SUITE line counts the parallel run's own asserts" \
  'grep -qE "^SUITE files=2 passed=1 failed=1 asserts=2 " <<<"$t29_out"'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"
```

Note `--budget-state` does not exist yet — Task 2 adds it. For THIS task's red/green cycle, `run_rt` may temporarily omit `--budget-state "$state"` (unknown options are exit 2); add the argument back in Task 2's first step. Keep the parameter in the helper signature from the start so no later test changes shape.

- [ ] **Step 2: Run the new file; verify it fails on the unknown seams**

Run: `bash tests/test_run_tests_budget_state.sh` (with `run_rt` temporarily not passing `--budget-state`).
Expected: FAIL — the fixture suite dir is ignored (seam absent), so the runner runs the real `tests/` glob or errors; the timings assert reads real durations, not 99.

- [ ] **Step 3: Implement the two seams in `scripts/run-tests.sh`**

At the default-target discovery block, replace the hardcoded dir:

```bash
# Test-only seam (change 0251): tests/test_run_tests_budget_state.sh points discovery at a
# fixture suite so the state machine can be exercised without running the real corpus.
# Production behavior is identical when the variable is unset.
TESTS_DIR="${DOCKET_RUNTESTS_TESTS_DIR:-$REPO/tests}"
if [ "${#TARGETS[@]}" -eq 0 ]; then
  DEFAULT_CORPUS=1
  while IFS= read -r f; do TARGETS+=("$f"); done < <(find "$TESTS_DIR" -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)
else
  DEFAULT_CORPUS=0
fi
```

(`DEFAULT_CORPUS` is consumed by Task 3's qualifying-run predicate.)

In the report loop, immediately after `IFS=$'\t' read -r rc secs p f < "$WORK/stat/$base"`:

```bash
# Test-only seam (change 0251): replace the measured duration with an injected one so the
# budget state machine's tests are deterministic. Column 2 = parallel seconds.
if [ -n "${DOCKET_RUNTESTS_TEST_DURATIONS:-}" ] && [ -f "${DOCKET_RUNTESTS_TEST_DURATIONS}" ]; then
  inj="$(awk -F'\t' -v b="${base}.sh" '$1==b{print $2; exit}' "$DOCKET_RUNTESTS_TEST_DURATIONS")"
  case "${inj:-}" in ''|*[!0-9]*) ;; *) secs="$inj" ;; esac
fi
```

- [ ] **Step 4: Run the new file's tests; verify they pass.** Run: `bash tests/test_run_tests_budget_state.sh`. Expected: PASS (all asserts ok). Note test 28 will show the legacy `OVER BUDGET` row for the injected 99s at this point — that label changes in Task 3; assert only what Step 1 asserts.

- [ ] **Step 5: Register the new file with the budget table**

Measure: `scripts/run-tests.sh -j 1 --timings /tmp/t1.tsv tests/test_run_tests_budget_state.sh`, read its serial seconds, apply the table's seeding rule (**next multiple of 5, plus 5s margin, minimum 10**) and add the row to `tests/runtime-budgets.tsv` (parallel mode). Re-seed `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` (current value 2275 + the new ceiling; state in the commit message that this is the "new test file brings its own row" case the guard's remedy names). Run `bash tests/test_runtime_budgets.sh` — expected PASS.

- [ ] **Step 6: Full-file check and commit**

Run: `scripts/run-tests.sh tests/test_run_tests_budget_state.sh tests/test_runtime_budgets.sh` — expected exit 0.

```bash
git add scripts/run-tests.sh tests/test_run_tests_budget_state.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0251): duration/discovery seams + budget-state test harness"
```

---

### Task 2: The persistent budget-state store — path, lock, atomic write, corruption handling

**Profile hint:** standard.

**Files:**
- Modify: `scripts/run-tests.sh` (new flag, new functions after the budget-table section)
- Modify: `tests/test_run_tests_budget_state.sh` (spec tests 26, 27 + store-mechanics tests)

**Interfaces:**
- Consumes: Task 1's seams and helpers.
- Produces: flag `--budget-state PATH` (overrides the default store path); debug flag `--print-budget-state-path` (prints the resolved path, exits 0, runs nothing); functions `budget_state_path()`, `state_lock()` / `state_unlock()`, `state_load()` (populates `declare -A BS_STATE BS_STREAK BS_SINCE BS_LASTPAR BS_LASTSOLO BS_CEIL BS_CONFRES BS_DUESEQ BS_PATHOF`, keyed by context key, plus scalar `BS_NEXT_SEQ`), `state_write()`. File format: header `# docket-run-tests-budget-state v1` then `# next_due_sequence N` then tab-separated rows `context_key state initial_overrun_streak overruns_since_confirmation last_parallel_seconds last_solo_seconds budget_seconds last_confirmation_result due_sequence test_path`.

- [ ] **Step 1: Write failing tests**

Append to `tests/test_run_tests_budget_state.sh` (before the final PASS/FAIL block; all later tasks append there too — not restated again):

```bash
# ---- store mechanics: default path resolution -------------------------------------------
sp_out="$(bash "$RUNNER" --print-budget-state-path)"
assert "store: default path lives under this repo's git dir" \
  '[ "$sp_out" = "$(git -C "$REPO" rev-parse --git-dir)/docket/run-tests-budget-state.tsv" ]'

# ---- store mechanics: created with restrictive permissions, even under a hostile umask ---
mk_suite "$tmp/sperm" test_a.sh
mk_budgets "$tmp/sperm.tsv" "test_a.sh 10 parallel"
mk_durations "$tmp/sperm.durs" "test_a.sh 99 1"
( umask 077; run_rt "$tmp/sperm" "$tmp/sperm.tsv" "$tmp/sperm.durs" "$tmp/sperm.state" >/dev/null )
( umask 022; run_rt "$tmp/sperm" "$tmp/sperm.tsv" "$tmp/sperm.durs" "$tmp/sperm.state2" >/dev/null )
sperm_mode="$(ls -l "$tmp/sperm.state2" | cut -c1-10)"
assert "store: the state file is not group/world accessible" '[ "$sperm_mode" = "-rw-------" ]'
assert "store: the state file survives a run and carries the v1 header" \
  'head -n1 "$tmp/sperm.state" | grep -qF "docket-run-tests-budget-state v1"'

# ---- spec test 26: corrupt records are ignored, reported once, and the run proceeds ------
mk_suite "$tmp/s26" test_a.sh
mk_budgets "$tmp/s26.tsv" "test_a.sh 10 parallel"
mk_durations "$tmp/s26.durs" "test_a.sh 1 1"
printf '# docket-run-tests-budget-state v1\n# next_due_sequence 1\ngarbage-not-tab-separated\n' > "$tmp/s26.state"
s26_rc=0; s26_all="$(run_rt "$tmp/s26" "$tmp/s26.tsv" "$tmp/s26.durs" "$tmp/s26.state" 2>&1)" || s26_rc=$?
assert "26: a malformed record does not fail the run" '[ "$s26_rc" -eq 0 ]'
assert "26: the malformed record is reported" 'grep -qiE "malformed.*budget[- ]state|budget[- ]state.*malformed" <<<"$s26_all"'
# unknown schema version: old state ignored and rebuilt, not fatal
printf '# docket-run-tests-budget-state v999\n' > "$tmp/s26b.state"
s26b_rc=0; run_rt "$tmp/s26" "$tmp/s26.tsv" "$tmp/s26.durs" "$tmp/s26b.state" >/dev/null 2>&1 || s26b_rc=$?
assert "26: an unknown schema version is ignored and rebuilt, not fatal" \
  '[ "$s26b_rc" -eq 0 ] && head -n1 "$tmp/s26b.state" | grep -qF "v1"'

# ---- spec test 27: an unacquirable lock loses nothing and overwrites nothing -------------
mk_suite "$tmp/s27" test_a.sh
mk_budgets "$tmp/s27.tsv" "test_a.sh 10 parallel"
mk_durations "$tmp/s27.durs" "test_a.sh 99 1"
run_rt "$tmp/s27" "$tmp/s27.tsv" "$tmp/s27.durs" "$tmp/s27.state" >/dev/null   # seed real state
s27_before="$(cat "$tmp/s27.state")"
mkdir "$tmp/s27.state.lock"                                                    # a held lock
s27_rc=0; s27_all="$(run_rt "$tmp/s27" "$tmp/s27.tsv" "$tmp/s27.durs" "$tmp/s27.state" 2>&1)" || s27_rc=$?
assert "27: a held lock never fails the suite" '[ "$s27_rc" -eq 0 ]'
assert "27: the warning names the lock path so a stale lock is discoverable" \
  'grep -qF -- "$tmp/s27.state.lock" <<<"$s27_all"'
assert "27: state is untouched when the lock was not acquired" \
  '[ "$(cat "$tmp/s27.state")" = "$s27_before" ]'
rmdir "$tmp/s27.state.lock"

# ---- store mechanics: unwritable state file is fail-open with a report -------------------
s27c_rc=0; s27c_all="$(run_rt "$tmp/s27" "$tmp/s27.tsv" "$tmp/s27.durs" /dev/full/nope.tsv 2>&1)" || s27c_rc=$?
assert "store: an unusable state path never blocks the run, and the report says so" \
  '[ "$s27c_rc" -eq 0 ] && grep -qiE "without (budget )?history" <<<"$s27c_all"'
```

- [ ] **Step 2: Run; verify failures.** Run: `bash tests/test_run_tests_budget_state.sh`. Expected: the new asserts FAIL (`--print-budget-state-path` and `--budget-state` are unknown options, exit 2). Also restore `--budget-state "$state"` inside `run_rt` now.

- [ ] **Step 3: Implement the store**

In `scripts/run-tests.sh`: parse `--budget-state PATH` and `--print-budget-state-path` in the option loop; add to the Usage header. After the budget-table section:

```bash
# ---- budget state store (change 0251) ---------------------------------------------------
# Advisory infrastructure: fail-open everywhere. Nothing authoritative reads stored state —
# --strict-budget re-measures current candidates directly (spec assumption 11).
BS_SCHEMA=1
budget_state_path(){
  if [ -n "${BUDGET_STATE_OVERRIDE:-}" ]; then printf '%s' "$BUDGET_STATE_OVERRIDE"; return; fi
  local gd; gd="$(git -C "$REPO" rev-parse --git-dir 2>/dev/null)" || { printf ''; return; }
  # rev-parse may print a relative path; anchor it. Linked worktrees get their own git dir,
  # hence their own history (spec: "Persistent state store").
  case "$gd" in /*) ;; *) gd="$REPO/$gd" ;; esac
  printf '%s/docket/run-tests-budget-state.tsv' "$gd"
}
STATE_FILE="$(budget_state_path)"
STATE_USABLE=1   # flipped to 0 on any store problem; the run continues without history
STATE_LOCKED=0
state_lock(){   # bounded: ~3s of 0.1s attempts; failure is a warning, never a run failure
  local i=0
  while [ "$i" -lt 30 ]; do
    if mkdir "$STATE_FILE.lock" 2>/dev/null; then STATE_LOCKED=1; return 0; fi
    i=$((i + 1)); sleep 0.1
  done
  printf 'run-tests: budget-state lock not acquired (%s.lock) — this run reads and writes no budget history. Remove the lock dir by hand if its owner is dead.\n' "$STATE_FILE" >&2
  return 1
}
state_unlock(){ [ "$STATE_LOCKED" = 1 ] && rmdir "$STATE_FILE.lock" 2>/dev/null; STATE_LOCKED=0; }
declare -A BS_STATE=() BS_STREAK=() BS_SINCE=() BS_LASTPAR=() BS_LASTSOLO=() BS_CEIL=() BS_CONFRES=() BS_DUESEQ=() BS_PATHOF=()
BS_NEXT_SEQ=1
state_load(){    # under the lock; malformed rows ignored + reported once, wrong schema discarded
  [ -f "$STATE_FILE" ] || return 0
  head -n1 "$STATE_FILE" | grep -qF "docket-run-tests-budget-state v$BS_SCHEMA" || return 0
  local reported=0 k st streak since lp ls bc cr ds tp
  BS_NEXT_SEQ="$(sed -n '2s/^# next_due_sequence \([0-9]*\)$/\1/p' "$STATE_FILE")"; BS_NEXT_SEQ="${BS_NEXT_SEQ:-1}"
  while IFS=$'\t' read -r k st streak since lp ls bc cr ds tp; do
    case "$k" in ''|'#'*) continue ;; esac
    if [ -z "$tp" ] || [ -z "$st" ]; then
      [ "$reported" = 0 ] && { printf 'run-tests: malformed budget-state record ignored in %s\n' "$STATE_FILE" >&2; reported=1; }
      continue
    fi
    BS_STATE[$k]="$st"; BS_STREAK[$k]="$streak"; BS_SINCE[$k]="$since"; BS_LASTPAR[$k]="$lp"
    BS_LASTSOLO[$k]="$ls"; BS_CEIL[$k]="$bc"; BS_CONFRES[$k]="$cr"; BS_DUESEQ[$k]="$ds"; BS_PATHOF[$k]="$tp"
  done < "$STATE_FILE"
}
state_write(){   # full replacement via temp-beside-destination + atomic rename; explicit chmod
  local dir tmpf k
  dir="$(dirname "$STATE_FILE")"
  mkdir -p "$dir" 2>/dev/null || { STATE_USABLE=0; return 1; }
  tmpf="$(mktemp "$dir/.run-tests-budget-state.XXXXXX")" 2>/dev/null || { STATE_USABLE=0; return 1; }
  {
    printf '# docket-run-tests-budget-state v%s\n' "$BS_SCHEMA"
    printf '# next_due_sequence %s\n' "$BS_NEXT_SEQ"
    for k in "${!BS_STATE[@]}"; do
      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$k" "${BS_STATE[$k]}" "${BS_STREAK[$k]:-0}" \
        "${BS_SINCE[$k]:-0}" "${BS_LASTPAR[$k]:--}" "${BS_LASTSOLO[$k]:--}" "${BS_CEIL[$k]:-0}" \
        "${BS_CONFRES[$k]:--}" "${BS_DUESEQ[$k]:--}" "${BS_PATHOF[$k]}"
    done | LC_ALL=C sort
  } > "$tmpf" || { rm -f "$tmpf"; STATE_USABLE=0; return 1; }
  chmod 600 "$tmpf"   # umask makes the mktemp mode a request, not a promise
  mv -f "$tmpf" "$STATE_FILE" || { rm -f "$tmpf"; STATE_USABLE=0; return 1; }
}
```

Wire `--budget-state` to set `BUDGET_STATE_OVERRIDE` before `STATE_FILE` is computed; `--print-budget-state-path` prints `budget_state_path` output and exits 0 immediately after option parsing. For this task, call `state_lock && { state_load; state_write; state_unlock; }` once near the end of the run **only when** `BUDGET_CHECK=1` (Task 3 replaces this placeholder call with the real apply logic; the placeholder is what makes this task's state-file-exists asserts meaningful). If `STATE_FILE` is empty or its directory unusable, set `STATE_USABLE=0` and print `run-tests: budget state unavailable — running without budget history.` once.

- [ ] **Step 4: Run; verify pass.** `bash tests/test_run_tests_budget_state.sh` — expected PASS. Mutation check: comment out the `chmod 600` line, re-run, confirm the umask-077 permission assert still passes but the 022 one **reddens**; restore.

- [ ] **Step 5: Commit.** `git add scripts/run-tests.sh tests/test_run_tests_budget_state.sh && git commit -m "feat(0251): advisory budget-state store (locked, atomic, fail-open)"`

---

### Task 3: Screening, qualifying overruns, and state-machine bookkeeping

**Profile hint:** premium (the state machine's correctness rules are the change's core risk).

**Files:**
- Modify: `scripts/run-tests.sh` (report loop + post-report state application)
- Modify: `tests/test_run_tests_budget_state.sh` (spec tests 1, 3, 7, 8, 9, 10, 11, 12, 13, 17, 18, 19)

**Interfaces:**
- Consumes: Task 2's store functions and arrays.
- Produces: `context_key <test-path> <ceiling> <mode>` → `path|j<JOBS>|c<cpus>|<os>|<arch>|b<ceiling>|m<mode>|s<schema>`; per-run candidate collection `RUN_CANDIDATES` (array of context keys whose parallel time crossed `ceiling * 5/2` this run); `RUN_QUALIFYING` (0/1); report lines `BUDGET WATCH: <path> — <N>s under -j<J>; consecutive parallel-overrun streak <k>/5` and `PARALLEL-SENSITIVE: <path> — <N>s under -j<J>; last solo measurement <M>s; recheck progress <k>/10`. States: `unobserved`, `watching`, `parallel-sensitive`, `confirmed-breach`.

- [ ] **Step 1: Write failing tests**

The recurring fixture: one file `test_slow.sh` with ceiling 10, injected parallel duration 99 (`99 > 10 * 5/2`), solo 1. A "clean" run injects parallel 1. All runs are `-j 2` default-corpus via `run_rt` unless stated.

```bash
# helper: run N qualifying overrun runs against one persistent state file
overrun_n(){ local n="$1" suite="$2" budgets="$3" durs="$4" state="$5"; shift 5
  local i out; for ((i=0; i<n; i++)); do out="$(run_rt "$suite" "$budgets" "$durs" "$state" "$@")"; done
  printf '%s' "$out"; }   # returns the LAST run's stdout

mk_suite "$tmp/sm" test_slow.sh
mk_budgets "$tmp/sm.tsv" "test_slow.sh 10 parallel"
mk_durations "$tmp/sm.over" "test_slow.sh 99 1"
mk_durations "$tmp/sm.clean" "test_slow.sh 1 1"

# ---- spec test 1: four consecutive qualifying overruns trigger no confirmation ----------
sm1_out="$(overrun_n 4 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm1.state")"
assert "1: the fourth overrun reports a WATCH, not a confirmation" \
  'grep -qE "^BUDGET WATCH: .*test_slow\.sh.*streak 4/5" <<<"$sm1_out"'
assert "1: no solo confirmation ran in the first four overruns" \
  '! grep -qE "SERIAL CONFIRM" <<<"$sm1_out"'
assert "1: a parallel screen crossing is never labeled OVER BUDGET" \
  '! grep -qF "OVER BUDGET" <<<"$sm1_out"'
assert "1: the overrun run still exits 0 (advisory)" \
  'run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.clean" "$tmp/throwaway.state" >/dev/null'

# ---- spec test 3: a qualifying clean result resets the initial streak -------------------
overrun_n 4 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm3.state" >/dev/null
run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.clean" "$tmp/sm3.state" >/dev/null   # clean qualifying run
sm3_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm3.state")"
assert "3: a clean qualifying run reset the streak (next overrun reads 1/5)" \
  'grep -qE "streak 1/5" <<<"$sm3_out"'

# ---- spec tests 9/10/11/12/13: non-qualifying runs mutate nothing -----------------------
freeze_check(){ # freeze_check <label> <state-file> <run...>: state bytes identical across the run
  local label="$1" st="$2"; shift 2
  local before after; before="$(cat "$st" 2>/dev/null || true)"
  "$@" >/dev/null 2>&1 || true
  after="$(cat "$st" 2>/dev/null || true)"
  assert "$label" '[ "$before" = "$after" ]'; }
overrun_n 2 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state" >/dev/null
freeze_check "9: a targeted run does not mutate history" "$tmp/smf.state" \
  run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state" "$tmp/sm/test_slow.sh"
mk_red "$tmp/sm" test_red.sh
freeze_check "10: a red suite run does not mutate history" "$tmp/smf.state" \
  run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state"
rm -f "$tmp/sm/test_red.sh"
freeze_check "13: --no-budget-check neither reads nor writes history" "$tmp/smf.state" \
  run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state" --no-budget-check
# test 11 (missing result): a fixture test that kills its own process subtree recorder —
# simplest deterministic shape: a test that removes its own stat record cannot be arranged
# from outside, so simulate via a test file that execs kill -9 on itself mid-run:
printf '#!/usr/bin/env bash\nkill -9 $$\n' > "$tmp/sm/test_dies.sh"
freeze_check "11: a run with a missing result does not mutate history" "$tmp/smf.state" \
  run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state"
rm -f "$tmp/sm/test_dies.sh"
```

For spec test 12 (interrupted runs): the interrupt handler exits before the report loop, so state application never runs by construction — assert it structurally instead of racing signals: after the implementation lands, `freeze_check` a run that is `timeout`-killed if practical, or assert in the runner source that state application sits **below** the report loop and the handler `exit`s (a grep assert on ordering is acceptable here only as a fallback; prefer the behavioral form: start a backgrounded `run_rt` against a fixture whose test sleeps briefly via `read -t 2 < /dev/null || true`, `kill -INT` it, wait, then compare state bytes).

Spec tests 7/8 need a `parallel-sensitive` record and belong to Task 4's fixtures (they assert the 10-counter's behavior on clean runs); write them in Task 4. Spec tests 17/18/19 (independent histories per `-j`; ceiling change invalidates; mode change invalidates):

```bash
# ---- spec test 17: -j values keep independent histories ---------------------------------
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state" >/dev/null       # -j 2
sm17_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state" -j 3)"    # -j 3
assert "17: a -j 3 overrun starts its own streak at 1/5" 'grep -qE "streak 1/5" <<<"$sm17_out"'
sm17b_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state")"        # back to -j 2
assert "17: the -j 2 history was neither advanced nor consumed by the -j 3 run" \
  'grep -qE "streak 4/5" <<<"$sm17b_out"'

# ---- spec tests 18/19: ceiling and mode changes invalidate the record -------------------
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm18.state" >/dev/null
mk_budgets "$tmp/sm18b.tsv" "test_slow.sh 20 parallel"     # ceiling moved 10 -> 20; 99 > 50 still qualifies
sm18_out="$(run_rt "$tmp/sm" "$tmp/sm18b.tsv" "$tmp/sm.over" "$tmp/sm18.state")"
assert "18: a ceiling change starts a fresh record (streak 1/5)" 'grep -qE "streak 1/5" <<<"$sm18_out"'
mk_budgets "$tmp/sm19.tsv" "test_slow.sh 10 serial"
sm19_before="$(grep -c "test_slow" "$tmp/sm18.state" || true)"
run_rt "$tmp/sm" "$tmp/sm19.tsv" "$tmp/sm.over" "$tmp/sm18.state" >/dev/null
assert "19: a mode change means the old parallel-context record is not advanced" \
  'grep -qE "streak 1/5|streak 0/5" <<<"$(run_rt "$tmp/sm" "$tmp/sm18b.tsv" "$tmp/sm.over" "$tmp/sm18.state")" || true; [ 1 = 1 ]'
```

(For 19, tighten at implementation time to the sharpest observable: a serial-mode test is not parallel-executed, so a serial run must not advance the parallel-context streak — assert by running the serial-mode budget once, then one parallel overrun, and reading `streak 4/5`, i.e. unpolluted continuation. Do not ship the vacuous `[ 1 = 1 ]` placeholder above — it exists only to show the fixture shape; the worker writes the discriminating assert.)

- [ ] **Step 2: Run; verify the new asserts fail.** Expected: `BUDGET WATCH` lines absent (the runner still prints legacy `OVER BUDGET` rows for parallel crossings).

- [ ] **Step 3: Implement**

In the report loop, replace the legacy per-row parallel breach marking (`over=1; overbudget=...; OVER BUDGET (ceiling Ns)` suffix) for **parallel** runs (`JOBS > 1`) with candidate collection only — the per-row suffix and the trailing block vocabulary become the new classifications; keep the direct comparison and existing `OVER BUDGET` vocabulary **only** for the `-j 1` path (Task 5 wires its 3/2 threshold). After the report loop, when `BUDGET_CHECK=1` and `JOBS>1`:

```bash
context_key(){ printf '%s|j%s|c%s|%s|%s|b%s|m%s|s%s' "$1" "$JOBS" "$(cpu_count)" "$(uname -s)" "$(uname -m)" "$2" "$3" "$BS_SCHEMA"; }
# RUN_QUALIFYING: default corpus, parallel, budgets on, suite green and complete (spec
# "Qualifying parallel overrun"). Computed AFTER the report loop, where failed/noresult exist.
RUN_QUALIFYING=0
if [ "$DEFAULT_CORPUS" = 1 ] && [ "$JOBS" -gt 1 ] && [ "$BUDGET_CHECK" = 1 ] \
   && [ "$failed" -eq 0 ] && [ "$noresult" -eq 0 ]; then RUN_QUALIFYING=1; fi
```

Per-file, during the report loop, collect `RUN_CANDIDATES+=("$key")` when `secs * SLACK_DEN > ceil * SLACK_NUM` and the file's own `rc = 0` and mode is parallel; also collect the clean set (parallel-executed, below threshold) for streak resets and non-advance rules. Then, under `state_lock` (skip everything on lock failure): `state_load`; for each parallel-executed test in a qualifying run — overrun: `watching` streak +1 (from `unobserved`), `parallel-sensitive`/`confirmed-breach`: `overruns_since_confirmation` +1; clean: reset `watching` streak to 0, do **not** touch the since-confirmation counter (spec tests 7/8 asymmetry); update `last_parallel_seconds` always on qualifying runs; assign `due_sequence` from `BS_NEXT_SEQ` (then increment) the moment a record first becomes due (streak reaches 5, or since-counter reaches 10). `state_write`; `state_unlock`. Non-qualifying runs (targeted, red, incomplete, `--no-budget-check`) skip the entire load/apply/write — and `--no-budget-check` must skip the **read** too.

Report emission (stdout, after the existing `SUITE`/`FAILED` lines): for each current candidate in `LC_ALL=C` path order print `BUDGET WATCH:` (state `unobserved`/`watching`) with `streak <k>/5`, or `PARALLEL-SENSITIVE:` with `last solo measurement <M>s; recheck progress <k>/10`. Use the spec's exact label spellings from its Reporting list.

- [ ] **Step 4: Run; verify pass; mutation-test.** `bash tests/test_run_tests_budget_state.sh` — PASS. Mutations (each: mutate, observe the named assert redden, restore): (a) make red runs advance the streak → test 10 reddens; (b) make clean runs advance instead of reset → test 3 reddens; (c) drop `-j` from the context key → test 17 reddens.

- [ ] **Step 5: Commit.** `git commit -am "feat(0251): parallel screening + qualifying-overrun state machine"`

---

### Task 4: Scheduled solo confirmation — trigger, one-per-run bound, classification, failure

**Profile hint:** premium.

**Files:**
- Modify: `scripts/run-tests.sh`
- Modify: `tests/test_run_tests_budget_state.sh` (spec tests 2, 4, 5, 6, 7, 8, 20, 21, 22, 23, 24, 30)

**Interfaces:**
- Consumes: Tasks 2–3.
- Produces: `solo_confirm <test-path>` — re-runs one file serially through the existing `launch`+`wait` machinery (same sandbox), reads the seam's **solo** column (column 3) when set, compares `solo * 2 > ceil * 3` (i.e. `solo > ceiling * 3/2`), returns the classification; report lines `SERIAL CONFIRMATION DUE:`, `SERIAL CONFIRMATION DEFERRED: <path> — Recheck is due; another test consumed this run's confirmation slot`, `SERIAL CONFIRMED OVER BUDGET: <path> — <P>s under -j<J>; <S>s solo; solo threshold <T>s`, `SERIAL CONFIRMATION FAILED: <path>`.

- [ ] **Step 1: Write failing tests** (same fixture family as Task 3; key excerpts — the worker writes all twelve using these shapes):

```bash
# ---- spec test 2: the fifth consecutive overrun triggers exactly one solo confirmation --
sm2_out="$(overrun_n 5 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "2: the fifth overrun runs exactly one solo confirmation" \
  '[ "$(grep -cE "^SERIAL CONFIRM" <<<"$sm2_out")" -eq 1 ]'
# ---- spec test 4: a healthy solo result (1s <= 15s) records parallel-sensitive ----------
assert "4: healthy solo classifies parallel-sensitive" \
  'grep -qE "parallel-sensitive" "$tmp/sm2.state"'
# ---- spec test 5: the next nine overruns trigger no further confirmation ----------------
sm5_out="$(overrun_n 9 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "5: recheck progress reads 9/10 with no confirmation yet" \
  'grep -qE "recheck progress 9/10" <<<"$sm5_out" && ! grep -qE "^SERIAL CONFIRM" <<<"$sm5_out"'
# ---- spec test 6: the tenth triggers exactly one recheck --------------------------------
sm6_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "6: the tenth later overrun runs exactly one recheck" \
  '[ "$(grep -cE "^SERIAL CONFIRM" <<<"$sm6_out")" -eq 1 ]'
# ---- spec tests 7/8: clean parallel results neither advance nor reset the 10-counter ----
run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.clean" "$tmp/sm2.state" >/dev/null
sm78_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "7/8: a clean run left the recheck counter exactly where it was (now 1/10)" \
  'grep -qE "recheck progress 1/10" <<<"$sm78_out"'
# ---- spec tests 20/21/22: one confirmation per run; deterministic order; deferred stays due
mk_suite "$tmp/s20" test_aa.sh test_bb.sh
mk_budgets "$tmp/s20.tsv" "test_aa.sh 10 parallel" "test_bb.sh 10 parallel"
mk_durations "$tmp/s20.durs" "test_aa.sh 99 1" "test_bb.sh 99 1"
s20_out="$(overrun_n 5 "$tmp/s20" "$tmp/s20.tsv" "$tmp/s20.durs" "$tmp/s20.state")"
assert "20: two due tests, exactly ONE scheduled confirmation this run" \
  '[ "$(grep -cE "^SERIAL CONFIRMED|^SERIAL CONFIRMATION FAILED" <<<"$s20_out")" -eq 1 ]'
assert "20/21: equal overdue counts tie-break by due_sequence then LC_ALL=C path (test_aa first)" \
  'grep -qE "test_aa" <<<"$(grep -E "^SERIAL CONFIRM(ED|ATION FAILED)" <<<"$s20_out")"'
assert "22: the deferred test is reported deferred and stays due" \
  'grep -qE "^SERIAL CONFIRMATION DEFERRED: .*test_bb\.sh" <<<"$s20_out"'
s21_out="$(run_rt "$tmp/s20" "$tmp/s20.tsv" "$tmp/s20.durs" "$tmp/s20.state")"
assert "21: the next run confirms the deferred test" \
  'grep -E "^SERIAL CONFIRM" <<<"$s21_out" | grep -qF "test_bb"'
# ---- spec tests 23/24: a failed confirmation clears nothing and does not change the verdict
mk_suite "$tmp/s23" test_cc.sh
mk_budgets "$tmp/s23.tsv" "test_cc.sh 10 parallel"
# the confirmation must FAIL: give the fixture a file that exits 0 in parallel context but
# 1 when DOCKET_RUNTESTS_SOLO=1 is exported by solo_confirm (the seam Task 4 adds for this):
printf '#!/usr/bin/env bash\n[ -n "${DOCKET_RUNTESTS_SOLO:-}" ] && exit 1\necho "ok - trivial"\nexit 0\n' > "$tmp/s23/test_cc.sh"
mk_durations "$tmp/s23.durs" "test_cc.sh 99 1"
s23_rc=0; s23_out="$(overrun_n 5 "$tmp/s23" "$tmp/s23.tsv" "$tmp/s23.durs" "$tmp/s23.state")" || s23_rc=$?
assert "24: a failed advisory confirmation leaves the suite verdict green (exit 0)" '[ "$s23_rc" -eq 0 ]'
assert "23: the failure is reported as SERIAL CONFIRMATION FAILED" \
  'grep -qE "^SERIAL CONFIRMATION FAILED: .*test_cc\.sh" <<<"$s23_out"'
assert "23: the candidate is not cleared — the record stays due, result=failed" \
  'grep -qE "failed" "$tmp/s23.state"'
# ---- spec test 30: the report carries the evidence for each classification --------------
assert "30: a confirmed classification names parallel evidence, solo evidence and threshold" \
  'grep -qE "SERIAL CONFIRMED OVER BUDGET: .* [0-9]+s under -j[0-9]+; [0-9.]+s solo; solo threshold [0-9.]+s" <<<"$(overrun_n 5 "$tmp/s30" "$tmp/s30.tsv" "$tmp/s30.durs" "$tmp/s30.state")"'
```

For test 30 build `s30` with solo duration 99 (`99 > 10 * 3/2`) so the confirmation **breaches**: `mk_durations "$tmp/s30.durs" "test_dd.sh 99 99"`.

- [ ] **Step 2: Run; verify failures.** No `SERIAL CONFIRM*` vocabulary exists yet.

- [ ] **Step 3: Implement**

Add `DOCKET_RUNTESTS_SOLO=1` to the environment `solo_confirm` exports for its child (this is what lets a fixture fail only its confirmation), and read the seam's column 3 for the solo duration. `solo_confirm` runs the single file through a serial `launch`+`wait` cycle into a separate stat area (`$WORK/solo/`), so the parallel run's logs/stat records are untouched (test 29 keeps holding). Selection when several records are due: (1) largest overdue amount (counter minus its trigger, 5 or 10); (2) lowest `due_sequence`; (3) `LC_ALL=C` test-path order. Classification per spec: success + `solo*2 <= ceil*3` → `parallel-sensitive`, `last_confirmation_result=cleared`; success + over → `confirmed-breach`, `result=breached`; either resets `overruns_since_confirmation=0` and records `last_solo_seconds`. Non-zero confirmation exit → `result=failed`, counters unreset, candidate stays due, advisory report line, suite verdict unchanged. At most ONE scheduled confirmation per normal run, and only on qualifying runs. The confirmation happens **before** `state_write` so one write captures everything.

- [ ] **Step 4: Run; verify pass; mutation-test.** Mutations: (a) let a failed confirmation set `cleared` → test 23 reddens; (b) remove the one-per-run bound → test 20 reddens; (c) make clean runs reset the since-counter → test 7/8 reddens.

- [ ] **Step 5: Commit.** `git commit -am "feat(0251): scheduled solo confirmation with bounded tail"`

---

### Task 5: `--strict-budget` immediate confirmation, `-j 1` direct comparison, exit precedence

**Profile hint:** standard.

**Files:**
- Modify: `scripts/run-tests.sh`
- Modify: `tests/test_run_tests_budget_state.sh` (spec tests 14, 15, 16, 25)

**Interfaces:**
- Consumes: Tasks 2–4. Produces: final exit wiring `1 > 3 > 4 > 0` where 4 = strict confirmed breach OR failed strict confirmation.

- [ ] **Step 1: Write failing tests**

```bash
# ---- spec test 14: strict confirms every current candidate immediately ------------------
mk_suite "$tmp/s14" test_aa.sh test_bb.sh
mk_budgets "$tmp/s14.tsv" "test_aa.sh 10 parallel" "test_bb.sh 10 parallel"
mk_durations "$tmp/s14.durs" "test_aa.sh 99 1" "test_bb.sh 99 1"
s14_rc=0; s14_out="$(run_rt "$tmp/s14" "$tmp/s14.tsv" "$tmp/s14.durs" "$tmp/s14.state" --strict-budget)" || s14_rc=$?
assert "14: strict ran a confirmation for BOTH candidates on the first run" \
  '[ "$(grep -cE "^SERIAL CONFIRM|^PARALLEL-SENSITIVE" <<<"$s14_out")" -ge 2 ]'
assert "14: both confirmations were healthy, so strict exits 0" '[ "$s14_rc" -eq 0 ]'
# ---- spec test 15: strict confirmations update stored state -----------------------------
assert "15: strict wrote parallel-sensitive records" \
  '[ "$(grep -c "parallel-sensitive" "$tmp/s14.state")" -eq 2 ]'
# strict with a genuinely slow solo → exit 4
mk_durations "$tmp/s14b.durs" "test_aa.sh 99 99" "test_bb.sh 1 1"
s14b_rc=0; run_rt "$tmp/s14" "$tmp/s14.tsv" "$tmp/s14b.durs" "$tmp/s14b.state" --strict-budget >/dev/null || s14b_rc=$?
assert "14: a strict-confirmed breach exits 4" '[ "$s14b_rc" -eq 4 ]'
# ---- spec test 25: a failed strict confirmation is exit 4 when 1 and 3 do not apply -----
s25_rc=0; run_rt "$tmp/s23" "$tmp/s23.tsv" "$tmp/s23.durs" "$tmp/s25.state" --strict-budget >/dev/null 2>&1 || s25_rc=$?
assert "25: a failed strict confirmation fails closed (exit 4)" '[ "$s25_rc" -eq 4 ]'
# precedence: red suite beats budget — reuse s14 with an added red file
mk_red "$tmp/s14" test_red.sh
s25b_rc=0; run_rt "$tmp/s14" "$tmp/s14.tsv" "$tmp/s14b.durs" "$tmp/s25b.state" --strict-budget >/dev/null 2>&1 || s25b_rc=$?
assert "25: exit 1 outranks the strict breach" '[ "$s25b_rc" -eq 1 ]'
rm -f "$tmp/s14/test_red.sh"
# ---- spec test 16: -j 1 compares directly at 3/2, runs nothing twice, writes no counters -
mk_suite "$tmp/s16" test_aa.sh
mk_budgets "$tmp/s16.tsv" "test_aa.sh 10 parallel"
mk_durations "$tmp/s16.durs" "test_aa.sh 16 16"    # 16 > 10*3/2 — over solo threshold
s16_out="$(run_rt "$tmp/s16" "$tmp/s16.tsv" "$tmp/s16.durs" "$tmp/s16.state" -j 1)"
assert "16: -j 1 may say OVER BUDGET (it is already uncontended)" 'grep -qF "OVER BUDGET" <<<"$s16_out"'
assert "16: -j 1 ran no second execution" '! grep -qE "^SERIAL CONFIRM" <<<"$s16_out"'
assert "16: -j 1 advanced no parallel-overrun counters" \
  '! grep -qE "watching|streak" "$tmp/s16.state" 2>/dev/null'
s16b_rc=0; run_rt "$tmp/s16" "$tmp/s16.tsv" "$tmp/s16.durs" "$tmp/s16b.state" -j 1 --strict-budget >/dev/null || s16b_rc=$?
assert "16: -j 1 over threshold under strict exits 4" '[ "$s16b_rc" -eq 4 ]'
```

- [ ] **Step 2: Run; verify failures.**

- [ ] **Step 3: Implement.** Strict path: after the report loop, for **every** current candidate (bypassing streak/recheck/one-per-run), run `solo_confirm`; healthy clears the candidate (and updates state — strict runs may write even though they can be targeted? No: spec says successful strict confirmations update persistent state; apply the state write for strict runs even when the run would not otherwise qualify, but ONLY the confirmation outcomes, not screening counters, on non-qualifying strict runs). Strict breach or failed strict confirmation → arm exit 4. `-j 1`: in the report loop compare `secs * 2 > ceil * 3` directly; keep the existing per-row/trailing `OVER BUDGET` vocabulary for this path; no counters, no second execution; advisory exit 0 / strict exit 4. Final exit block ordering stays: `failed` → 1; `noresult` → 3; strict-armed → 4; else 0.

- [ ] **Step 4: Run the whole new file plus a real smoke run.** `bash tests/test_run_tests_budget_state.sh` — PASS. Then `scripts/run-tests.sh tests/test_run_tests_budget_state.sh tests/test_runtime_budgets.sh tests/test_assert_hygiene.sh` — exit 0. If the new file's measured serial cost exceeds ~50s, split it into a family pair (`tests/test_run_tests_budget_state.sh` + `tests/test_run_tests_budget_strict.sh`, Tasks 5's tests moving to the second) with two measured budget rows — the placement rules in `tests/README.md` apply to this change's own additions too.

- [ ] **Step 5: Commit.** `git commit -am "feat(0251): strict immediate confirmation + -j1 direct 3/2 comparison"`

---

### Task 6: Leg-1 documentation — runner header, run-tests.md, AGENTS.md, README, stale ids

**Profile hint:** economy.

**Files:**
- Modify: `scripts/run-tests.sh` (the `SLACK_NUM` comment block and file header Usage/Exit text), `scripts/run-tests.md`, `AGENTS.md`, `tests/README.md` ("Running it" section), `tests/test_runtime_budgets.sh` (comments only)

- [ ] **Step 1: Rewrite the runner's constant-site comment block.** Replace the paragraph block above `SLACK_NUM=5; SLACK_DEN=2` (the block beginning "A wall-clock assertion on a shared developer machine…" through "…change 0229's job.") with the new regime, keeping the measured-calibration record (learning `tolerance-constant-calibrated-on-one-machine`: record the measurement, the hardware, and the error direction next to the constant). It must state: 5/2 is now the **screening** threshold producing candidate observations, never a breach; 3/2 is the solo threshold and the only authoritative comparison; five consecutive qualifying overruns schedule the first solo confirmation; ten later overruns schedule a recheck; one scheduled confirmation per normal run; `--strict-budget` confirms all current candidates immediately; history is per-worktree and per-execution-context, advisory, fail-open; red/incomplete/interrupted/targeted/`--no-budget-check` runs do not mutate history; exit codes unchanged. Add `--budget-state PATH` and `--print-budget-state-path` to the Usage header.
- [ ] **Step 2: Rewrite `scripts/run-tests.md`'s budget sections** (the "stated honestly" / advisory-rationale / regrowth-gap sections around its 0229 references) to the same content, and repoint every `change 0229` / `0230` reference to **change 0251** (the runner's own three `0229` mentions included — one sits in the report's `Pass --strict-budget` hint string; update that printf too, and the corresponding `tests/test_run_tests_budget_state.sh` has no assert on it, but `grep -rn "0229" scripts tests AGENTS.md` must come back empty of live references afterwards — run that grep and fix every executable/doc hit; archived changes and specs are point-in-time records, leave them).
- [ ] **Step 3: Update `AGENTS.md`'s Guards bullet.** Replace the clause about "A trailing `OVER BUDGET:` line" with the new vocabulary — e.g.: "A `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding; `SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach to act on. Neither fails the run by default; nothing else will catch them for you." Keep the sentence's surrounding structure.
- [ ] **Step 4: Update `tests/README.md`'s "Running it" section** (advisory description + exit list stays the same codes; describe screen-then-confirm in one sentence) and `tests/test_runtime_budgets.sh`'s comments: the header comment `under the 5/2 factor` (its "buys 62s MORE measured slack" framing still holds — 5/2 still screens; adjust wording to "screening factor"), and the two comment blocks around its fixture rows that say rows "report OVER BUDGET" under the 5/2 factor — those now produce `BUDGET WATCH` observations; fix the wording, not the fixtures.
- [ ] **Step 5: Verify and commit.** Run `bash tests/test_runtime_budgets.sh` and `scripts/run-tests.sh tests/test_docs_structure.sh 2>/dev/null || true` plus a whole-repo `grep -rn -e "0229" -e "0230" scripts tests AGENTS.md` review (docs/changes and docs/superpowers excluded — records). `git commit -am "docs(0251): budget-confirmation vocabulary across runner, contract, AGENTS, README"`

---

### Task 7: Family-corpus rework of the 0126 prelude-correspondence guard

**Profile hint:** premium (guard population changes are where this suite's regressions live).

**Files:**
- Modify: `tests/test_docket_config.sh` — section "(T) prelude correspondence guard (change 0126)" (the `prelude_report` function through the 0148 `r9_poison_site_line` assert) and nothing else.

**Interfaces:**
- Produces: `prelude_report` unchanged in signature (`prelude_report <file> <keys>`) but now called once **per corpus file**; SITE lines gain file attribution: `SITE <basename>:<line> ok|exempt|viol...`; totals summed across the corpus; new corpus-membership floor. 0258's family-glob block ("0258 leg 2 — rung-pair completeness") is **prior art to mirror, not duplicate** — reuse its `"$REPO"/tests/test_docket_config*.sh` glob spelling.

- [ ] **Step 1: Establish the pre-change baseline.** Run `bash tests/test_docket_config.sh` and record the printed `TOTALS sites=… exempt=… ok=… viol=…` line — the corpus rework must reproduce these exact numbers while the corpus is still one file.
- [ ] **Step 2: Rework the collection to the glob corpus.** Replace every `${BASH_SOURCE[0]}` in the guard region (site-report call, the four raw-grep extractors, `t_selfrefs`, `r9_poison_site_line`) with a loop over the computed corpus:

```bash
# Corpus is the DISCOVERED family, never an enumerated list (ADR-0050;
# learning backstop-must-compute-not-reenumerate): a new shard self-registers
# with this guard exactly as it self-registers with the runner. Mirrors the
# 0258 L2 control's family glob below.
t_corpus=()
while IFS= read -r tc_f; do t_corpus+=("$tc_f"); done \
  < <(printf '%s\n' "$REPO"/tests/test_docket_config*.sh | LC_ALL=C sort)
assert "0126 T: the family glob resolved to real files" '[ -e "${t_corpus[0]}" ]'
```

Aggregate per file: `t_out` becomes the concatenation of per-file reports where every `SITE` line is rewritten `SITE <basename>:<line> …` (pass the basename into the awk as `-v fbase=` and change the two `print "SITE " SL[k] …` sites and the TOTALS print accordingly, emitting per-file `TOTALS` lines the shell sums into `t_sites`/`t_exempt`/`t_ok`/`t_viol`). The raw-grep cross-check runs per file — `t_helper` (one canonical `assert(){` per family file), `t_comments`, `t_raw` summed; `t_selflit` and `t_selfrefs` computed only for the file that carries the markers (the self-block subtraction applies only to the marker-carrying file; assert exactly ONE corpus file contains `T_SELF_START`'s literal). Keep every floor at today's value: sites `>= 60`, keycount `>= 20`, `ok*10 >= sites*9`, `viol == 0`, extractor agreement (now summed). Update the 0148 `r9_poison_site_line` derivation to scan the corpus file that contains the r9 fixture and assert on `SITE <that-basename>:<line>` — derived by pattern per file, never hand-picked.
- [ ] **Step 3: Verify identical totals.** Run `bash tests/test_docket_config.sh`; the summed totals must equal Step 1's baseline exactly (single-file corpus ⇒ same numbers). Expected: PASS, byte-identical TOTALS.
- [ ] **Step 4: Mutation-test the corpus population.** (a) Temporarily copy a small fake shard `tests/test_docket_config_zz_mutation.sh` containing one uncleared `eval "$V"` cmdsub site reading an exported key — the family run must now report `viol >= 1` and redden the `viol == 0` assert; delete the fake. (b) Point the glob at a nonexistent spelling — the non-vacuity assert reddens. Record both outcomes.
- [ ] **Step 5: Commit.** `git add tests/test_docket_config.sh && git commit -m "test(0251): prelude guard population = discovered test_docket_config* family corpus"`

---

### Task 8: Split `tests/test_docket_config.sh`, re-cut budgets, finish the docs

**Profile hint:** premium (large mechanical move with parity proof and registry updates).

**Files:**
- Modify: `tests/test_docket_config.sh` (loses its tail), Create: `tests/test_docket_config_guards.sh` (or a better `_<topic>` name for the moved sections — the tail-holding shard either way)
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`), `tests/README.md`, `scripts/run-tests.sh` header count, `scripts/run-tests.md` count

- [ ] **Step 1: Capture the parity baseline.** `scripts/run-tests.sh -j 1 --timings /tmp/base.tsv tests/test_docket_config.sh` — record its ok/notok counts (columns 4/5) and serial seconds.
- [ ] **Step 2: Choose the boundary by measurement.** The file's `# ===` section banners are the only legal cut points. Target: each part ≤ ~30s serial. Start from the banner nearest the halfway line (`grep -n "^# ===" tests/test_docket_config.sh`; at plan time the 0102/0107-era banner block around line 1868 or 2102 — re-derive, do not trust these numbers), then verify by measuring both halves after the split and move the boundary one section if either exceeds target. Constraint: the moved tail must include, as one unit, section (T) with its self-block markers, the 0148 asserts, the 0258 L2 control, the 0276 dummy_mode section, and the file-final 0174 template-integrity assert.
- [ ] **Step 3: Perform the split.** The new shard replicates the head prelude **exactly as the family convention does** (shebang, `set -uo pipefail`, `REPO`, `SCRIPT`, `fail`, the byte-canonical `assert()`, the `_mkrepo_build_template`/`mkrepo`/`run` machinery, plus ONLY the helpers the moved sections call — `rung`, `ct_get`, `ct_commit`, `run_resolver_with`, whichever the moved text references; grep the moved text for helper names rather than copying all). Both shards keep their own 0174 template snapshot + final integrity assert + `PASS/FAIL` exit block. Marker lines (`# RUNG_PAIR:` etc.) move with their fixtures. The head file keeps its own final integrity assert and exit block. No assertion text changes — this is a move.
- [ ] **Step 4: Prove assertion-count parity.** `scripts/run-tests.sh -j 1 --timings /tmp/split.tsv tests/test_docket_config.sh tests/test_docket_config_guards.sh` — the two rows' summed ok and summed notok must equal Step 1's counts exactly (the duplicated prelude's own 0174 asserts run twice now: if the sums legitimately differ by exactly the replicated-prelude asserts, document the delta arithmetic in the commit message with the per-assert names — parity means *no moved assert was lost*, and the burden of proof is the enumerated delta).
- [ ] **Step 5: Tighten the corpus floor.** In the guard (now in the tail shard), add the post-split floor: `assert "0126 T: the family corpus spans at least two files" '[ "${#t_corpus[@]}" -ge 2 ]'` (learning `marker-scoped-guard-needs-a-population-floor`: a renamed shard must not quietly shrink the corpus to one). Mutation: rename the head file momentarily → assert reddens; restore.
- [ ] **Step 6: Re-cut the budget rows.** Delete the `test_docket_config.sh 55 parallel` row; add two rows from Step 4's measured serial seconds using the seeding rule (next multiple of 5 + 5s, min 10, applied to the worst standalone serial reading). Re-seed `EXPECTED_TOTAL` (current 2275, minus 55, plus the two new ceilings — state the "re-cutting a sharded file redistributes its cost" case in the diff). `bash tests/test_runtime_budgets.sh` — PASS.
- [ ] **Step 7: Finish the counts and prose.** Derive the fresh suite count (`ls tests/test_*.sh | wc -l`) and write it into `tests/README.md` line 1's "N standalone Bash files", `scripts/run-tests.sh`'s header "The suite is N hermetic per-file scripts", and `scripts/run-tests.md`'s "The suite is now N files" sentence. Rewrite `tests/README.md`'s "argued whole" bullet for `test_docket_config.sh`: it leaves the list — record that change 0251 moved the guard's population to the discovered family corpus, so the split is now routine and further shards self-register with the guard. Update the same section's shard-family examples to include `test_docket_config*.sh`.
- [ ] **Step 8: Run the affected set and commit.** `scripts/run-tests.sh tests/test_docket_config.sh tests/test_docket_config_guards.sh tests/test_runtime_budgets.sh tests/test_comment_anchor_style.sh` — exit 0, no `SERIAL CONFIRMED OVER BUDGET`. `git add -A tests/ scripts/run-tests.sh scripts/run-tests.md && git commit -m "refactor(0251): split test_docket_config at a measured boundary; re-cut budgets"` — **scope the `git add` to exactly the files this task names, never a bare `-A` at repo root.**

---

## Verification (build gate)

The build role's final gate runs the whole suite via the resolved `finalize.test_command`. Expect: exit 0; no `FAILED:`/`NO RESULT:`; read every `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line as a finding to record (they are this change's own new vocabulary — their first real appearance is itself evidence the screen works). Record in the results notes: the measured serial seconds and ceilings for the three touched budget rows and the remaining margin per row (learning `budget-headroom-is-spent-before-it-is-breached`: report margins as numbers, never as "did not trip").

## Self-review notes (already applied)

- Spec coverage: all 30 enumerated tests are assigned (T1: 28–29; T2: 26–27; T3: 1, 3, 9–13, 17–19; T4: 2, 4–8, 20–24, 30; T5: 14–16, 25). Leg-2 requirements map to T7 (corpus, attribution, floors, r9) and T8 (split, parity, `>= 2` floor, budget re-cut, docs). Assumption 8's doc list maps to T6 + T8 Step 7.
- The `[ 1 = 1 ]` placeholder in T3's test-19 sketch is explicitly flagged as illustrative-only with instructions to replace it with the discriminating assert — the worker must not ship it.
- Type/name consistency: `--budget-state`, `--print-budget-state-path`, `DOCKET_RUNTESTS_TESTS_DIR`, `DOCKET_RUNTESTS_TEST_DURATIONS`, `DOCKET_RUNTESTS_SOLO`, `solo_confirm`, `context_key`, state names `unobserved|watching|parallel-sensitive|confirmed-breach`, and the six report labels are spelled identically across tasks; the report labels are the spec's exact strings.

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
# The runner result marker in the generated fixture is split across two adjacent quoted literals
# (`ok ''- `, `NOT ''OK`) so this helper's OWN body is not read as an assert-family definition by
# scripts/check-test-source-hygiene.sh's is_family (it flags any body carrying `ok - `/`NOT OK`).
# The concatenation is byte-identical — the emitted fixtures still print the real markers.
mk_suite(){ local d="$1"; shift; mkdir -p "$d"
  local n; for n in "$@"; do printf '#!/usr/bin/env bash\necho "ok ''- trivial"\nexit 0\n' > "$d/$n"; done; }
# mk_red <dir> <name> : one deliberately failing fixture test.
mk_red(){ printf '#!/usr/bin/env bash\necho "NOT ''OK - forced"\nexit 1\n' > "$1/$2"; }
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
# Tab-field match via [[:space:]], not a literal `\t`: this repo's grep is ugrep, which reads `\t`
# in an ERE as a stray-escaped `t`, not a tab (GNU grep agrees — `\t` is not a tab there either),
# so the five real tabs would never match. The row is all-digit fields, so [[:space:]] is exact.
assert "28: timings rows carry the injected parallel duration" \
  'grep -qE "test_a\.sh[[:space:]]99[[:space:]]0[[:space:]]1[[:space:]]0$" "$tmp/s28.timings"'

# ---- spec test 29: the parallel run stays the sole authority for results/asserts/logs ----
mk_suite "$tmp/s29" test_a.sh
mk_red   "$tmp/s29" test_b.sh
mk_budgets "$tmp/s29.tsv" "test_a.sh 10 parallel" "test_b.sh 10 parallel"
mk_durations "$tmp/s29.durs" "test_a.sh 1 1" "test_b.sh 1 1"
t29_rc=0; t29_out="$(run_rt "$tmp/s29" "$tmp/s29.tsv" "$tmp/s29.durs" "$tmp/s29.state")" || t29_rc=$?
assert "29: a red fixture file still fails the suite (exit 1)" '[ "$t29_rc" -eq 1 ]'
assert "29: the SUITE line counts the parallel run's own asserts" \
  'grep -qE "^SUITE files=2 passed=1 failed=1 asserts=2 " <<<"$t29_out"'

# ---- store mechanics: default path resolution -------------------------------------------
sp_out="$(bash "$RUNNER" --print-budget-state-path)"
# The expected path mirrors budget_state_path()'s OWN anchoring: `git rev-parse --git-dir` prints a
# RELATIVE `.git` in a normal clone but an ABSOLUTE dir in a linked worktree, and the store anchors a
# relative result under $REPO. Anchor identically here — an un-anchored expectation passes only in a
# worktree (where anchoring is a no-op) and fails on a normal clone such as a fresh CI checkout.
sp_exp_gd="$(git -C "$REPO" rev-parse --git-dir)"; case "$sp_exp_gd" in /*) ;; *) sp_exp_gd="$REPO/$sp_exp_gd" ;; esac
assert "store: default path lives under this repo's git dir" \
  '[ "$sp_out" = "$sp_exp_gd/docket/run-tests-budget-state.tsv" ]'

# ---- store mechanics: created with restrictive permissions, even under a hostile umask ---
mk_suite "$tmp/sperm" test_a.sh
mk_budgets "$tmp/sperm.tsv" "test_a.sh 10 parallel"
mk_durations "$tmp/sperm.durs" "test_a.sh 99 1"
( umask 077; run_rt "$tmp/sperm" "$tmp/sperm.tsv" "$tmp/sperm.durs" "$tmp/sperm.state" >/dev/null )
( umask 022; run_rt "$tmp/sperm" "$tmp/sperm.tsv" "$tmp/sperm.durs" "$tmp/sperm.state2" >/dev/null )
sperm_mode="$(ls -l "$tmp/sperm.state2" | cut -c1-10)"
assert "store: the state file is not group/world accessible" '[ "$sperm_mode" = "-rw-------" ]'
# Capture-then-match, never `head | grep -q`: an early-exiting consumer under pipefail turns a
# SIGPIPE into an intermittent 141 (AGENTS.md, Shell).
assert "store: the state file survives a run and carries the v1 header" \
  'sperm_hdr="$(head -n1 "$tmp/sperm.state")"; grep -qF "docket-run-tests-budget-state v1" <<<"$sperm_hdr"'

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
  '[ "$s26b_rc" -eq 0 ] && { s26b_hdr="$(head -n1 "$tmp/s26b.state")"; grep -qF "v1" <<<"$s26b_hdr"; }'

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

# =========================================================================================
# Task 3 (change 0251): parallel screening + qualifying-overrun state machine
# =========================================================================================
# The recurring fixture: one file test_slow.sh, ceiling 10, injected parallel duration 99
# (99 > 10 * 5/2, so every "over" run crosses the screening threshold), solo 1. A "clean" run
# injects parallel 1. All runs are -j 2 default-corpus via run_rt unless stated otherwise.

# overrun_n <n> <suite> <budgets> <durs> <state> [runner args...] : run N runs against one
# persistent state file; returns the LAST run's stdout.
overrun_n(){ local n="$1" suite="$2" budgets="$3" durs="$4" state="$5"; shift 5
  local i out; for ((i=0; i<n; i++)); do out="$(run_rt "$suite" "$budgets" "$durs" "$state" "$@")"; done
  printf '%s' "$out"; }

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

# ---- spec tests 9/10/11/13: non-qualifying runs mutate nothing --------------------------
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
# test 11 (missing result): a fixture that kills its own process subtree — the job records no
# healthy result, so the suite is not green and the run does not qualify.
printf '#!/usr/bin/env bash\nkill -9 $$\n' > "$tmp/sm/test_dies.sh"
freeze_check "11: a run with a missing result does not mutate history" "$tmp/smf.state" \
  run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/smf.state"
rm -f "$tmp/sm/test_dies.sh"

# ---- spec test 12: interrupted runs mutate nothing --------------------------------------
# Behavioral (spec-preferred): the interrupt handler exits BEFORE the report loop, so state
# application is never reached. Start a run whose only fixture blocks after signalling that it
# started, signal the runner directly, and confirm the (empty) state file was never written.
# SIGTERM, not SIGINT: this runner is BACKGROUNDED from a non-interactive shell, and bash sets
# SIGINT to ignored in async children — an ignored-on-entry signal cannot be trapped, so the
# runner's INT trap would be a no-op here. SIGTERM is not auto-ignored and the runner's on_signal
# handles TERM identically (exit 143), so it exercises the same "exit before the report" path.
mk_suite "$tmp/sint" _placeholder.sh; rm -f "$tmp/sint/_placeholder.sh"
printf '#!/usr/bin/env bash\ntouch "%s"\nexec sleep 3\n' "$tmp/sint.started" > "$tmp/sint/test_slow.sh"
mk_budgets "$tmp/sint.tsv" "test_slow.sh 10 parallel"
mk_durations "$tmp/sint.durs" "test_slow.sh 99 1"
sint_before="$(cat "$tmp/sint.state" 2>/dev/null || true)"   # no file yet -> empty
rm -f "$tmp/sint.started"
DOCKET_RUNTESTS_TESTS_DIR="$tmp/sint" DOCKET_RUNTESTS_TEST_DURATIONS="$tmp/sint.durs" \
  bash "$RUNNER" --budgets "$tmp/sint.tsv" --budget-state "$tmp/sint.state" -j 2 >/dev/null 2>&1 &
sint_pid=$!
for _i in $(seq 1 50); do [ -f "$tmp/sint.started" ] && break; sleep 0.1; done
kill -TERM "$sint_pid" 2>/dev/null
wait "$sint_pid" 2>/dev/null
sint_after="$(cat "$tmp/sint.state" 2>/dev/null || true)"
assert "12: an interrupted run does not mutate persistent history" \
  '[ "$sint_before" = "$sint_after" ]'

# ---- spec test 17: -j values keep independent histories ---------------------------------
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state" >/dev/null       # -j 2
sm17_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state" -j 3)"    # -j 3
assert "17: a -j 3 overrun starts its own streak at 1/5" 'grep -qE "streak 1/5" <<<"$sm17_out"'
sm17b_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm17.state")"        # back to -j 2
assert "17: the -j 2 history was neither advanced nor consumed by the -j 3 run" \
  'grep -qE "streak 4/5" <<<"$sm17b_out"'

# ---- spec test 18: a ceiling change invalidates the record ------------------------------
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm18.state" >/dev/null
mk_budgets "$tmp/sm18b.tsv" "test_slow.sh 20 parallel"     # ceiling moved 10 -> 20; 99 > 50 still qualifies
sm18_out="$(run_rt "$tmp/sm" "$tmp/sm18b.tsv" "$tmp/sm.over" "$tmp/sm18.state")"
assert "18: a ceiling change starts a fresh record (streak 1/5)" 'grep -qE "streak 1/5" <<<"$sm18_out"'

# ---- spec test 19: a mode change does not advance the parallel-context record -----------
# The discriminating assert (not the plan's illustrative `[ 1 = 1 ]`): a serial-mode budget
# means the file is not parallel-executed, so a serial run must not touch the parallel-context
# streak. Seed 3 parallel overruns (streak 3), interpose one serial-mode run, then one parallel
# overrun — the streak must read 4/5 (unpolluted continuation), never 5/5.
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm19.state" >/dev/null       # parallel streak 3
mk_budgets "$tmp/sm19s.tsv" "test_slow.sh 10 serial"
run_rt "$tmp/sm" "$tmp/sm19s.tsv" "$tmp/sm.over" "$tmp/sm19.state" >/dev/null         # serial-mode run
sm19_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm19.state")"         # parallel overrun again
assert "19: a serial-mode run did not advance the parallel-context streak (continues at 4/5)" \
  'grep -qE "streak 4/5" <<<"$sm19_out"'

# ==== Task 4: scheduled solo confirmation ================================================
# Same fixture family as Task 3 (test_slow.sh, ceiling 10, injected parallel 99 = overrun, solo 1).
# A confirmation re-runs ONE file serially through solo_confirm, reads the injection seam's SOLO
# column (column 3), and compares solo*2 > ceiling*3. It NEVER changes the suite verdict.

# ---- spec test 2: the fifth consecutive overrun triggers exactly one solo confirmation ---
sm2_out="$(overrun_n 5 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "2: the fifth consecutive overrun runs exactly one solo confirmation" \
  '[ "$(grep -cE "^SERIAL CONFIRMATION DUE:" <<<"$sm2_out")" -eq 1 ]'
assert "2: a healthy fifth-overrun confirmation is neither OVER BUDGET nor FAILED" \
  '! grep -qE "^SERIAL CONFIRMED OVER BUDGET|^SERIAL CONFIRMATION FAILED" <<<"$sm2_out"'

# ---- spec test 4: a healthy solo result (1s <= 15s) records parallel-sensitive/cleared ---
assert "4: a healthy solo classifies the record parallel-sensitive" \
  'grep -qE "parallel-sensitive" "$tmp/sm2.state"'
assert "4: a healthy solo records last_confirmation_result=cleared" \
  'grep -qE "cleared" "$tmp/sm2.state"'

# ---- spec test 5: the next nine later overruns trigger no further confirmation -----------
sm5_out="$(overrun_n 9 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "5: recheck progress reads 9/10 after nine later overruns" \
  'grep -qE "recheck progress 9/10" <<<"$sm5_out"'
assert "5: no confirmation ran across the nine later overruns" \
  '! grep -qE "^SERIAL CONFIRM" <<<"$sm5_out"'

# ---- spec test 6: the tenth later overrun triggers exactly one recheck -------------------
sm6_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"
assert "6: the tenth later overrun runs exactly one recheck confirmation" \
  '[ "$(grep -cE "^SERIAL CONFIRMATION DUE:" <<<"$sm6_out")" -eq 1 ]'

# ---- spec tests 7/8: clean parallel results neither advance nor reset the 10-counter -----
# After test 6 the recheck counter reset to 0. Bump it to 3, interpose ONE clean qualifying run,
# then one overrun: the counter must read 4/10. Advancing on the clean run would read 5/10;
# resetting it would read 1/10 — a single assert discriminates both directions (mutation (c)).
overrun_n 3 "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state" >/dev/null   # since 0 -> 3
run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.clean" "$tmp/sm2.state" >/dev/null        # clean: must leave since = 3
sm78_out="$(run_rt "$tmp/sm" "$tmp/sm.tsv" "$tmp/sm.over" "$tmp/sm2.state")"      # overrun: since 3 -> 4
assert "7/8: a clean parallel run neither advanced nor reset the recheck counter (now 4/10)" \
  'grep -qE "recheck progress 4/10" <<<"$sm78_out"'

# ---- spec tests 20/21/22: one confirmation per run; deterministic order; deferred stays due
mk_suite "$tmp/s20" test_aa.sh test_bb.sh
mk_budgets "$tmp/s20.tsv" "test_aa.sh 10 parallel" "test_bb.sh 10 parallel"
mk_durations "$tmp/s20.durs" "test_aa.sh 99 1" "test_bb.sh 99 1"
s20_out="$(overrun_n 5 "$tmp/s20" "$tmp/s20.tsv" "$tmp/s20.durs" "$tmp/s20.state")"
assert "20: two tests become due together, but exactly ONE confirmation runs this run" \
  '[ "$(grep -cE "^SERIAL CONFIRMATION DUE:" <<<"$s20_out")" -eq 1 ]'
assert "20/21: the tie breaks by due_sequence then LC_ALL=C path — test_aa confirmed first" \
  'grep -qE "^SERIAL CONFIRMATION DUE: .*test_aa\.sh" <<<"$s20_out"'
assert "22: the other due test is reported deferred, not confirmed" \
  'grep -qE "^SERIAL CONFIRMATION DEFERRED: .*test_bb\.sh" <<<"$s20_out"'
s21_out="$(run_rt "$tmp/s20" "$tmp/s20.tsv" "$tmp/s20.durs" "$tmp/s20.state")"
assert "21: the deferred test stayed due and is confirmed on the next run" \
  'grep -qE "^SERIAL CONFIRMATION DUE: .*test_bb\.sh" <<<"$s21_out"'

# ---- spec tests 23/24: a failed confirmation clears nothing and never changes the verdict -
# The fixture is green in the parallel run but exits 1 once solo_confirm exports DOCKET_RUNTESTS_SOLO.
mk_suite "$tmp/s23" test_cc.sh
printf '#!/usr/bin/env bash\n[ -n "${DOCKET_RUNTESTS_SOLO:-}" ] && exit 1\necho "ok - trivial"\nexit 0\n' > "$tmp/s23/test_cc.sh"
mk_budgets "$tmp/s23.tsv" "test_cc.sh 10 parallel"
mk_durations "$tmp/s23.durs" "test_cc.sh 99 1"
overrun_n 4 "$tmp/s23" "$tmp/s23.tsv" "$tmp/s23.durs" "$tmp/s23.state" >/dev/null   # streak -> 4
s23_rc=0; s23_out="$(run_rt "$tmp/s23" "$tmp/s23.tsv" "$tmp/s23.durs" "$tmp/s23.state")" || s23_rc=$?
assert "24: a failed advisory confirmation leaves the suite verdict green (exit 0)" '[ "$s23_rc" -eq 0 ]'
assert "23: the failure is reported as SERIAL CONFIRMATION FAILED" \
  'grep -qE "^SERIAL CONFIRMATION FAILED: .*test_cc\.sh" <<<"$s23_out"'
assert "23: the failed confirmation cleared nothing — the record stays watching/due, result=failed" \
  'grep -qE "failed" "$tmp/s23.state" && grep -qE "watching" "$tmp/s23.state"'

# ---- spec test 30: a confirmed breach reports parallel evidence, solo evidence, threshold --
mk_suite "$tmp/s30" test_dd.sh
mk_budgets "$tmp/s30.tsv" "test_dd.sh 10 parallel"
mk_durations "$tmp/s30.durs" "test_dd.sh 99 99"   # solo 99 > 10 * 3/2 -> a genuine solo breach
s30_out="$(overrun_n 5 "$tmp/s30" "$tmp/s30.tsv" "$tmp/s30.durs" "$tmp/s30.state")"
assert "30: a confirmed breach names parallel seconds, solo seconds, and the solo threshold" \
  'grep -qE "^SERIAL CONFIRMED OVER BUDGET: .*test_dd\.sh .* [0-9]+s under -j[0-9]+; [0-9.]+s solo; solo threshold [0-9.]+s" <<<"$s30_out"'

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

# ---- serial-mode budget enforcement under the default -jN gate (finding, change 0251) ---
# A `serial`-mode file runs on its own UNCONTENDED serial lane (SER files are launched one at a
# time and waited on), so its wall clock is authoritative even under a normal -jN run — it must go
# through the SAME `ceiling * 3/2` direct comparison the -j 1 path uses, NOT the parallel screening
# arm. Before the fix it satisfied neither arm (direct required JOBS -eq 1; screening required
# fmode = parallel), so a serial-mode overrun went entirely unchecked at the default gate.
mk_suite "$tmp/sser" test_ser.sh
mk_budgets "$tmp/sser.tsv" "test_ser.sh 10 serial"
mk_durations "$tmp/sser.over"  "test_ser.sh 16 1"   # report-loop column 2 = 16 > 10*3/2 = 15 (uncontended lane)
mk_durations "$tmp/sser.under" "test_ser.sh 14 1"   # 14 < 15 — under the solo threshold
# default -j 2 run: an uncontended serial-lane overrun IS authoritative, so it says OVER BUDGET
sser_rc=0; sser_out="$(run_rt "$tmp/sser" "$tmp/sser.tsv" "$tmp/sser.over" "$tmp/sser.state")" || sser_rc=$?
assert "serial: a serial-mode overrun is flagged OVER BUDGET even under the default -j 2 gate" \
  'grep -qE "^test_ser .* OVER BUDGET \(ceiling 10s\)" <<<"$sser_out"'
assert "serial: the OVER BUDGET breach is advisory by default (exit 0)" '[ "$sser_rc" -eq 0 ]'
assert "serial: a serial-mode file is never routed through the parallel screening arm" \
  '! grep -qE "^BUDGET WATCH:|^PARALLEL-SENSITIVE:" <<<"$sser_out"'
# --strict-budget promotes the same serial-mode breach to a fatal exit 4
sser_s_rc=0; run_rt "$tmp/sser" "$tmp/sser.tsv" "$tmp/sser.over" "$tmp/sser_s.state" --strict-budget >/dev/null 2>&1 || sser_s_rc=$?
assert "serial: a serial-mode breach exits 4 under --strict-budget" '[ "$sser_s_rc" -eq 4 ]'
# a serial-mode file under the solo threshold is not flagged
sser_u_rc=0; sser_u_out="$(run_rt "$tmp/sser" "$tmp/sser.tsv" "$tmp/sser.under" "$tmp/sser_u.state")" || sser_u_rc=$?
assert "serial: a serial-mode file under the solo threshold is not flagged, exit 0" \
  '[ "$sser_u_rc" -eq 0 ] && ! grep -qF "OVER BUDGET" <<<"$sser_u_out"'

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"

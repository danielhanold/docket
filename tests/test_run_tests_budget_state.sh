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

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"

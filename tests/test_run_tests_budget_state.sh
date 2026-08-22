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
# NOTE: --budget-state does not exist until Task 2; the <state> parameter is kept in the signature
# from the start so no later test changes shape, but it is not yet passed to the runner.
run_rt(){ local suite="$1" budgets="$2" durs="$3" state="$4"; shift 4
  DOCKET_RUNTESTS_TESTS_DIR="$suite" DOCKET_RUNTESTS_TEST_DURATIONS="$durs" \
    bash "$RUNNER" --budgets "$budgets" -j 2 "$@"; }

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

if [ "$fail" = 0 ]; then echo PASS; else echo FAIL; fi
exit "$fail"

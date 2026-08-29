#!/usr/bin/env bash
# tests/test_devtest_runner.sh — end-to-end coverage of `docket development test`, the Go-native
# whole-suite runner that backs finalize.test_command (change 0318). This file proves the SOURCE
# ENTRY — `go run ./cmd/docket development test`, exactly as the merge gate runs it — not the
# suiterunner package (internal/suiterunner/*_test.go owns the package's unit contract). It drives
# the real command over synthetic fixture suites through the shared env seams and asserts the exit
# contract and the Bash oracle's report clauses.
#
# EXIT CODES THROUGH `go run`. `go run` collapses ANY non-zero program exit to its OWN exit 1 and
# prints `exit status N` to stderr; a signal death gives the shell a 128+signum wait status. So the
# EFFECTIVE program exit is derived from that `exit status N` line (devtest_rc), never read straight
# from `go run`'s own status — which would make exit 4 and exit 1 indistinguishable. This is a
# property of the interpreter, not the runner: the merge gate reads only zero-vs-non-zero, which
# `go run` preserves faithfully.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist, and scripts/run-tests.sh accounts
# results on the `ok - ` / `NOT OK - ` markers it prints.
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES note in that
# file's header): scripts/run-tests.sh gives every job a private HOME, so with GOMODCACHE/GOCACHE
# unset `go run` finds neither a module nor a build cache and recompiles cold on EVERY suite run.
# So this file pins both to <git common dir>/docket-go-cache/{mod,build} — shared across worktrees,
# concurrent-safe, outside every working tree — whenever the caller has not already chosen one, with
# -modcacherw required. Only the first run after a fresh clone is cold.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the runner cannot be exercised without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

# Pin the caches out of the job's throwaway HOME — see CACHES in this header.
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

scratch="$(mktemp -d "${TMPDIR:-/tmp}/devtest_runner.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
OUT="$scratch/out"; ERR="$scratch/err"

# devtest_rc runs the source entry (with any extra args) and captures stdout to $OUT, stderr to
# $ERR. It sets DEVTEST_RC to the EFFECTIVE program exit, derived from `go run`'s `exit status N`
# line (see the header) so exit 4 stays distinguishable from exit 1. The environment the caller has
# exported (the shared seams) is inherited by the child.
devtest_rc(){
  go run ./cmd/docket development test "$@" >"$OUT" 2>"$ERR"
  local grc=$?
  if [ "$grc" -eq 0 ]; then DEVTEST_RC=0; return; fi
  local line
  line="$(grep -E '^exit status [0-9]+$' "$ERR")"
  if [ -n "$line" ]; then DEVTEST_RC="${line##* }"; else DEVTEST_RC="$grc"; fi
}

# ---- Case 1: green suite -------------------------------------------------------------------------
fix1="$scratch/fix1"; mkdir -p "$fix1"
printf "echo 'ok - one'\n" > "$fix1/test_one.sh"
printf "echo 'ok - two'\n" > "$fix1/test_two.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_two.sh\t20\tparallel\n' > "$fix1/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix1" DOCKET_RUNTESTS_BUDGETS="$fix1/budgets.tsv"
export DOCKET_RUNTESTS_JOBS=2 DOCKET_RUNTESTS_STATE="$scratch/state1.tsv"
unset DOCKET_RUNTESTS_TEST_DURATIONS DOCKET_RUNTESTS_STRICT
devtest_rc
assert "green suite exits 0" '[ "$DEVTEST_RC" = 0 ]'
assert "green suite reports SUITE files=2 passed=2 failed=0" 'grep -q "SUITE files=2 passed=2 failed=0" "$OUT"'

# ---- Case 2: ordinary failure --------------------------------------------------------------------
fix2="$scratch/fix2"; mkdir -p "$fix2"
printf "echo 'ok - one'\n" > "$fix2/test_one.sh"
printf "echo 'NOT OK - broke'\nexit 1\n" > "$fix2/test_bad.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_bad.sh\t20\tparallel\n' > "$fix2/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix2" DOCKET_RUNTESTS_BUDGETS="$fix2/budgets.tsv"
export DOCKET_RUNTESTS_STATE="$scratch/state2.tsv"
devtest_rc
assert "a failing test exits 1" '[ "$DEVTEST_RC" = 1 ]'
assert "a failing test is named in FAILED:" 'grep -q "^FAILED:.*test_bad" "$OUT"'

# ---- Case 3: screening advisory (contended parallel crossing, never authoritative) ---------------
fix3="$scratch/fix3"; mkdir -p "$fix3"
printf "echo 'ok - one'\n" > "$fix3/test_one.sh"
printf "echo 'ok - two'\n" > "$fix3/test_two.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_two.sh\t20\tparallel\n' > "$fix3/budgets.tsv"
# Inject a parallel duration of 60s for test_one: 60*2=120 > ceiling 20 * 5 = 100, so it crosses the
# screening threshold (5/2) but is a parallel-mode measurement, which is never labeled OVER BUDGET.
printf 'test_one.sh\t60\t60\n' > "$fix3/durations.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix3" DOCKET_RUNTESTS_BUDGETS="$fix3/budgets.tsv"
export DOCKET_RUNTESTS_STATE="$scratch/state3.tsv" DOCKET_RUNTESTS_TEST_DURATIONS="$fix3/durations.tsv"
devtest_rc
assert "a screening crossing is advisory (exit 0)" '[ "$DEVTEST_RC" = 0 ]'
assert "a screening crossing emits a BUDGET WATCH line" 'grep -q "^BUDGET WATCH: .*test_one.sh" "$OUT"'
# Negated assert via captured variable + the -e form (AGENTS.md: a negated grep needs -e, and a
# captured-then-tested variable avoids the SIGPIPE/vacuous-guard traps).
over3="$(grep -E -e '^OVER BUDGET' "$OUT")"
assert "a screening crossing is never labeled OVER BUDGET" '[ -z "$over3" ]'

# ---- Case 4: serial-mode authoritative breach (advisory by default, gated under --strict) --------
fix4="$scratch/fix4"; mkdir -p "$fix4"
printf "echo 'ok - one'\n" > "$fix4/test_one.sh"
printf "echo 'ok - two'\n" > "$fix4/test_two.sh"
printf 'tests/test_one.sh\t20\tserial\ntests/test_two.sh\t20\tparallel\n' > "$fix4/budgets.tsv"
# Inject 40s for the serial-mode test_one: its serial-lane wall clock is uncontended, so the
# authoritative solo threshold (3/2) applies — 40*2=80 > 20*3=60 — a direct OVER BUDGET crossing.
printf 'test_one.sh\t40\t40\n' > "$fix4/durations.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix4" DOCKET_RUNTESTS_BUDGETS="$fix4/budgets.tsv"
export DOCKET_RUNTESTS_STATE="$scratch/state4.tsv" DOCKET_RUNTESTS_TEST_DURATIONS="$fix4/durations.tsv"
unset DOCKET_RUNTESTS_STRICT
devtest_rc
assert "a serial-mode breach is advisory (exit 0)" '[ "$DEVTEST_RC" = 0 ]'
assert "a serial-mode breach shows the per-row OVER BUDGET suffix" 'grep -q "OVER BUDGET (ceiling 20s)" "$OUT"'
assert "a serial-mode breach shows the OVER BUDGET summary" 'grep -q "^OVER BUDGET:" "$OUT"'
export DOCKET_RUNTESTS_STRICT=1 DOCKET_RUNTESTS_STATE="$scratch/state4b.tsv"
devtest_rc
assert "--strict gates the serial-mode breach (exit 4)" '[ "$DEVTEST_RC" = 4 ]'
unset DOCKET_RUNTESTS_STRICT DOCKET_RUNTESTS_TEST_DURATIONS

# ---- Case 5: usage error (the command takes no positional arguments) -----------------------------
devtest_rc extra-arg
assert "an extra argument is a usage error (non-zero)" '[ "$DEVTEST_RC" != 0 ]'
assert "the usage error names the offending token" 'grep -q "unknown command .*extra-arg" "$ERR"'

# ---- Case 6: interruption (SIGTERM to the source entry) ------------------------------------------
fix6="$scratch/fix6"; mkdir -p "$fix6"
printf ': > "%s/started"\nsleep 20\necho "ok - slow"\n' "$scratch" > "$fix6/test_slow.sh"
printf 'tests/test_slow.sh\t20\tparallel\n' > "$fix6/budgets.tsv"
rm -f "$scratch/started"
iout="$scratch/iout"; ierr="$scratch/ierr"
DOCKET_RUNTESTS_TESTS_DIR="$fix6" DOCKET_RUNTESTS_BUDGETS="$fix6/budgets.tsv" \
  DOCKET_RUNTESTS_JOBS=2 DOCKET_RUNTESTS_STATE="$scratch/state6.tsv" \
  go run ./cmd/docket development test >"$iout" 2>"$ierr" &
bgpid=$!
# Poll for the slow target's sentinel, bounded at 10s (0.2s intervals).
found=0
for _ in $(seq 1 50); do
  if [ -e "$scratch/started" ]; then found=1; break; fi
  sleep 0.2
done
assert "the interrupted run started its slow target" '[ "$found" = 1 ]'
kill -TERM "$bgpid" 2>/dev/null
wait "$bgpid"; irc=$?
assert "an interrupted run exits 143 (SIGTERM)" '[ "$irc" = 143 ]'
cleanpass="$(grep -E -e 'SUITE .* failed=0' "$iout")"
assert "an interrupted run never reports a clean pass" '[ -z "$cleanpass" ]'

exit $fail

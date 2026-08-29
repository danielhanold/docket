#!/usr/bin/env bash
# tests/test_devtest_differential.sh — the Bash-vs-Go differential parity harness (change 0318,
# AC 18). It drives the FROZEN oracle scripts/run-tests.sh and the Go-native source entry
# `go run ./cmd/docket development test` over the SAME synthetic fixture suites, through the shared
# env seams, and asserts that what a caller keys on is identical: target identity, row order, exit
# codes, ok/notok counts, failure-category lines, and budget-clause KINDS. It is the parity evidence
# that lets the cutover (Task 9) repoint finalize.test_command at the Go entry while the Bash runner
# stays as the byte-exact oracle.
#
# WHAT IS COMPARED — NORMALIZED observations, not raw bytes. A raw diff would flag machine noise the
# contract does not care about, and two REAL divergences the plan documents as intended: the fixture
# suite's own absolute temp path (BSD `find` preserves a `TMPDIR` trailing slash — `T//fix/...` —
# while Go's filepath cleans it to `T/fix/...`), and wall-clock seconds. So each captured report is
# reduced by normalize() to its parity skeleton: every `/…/test_X.sh` path collapses to its basename
# (the IDENTITY the join key is built on), every `<N>s` wall-clock number becomes `Ns`, and every
# `-jN` becomes `-jJ`. KEPT, deliberately: target identity, row ORDER, rc values, ok/notok counts,
# the `FAILED:` / `NO RESULT:` / `OVER BUDGET:` category lines, and the budget-clause KINDS
# (`BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMATION DUE:` / `SERIAL CONFIRMED OVER
# BUDGET:`) with their streak/recheck counters — the observations a merge gate and a human read.
#
# EXIT CODES THROUGH `go run` (same reasoning as tests/test_devtest_runner.sh's header). `go run`
# collapses ANY non-zero program exit to its OWN exit 1 and prints `exit status N` to stderr, so the
# EFFECTIVE Go program exit is derived from that line — never read straight from `go run`'s status,
# which would make exit 4 and exit 1 indistinguishable. The Bash oracle exits with its code directly.
#
# The oracle side uses FLAGS for what the Go side takes as env (`-j`/`--budgets`/`--budget-state`
# vs DOCKET_RUNTESTS_JOBS/_BUDGETS/_STATE); both read the SHARED seams DOCKET_RUNTESTS_TESTS_DIR and
# DOCKET_RUNTESTS_TEST_DURATIONS from the environment, so one setup drives both. Interruption parity
# is deliberately OUT of this file (machine-timing flake under suite contention): it is covered by
# internal/suiterunner's TestInterruptedRunCannotPass plus the documented deviation that the Go
# runner preserves an interrupted run's report while the oracle discards it.
#
# The assert helper is the tree's canonical one byte for byte (scripts/check-test-source-hygiene.sh
# rule (a) is a byte-exact allowlist), and every fixture that writes an `ok - ` / `NOT OK - ` marker
# does so at TOP LEVEL, never inside a function body — a helper body carrying either marker (or a
# literal `eval "$2"`) reads as an assert-family definition and trips the DEFN-DRIFT class.
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES note there):
# scripts/run-tests.sh gives every job a private HOME, so with GOMODCACHE/GOCACHE unset `go run`
# recompiles cold on every suite run. This pins both to <git common dir>/docket-go-cache/{mod,build}
# — shared across worktrees, concurrent-safe, outside every working tree — whenever the caller has
# not already chosen one, with -modcacherw required. Only the first run after a fresh clone is cold.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the differential harness cannot be exercised without a Go toolchain\n'
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

scratch="$(mktemp -d "${TMPDIR:-/tmp}/devtest_diff.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Both runners are driven at the same job count: the oracle via -j, the Go entry via the env seam.
JOBS=2

# normalize <report-file>: reduce a captured runner report to its parity skeleton (see the header).
# Order matters — collapse paths to basenames FIRST (so the seconds pass never sees a path), then
# the wall-clock numbers, then the job count. Plain sed over a FILE (no pipeline), so there is no
# producer|early-exiting-consumer hazard under pipefail.
normalize(){
  sed -e 's#/[^ ]*/\(test_[A-Za-z0-9_]*\.sh\)#\1#g' \
      -e 's/[0-9][0-9]*s/Ns/g' \
      -e 's/-j[0-9][0-9]*/-jJ/g' "$1"
}

# bash_run <stdout> <stderr> <budget-state> [extra run-tests.sh args...]: drive the frozen oracle.
# It reads DOCKET_RUNTESTS_TESTS_DIR / _TEST_DURATIONS from the environment; the caller passes
# --budgets and any positional targets as extra args. Sets BASH_RC to the process exit.
BASH_RC=0
bash_run(){
  local out="$1" err="$2" state="$3"; shift 3
  bash scripts/run-tests.sh -j "$JOBS" --budget-state "$state" "$@" >"$out" 2>"$err"
  BASH_RC=$?
}

# go_run <stdout> <stderr> <budget-state> <budgets>: drive the source entry over the same seams.
# Sets GO_RC to the EFFECTIVE program exit derived from `go run`'s `exit status N` line (see the
# header) so exit 4 stays distinct from exit 1.
GO_RC=0
go_run(){
  local out="$1" err="$2" state="$3" budgets="$4"
  DOCKET_RUNTESTS_BUDGETS="$budgets" DOCKET_RUNTESTS_JOBS="$JOBS" DOCKET_RUNTESTS_STATE="$state" \
    go run ./cmd/docket development test >"$out" 2>"$err"
  local grc=$?
  if [ "$grc" -eq 0 ]; then GO_RC=0; return; fi
  local line
  line="$(grep -E '^exit status [0-9]+$' "$err")"
  if [ -n "$line" ]; then GO_RC="${line##* }"; else GO_RC="$grc"; fi
}

# ---- Scenario 1: discovery + stable order ------------------------------------------------------
# Four files named to exercise C collation (uppercase sorts before lowercase). Both runners must
# discover the same set and render rows in the same basename-sorted order, independent of completion
# order. This is the harness's mutation anchor: flipping the Go aggregate to completion order (or
# any non-basename order) reddens the normalized-equality assertion here.
fix1="$scratch/fix1"; mkdir -p "$fix1"
printf "echo 'ok - z'\n" > "$fix1/test_Z.sh"
printf "echo 'ok - a'\n" > "$fix1/test_a.sh"
printf "echo 'ok - b'\n" > "$fix1/test_b.sh"
printf "echo 'ok - c'\n" > "$fix1/test_c.sh"
printf 'tests/test_Z.sh\t20\tparallel\ntests/test_a.sh\t20\tparallel\ntests/test_b.sh\t20\tparallel\ntests/test_c.sh\t20\tparallel\n' > "$fix1/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix1"
unset DOCKET_RUNTESTS_TEST_DURATIONS
bash_run "$scratch/b1.out" "$scratch/b1.err" "$scratch/s1b.tsv" --budgets "$fix1/budgets.tsv"
go_run   "$scratch/g1.out" "$scratch/g1.err" "$scratch/s1g.tsv" "$fix1/budgets.tsv"
n1b="$(normalize "$scratch/b1.out")"; n1g="$(normalize "$scratch/g1.out")"
assert "s1: both runners exit 0 on a green suite" '[ "$BASH_RC" = 0 ] && [ "$GO_RC" = 0 ]'
assert "s1: discovery and row order are identical (C collation, basename-sorted)" '[ "$n1b" = "$n1g" ]'
assert "s1: the SUITE tallies agree (files=4 passed=4)" 'grep -q "SUITE files=4 passed=4 failed=0" "$scratch/b1.out" && grep -q "SUITE files=4 passed=4 failed=0" "$scratch/g1.out"'

# ---- Scenario 2: success and ordinary failure --------------------------------------------------
# One passing file, one failing. Both runners exit 1, name the same file in FAILED:, and — since the
# oracle dumps the failing file's log inline — render byte-identical reports once normalized.
fix2="$scratch/fix2"; mkdir -p "$fix2"
printf "echo 'ok - one'\n" > "$fix2/test_one.sh"
printf "echo 'NOT OK - broke'\nexit 1\n" > "$fix2/test_bad.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_bad.sh\t20\tparallel\n' > "$fix2/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix2"
unset DOCKET_RUNTESTS_TEST_DURATIONS
bash_run "$scratch/b2.out" "$scratch/b2.err" "$scratch/s2b.tsv" --budgets "$fix2/budgets.tsv"
go_run   "$scratch/g2.out" "$scratch/g2.err" "$scratch/s2g.tsv" "$fix2/budgets.tsv"
n2b="$(normalize "$scratch/b2.out")"; n2g="$(normalize "$scratch/g2.out")"
assert "s2: both runners exit 1 on a red suite" '[ "$BASH_RC" = 1 ] && [ "$GO_RC" = 1 ]'
assert "s2: the normalized reports (including the inline log dump) are identical" '[ "$n2b" = "$n2g" ]'
assert "s2: both name the failing file in FAILED:" 'grep -q "^FAILED:.*test_bad" "$scratch/b2.out" && grep -q "^FAILED:.*test_bad" "$scratch/g2.out"'

# ---- Scenario 3: launch / infrastructure failure -----------------------------------------------
# A 0-length UNREADABLE target (chmod 000). CARVE-OUT: this is the plan's "compare category, not
# wording" scenario. The two runners decline to certify the bad target by DIFFERENT routes and with
# different process codes — the oracle refuses at its source-hygiene preflight (the checker cannot
# read the file: exit 2, zero files executed), while the Go runner schedules it and the child bash
# fails to open it (exit 1). So this scenario asserts the CATEGORY both share — both exit NON-ZERO,
# and NEITHER reports a clean two-of-two pass — not exit-code or per-line equality. (A directory
# target was rejected as the fixture: Go's Discover skips os.DirEntry.IsDir() entries, so a directory
# would diverge into "Go never schedules it" rather than a shared failure.)
fix3="$scratch/fix3"; mkdir -p "$fix3"
printf "echo 'ok - good'\n" > "$fix3/test_good.sh"
: > "$fix3/test_bad.sh"; chmod 000 "$fix3/test_bad.sh"
printf 'tests/test_good.sh\t20\tparallel\ntests/test_bad.sh\t20\tparallel\n' > "$fix3/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix3"
unset DOCKET_RUNTESTS_TEST_DURATIONS
bash_run "$scratch/b3.out" "$scratch/b3.err" "$scratch/s3b.tsv" --budgets "$fix3/budgets.tsv"
go_run   "$scratch/g3.out" "$scratch/g3.err" "$scratch/s3g.tsv" "$fix3/budgets.tsv"
chmod 644 "$fix3/test_bad.sh"   # restore so the EXIT-trap rm can remove it
assert "s3: both runners exit non-zero on an unreadable target" '[ "$BASH_RC" != 0 ] && [ "$GO_RC" != 0 ]'
clean3b="$(grep -E -e '^SUITE .* passed=2 failed=0' "$scratch/b3.out")"
clean3g="$(grep -E -e '^SUITE .* passed=2 failed=0' "$scratch/g3.out")"
assert "s3: neither runner reports a clean two-of-two pass" '[ -z "$clean3b" ] && [ -z "$clean3g" ]'

# ---- Scenario 4: concurrency classification (a serial-mode file) -------------------------------
# The budget table marks one file `serial`. Both runners report it, neither overlaps it, and — since
# a non-breaching serial file just runs and reports like any other — the basename-sorted reports are
# identical once normalized, and both exit 0.
fix4="$scratch/fix4"; mkdir -p "$fix4"
printf "echo 'ok - one'\n" > "$fix4/test_one.sh"
printf "echo 'ok - two'\n" > "$fix4/test_two.sh"
printf 'tests/test_one.sh\t20\tserial\ntests/test_two.sh\t20\tparallel\n' > "$fix4/budgets.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix4"
unset DOCKET_RUNTESTS_TEST_DURATIONS
bash_run "$scratch/b4.out" "$scratch/b4.err" "$scratch/s4b.tsv" --budgets "$fix4/budgets.tsv"
go_run   "$scratch/g4.out" "$scratch/g4.err" "$scratch/s4g.tsv" "$fix4/budgets.tsv"
n4b="$(normalize "$scratch/b4.out")"; n4g="$(normalize "$scratch/g4.out")"
assert "s4: both runners exit 0 with a serial-mode file in the table" '[ "$BASH_RC" = 0 ] && [ "$GO_RC" = 0 ]'
assert "s4: the normalized reports (row order included) are identical" '[ "$n4b" = "$n4g" ]'

# ---- Scenario 5: duplicate-basename rejection (Bash-behavior only) ------------------------------
# `a/test_x.sh b/test_x.sh` as literal run-tests.sh arguments: the oracle rejects the basename
# collision before any job launches (exit 2). The Go leg is unreachable here BY CONSTRUCTION — the
# command takes NO target arguments and its maxdepth-1 discovery over one directory cannot produce a
# basename collision — so this scenario is oracle-only. The Go runner's equivalent guard is covered
# in the package by TestResolveTargetsRejectsDuplicateBasenamesAndMissingFilesTogether.
mkdir -p "$scratch/dupa" "$scratch/dupb"
printf "echo 'ok - x'\n" > "$scratch/dupa/test_x.sh"
printf "echo 'ok - x'\n" > "$scratch/dupb/test_x.sh"
bash_run "$scratch/b5.out" "$scratch/b5.err" "$scratch/s5b.tsv" "$scratch/dupa/test_x.sh" "$scratch/dupb/test_x.sh"
assert "s5: the oracle rejects a duplicate basename with exit 2" '[ "$BASH_RC" = 2 ]'
assert "s5: the rejection names the duplicate-basename cause" 'grep -q "duplicate test basename" "$scratch/b5.err"'

# ---- Scenario 6: budget screening equivalence --------------------------------------------------
# Identical injected durations put one PARALLEL-mode file over the 5/2 screening threshold
# (60s parallel vs a 20s ceiling: 60*2 > 20*5). Both runners emit a BUDGET WATCH: line for the same
# target, neither labels the contended crossing OVER BUDGET, and both exit 0 (advisory).
fix6="$scratch/fix6"; mkdir -p "$fix6"
printf "echo 'ok - one'\n" > "$fix6/test_one.sh"
printf "echo 'ok - two'\n" > "$fix6/test_two.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_two.sh\t20\tparallel\n' > "$fix6/budgets.tsv"
printf 'test_one.sh\t60\t10\n' > "$fix6/dur.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix6" DOCKET_RUNTESTS_TEST_DURATIONS="$fix6/dur.tsv"
bash_run "$scratch/b6.out" "$scratch/b6.err" "$scratch/s6b.tsv" --budgets "$fix6/budgets.tsv"
go_run   "$scratch/g6.out" "$scratch/g6.err" "$scratch/s6g.tsv" "$fix6/budgets.tsv"
n6b="$(normalize "$scratch/b6.out")"; n6g="$(normalize "$scratch/g6.out")"
assert "s6: both runners exit 0 on a screening crossing (advisory)" '[ "$BASH_RC" = 0 ] && [ "$GO_RC" = 0 ]'
assert "s6: both emit a BUDGET WATCH: line for the same target" 'grep -q "^BUDGET WATCH: .*test_one.sh" "$scratch/b6.out" && grep -q "^BUDGET WATCH: .*test_one.sh" "$scratch/g6.out"'
over6b="$(grep -E -e '^OVER BUDGET' "$scratch/b6.out")"
over6g="$(grep -E -e '^OVER BUDGET' "$scratch/g6.out")"
assert "s6: neither labels the contended crossing OVER BUDGET" '[ -z "$over6b" ] && [ -z "$over6g" ]'
assert "s6: the normalized screening reports are identical" '[ "$n6b" = "$n6g" ]'

# ---- Scenario 7: serial confirmation + authoritative breach ------------------------------------
# Seed BOTH state stores to due-state by running the same qualifying overrun five times per runner
# against a stable fixture path (injected durations make each run instant: 60s parallel screens over
# 5/2, 40s solo breaches 3/2). On the fifth run the streak reaches five, the scheduler stamps the
# record due and takes its one solo confirmation THAT run, so both runners must emit
# SERIAL CONFIRMATION DUE: then SERIAL CONFIRMED OVER BUDGET: for the same target — and still exit 0,
# because an authoritative breach is advisory by default. Only the fifth run's report is compared;
# the first four only build the streak.
fix7="$scratch/fix7"; mkdir -p "$fix7"
printf "echo 'ok - one'\n" > "$fix7/test_one.sh"
printf "echo 'ok - two'\n" > "$fix7/test_two.sh"
printf 'tests/test_one.sh\t20\tparallel\ntests/test_two.sh\t20\tparallel\n' > "$fix7/budgets.tsv"
printf 'test_one.sh\t60\t40\n' > "$fix7/dur.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix7" DOCKET_RUNTESTS_TEST_DURATIONS="$fix7/dur.tsv"
s7b="$scratch/s7b.tsv"; s7g="$scratch/s7g.tsv"
for i in 1 2 3 4; do
  bash_run "$scratch/b7warm.out" "$scratch/b7warm.err" "$s7b" --budgets "$fix7/budgets.tsv"
  go_run   "$scratch/g7warm.out" "$scratch/g7warm.err" "$s7g" "$fix7/budgets.tsv"
done
bash_run "$scratch/b7.out" "$scratch/b7.err" "$s7b" --budgets "$fix7/budgets.tsv"
go_run   "$scratch/g7.out" "$scratch/g7.err" "$s7g" "$fix7/budgets.tsv"
n7b="$(normalize "$scratch/b7.out")"; n7g="$(normalize "$scratch/g7.out")"
assert "s7: both runners exit 0 on the confirmed authoritative breach (advisory)" '[ "$BASH_RC" = 0 ] && [ "$GO_RC" = 0 ]'
assert "s7: both announce the scheduled serial confirmation (DUE)" 'grep -q "^SERIAL CONFIRMATION DUE: .*test_one.sh" "$scratch/b7.out" && grep -q "^SERIAL CONFIRMATION DUE: .*test_one.sh" "$scratch/g7.out"'
assert "s7: both report the authoritative breach (SERIAL CONFIRMED OVER BUDGET)" 'grep -q "^SERIAL CONFIRMED OVER BUDGET: .*test_one.sh" "$scratch/b7.out" && grep -q "^SERIAL CONFIRMED OVER BUDGET: .*test_one.sh" "$scratch/g7.out"'
assert "s7: the normalized confirmation reports are identical" '[ "$n7b" = "$n7g" ]'

# ---- Scenario 8: tri-state interpretation ------------------------------------------------------
# The three verdict states a caller distinguishes. GREEN-WITH-ADVISORY-BREACH (exit 0 + finding):
# a serial-mode file whose uncontended lane time crosses the authoritative 3/2 threshold is labeled
# OVER BUDGET yet the run still passes. RED (exit 1): reuses scenario 2's outcome. NO-RESULT (exit 3):
# the oracle offers no seam to force a lost job in this harness (its stat dir is internal), so — per
# the plan — the exit-3 leg is asserted against the CONTRACT DOC's documented meaning rather than run.
# The Go runner's exit-3 path is exercised directly by TestRunNoResultExitsThree in the package.
fix8="$scratch/fix8"; mkdir -p "$fix8"
printf "echo 'ok - one'\n" > "$fix8/test_one.sh"
printf "echo 'ok - two'\n" > "$fix8/test_two.sh"
printf 'tests/test_one.sh\t20\tserial\ntests/test_two.sh\t20\tparallel\n' > "$fix8/budgets.tsv"
printf 'test_one.sh\t40\t40\n' > "$fix8/dur.tsv"
export DOCKET_RUNTESTS_TESTS_DIR="$fix8" DOCKET_RUNTESTS_TEST_DURATIONS="$fix8/dur.tsv"
bash_run "$scratch/b8.out" "$scratch/b8.err" "$scratch/s8b.tsv" --budgets "$fix8/budgets.tsv"
go_run   "$scratch/g8.out" "$scratch/g8.err" "$scratch/s8g.tsv" "$fix8/budgets.tsv"
n8b="$(normalize "$scratch/b8.out")"; n8g="$(normalize "$scratch/g8.out")"
assert "s8 green-advisory: both runners exit 0 with the breach as a finding" '[ "$BASH_RC" = 0 ] && [ "$GO_RC" = 0 ]'
assert "s8 green-advisory: both emit the OVER BUDGET: summary line" 'grep -q "^OVER BUDGET:" "$scratch/b8.out" && grep -q "^OVER BUDGET:" "$scratch/g8.out"'
assert "s8 green-advisory: the normalized advisory-breach reports are identical" '[ "$n8b" = "$n8g" ]'
# RED leg: scenario 2 already drove both runners to exit 1 on the same fixture.
assert "s8 red: scenario 2 confirmed both runners exit 1 on a test failure" 'grep -q "^FAILED:.*test_bad" "$scratch/b2.out" && grep -q "^FAILED:.*test_bad" "$scratch/g2.out"'
# NO-RESULT leg: exit 3 is documented in the contract (scripts/run-tests.md's exit-code table),
# paired with the NO RESULT: report clause the runner prints. Captured-then-tested, never
# producer|early-exiting-consumer under pipefail.
md_exit3="$(grep -F 'no result at all' scripts/run-tests.md)"
md_clause="$(grep -F 'NO RESULT:' scripts/run-tests.md)"
assert "s8 no-result: the contract documents exit 3 as 'no result at all'" '[ -n "$md_exit3" ]'
assert "s8 no-result: the contract pairs it with the NO RESULT: report clause" '[ -n "$md_clause" ]'

exit $fail

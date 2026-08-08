#!/usr/bin/env bash
# scripts/run-tests.sh — parallel runner for docket's OWN test suite (change 0227).
#
# The suite is 86 hermetic per-file scripts with no ordering dependencies, so serial execution buys
# nothing and costs ~10 minutes. This runs N at a time, each job in its own HOME/TMPDIR/git-config
# sandbox, buffering per-file output and reporting a deterministic aggregate.
#
# WHAT ISOLATION MEANS HERE: a job cannot see the developer's home directory, the developer's global
# git config, another job's temp files, or an interactive prompt. It is NOT a container — a test that
# writes inside the repo still writes inside the repo, which is why tests/runtime-budgets.tsv carries
# a `serial` mode for files that legitimately cannot share the tree.
#
# STDOUT vs STDERR: stdout carries the report, emitted after every job has finished and sorted by
# basename, so it is byte-stable across -j (modulo the wall clock). Stderr carries a live progress
# ticker in COMPLETION order, which is racy by construction — that is the point of a ticker, and it
# is why nothing keys on its order.
#
# Usage: run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check]
#                     [--strict-budget] [TEST ...]
#   -j N               parallel jobs (default: CPU count; -j 1 is serial)
#   --verbose          print every file's output, not only failing files'
#   --timings PATH     write <relpath>\t<seconds>\t<rc>\t<passes>\t<failures> per file
#   --budgets PATH     budget table (default: tests/runtime-budgets.tsv when present)
#   --no-budget-check  skip the budget comparison entirely — no breach is measured or reported
#   --strict-budget    make a breach FATAL (exit 4); by default a breach is reported, not fatal
#   TEST ...           test files to run (default: tests/test_*.sh)
# Exit: 0 every test file passed — including green-but-over-budget, which is reported loudly and
#       is fatal only under --strict-budget; 1 a test file failed; 3 a job produced no result at
#       all, so the run certified nothing (harness failure, not a test failure); 4 --strict-budget
#       and every test passed but a budget was exceeded; 2 usage error (including two targets that
#       share a basename) or unmet Bash floor; 130/143 interrupted by SIGINT/SIGTERM, which reaps
#       the in-flight jobs and reports what was lost instead of producing a report.
#
# Dev tooling for THIS repo's suite — deliberately NOT a docket.sh facade op, like profile-asserts.sh.
set -uo pipefail

# `wait -n` is Bash 4.3+, above docket's own 4+ runtime floor. Re-exec under the configured runtime
# when the invoking interpreter is older (macOS still ships 3.2 as /bin/bash); the sentinel keeps a
# runtime that is itself pre-4.3 from re-exec'ing forever. Mirrors the prologue in profile-asserts.sh.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ] || { [ "${BASH_VERSINFO[0]:-0}" -eq 4 ] && [ "${BASH_VERSINFO[1]:-0}" -lt 3 ]; }; then
  if [ -z "${DOCKET_RUNTESTS_REEXEC:-}" ] && [ -n "${DOCKET_BASH_PATH:-}" ] && [ -x "${DOCKET_BASH_PATH:-}" ]; then
    DOCKET_RUNTESTS_REEXEC=1 exec "$DOCKET_BASH_PATH" "$0" "$@"
  fi
  printf 'run-tests: needs GNU Bash 4.3+ (wait -n); configured runtime.bash is %s — install Bash 4.3+ and re-run docket/install.sh\n' \
    "${DOCKET_BASH_PATH:-unset}" >&2
  exit 2
fi

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_BASH="${DOCKET_BASH_PATH:-$(command -v bash)}"

# Default ceiling for a file with no budget row. Also the value tests/test_runtime_budgets.sh
# asserts no row exceeds.
DEFAULT_CEILING=60
# A wall-clock assertion on a shared developer machine must tolerate load, or it becomes a flake
# that teaches people to pass --no-budget-check. It must ALSO tolerate this runner's own
# contention: a budget row is a claim about a file's cost measured SERIALLY, but enforcement
# happens during a parallel run where every job competes. Measured inflation on the change-0227
# hardware reached 2.22x (test_render_board.sh 18s -> 40s; test_harness_defaults.sh 39s -> 86s),
# so 3/2 rejected 11 healthy files. 5/2 covers the measured worst case with margin while still
# catching the regrowth this table exists to prevent — a file that doubles its OWN serial cost
# breaches, because the ceiling it is measured against did not move.
# Breach = measured > ceiling * 5/2.
#
# AND THAT IS WHY A BREACH IS ADVISORY BY DEFAULT. This constant is calibrated to ONE machine's
# measured contention, so the comparison it drives is hardware- and load-dependent in both
# directions (change 0229 tracks settling a contention-independent basis for it). A measurement
# that shaky may inform a merge, but it must not BLOCK one — and it especially must not block one
# by exiting non-zero, because "non-zero" is the only budget vocabulary this runner's callers
# have. finalize's configured-bash-finalize block and docket-build's build gate both read any
# non-zero exit as "the suite is red" and answer it by dispatching a repair agent to root-cause
# failing tests, of which a breach has none. The breach therefore leaves by the channel every
# caller does read — the report — and turns fatal only for a caller that opted in with
# --strict-budget. What that costs is stated plainly in scripts/run-tests.md: nothing in this repo
# runs the strict path automatically today, so the guard against regrowth is a loud line in every
# run's output plus tests/test_runtime_budgets.sh's structural discipline, and closing that gap is
# change 0229's job.
SLACK_NUM=5; SLACK_DEN=2

cpu_count(){
  if command -v nproc >/dev/null 2>&1; then nproc
  elif command -v sysctl >/dev/null 2>&1; then sysctl -n hw.ncpu 2>/dev/null || echo 4
  else echo 4; fi
}

JOBS=""; VERBOSE=0; TIMINGS=""; BUDGETS=""; BUDGET_CHECK=1; BUDGET_STRICT=0; TARGETS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -j) JOBS="${2:-}"; shift 2 || exit 2 ;;
    -j*) JOBS="${1#-j}"; shift ;;
    --verbose) VERBOSE=1; shift ;;
    --timings) TIMINGS="${2:-}"; shift 2 || exit 2 ;;
    --budgets) BUDGETS="${2:-}"; shift 2 || exit 2 ;;
    --no-budget-check) BUDGET_CHECK=0; shift ;;
    --strict-budget) BUDGET_STRICT=1; shift ;;
    # Range ends at the blank comment line AFTER the Exit block, not at `# Exit:` itself — Exit
    # wraps onto a continuation line, and a range ending on its first line silently drops the rest.
    -h|--help) sed -n '/^# Usage:/,/^#$/p' "${BASH_SOURCE[0]}" | sed -e '/^# *$/d' -e 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; TARGETS+=("$@"); break ;;
    -*) printf 'run-tests: unknown option: %s\n' "$1" >&2; exit 2 ;;
    *) TARGETS+=("$1"); shift ;;
  esac
done
JOBS="${JOBS:-$(cpu_count)}"
case "$JOBS" in ''|*[!0-9]*|0) printf 'run-tests: -j needs a positive integer, got "%s"\n' "$JOBS" >&2; exit 2 ;; esac

# Contradictory, and the contradiction is the dangerous direction: --no-budget-check measures no
# breach at all, so silently letting it win would hand a caller that explicitly asked to be gated
# on budgets a guard that is disarmed and green. Refuse instead of picking a winner.
if [ "$BUDGET_CHECK" = 0 ] && [ "$BUDGET_STRICT" = 1 ]; then
  printf 'run-tests: --no-budget-check and --strict-budget contradict — one skips the comparison, the other gates on it\n' >&2
  exit 2
fi

if [ "${#TARGETS[@]}" -eq 0 ]; then
  while IFS= read -r f; do TARGETS+=("$f"); done < <(find "$REPO/tests" -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)
fi
[ "${#TARGETS[@]}" -gt 0 ] || { printf 'run-tests: no test files to run\n' >&2; exit 2; }

# A mistyped path must be a usage error, not a silent rc=127 "test failure": the two read the same
# in the summary, and only one of them is a bug in the suite.
#
# The same posture covers colliding BASENAMES, and for the same reason. Logs, stat records and
# budget rows are all keyed on the basename (see "Basename is the join key" in run-tests.md), so
# `a/test_x.sh b/test_x.sh` — or one path passed twice — launches two jobs writing the same
# $WORK/logs/<base>.log and $WORK/stat/<base> concurrently: interleaved logs, doubled assert
# counts, a double-printed report row, and a SUITE line that is quietly wrong. That is strictly
# worse than the mistyped path above, because nothing about it looks like an error. Reject it here,
# before any job launches, rather than corrupting the run.
missing=0; collide=0
declare -A FIRST_PATH_OF=()
for t in "${TARGETS[@]}"; do
  [ -f "$t" ] || { printf 'run-tests: no such test file: %s\n' "$t" >&2; missing=1; continue; }
  tbase="${t##*/}"
  if [ -n "${FIRST_PATH_OF[$tbase]:-}" ]; then
    printf 'run-tests: duplicate test basename: %s (%s and %s) — logs, stat records and budget rows are keyed on basename, so both jobs would write the same files\n' \
      "$tbase" "${FIRST_PATH_OF[$tbase]}" "$t" >&2
    collide=1
  else
    FIRST_PATH_OF[$tbase]="$t"
  fi
done
{ [ "$missing" = 0 ] && [ "$collide" = 0 ]; } || exit 2

[ -n "$BUDGETS" ] || { [ -f "$REPO/tests/runtime-budgets.tsv" ] && BUDGETS="$REPO/tests/runtime-budgets.tsv"; }

if [ -n "$TIMINGS" ] && ! : > "$TIMINGS" 2>/dev/null; then
  printf 'run-tests: --timings path is not writable: %s\n' "$TIMINGS" >&2; exit 2
fi

# ---- budget table -----------------------------------------------------------------------------
# Keyed by BASENAME so a table row written repo-relative matches a target given as an absolute path.
declare -A CEILING=() MODE=()
if [ -n "$BUDGETS" ] && [ -f "$BUDGETS" ]; then
  # `|| [ -n "$bfile" ]` so a final row with no trailing newline is still read.
  while IFS=$'\t' read -r bfile bsec bmode || [ -n "${bfile:-}" ]; do
    case "$bfile" in ''|'#'*) continue ;; esac
    # A malformed seconds field falls back to the default ceiling rather than crashing the run in
    # $(( )). tests/test_runtime_budgets.sh is what makes a malformed row loud; the runner's job is
    # to keep running the tests.
    case "${bsec:-}" in ''|*[!0-9]*) bsec="$DEFAULT_CEILING" ;; esac
    CEILING["${bfile##*/}"]="$bsec"
    case "${bmode:-}" in serial) MODE["${bfile##*/}"]=serial ;; *) MODE["${bfile##*/}"]=parallel ;; esac
  done < "$BUDGETS"
fi

ceiling_of(){ local k="${1##*/}"; printf '%s' "${CEILING[$k]:-$DEFAULT_CEILING}"; }
mode_of(){    local k="${1##*/}"; printf '%s' "${MODE[$k]:-parallel}"; }

# ---- ordering: longest budget first, so the tail starts immediately ----------------------------
PAR=(); SER=()
for t in "${TARGETS[@]}"; do
  if [ "$(mode_of "$t")" = serial ]; then SER+=("$t"); else PAR+=("$t"); fi
done
if [ "${#PAR[@]}" -gt 1 ]; then
  mapfile -t PAR < <(
    for t in "${PAR[@]}"; do printf '%s\t%s\n' "$(ceiling_of "$t")" "$t"; done |
      LC_ALL=C sort -t$'\t' -k1,1nr -k2,2 | cut -f2-
  )
fi

# Started here rather than at the launch loop so the interrupt handler below can always report an
# elapsed time under `set -u`; the difference is a mkdir and a trap install.
SUITE_START=$(date +%s)
WORK="$(mktemp -d "${TMPDIR:-/tmp}/run-tests.XXXXXX")"; trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/logs" "$WORK/stat" "$WORK/jobs"

# ---- interruption ------------------------------------------------------------------------------
# Without this, Ctrl-C is a data-destroying operation. Output is buffered until every job finishes,
# so an interrupted run has printed nothing but the stderr ticker; the EXIT trap then deletes $WORK
# out from under jobs that are still running and still writing into it. Worse, the jobs SURVIVE:
# this runner has no job control, so bash sets SIGINT to ignored in every async child, and the test
# processes those children forked inherit that — the terminal's Ctrl-C reaches the runner and
# nothing else. The result is orphaned test processes writing into a deleted directory and no
# report at all, on a run whose likeliest interactive ending is exactly Ctrl-C.
#
# NOT `kill 0`. A group-wide kill is only safe when the runner is a process-group leader, which it
# is solely when an interactive shell's job control made it one. Invoked the way this script
# actually gets invoked in anger — from another script, from finalize's `eval`, from a test file —
# job control is off and the runner shares its caller's process group, so `kill 0` would take the
# caller and its siblings down with it. It would also re-enter this handler by signalling the
# runner itself. So signal exactly what we launched, by pid, and nothing else.
#
# Order matters: reap first, THEN remove $WORK. Removing it concurrently is the same race the EXIT
# trap already loses.
on_signal(){  # on_signal <signame> <exit-code>
  trap - INT TERM  # a second Ctrl-C must not re-enter this while it is reaping
  local pf sp tp
  # Each in-flight job publishes "<subshell-pid> <test-pid>" and unlinks it on the way out, so this
  # glob is the set of jobs still running. Test process first, then the subshell waiting on it.
  for pf in "$WORK"/jobs/*/pid; do
    [ -f "$pf" ] || continue
    while read -r sp tp; do kill -TERM "$tp" "$sp" 2>/dev/null; done < "$pf"
  done
  wait 2>/dev/null
  local finished; finished="$(find "$WORK/stat" -type f 2>/dev/null | wc -l | tr -d ' ')"
  # Say what was lost. A run that dies silently after two minutes teaches the reader nothing about
  # whether it was nearly done or had barely started.
  printf 'run-tests: interrupted (%s) after %ss — %s of %s test files had finished; their output is discarded and no report is produced.\n' \
    "$1" "$(( $(date +%s) - SUITE_START ))" "$finished" "${#TARGETS[@]}" >&2
  rm -rf "$WORK"
  exit "$2"
}
trap 'on_signal INT 130' INT
trap 'on_signal TERM 143' TERM

launch(){  # launch <test-path>
  local t="$1" base; base="${t##*/}"; base="${base%.sh}"
  (
    jobdir="$WORK/jobs/$base"
    mkdir -p "$jobdir/home/.config" "$jobdir/tmp"
    export HOME="$jobdir/home"
    export TMPDIR="$jobdir/tmp"
    export XDG_CONFIG_HOME="$jobdir/home/.config"
    export GIT_CONFIG_GLOBAL="$jobdir/home/.gitconfig"
    export GIT_CONFIG_SYSTEM="$jobdir/home/.gitconfig-system"
    : > "$GIT_CONFIG_SYSTEM"
    # A synthetic identity, not an absent one: a test that commits must still be able to. Written
    # directly rather than through `git config` — three fewer forks per job, and the file lands at
    # $HOME/.gitconfig so a pre-2.32 git that ignores GIT_CONFIG_GLOBAL still reads exactly this.
    printf '[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n' \
      > "$GIT_CONFIG_GLOBAL"
    # Nothing may block on a human: a hung prompt in a background job is invisible.
    export GIT_TERMINAL_PROMPT=0 GIT_ASKPASS=true GIT_EDITOR=true EDITOR=true VISUAL=true
    export GIT_PAGER=cat PAGER=cat GIT_MERGE_AUTOEDIT=no
    start=$(date +%s)
    # Backgrounded and waited on, rather than run in the foreground, for one reason: it is the only
    # way this subshell can learn the test process's pid. The interrupt handler needs BOTH — killing
    # the subshell alone would leave the test itself orphaned, which is half of what the handler
    # exists to prevent. `wait` yields the test's own exit status, so rc is unchanged.
    "$TEST_BASH" "$t" > "$WORK/logs/$base.log" 2>&1 &
    tpid=$!
    printf '%s %s\n' "$BASHPID" "$tpid" > "$jobdir/pid"
    wait "$tpid"
    rc=$?
    # Unlink on the way out so the handler's glob is the set of jobs still IN FLIGHT — a finished
    # job's pid is reusable, and signalling a recycled pid is a worse bug than the one being fixed.
    rm -f "$jobdir/pid"
    end=$(date +%s)
    # NOT `grep -c ... || echo 0`: grep -c PRINTS 0 and EXITS 1 on no match, so the `||` branch
    # appends a second 0 and the field becomes a two-line value that corrupts the stat record.
    p="$(grep -cE '^ok[[:space:]]*-' "$WORK/logs/$base.log" 2>/dev/null)"; p="${p:-0}"
    f="$(grep -cE '^NOT OK' "$WORK/logs/$base.log" 2>/dev/null)"; f="${f:-0}"
    printf '%s\t%s\t%s\t%s\n' "$rc" "$((end - start))" "$p" "$f" > "$WORK/stat/$base"
    printf '  %-52s %s\n' "$base" "$([ "$rc" = 0 ] && echo PASS || echo FAIL)" >&2
  ) &
}

running=0
for t in ${PAR[@]+"${PAR[@]}"}; do
  # Hold at most $JOBS in flight. `wait -n` returns as soon as ONE job finishes, so the slot frees
  # on completion rather than on a batch boundary. It returns 127 with no children, which cannot
  # spin: `running` is decremented every iteration, so the loop drains either way.
  while [ "$running" -ge "$JOBS" ]; do wait -n 2>/dev/null; running=$((running - 1)); done
  launch "$t"; running=$((running + 1))
done
wait
for t in ${SER[@]+"${SER[@]}"}; do launch "$t"; wait; done
SUITE_WALL=$(( $(date +%s) - SUITE_START ))

# ---- report: deterministic, sorted by basename, independent of completion order ----------------
files=0; passed=0; failed=0; asserts=0; overbudget=0; noresult=0
failed_names=""; over_names=""; noresult_names=""

mapfile -t ORDERED < <(
  for t in "${TARGETS[@]}"; do printf '%s\t%s\n' "${t##*/}" "$t"; done |
    LC_ALL=C sort -t$'\t' -k1,1 -k2,2 | cut -f2-
)

for t in "${ORDERED[@]}"; do
  base="${t##*/}"; base="${base%.sh}"
  # No stat record means the job's subshell died between launch and its write — an OOM kill under
  # -j, a full disk, an external signal. `files` counts records that EXIST, so skipping quietly
  # would drop the file from the report and still exit 0, certifying a suite that ran fewer files
  # than it was asked to. Name it here and answer it below; the run is not trustworthy.
  if [ ! -f "$WORK/stat/$base" ]; then
    noresult=$((noresult + 1)); noresult_names="$noresult_names $base"
    printf '%-52s %4ss  rc=%-3s ok=%-5s notok=%-4s  NO RESULT (the job died before writing one)\n' \
      "$base" "?" "?" "?" "?"
    continue
  fi
  IFS=$'\t' read -r rc secs p f < "$WORK/stat/$base"
  files=$((files + 1)); asserts=$((asserts + p + f))
  ceil="$(ceiling_of "$t")"
  over=0
  if [ "$BUDGET_CHECK" = 1 ] && [ $((secs * SLACK_DEN)) -gt $((ceil * SLACK_NUM)) ]; then
    over=1; overbudget=$((overbudget + 1)); over_names="$over_names $base"
  fi
  if [ "$rc" = 0 ]; then passed=$((passed + 1)); else failed=$((failed + 1)); failed_names="$failed_names $base"; fi
  printf '%-52s %4ss  rc=%s  ok=%-5s notok=%-4s%s\n' "$base" "$secs" "$rc" "$p" "$f" \
    "$([ "$over" = 1 ] && printf '  OVER BUDGET (ceiling %ss)' "$ceil")"
  if [ "$VERBOSE" = 1 ] || [ "$rc" != 0 ]; then
    printf -- '---- %s ----\n' "$base"; cat "$WORK/logs/$base.log"; printf -- '---- end %s ----\n' "$base"
  fi
  [ -n "$TIMINGS" ] && printf '%s\t%s\t%s\t%s\t%s\n' "$t" "$secs" "$rc" "$p" "$f" >> "$TIMINGS"
done

printf 'SUITE files=%s passed=%s failed=%s asserts=%s wall=%ss\n' "$files" "$passed" "$failed" "$asserts" "$SUITE_WALL"
[ -n "$failed_names" ] && printf 'FAILED:%s\n' "$failed_names"
if [ "$noresult" -gt 0 ]; then
  # Named, not counted: a bare count sends the reader hunting through 80-odd basenames for the one
  # that is absent from a report that is sorted but no longer complete.
  printf 'NO RESULT:%s\n' "$noresult_names"
  printf '%s of %s test files produced no result — those jobs died before recording one (OOM kill under -j, a full disk, an external signal).\n' \
    "$noresult" "${#TARGETS[@]}"
  printf 'This run certified nothing about them: re-run, and if it recurs lower -j.\n'
fi
if [ -n "$over_names" ]; then
  printf 'OVER BUDGET:%s\n' "$over_names"
  # The remedy leads with the substantive fix. It must NOT suggest raising the ceiling — a budget
  # guard whose remedy is "raise the number" teaches the evasion it exists to catch.
  printf 'Remedy: shard this file or extend an existing shard so each part stays under its ceiling.\n'
  # Say the posture out loud in the same breath, both ways round. A reader who sees a breach and an
  # exit of 0 must not conclude the check is broken, and a caller that wants to be gated on this
  # must be told the flag exists here, where the evidence for using it is on screen.
  #
  # The red branch comes FIRST because the other two would otherwise state something false: a
  # breach is reported even when the run is red, and "the tests all passed" is exactly the sentence
  # a reader of a failing run must not be handed.
  if [ "$failed" -gt 0 ]; then
    printf 'Note: this run already fails on test failures (exit 1). The breach above is a separate finding.\n'
  elif [ "$noresult" -gt 0 ]; then
    printf 'Note: this run already fails on missing results (exit 3). The breach above is a separate finding.\n'
  elif [ "$BUDGET_STRICT" = 1 ]; then
    printf 'Strict: --strict-budget was given, so this breach fails the run (exit 4). The tests themselves passed.\n'
  else
    printf 'Advisory: the tests all passed, so this run does not fail on the breach (exit 0).\n'
    printf 'Pass --strict-budget to gate on it — but see scripts/run-tests.md first: the slack factor is calibrated to one machine (change 0229).\n'
  fi
fi

[ "$failed" -gt 0 ] && exit 1
# 3, not 1: no test file failed here, so a caller that answers 1 by dispatching a repair agent to
# root-cause failing tests would send it hunting for something that is not in any log. It is still
# non-zero, which is the only part every caller reads — a run missing a file's verdict must never
# certify the suite. It ranks BELOW a real failure: when a run is both red and incomplete, exit 1
# is the more actionable signal and the NO RESULT block above is printed either way.
[ "$noresult" -gt 0 ] && exit 3
[ "$overbudget" -gt 0 ] && [ "$BUDGET_STRICT" = 1 ] && exit 4
exit 0

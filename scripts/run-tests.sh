#!/usr/bin/env bash
# scripts/run-tests.sh — parallel runner for docket's OWN test suite (change 0227).
#
# The suite is 79 hermetic per-file scripts with no ordering dependencies, so serial execution buys
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
# Usage: run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check] [TEST ...]
#   -j N               parallel jobs (default: CPU count; -j 1 is serial)
#   --verbose          print every file's output, not only failing files'
#   --timings PATH     write <relpath>\t<seconds>\t<rc>\t<passes>\t<failures> per file
#   --budgets PATH     budget table (default: tests/runtime-budgets.tsv when present)
#   --no-budget-check  run the tests, report the times, never fail on a breach
#   TEST ...           test files to run (default: tests/test_*.sh)
# Exit: 0 green and in budget; 1 a test file failed; 4 all green but a budget was exceeded;
#       2 usage error or unmet Bash floor.
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
# that teaches people to pass --no-budget-check. Breach = measured > ceiling * 3/2.
SLACK_NUM=3; SLACK_DEN=2

cpu_count(){
  if command -v nproc >/dev/null 2>&1; then nproc
  elif command -v sysctl >/dev/null 2>&1; then sysctl -n hw.ncpu 2>/dev/null || echo 4
  else echo 4; fi
}

JOBS=""; VERBOSE=0; TIMINGS=""; BUDGETS=""; BUDGET_CHECK=1; TARGETS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -j) JOBS="${2:-}"; shift 2 || exit 2 ;;
    -j*) JOBS="${1#-j}"; shift ;;
    --verbose) VERBOSE=1; shift ;;
    --timings) TIMINGS="${2:-}"; shift 2 || exit 2 ;;
    --budgets) BUDGETS="${2:-}"; shift 2 || exit 2 ;;
    --no-budget-check) BUDGET_CHECK=0; shift ;;
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

if [ "${#TARGETS[@]}" -eq 0 ]; then
  while IFS= read -r f; do TARGETS+=("$f"); done < <(find "$REPO/tests" -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)
fi
[ "${#TARGETS[@]}" -gt 0 ] || { printf 'run-tests: no test files to run\n' >&2; exit 2; }

# A mistyped path must be a usage error, not a silent rc=127 "test failure": the two read the same
# in the summary, and only one of them is a bug in the suite.
missing=0
for t in "${TARGETS[@]}"; do
  [ -f "$t" ] || { printf 'run-tests: no such test file: %s\n' "$t" >&2; missing=1; }
done
[ "$missing" = 0 ] || exit 2

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

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/logs" "$WORK/stat" "$WORK/jobs"

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
    "$TEST_BASH" "$t" > "$WORK/logs/$base.log" 2>&1
    rc=$?
    end=$(date +%s)
    # NOT `grep -c ... || echo 0`: grep -c PRINTS 0 and EXITS 1 on no match, so the `||` branch
    # appends a second 0 and the field becomes a two-line value that corrupts the stat record.
    p="$(grep -cE '^ok[[:space:]]*-' "$WORK/logs/$base.log" 2>/dev/null)"; p="${p:-0}"
    f="$(grep -cE '^NOT OK' "$WORK/logs/$base.log" 2>/dev/null)"; f="${f:-0}"
    printf '%s\t%s\t%s\t%s\n' "$rc" "$((end - start))" "$p" "$f" > "$WORK/stat/$base"
    printf '  %-52s %s\n' "$base" "$([ "$rc" = 0 ] && echo PASS || echo FAIL)" >&2
  ) &
}

SUITE_START=$(date +%s)
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
files=0; passed=0; failed=0; asserts=0; overbudget=0
failed_names=""; over_names=""

mapfile -t ORDERED < <(
  for t in "${TARGETS[@]}"; do printf '%s\t%s\n' "${t##*/}" "$t"; done |
    LC_ALL=C sort -t$'\t' -k1,1 -k2,2 | cut -f2-
)

for t in "${ORDERED[@]}"; do
  base="${t##*/}"; base="${base%.sh}"
  [ -f "$WORK/stat/$base" ] || continue
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
if [ -n "$over_names" ]; then
  printf 'OVER BUDGET:%s\n' "$over_names"
  # The remedy leads with the substantive fix. It must NOT suggest raising the ceiling — a budget
  # guard whose remedy is "raise the number" teaches the evasion it exists to catch.
  printf 'Remedy: shard this file or extend an existing shard so each part stays under its ceiling.\n'
fi

[ "$failed" -gt 0 ] && exit 1
[ "$overbudget" -gt 0 ] && exit 4
exit 0

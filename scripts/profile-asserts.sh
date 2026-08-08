#!/usr/bin/env bash
# scripts/profile-asserts.sh — per-ASSERTION clock times across docket's own test suite.
#
# Every file in tests/ prints exactly one line per assertion: the `assert` helper most of them
# define and the `ok`/`no`/`nok` trio the rest use all emit `ok - <name>` or `NOT OK - <name>`.
# That output IS an assertion protocol, so this script runs each test under the configured
# runtime.bash and timestamps those lines as they arrive. No test file is edited and nothing is
# injected into the test's shell, so what it measures is what a plain run does.
#
# WHAT ONE NUMBER MEANS: an assertion's time is its SEGMENT — everything since the previous
# assertion line, i.e. the fixture setup plus the assertion itself. That is the unit that locates
# an expensive region, but it does mean a cheap assert standing after slow setup carries that
# setup's cost. For per-COMMAND attribution inside a segment, use profile-one-test.sh.
#
# Timing rests on Bash flushing builtin output per command, so a line reaches the reader when the
# test prints it rather than when a block buffer fills. Verified on this suite; it is why the
# reader can be a plain `read` loop and not a pty.
#
# Usage: profile-asserts.sh [--top N] [--tsv PATH] [--verbose] [TEST ...]
#   --top N      rows in the slowest-segments table (default 25)
#   --tsv PATH   keep the per-assertion records at PATH (default: a temp file, path printed)
#   --verbose    stream each test's own output through as it runs
#   TEST ...     test files to profile, repo-relative or absolute (default: tests/test_*.sh)
# Exit: 0 when every profiled test exited 0, 1 when any failed, 2 on a usage error.
#
# Dev tooling for THIS repo's suite — it is not part of the convention a consuming repo adopts.
set -uo pipefail

# EPOCHREALTIME is Bash 5.0+, one major above docket's own 4+ runtime floor. Re-exec under the
# configured runtime when the invoking interpreter is older (macOS still ships 3.2 as /bin/bash);
# the sentinel keeps a runtime that is itself pre-5 from re-exec'ing forever.
if [ "${BASH_VERSINFO[0]:-0}" -lt 5 ]; then
  if [ -z "${DOCKET_PROFILE_REEXEC:-}" ] && [ -n "${DOCKET_BASH_PATH:-}" ] && [ -x "${DOCKET_BASH_PATH:-}" ]; then
    DOCKET_PROFILE_REEXEC=1 exec "$DOCKET_BASH_PATH" "$0" "$@"
  fi
  printf 'profile-asserts: needs GNU Bash 5+ (EPOCHREALTIME); configured runtime.bash is %s — install Bash 5+ and re-run docket/install.sh\n' \
    "${DOCKET_BASH_PATH:-unset}" >&2
  exit 1
fi

# EPOCHREALTIME renders with the locale's decimal separator; C pins it to '.' so the microsecond
# arithmetic below (delete the separator, read the rest as an integer) stays correct.
export LC_ALL=C

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BASH_BIN="${DOCKET_BASH_PATH:-${BASH:-bash}}"

TOP=25
TSV=""
VERBOSE=0
tests=()

while [ $# -gt 0 ]; do
  case "$1" in
    --top)     [ $# -ge 2 ] || { printf 'profile-asserts: --top requires an argument\n' >&2; exit 2; }
               TOP="$2"; shift ;;
    --tsv)     [ $# -ge 2 ] || { printf 'profile-asserts: --tsv requires an argument\n' >&2; exit 2; }
               TSV="$2"; shift ;;
    --verbose) VERBOSE=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*)        printf 'profile-asserts: unknown argument: %s\n' "$1" >&2; exit 2 ;;
    *)         tests+=("$1") ;;
  esac
  shift
done
case "$TOP" in ''|*[!0-9]*) printf 'profile-asserts: --top must be a number, got %s\n' "$TOP" >&2; exit 2 ;; esac

cd "$ROOT" || exit 2
[ "${#tests[@]}" -gt 0 ] || tests=(tests/test_*.sh)

# Deliberately NOT removed on exit: the TSV is this script's artifact and its path is printed at
# the end, so a cleanup trap would delete exactly what the caller was told to go read.
tmp="$(mktemp -d "${TMPDIR:-/tmp}/profile-asserts.XXXXXX")"
: "${TSV:=$tmp/asserts.tsv}"
: > "$TSV" || { printf 'profile-asserts: cannot write --tsv path: %s\n' "$TSV" >&2; exit 2; }

# EPOCHREALTIME is "<seconds>.<microseconds>"; deleting the separator yields integer microseconds,
# which keeps every delta inside shell integer arithmetic (Bash has no floats).
now_us(){ printf '%s' "${EPOCHREALTIME/./}"; }

# stamp_stream <test> <start_us> : read one test's merged output and append
# "<dur_us>\t<status>\t<test>\t<index>\t<name>" to $TSV for each assertion line. This runs in the
# pipe's subshell, so every result travels by file — a variable assigned here dies with it.
stamp_stream(){
  local t="$1" prev="$2" idx=0 line now dur status name
  while IFS= read -r line; do
    now="$(now_us)"
    [ "$VERBOSE" -eq 1 ] && printf '%s\n' "$line"
    if [[ $line =~ ^(ok|NOT[[:space:]]OK)[[:space:]]*-[[:space:]]*(.*)$ ]]; then
      status="${BASH_REMATCH[1]}"; name="${BASH_REMATCH[2]}"
      if [ "$status" = ok ]; then status=PASS; else status=FAIL; fi
      dur=$(( now - prev )); prev="$now"; idx=$(( idx + 1 ))
      printf '%s\t%s\t%s\t%s\t%s\n' "$dur" "$status" "$t" "$idx" "$name" >> "$TSV"
    fi
  done
  # Whatever ran after the last assertion line — teardown, an EXIT trap, a final push. Sorted with
  # the segments so a test whose cost is all in cleanup cannot hide.
  now="$(now_us)"
  printf '%s\t%s\t%s\t%s\t%s\n' "$(( now - prev ))" TAIL "$t" 999999 '<after last assertion>' >> "$TSV"
}

# Validate the whole set BEFORE running anything: a typo in the last argument should not surface
# after several minutes of profiling the ones that did resolve.
for t in "${tests[@]}"; do
  [ -f "$t" ] || { printf 'profile-asserts: no such test file: %s\n' "$t" >&2; exit 2; }
done

printf 'profiling %d test file(s) under %s\n' "${#tests[@]}" "$BASH_BIN"
# The records path up front for the same reason the per-test line below exists: during a hang the
# end-of-run print never arrives. `$TSV` is already resolved by this point.
printf 'per-assertion records: %s\n' "$TSV"
suite_start="$(now_us)"
failed=0

for t in "${tests[@]}"; do
  rc_file="$tmp/rc"; : > "$rc_file"
  t_start="$(now_us)"
  # The pre-loop line says how MANY files; only this says WHICH one is executing. Without it a hung
  # test is anonymous — the per-test rollup below prints only after the file completes.
  printf 'running %s\n' "$t"
  # The rc redirect keeps the exit status out of the stream the reader is timestamping.
  { "$BASH_BIN" "$t" 2>&1; printf '%s' "$?" > "$rc_file"; } | stamp_stream "$t" "$t_start"
  t_end="$(now_us)"
  rc="$(cat "$rc_file" 2>/dev/null)"; rc="${rc:-1}"
  n="$(awk -F'\t' -v t="$t" '$3==t && $2!="TAIL"{ c++ } END{ print c+0 }' "$TSV")"
  [ "$rc" -eq 0 ] || failed=$(( failed + 1 ))
  awk -v t="$t" -v ms="$(( (t_end - t_start) / 1000 ))" -v n="$n" -v rc="$rc" \
    'BEGIN{ k=split(t,p,"/"); printf "  %-46s %8.1fs  %3d asserts  %s\n", p[k], ms/1000, n, (rc==0?"PASS":"FAIL") }'
done

suite_ms=$(( ($(now_us) - suite_start) / 1000 ))

printf '\n=== %s slowest assertion segments (setup since the previous assert, plus the assert) ===\n' "$TOP"
# `awk NR<=top` rather than `head`: an early-exiting consumer under `set -o pipefail` SIGPIPEs the
# producer and turns a clean run into an intermittent 141 (AGENTS.md, Shell).
sort -rn -t$'\t' -k1,1 "$TSV" \
  | awk -F'\t' -v top="$TOP" 'NR<=top{
      k=split($3,p,"/")
      printf "%8.1f ms  %-4s %-40s #%-4s %s\n", $1/1000, $2, p[k], ($4=="999999"?"tail":$4), $5
    }'

printf '\n=== per-test rollup (slowest first) ===\n'
awk -F'\t' '
  { total[$3]+=$1; if($2!="TAIL"){ n[$3]++; if($1>max[$3]){ max[$3]=$1; who[$3]=$5 } } }
  END{ for(t in total) printf "%9.2f\t%s\t%d\t%d\t%s\n", total[t]/1000000, t, n[t], max[t]/1000, who[t] }
' "$TSV" | sort -rn \
  | awk -F'\t' '{ k=split($2,p,"/"); printf "%8.2fs  %-42s %3d asserts   slowest %6dms  %s\n", $1, p[k], $3, $4, $5 }'

printf '\n=== failing assertions ===\n'
fails="$(awk -F'\t' '$2=="FAIL"{ k=split($3,p,"/"); printf "  %-40s #%-4s %s\n", p[k], $4, $5 }' "$TSV")"
if [ -n "$fails" ]; then printf '%s\n' "$fails"; else printf '  none\n'; fi

n_all="$(awk -F'\t' '$2!="TAIL"{ c++ } END{ print c+0 }' "$TSV")"
printf '\ntotal: %d test file(s), %d failed, %d assertions, %ds (%dm%02ds)\n' \
  "${#tests[@]}" "$failed" "$n_all" "$((suite_ms/1000))" "$((suite_ms/60000))" "$(((suite_ms/1000)%60))"
printf 'per-assertion records: %s\n' "$TSV"

[ "$failed" -eq 0 ]

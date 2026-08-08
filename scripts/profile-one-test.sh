#!/usr/bin/env bash
# scripts/profile-one-test.sh — command-level profile of ONE test in docket's suite.
#
# profile-asserts.sh names the slow assertion SEGMENT; this names the COMMAND inside it. It turns
# on xtrace through the ENVIRONMENT (SHELLOPTS plus BASH_XTRACEFD) rather than editing or sourcing
# the test, so `$0`, BASH_SOURCE, EXIT traps and the exit status all behave as in a plain run —
# which matters here, because many tests in this suite read `$0` to locate the repo root and would
# resolve it against the profiler instead if they were sourced.
#
# PS4 carries $EPOCHREALTIME, so a traced command's SELF TIME is the gap until the next trace line.
# Processes run sequentially (the parent waits on each child), so the interleaved trace is one
# sound timeline. SHELLOPTS is exported, so child Bash processes — docket's own scripts under test
# — are traced too: you get the offending line inside mint-stub.sh rather than "the whole script
# was slow".
#
# READ THE RANKING, NOT THE CLOCK. xtrace writes a line per command, so absolute times run above a
# clean run's. A test that asserts on a child's exact stderr could in principle be perturbed; the
# reported exit status is what tells you whether the run stayed honest.
#
# Usage: profile-one-test.sh [--top N] [--trace PATH] [--asserts] TEST
#   --top N       rows per table (default 30)
#   --trace PATH  keep the raw trace at PATH (default: a temp file, path printed)
#   --asserts     also print each assertion segment in run order, call to call
#   TEST          the test file to profile, repo-relative or absolute
# Exit: the profiled test's own exit status, or 2 on a usage error.
#
# Dev tooling for THIS repo's suite — it is not part of the convention a consuming repo adopts.
set -uo pipefail

# EPOCHREALTIME is Bash 5.0+, one major above docket's own 4+ runtime floor. See the matching
# prologue in profile-asserts.sh; the sentinel keeps a pre-5 runtime from re-exec'ing forever.
if [ "${BASH_VERSINFO[0]:-0}" -lt 5 ]; then
  if [ -z "${DOCKET_PROFILE_REEXEC:-}" ] && [ -n "${DOCKET_BASH_PATH:-}" ] && [ -x "${DOCKET_BASH_PATH:-}" ]; then
    DOCKET_PROFILE_REEXEC=1 exec "$DOCKET_BASH_PATH" "$0" "$@"
  fi
  printf 'profile-one-test: needs GNU Bash 5+ (EPOCHREALTIME); configured runtime.bash is %s — install Bash 5+ and re-run docket/install.sh\n' \
    "${DOCKET_BASH_PATH:-unset}" >&2
  exit 1
fi

export LC_ALL=C

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BASH_BIN="${DOCKET_BASH_PATH:-${BASH:-bash}}"

TOP=30
TRACE=""
SHOW_ASSERTS=0
TEST=""

while [ $# -gt 0 ]; do
  case "$1" in
    --top)     [ $# -ge 2 ] || { printf 'profile-one-test: --top requires an argument\n' >&2; exit 2; }
               TOP="$2"; shift ;;
    --trace)   [ $# -ge 2 ] || { printf 'profile-one-test: --trace requires an argument\n' >&2; exit 2; }
               TRACE="$2"; shift ;;
    --asserts) SHOW_ASSERTS=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*)        printf 'profile-one-test: unknown argument: %s\n' "$1" >&2; exit 2 ;;
    *)         [ -z "$TEST" ] || { printf 'profile-one-test: one test file at a time, got %s and %s\n' "$TEST" "$1" >&2; exit 2; }
               TEST="$1" ;;
  esac
  shift
done
[ -n "$TEST" ] || { printf 'profile-one-test: missing TEST (usage: profile-one-test.sh [--top N] [--trace PATH] [--asserts] TEST)\n' >&2; exit 2; }
case "$TOP" in ''|*[!0-9]*) printf 'profile-one-test: --top must be a number, got %s\n' "$TOP" >&2; exit 2 ;; esac

cd "$ROOT" || exit 2
[ -f "$TEST" ] || { printf 'profile-one-test: no such test file: %s\n' "$TEST" >&2; exit 2; }

# Not removed on exit: the trace and the captured stdout are this script's artifacts and their
# paths are printed for follow-up reading.
tmp="$(mktemp -d "${TMPDIR:-/tmp}/profile-one-test.XXXXXX")"
: "${TRACE:=$tmp/trace.log}"
out="$tmp/stdout.log"

printf 'tracing %s under %s ...\n' "$TEST" "$BASH_BIN"
# Printed BEFORE the child launches, not only in the end-of-run summary: when a test HANGS the
# summary never arrives, and the growing trace file read from another shell is the only way to see
# where it stopped. Same stream as the summary — nothing parsing this output changes shape, and a
# duplicated path line is harmless where a missing one is a dead end.
printf 'trace:  %s\nstdout: %s\n' "$TRACE" "$out"
# SHELLOPTS is read from the environment at Bash startup and applied before the script runs — the
# one way to enable xtrace in a file you are not allowed to modify. It must be set with `env` and
# not a command-prefix assignment: SHELLOPTS is READONLY in the shell running this script, so a
# prefix assignment aborts the command instead of exporting it. PS4 stays single-quoted so the
# CHILD expands it, once per traced command.
env SHELLOPTS=xtrace \
    BASH_XTRACEFD=9 \
    PS4='+ ${EPOCHREALTIME} ${BASHPID} ${BASH_SOURCE##*/}:${LINENO} ' \
  "$BASH_BIN" "$TEST" >"$out" 2>&1 9>"$TRACE"
rc=$?

n_ok="$(awk '/^ok[[:space:]]*-/{ c++ } END{ print c+0 }' "$out")"
n_no="$(awk '/^NOT OK[[:space:]]*-/{ c++ } END{ print c+0 }' "$out")"
n_tr="$(awk 'END{ print NR+0 }' "$TRACE")"
printf 'exit=%s   %s assertions passed, %s failed   (%s trace lines)\n' "$rc" "$n_ok" "$n_no" "$n_tr"

# Self time of a traced command = the gap until the next traced command starts. Timestamps are
# split on the separator and recombined as integer microseconds: exact under awk's doubles, where
# parsing the whole "<seconds>.<microseconds>" string as a float would not be.
awk '
  function us(t,   p) { p = index(t, "."); return substr(t,1,p-1) * 1000000 + substr(t,p+1) + 0 }
  /^\++ [0-9]+\.[0-9]+ / {
    ts = us($2); src = $4
    cmd = ""; for (i = 5; i <= NF; i++) cmd = cmd (i > 5 ? " " : "") $i
    if (have) printf "%d\t%s\t%s\n", ts - pts, psrc, pcmd
    pts = ts; pcmd = cmd; psrc = src; have = 1
  }
' "$TRACE" > "$tmp/self.tsv"

printf '\n=== top %s source lines by cumulative self time ===\n' "$TOP"
awk -F'\t' '{ tot[$2]+=$1; n[$2]++ } END{ for(s in tot) printf "%d\t%d\t%s\n", tot[s], n[s], s }' "$tmp/self.tsv" \
  | sort -rn \
  | awk -F'\t' -v top="$TOP" 'NR<=top{ printf "%9.1f ms  %5d call(s)  %s\n", $1/1000, $2, $3 }'

printf '\n=== top %s single command invocations ===\n' "$TOP"
sort -rn -t$'\t' -k1,1 "$tmp/self.tsv" \
  | awk -F'\t' -v top="$TOP" 'NR<=top{
      c=$3; if (length(c) > 88) c = substr(c,1,85) "..."
      printf "%9.1f ms  %-26s %s\n", $1/1000, $2, c
    }'

awk -F'\t' '{ s+=$1 } END{ printf "\ntraced wall time: %.2fs\n", s/1000000 }' "$tmp/self.tsv"

if [ "$SHOW_ASSERTS" -eq 1 ]; then
  printf '\n=== assertion segments, in run order (exact call to call) ===\n'
  awk '
    function us(t,   p) { p = index(t, "."); return substr(t,1,p-1) * 1000000 + substr(t,p+1) + 0 }
    # Depth 1 only (PS4 repeats its first character per nesting level), so the helper call is
    # timed, not the eval and echo inside it.
    /^\+ [0-9]+\.[0-9]+ / {
      cmd = ""; for (i = 5; i <= NF; i++) cmd = cmd (i > 5 ? " " : "") $i
      if (cmd !~ /^(assert|ok|no|nok) /) next
      ts = us($2)
      if (have) printf "%9.1f ms  %s\n", (ts - pts)/1000, pcmd
      pts = ts; pcmd = cmd; have = 1
    }
  ' "$TRACE"
fi

printf '\ntrace:  %s\n' "$TRACE"
printf 'stdout: %s\n' "$out"
exit "$rc"

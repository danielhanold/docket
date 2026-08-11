#!/usr/bin/env bash
# tests/lib/bounded_arg_probe.sh — the bounded run every "a valueless trailing flag must die,
# never hang" leg uses (change 0208 leg (c), extended to the adapters). SOURCED, never executed:
# it lives under tests/lib/ so the tests/test_*.sh discovery glob never picks it up, the same
# arrangement as tests/lib/runner_dispatch_detach_common.sh.
#
# WHY IT IS SHARED: `scripts/runner-dispatch.sh` and all three of `scripts/runners/*.sh` parse
# value-taking flags with an arm ending in `shift 2`, run under `set -uo pipefail` with no `-e`,
# and have no trailing shift. bash's `shift` FAILS rather than truncating at `$# = 1`, so an
# unguarded value-taking flag in FINAL position spins the `while [ $# -gt 0 ]` loop forever.
# Measured before the fix: every one of the twelve arms returned HUNG under a 3s bound. Four call
# sites needing the same subtle bound is exactly the shape that goes divergent when copied — the
# facade being fixed while its three adapter twins stayed live is what produced this leg.
#
# The bound is a background job plus a SENTINEL FILE, and every half is load-bearing:
#   * The stop must be INDEPENDENT of the guard under test, or deleting the guard deletes the stop
#     and the mutation hangs instead of reddening (LEARNINGS: mutation-target-needs-a-forced-exit).
#   * Completion is the sentinel FILE, never `kill -0` on the pid: a finished-but-unwaited child is
#     a zombie whose pid still answers `kill -0`, so a liveness poll would report HUNG for every
#     healthy run — the assert would pass for the wrong reason and go vacuous the moment it is fixed.
#   * `set -m` makes the job a process-group LEADER so the give-up path can signal the whole tree.
#     Without it the subshell dies and the spinning child is orphaned into the rest of the suite.
# `timeout(1)` is deliberately not used: stock macOS ships none and no existing test depends on one.
#
# Contract:
#   BOUNDED_DIR  (required) scratch dir the rc + stderr files are written under.
#   BOUNDED_CWD  (optional) directory the probed command runs in; defaults to BOUNDED_DIR.
#   bounded_probe_err          -> prints the stderr path, derived rather than assigned.
#   run_bounded_cmd SECS ARGV… -> prints the command's exit code, or the literal `HUNG`.
#
# The stderr path is DERIVED by `bounded_probe_err` rather than assigned inside `run_bounded_cmd`,
# because callers read the exit code through `$( )` — a SUBSHELL, whose variable assignments cannot
# reach the caller. A path set only inside the helper would still be empty at the assert, and
# `grep -qF -- "…" ""` fails for a missing operand rather than for a missing diagnostic: an assert
# that can never go green, i.e. one that stays red after the fix.

bounded_probe_err(){ printf '%s' "${BOUNDED_DIR:?BOUNDED_DIR must be set before using the bounded probe}/bounded.err"; }

run_bounded_cmd(){  # $1 = seconds to wait; $2... = the command and its arguments
  local secs="$1"; shift
  local dir="${BOUNDED_DIR:?BOUNDED_DIR must be set before using the bounded probe}"
  local cwd="${BOUNDED_CWD:-$dir}"
  local rcf="$dir/bounded.rc" errf
  errf="$(bounded_probe_err)"
  rm -f "$rcf" "$rcf.partial"; : > "$errf"
  set -m
  ( cd "$cwd" && "$@" >/dev/null 2>"$errf"
    printf '%s' "$?" > "$rcf.partial"; mv -f "$rcf.partial" "$rcf" ) &
  local p=$! i=0
  set +m
  while [ "$i" -lt $(( secs * 10 )) ] && [ ! -f "$rcf" ]; do sleep 0.1; i=$(( i + 1 )); done
  if [ ! -f "$rcf" ]; then
    kill -TERM "-$p" 2>/dev/null || kill -TERM "$p" 2>/dev/null
    wait "$p" 2>/dev/null
    printf 'HUNG'
    return 0
  fi
  wait "$p" 2>/dev/null
  cat "$rcf"
}

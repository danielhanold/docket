# tests/lib/gate_run_common.sh — the prologue every tests/test_gate_run*.sh shard sources
# (change 0282). NOT matched by the tests/test_*.sh discovery glob, so it never runs as a test —
# the same arrangement as tests/lib/runner_dispatch_detach_common.sh and tests/lib/sync_agents_common.sh.
#
# WHY THE SHARDS EXIST: `scripts/gate-run.sh` launches REAL detached children and waits on REAL
# barriers, so its cost is wall clock that cannot be mocked away. Split along the verb seam:
#   tests/test_gate_run.sh      — --launch, the records, the terminal record, and --observe.
#   tests/test_gate_run_stop.sh — --stop and its deterministic interleaving fixtures.
#
# This file is SOURCED, so BASH_SOURCE points at tests/lib/ — ROOT is TWO levels up.
#
# Hermetic: every run dir is minted beneath this file's own sandbox, and every detached group the
# sandbox recorded is reaped at EXIT. A test that launches a real `sleep` and then dies on a failing
# assert must not leave that sleep on the machine.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
# `REPO` is the name the plan's later asserts use for the same path; both are exported so a shard
# may reach for either without knowing which era it was written in.
REPO="$ROOT"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

GATE_RUN="$ROOT/scripts/gate-run.sh"
# The invoker. Runs the script under the resolved Bash 4+ runtime rather than PATH `bash`, because
# macOS's /bin/bash is 3.2 and the script uses the repo's Bash 4 floor.
gate_run(){ "$DOCKET_BASH_PATH" "$GATE_RUN" "$@"; }

SBX="$(mktemp -d "${TMPDIR:-/tmp}/gate-run-test.XXXXXX")"; SBX="$(cd "$SBX" && pwd -P)"

# BOUNDED wait for the terminal record — never an unbounded one. "The wrapper died without
# recording" is precisely the condition several asserts exist to catch, so a missing record has to
# surface as a red assert on the record's CONTENT, not as a shard that hangs until the harness's
# wall-clock budget notices. Returns non-zero on timeout and the caller reads an empty record.
await_terminal(){  # $1 = run dir, $2 = deadline in tenths of a second (default 100 = 10s)
  local rd="${1:-}" ticks="${2:-100}" t=0
  while [ "$t" -lt "$ticks" ]; do
    [ -s "$rd/terminal" ] && return 0
    sleep 0.1; t=$(( t + 1 ))
  done
  return 1
}

# BOUNDED wait for a file to have content. The launcher's handshake returns as soon as the run is
# ESTABLISHED, which is strictly before the user's command has forked, so any assert about bytes the
# command wrote has to wait for them rather than assume them. Returns non-zero on timeout and the
# caller reads an empty file.
await_nonempty(){  # $1 = file, $2 = deadline in tenths of a second (default 100 = 10s)
  local f="${1:-}" ticks="${2:-100}" t=0
  while [ "$t" -lt "$ticks" ]; do
    [ -s "$f" ] && return 0
    sleep 0.1; t=$(( t + 1 ))
  done
  return 1
}

# BOUNDED wait for a process group to disappear. A `kill -KILL` returns as soon as the signal is
# QUEUED, not once the group is reaped, and a zombie still answers `kill -0` — so a fixture that
# kills a group and immediately observes can read `running` off corpses. Every fixture that kills a
# group before observing it goes through here instead of a bare sleep.
await_group_gone(){  # $1 = pgid, $2 = deadline in tenths of a second (default 100 = 10s)
  local pg="${1:-}" ticks="${2:-100}" t=0
  case "$pg" in ''|*[!0-9]*) return 0 ;; esac
  while [ "$t" -lt "$ticks" ]; do
    kill -0 -"$pg" 2>/dev/null || return 0
    sleep 0.1; t=$(( t + 1 ))
  done
  return 1
}

# Start a live BYSTANDER process that leads its own process group, and print that pgid. This is the
# recycled-pgid fixture: a test rewrites a run's recorded `pgid` to point here, so the recorded
# group is alive but is NOT this run's — exactly what an identity cross-check must refuse and a bare
# `kill -0` cannot.
#
# `set -m` is what makes the backgrounded subshell a group LEADER; without job control it would join
# this harness's own group, and then every `kill -0 -"$pgid"` in the caller would be reading the
# harness itself — a permanently green guard and a group-directed kill aimed at the test runner. The
# checks below refuse to hand back a pgid that failed to separate, and job control is restored to
# whatever it was rather than left on.
start_foreign_group(){  # -> pgid of a live foreign group leader; empty (and non-zero) on failure
  local pid pg mine had_m=0
  case "$-" in *m*) had_m=1 ;; esac
  set -m
  ( exec sleep 300 ) </dev/null >/dev/null 2>&1 &
  pid=$!
  [ "$had_m" = 1 ] || set +m
  pg="$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ')"
  mine="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  case "$pg" in ''|*[!0-9]*) return 1 ;; esac
  [ "$pg" = "$pid" ] && [ "$pg" != "$mine" ] || { kill -KILL "$pid" 2>/dev/null; return 1; }
  printf '%s' "$pg"
}

# Tear down a leftover detached group. It REFUSES to signal this file's own group: a group-directed
# signal aimed at ourselves takes the harness running this file with it — the same hazard
# tests/lib/runner_dispatch_detach_common.sh's `reap` exists for.
reap(){  # $1 = pgid
  local pg="${1:-}" mine
  case "$pg" in ''|*[!0-9]*) return 0 ;; esac
  mine="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  [ "$pg" != "$mine" ] || return 0
  kill -KILL -"$pg" 2>/dev/null
  return 0
}

# Reap every group any run dir in the sandbox recorded, then delete the sandbox. Driven off the
# `launch` records rather than a hand-kept list, so a shard that mints a run dir gets its cleanup
# for free and cannot forget one.
gate_run_cleanup(){
  local rec pg
  while IFS= read -r rec; do
    [ -n "$rec" ] || continue
    pg="$(sed -n 's/^pgid=//p' "$rec" 2>/dev/null)"
    reap "${pg%%$'\n'*}"
  done < <(find "$SBX" -type f -name launch 2>/dev/null)
  rm -rf "$SBX"
}
trap gate_run_cleanup EXIT

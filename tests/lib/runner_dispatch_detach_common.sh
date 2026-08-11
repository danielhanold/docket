# tests/lib/runner_dispatch_detach_common.sh — the prologue every tests/test_runner_dispatch_*.sh
# detachment shard sources (change 0271 review, finding 13). NOT matched by the tests/test_*.sh
# discovery glob, so it never runs as a test — the same arrangement as tests/lib/sync_agents_common.sh.
#
# WHY THE SHARDS EXIST: the unsharded file sat at the table's hard 60s ceiling with zero headroom,
# and its cost is FIXED SLEEPS (a launcher that returns before its child finished, and a child that
# outlives the call, are only observable in wall-clock time), so it grew with every arm added. The
# table's own remedy for a file at its ceiling is a shard, never a bigger number
# (tests/runtime-budgets.tsv, "NO row may exceed 60 seconds"). Cut along the file's own seams:
#   tests/test_runner_dispatch_detach.sh     — LAUNCH: detachment, the record, the sentinel, streams,
#                                              retention, and the argument gates.
#   tests/test_runner_dispatch_observe.sh    — OBSERVE: dispositions, the relay, durability across
#                                              `git worktree remove`, the budget and its kill.
#   tests/test_runner_dispatch_build_gate.sh — the BUILD verdict family, implement-next's run gate on
#                                              the observe seam, and the posture-doc asserts.
#
# This file is SOURCED, so BASH_SOURCE points at tests/lib/ — ROOT is TWO levels up, where the
# unsharded file needed one. That is the only line that differs from the prologue it replaces.
#
# Hermetic: a FAKE adapter script stands in for every runner, so nothing here needs a child harness
# CLI. The fake sleeps for a caller-controlled duration and writes a marker, which is what makes
# "survived the group teardown" observable rather than asserted.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

FACADE="$ROOT/scripts/runner-dispatch.sh"

# Every fixture is a `mktemp -d`, and this file mints several — register each as it is created so
# an early exit (a `set -u` slip, a failing arm) cannot litter the machine.
FIXTURES=()
cleanup(){ local d; for d in "${FIXTURES[@]:-}"; do [ -n "$d" ] && rm -rf "$d"; done; }
trap cleanup EXIT

# Tear down a leftover detached group. It REFUSES to signal this file's own group: a
# group-directed signal aimed at ourselves takes the harness running this file with it, which is
# the same hazard the facade's own observe guard exists for.
reap(){  # $1 = pgid
  local pg="${1:-}" mine
  case "$pg" in ''|*[!0-9]*) return 0 ;; esac
  mine="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  [ "$pg" != "$mine" ] || return 0
  kill -KILL -"$pg" 2>/dev/null
  return 0
}

make_fixture(){  # sets SBX (repo), RDIR (fake runners dir)
  SBX="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach.XXXXXX")"; SBX="$(cd "$SBX" && pwd -P)"
  FIXTURES+=("$SBX")
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  RDIR="$SBX/fake-runners"; mkdir -p "$RDIR"
  # The fake adapter: sleeps FAKE_SLEEP, then writes a marker, then LINGERS for FAKE_TAIL before
  # exiting FAKE_RC. The tail defaults to 0, so every arm written before it existed is unchanged;
  # it is what makes "the adapter never completed its work" a real measurement rather than a
  # tautology — an adapter that only slept would have written no marker inside the observation
  # window whether it was killed or not.
  # The argv log (change 0277) records the adapter's own argument vector, ONE ARGUMENT PER LINE, so
  # an arm can assert on the shape the facade composed — which payload channel it used, and which
  # path it handed over — rather than only on what the child produced. It defaults to /dev/null, so
  # every arm and every sibling shard written before it existed is unchanged.
  cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$@" >> "${FAKE_ARGV_LOG:-/dev/null}"
sleep "${FAKE_SLEEP:-0}"
printf 'adapter-ran\n' > "$FAKE_MARKER"
printf 'fake adapter stdout\n'
printf 'fake adapter stderr\n' >&2
sleep "${FAKE_TAIL:-0}"
exit "${FAKE_RC:-0}"
FAKE
  chmod +x "$RDIR/fake.sh"
}
# Extra arguments after the agent are forwarded to the facade verbatim (change 0277), so an arm can
# launch with `--brief-file` or with a trailing `--` payload. `shift` FAILS rather than truncating
# when there is nothing to shift, so the `|| true` keeps a bare `launch` (no agent) working.
launch(){ local agent="${1:-status}"; shift 2>/dev/null || true
  ( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_ARGV_LOG="${FAKE_ARGV_LOG:-/dev/null}" \
    FAKE_SLEEP="${FAKE_SLEEP:-0}" FAKE_TAIL="${FAKE_TAIL:-0}" FAKE_RC="${FAKE_RC:-0}" \
    bash "$FACADE" --launch --runner fake --agent "$agent" "$@" ); }
# The per-dispatch dir for KEY, resolved the way an outside reader must: from the repo's git
# COMMON dir, never from the worktree.
ddir_for(){ local c; c="$(cd "$SBX" && git rev-parse --git-common-dir)"
  printf '%s/docket/dispatch/%s' "$(cd "$SBX/$c" 2>/dev/null || cd "$c"; pwd -P)" "$1"; }

# The observation helper. Shared because two shards observe: BUDGET is read per call, so an arm
# can drive the budget path by setting it around a single observation.
observe(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" \
    DELEGATION_OBSERVATION_BUDGET="${BUDGET:-60}" \
    bash "$FACADE" --observe "$1" --runner fake --agent "${2:-status}" ); }

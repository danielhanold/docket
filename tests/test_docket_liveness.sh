#!/usr/bin/env bash
# tests/test_docket_liveness.sh — scripts/lib/docket-liveness.sh, the ONE identity-checked liveness
# predicate shared by gate-run.sh and runner-dispatch.sh (change 0284).
# Run: bash tests/test_docket_liveness.sh
#
# The lib takes VALUES, never run dirs: its two consumers store their records in incompatible
# layouts and each keeps its own reader. So every case here passes a pgid and a token directly,
# which is also what makes the file hermetic — no run dir, no launcher, no git.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$ROOT/scripts/lib/docket-liveness.sh"

# --- a real self-spawned group -----------------------------------------------------
# `set -m` in a subshell makes the background job a PROCESS-GROUP LEADER, so its pid IS its pgid —
# the same construction runner-dispatch.sh's --launch uses and gate-run.sh's rung 3 falls back to.
PGIDS=()
cleanup(){ local p; for p in "${PGIDS[@]:-}"; do case "$p" in ''|*[!0-9]*) continue ;; esac
  kill -KILL -"$p" 2>/dev/null; done; }
trap cleanup EXIT

spawn_group(){  # sets SPAWN_PID (== its own pgid); the child sleeps until killed
  set -m
  sleep 300 &
  SPAWN_PID=$!
  set +m
  PGIDS+=("$SPAWN_PID")
  # Fixture sanity is asserted by the caller, not assumed here.
}

# ---- docket_identity_of ----------------------------------------------------------
spawn_group
assert "fixture sanity: the spawned job leads its own process group" \
  '[ "$(ps -o pgid= -p "$SPAWN_PID" 2>/dev/null | tr -d " ")" = "$SPAWN_PID" ]'

tok="$(docket_identity_of "$SPAWN_PID")"
assert "docket_identity_of returns a non-empty token for a live pid" '[ -n "$tok" ]'
assert "docket_identity_of is stable across calls" '[ "$(docket_identity_of "$SPAWN_PID")" = "$tok" ]'
assert "the token is whitespace-normalized (no runs, no edges)" \
  '[ "$tok" = "$(tr -s "[:space:]" " " <<<"$tok" | sed -e "s/^ //" -e "s/ $//")" ]'
# Always-0 is load-bearing: gate-run.sh runs under `set -e` and ASSIGNS from this.
docket_identity_of 999999 >/dev/null; rc=$?
assert "docket_identity_of returns 0 even for a gone pid (set -e callers assign from it)" '[ "$rc" = "0" ]'
assert "docket_identity_of is empty for a gone pid" '[ -z "$(docket_identity_of 999999)" ]'

# ---- docket_group_alive_and_ours: the happy leg ----------------------------------
DOCKET_LIVENESS_WHY="stale-sentinel"
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a live group with a MATCHING token is alive" '[ "$rc" = "0" ]'
assert "DOCKET_LIVENESS_WHY is cleared on the alive leg" '[ -z "$DOCKET_LIVENESS_WHY" ]'

# ---- the pid-reuse case: live group, MISMATCHED token ----------------------------
docket_group_alive_and_ours "$SPAWN_PID" "Thu Jan  1 00:00:00 1970"; rc=$?
assert "a live group with a MISMATCHED token is DEAD (the pid-reuse case)" '[ "$rc" != "0" ]'
assert "and the reason names the mismatch, quoting both tokens" \
  '[ -n "$DOCKET_LIVENESS_WHY" ] && grep -qF "1970" <<<"$DOCKET_LIVENESS_WHY" && grep -qF "$tok" <<<"$DOCKET_LIVENESS_WHY"'

# ---- an empty expected token fails CLOSED ----------------------------------------
docket_group_alive_and_ours "$SPAWN_PID" ""; rc=$?
assert "an EMPTY expected token is dead — nothing to compare is not agreement" '[ "$rc" != "0" ]'
assert "and it says so" '[ -n "$DOCKET_LIVENESS_WHY" ]'

# ---- the same group after it exits ------------------------------------------------
kill -KILL -"$SPAWN_PID" 2>/dev/null
waited=0
while kill -0 -"$SPAWN_PID" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "fixture sanity: the group is really gone" '! kill -0 -"$SPAWN_PID" 2>/dev/null'
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a group that has exited is dead" '[ "$rc" != "0" ]'
assert "and the reason is non-empty" '[ -n "$DOCKET_LIVENESS_WHY" ]'

# ---- the syntactic floor: NOTHING is probed, so no kill is issued ----------------
# `kill -0 -0` means THIS caller's own group and `-1` means every process the user can signal, so
# neither may ever be treated as a recorded group. The assert is that no `kill` reaches the OS at
# all: a `kill` shim on PATH-free ground is not available inside a sourced function, so the probe
# is a FUNCTION OVERRIDE of the builtin's wrapper — shadowing `kill` in this shell records every
# call the lib makes.
#
# THE OVERRIDE IS ITSELF PROVEN FIRST, by a POSITIVE CONTROL. An empty log is the assertion, and an
# empty log is also what a shim that never intercepts anything produces — the two are
# indistinguishable without a case that MUST log. `$SPAWN_PID` is syntactically fine and merely
# dead, so it clears the floor and reaches the `kill -0` rung: it must log. If it does not, every
# no-kill assert below is vacuous.
KILLLOG="$(mktemp "${TMPDIR:-/tmp}/liveness-kill.XXXXXX")"
kill(){ printf '%s\n' "$*" >> "$KILLLOG"; builtin kill "$@"; }
: > "$KILLLOG"
docket_group_alive_and_ours "$SPAWN_PID" "$tok" || true
assert "positive control: the kill override DOES intercept the lib's probe (else the asserts below are vacuous)" \
  '[ -s "$KILLLOG" ]'
assert "positive control: and what it logged is the group probe itself" \
  'grep -qF -- "-0 -$SPAWN_PID" "$KILLLOG"'
for bad in "" "abc" "0" "1" "12x"; do
  : > "$KILLLOG"
  docket_group_alive_and_ours "$bad" "$tok"; rc=$?
  assert "a pgid of '${bad:-<empty>}' is dead" '[ "$rc" != "0" ]'
  assert "a pgid of '${bad:-<empty>}' explains itself" '[ -n "$DOCKET_LIVENESS_WHY" ]'
  assert "a pgid of '${bad:-<empty>}' probes NOTHING — no kill is issued" '[ ! -s "$KILLLOG" ]'
done
unset -f kill
rm -f "$KILLLOG"

exit "$fail"

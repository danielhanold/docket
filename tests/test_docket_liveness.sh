#!/usr/bin/env bash
# tests/test_docket_liveness.sh — scripts/lib/docket-liveness.sh, the ONE identity-checked liveness
# predicate whose sole consumer is now runner-dispatch.sh (change 0284 extracted it; the retired
# gate-run.sh was the other consumer until change 0339).
# Run: bash tests/test_docket_liveness.sh
#
# The lib takes VALUES, never run dirs: its two consumers store their records in incompatible
# layouts and each keeps its own reader. So every case here passes a pgid and a token directly,
# which is also what makes the file hermetic — no run dir, no launcher, no git.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

. "$ROOT/scripts/lib/docket-liveness.sh"

# --- a real self-spawned group -----------------------------------------------------
# `set -m` in a subshell makes the background job a PROCESS-GROUP LEADER, so its pid IS its pgid —
# the same construction runner-dispatch.sh's --launch uses and the retired gate-run.sh's rung 3 fell
# back to (gate-run.sh retired by change 0339).
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

# ---- the rendering is PINNED, not merely normalized (0284 review, finding 1a) -----
# `ps -o lstart=` renders through the CALLER's environment, so an unpinned reader hands two
# processes that ask about the SAME pid two different tokens — and the comparison downstream reads
# that as "not the recorded process". A `--launch` under one TZ and an `--observe` under another
# would make a healthy long-running child unprovable, which on the observe seam is a terminal
# verdict. The reader therefore fixes the rendering itself.
#
# THE CONTROL COMES FIRST AND IS ASSERTED, not assumed: an environment-invariant assert is exactly
# what a platform whose `ps` ignored the environment would also produce, and the two are
# indistinguishable without a case that MUST differ. TZ is the leg every supported platform honors.
assert "control: the AMBIENT rendering really is TZ-dependent (else the pin asserts are vacuous)" \
  '[ "$(TZ=Asia/Tokyo ps -o lstart= -p "$SPAWN_PID")" != "$(TZ=America/New_York ps -o lstart= -p "$SPAWN_PID")" ]'
assert "docket_identity_of renders identically under any caller TZ" \
  '[ "$(TZ=Asia/Tokyo docket_identity_of "$SPAWN_PID")" = "$(TZ=America/New_York docket_identity_of "$SPAWN_PID")" ]'
assert "and an exported TZ does not move it either" \
  '[ "$( export TZ=Asia/Tokyo; docket_identity_of "$SPAWN_PID" )" = "$tok" ]'
# LC_TIME moves the weekday and month names on both platforms. This leg is only discriminating where
# the locale is installed — the TZ control above is what proves the mechanism is live.
assert "docket_identity_of renders identically under any caller locale" \
  '[ "$(LC_ALL=fr_FR.UTF-8 docket_identity_of "$SPAWN_PID")" = "$tok" ]'
assert "the token is whitespace-normalized (no runs, no edges)" \
  '[ "$tok" = "$(tr -s "[:space:]" " " <<<"$tok" | sed -e "s/^ //" -e "s/ $//")" ]'
# Always-0 is load-bearing: a `set -e` caller ASSIGNS from this (the retired gate-run.sh did).
docket_identity_of 999999 >/dev/null; rc=$?
assert "docket_identity_of returns 0 even for a gone pid (set -e callers assign from it)" '[ "$rc" = "0" ]'
assert "docket_identity_of is empty for a gone pid" '[ -z "$(docket_identity_of 999999)" ]'

# ---- docket_group_alive_and_ours: the happy leg ----------------------------------
DOCKET_LIVENESS_WHY="stale-sentinel"
DOCKET_LIVENESS_CLASS="stale-class"
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a live group with a MATCHING token is alive" '[ "$rc" = "0" ]'
assert "DOCKET_LIVENESS_WHY is cleared on the alive leg" '[ -z "$DOCKET_LIVENESS_WHY" ]'
assert "DOCKET_LIVENESS_CLASS is cleared on the alive leg" '[ -z "$DOCKET_LIVENESS_CLASS" ]'

# ---- THE REASON CLASS (0284 review, finding 1b) ----------------------------------
# Every non-zero return is "not alive", and a cheap-false-dead consumer (the retired gate-run.sh was
# one) may keep reading it that way. But the two
# ways of being not-alive are not the same fact: `kill -0` failing is POSITIVE EVIDENCE that the
# group is gone, while every other leg says only that the question could not be answered this pass.
# A consumer for whom a false `dead` is terminal and irreversible must be able to tell them apart,
# so the class is carried alongside the reason.

# ---- the pid-reuse case: live group, MISMATCHED token ----------------------------
docket_group_alive_and_ours "$SPAWN_PID" "Thu Jan  1 00:00:00 1970"; rc=$?
assert "a live group with a MISMATCHED token is not alive (the pid-reuse case)" '[ "$rc" != "0" ]'
assert "and the reason names the mismatch, quoting both tokens" \
  '[ -n "$DOCKET_LIVENESS_WHY" ] && grep -qF "1970" <<<"$DOCKET_LIVENESS_WHY" && grep -qF "$tok" <<<"$DOCKET_LIVENESS_WHY"'
# NOT `gone`: the group is demonstrably ALIVE, and what failed is the comparison. A token rendered
# by an older, unpinned build of this reader mismatches for a reason that has nothing to do with the
# process — so this direction must fail closed toward "cannot be proven", never toward a verdict.
assert "a mismatched token is UNPROVABLE, never proof the child is gone" \
  '[ "$DOCKET_LIVENESS_CLASS" = "unprovable" ]'
# THE LEGACY-TOKEN DIRECTION, spelled as its own case because it is the one the pin cannot repair:
# a token recorded BEFORE the rendering was pinned may have been rendered under any TZ at all.
assert "an unpinned-legacy-style token is unprovable too, not a false death" \
  'legacy="$(TZ=Asia/Tokyo ps -o lstart= -p "$SPAWN_PID" | tr -s "[:space:]" " " | sed -e "s/^ //" -e "s/ $//")"
   docket_group_alive_and_ours "$SPAWN_PID" "$legacy"; rc2=$?
   [ "$rc2" != "0" ] && [ "$DOCKET_LIVENESS_CLASS" = "unprovable" ]'

# ---- an empty expected token fails CLOSED ----------------------------------------
docket_group_alive_and_ours "$SPAWN_PID" ""; rc=$?
assert "an EMPTY expected token is not alive — nothing to compare is not agreement" '[ "$rc" != "0" ]'
assert "and it says so" '[ -n "$DOCKET_LIVENESS_WHY" ]'
assert "an empty recorded token is UNPROVABLE — no evidence of death was gathered" \
  '[ "$DOCKET_LIVENESS_CLASS" = "unprovable" ]'

# ---- the same group after it exits ------------------------------------------------
kill -KILL -"$SPAWN_PID" 2>/dev/null
waited=0
while kill -0 -"$SPAWN_PID" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "fixture sanity: the group is really gone" '! kill -0 -"$SPAWN_PID" 2>/dev/null'
docket_group_alive_and_ours "$SPAWN_PID" "$tok"; rc=$?
assert "a group that has exited is dead" '[ "$rc" != "0" ]'
assert "and the reason is non-empty" '[ -n "$DOCKET_LIVENESS_WHY" ]'
# THE ONLY LEG THAT CLAIMS A DEATH. `kill -0` failing means no process in that group answers — not
# the leader, and not anything it spawned, since group membership survives the leader.
assert 'an exited group is the ONE `gone` class — positive evidence, not an unanswered question' \
  '[ "$DOCKET_LIVENESS_CLASS" = "gone" ]'

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
  # Nothing was probed, so nothing was learned: a record that names no usable group is the purest
  # `unprovable` there is, and reading it as a death would be a verdict manufactured out of a typo.
  assert "a pgid of '${bad:-<empty>}' is UNPROVABLE, never gone" \
    '[ "$DOCKET_LIVENESS_CLASS" = "unprovable" ]'
done
unset -f kill
rm -f "$KILLLOG"

exit "$fail"

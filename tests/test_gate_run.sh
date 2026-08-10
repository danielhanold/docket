#!/usr/bin/env bash
# tests/test_gate_run.sh — scripts/gate-run.sh's launch half (change 0282).
#
# WHAT THIS FILE IS FOR: the helper's whole reason to exist is that a wait keyed on a success
# marker cannot tell "still running" from "died", so every assert here is about the launch being
# DETACHED, DURABLE, and RECORDED BEFORE the user's command ever runs — the three properties the
# observation half is later allowed to assume.
#
# Contract: scripts/gate-run.md. Prologue and sandbox: tests/lib/gate_run_common.sh.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/gate_run_common.sh"

# ---- launch returns a handle, and returns it FAST -------------------------------
# The child outlives every assert in this section by a wide margin, deliberately: several of them
# ("no terminal record while the child runs", the live `ps` group reads) are only meaningful against
# a process that is still there, and a short-lived child would turn them into load-dependent flakes.
start=$(date +%s)
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30; exit 0')"; rc=$?
elapsed=$(( $(date +%s) - start ))
assert "launch exits 0" '[ "$rc" = "0" ]'
assert "launch prints an absolute path handle" '[ "${RD:0:1}" = "/" ]'
assert "the handle is a directory that exists" '[ -d "$RD" ]'
# THE POINT: launch must not block for the child's duration.
assert "launch returned well before the child finished" '[ "$elapsed" -lt 3 ]'

# ---- the run dir is private and fully recorded ----------------------------------
perms="$(stat -f '%Lp' "$RD" 2>/dev/null || stat -c '%a' "$RD")"
assert "run dir is 0700 (umask 077)" '[ "$perms" = "700" ]'
assert "launch record exists" '[ -f "$RD/launch" ]'
assert "identity token exists and is non-empty" '[ -s "$RD/identity" ]'
launch_rec="$(cat "$RD/launch")"
assert "launch record carries pid"  'grep -q "^pid="  <<<"$launch_rec"'
assert "launch record carries pgid" 'grep -q "^pgid=" <<<"$launch_rec"'
assert "launch record carries the command line" 'grep -q "^cmd=" <<<"$launch_rec"'
assert "streams are separate durable files" '[ -f "$RD/stdout.log" ] && [ -f "$RD/stderr.log" ]'
assert "no terminal record while the child runs" '[ ! -f "$RD/terminal" ]'

# ---- the child really is detached: its own process group ------------------------
pgid_rec="$(sed -n 's/^pgid=//p' "$RD/launch")"
child_pid="$(sed -n 's/^pid=//p' "$RD/launch")"
os_pgid="$(ps -o pgid= -p "$child_pid" | tr -d ' ')"
assert "recorded pgid matches the OS" '[ "$pgid_rec" = "$os_pgid" ]'
assert "the child is a group leader, not in the launcher's group" \
  '[ "$pgid_rec" = "$child_pid" ] && [ "$pgid_rec" != "$$" ]'
# The identity token is the leader's OS start time, so it must agree with what the OS says now —
# this is what a later --observe compares against to refuse a recycled pgid.
assert "the recorded identity token matches the live group leader" \
  '[ -n "$(cat "$RD/identity")" ] && [ "$(cat "$RD/identity")" = "$(ps -o lstart= -p "$pgid_rec" | tr -s "[:space:]" " " | sed "s/^ //;s/ $//")" ]'
reap "$pgid_rec"

# ---- streams land unmerged and unframed -----------------------------------------
RD2="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'echo to-out; echo to-err >&2')"
for _ in $(seq 1 40); do [ -f "$RD2/terminal" ] && break; sleep 0.25; done
out="$(cat "$RD2/stdout.log")"; err="$(cat "$RD2/stderr.log")"
assert "stdout holds exactly the child's stdout" '[ "$out" = "to-out" ]'
assert "stderr holds exactly the child's stderr" '[ "$err" = "to-err" ]'
# The script(1) rung was rejected at plan time for exactly this: no injected framing, no CR.
CR=$'\r'
assert "no primitive-injected framing in the durable log" '! grep -q "$CR" "$RD2/stdout.log"'

# ---- an existing run dir is REFUSED, never reused --------------------------------
mkdir -p "$SBX/collide/run-fixed"
out_line="$(gate_run --launch --root "$SBX/collide" --run-name run-fixed -- /bin/true 2>/dev/null)"
assert "an existing run dir reports the launch-failed token" '[ "$out_line" = "launch-failed" ]'
assert "the failure token carries no slash (shape-distinct from a handle)" \
  '[ "${out_line#*/}" = "$out_line" ]'

# ---- an unwritable root is the SAME token — one failure shape, never a taxonomy -----
mkdir -p "$SBX/readonly"; chmod 555 "$SBX/readonly"
ro_line="$(gate_run --launch --root "$SBX/readonly" -- /bin/true 2>/dev/null)"
assert "an unwritable root reports the same launch-failed token" '[ "$ro_line" = "launch-failed" ]'
chmod 755 "$SBX/readonly"

# ---- THE TWO WEDGE POINTS --------------------------------------------------------
# These are the mutation tests for the ordering rule, expressed as asserts. `GATE_RUN_TEST_WEDGE`
# is inert when unset, so neither point can become a hang site in production. Both launches are
# made inside a command substitution or a subshell so the wedge variable cannot leak onto a later
# call in this file.

# Wedge AFTER the record lands: the recorded group must die, nothing leaks.
# `--run-name` pins the run dir so the pgid below is read from THE wedged run and not from whichever
# earlier run a glob happened to sort last — a stale, already-dead pgid would make the kill assert
# permanently green and the mutation invisible.
RD3="$(GATE_RUN_TEST_WEDGE=post-record GATE_RUN_ESTABLISH_SECS=2 \
        gate_run --launch --root "$SBX/runs" --run-name wedged -- /bin/sh -c 'sleep 30' 2>/dev/null)"
assert "a wedged launch reports the failure token" '[ "$RD3" = "launch-failed" ]'
wedged_pgid="$(sed -n 's/^pgid=//p' "$SBX/runs/wedged/launch" 2>/dev/null || true)"
assert "the wedged launch DID record its group, so the kill assert below is not vacuous" \
  '[ -n "$wedged_pgid" ]'
assert "the recorded group was killed by the failure path" \
  '! kill -0 -"$wedged_pgid" 2>/dev/null'
assert "a failed launch keeps its run dir for inspection" '[ -d "$SBX/runs/wedged" ]'

# Wedge BEFORE the record: the command may never have RUN, and no process of it may exist. Two
# independent witnesses, because neither one alone is trustworthy here:
#   * a marker file the command touches on its first line — direct evidence that it ran at all,
#     and it survives the process, so it cannot be raced;
#   * a `pgrep -f` canary — evidence that nothing was left behind. With no launch record the
#     failure path is pid-directed at the plumbing process by design, so a command forked before
#     the record is NOT cleaned up and shows here.
# THE CANARY'S SHAPE IS LOAD-BEARING. `sh -c 'sleep 30 # gate-run-canary'` is a single simple
# command, so the shell EXECs it and the process's command line becomes a bare `sleep 30` with the
# canary gone — the marker is invisible to `pgrep -f` and the assert is permanently green. Measured,
# not assumed. Ending the `-c` string with a builtin defeats the exec optimization and keeps the
# whole string on the shell's command line.
canary_marker="$SBX/canary-ran"
before="$(pgrep -f 'gate-run-canary' | wc -l | tr -d ' ')"
( GATE_RUN_TEST_WEDGE=pre-record GATE_RUN_ESTABLISH_SECS=2 \
    gate_run --launch --root "$SBX/runs" -- \
      /bin/sh -c "touch '$canary_marker'; sleep 30; : gate-run-canary" >/dev/null 2>&1 ) || true
after="$(pgrep -f 'gate-run-canary' | wc -l | tr -d ' ')"
assert "the command never RAN when the wedge precedes the record" '[ ! -f "$canary_marker" ]'
assert "no command process exists when the wedge precedes the record" '[ "$after" = "$before" ]'

exit "$fail"

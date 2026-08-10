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

# ---- THE TERMINAL RECORD: a termination KIND, never a bare integer -----------------
# WHY A KIND AND NOT A NUMBER: a child killed by a signal NEVER FINISHED. If the record collapsed
# both outcomes into "nonzero", an observation would read a signal death as `failed` — and `failed`
# is the one state allowed to feed repair work, so a suite that never ran would mint work for tests
# that never executed. The kind is what keeps those two apart at the only place the distinction is
# still observable.

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"
await_terminal "$RD"
rec="$(cat "$RD/terminal" 2>/dev/null || true)"
assert "a clean exit records kind=exit code=0" '[ "$rec" = "kind=exit code=0" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 1')"
await_terminal "$RD"
rec="$(cat "$RD/terminal" 2>/dev/null || true)"
assert "a red exit records kind=exit code=1" '[ "$rec" = "kind=exit code=1" ]'

# The headline assert. The group-directed TERM is the same shape `--stop` uses, so this also pins
# that the wrapper OUTLIVES a teardown of its own group far enough to witness the child's death —
# a wrapper that died alongside the child would leave no record at all and this would read empty.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
term_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -TERM -"$term_pgid" 2>/dev/null || true
await_terminal "$RD"
rec="$(cat "$RD/terminal" 2>/dev/null || true)"
assert "a TERMed child records kind=signal, never kind=exit" 'grep -q "^kind=signal" <<<"$rec"'
assert "the signal number is recorded" 'grep -q "signal=15" <<<"$rec"'
reap "$term_pgid"

# ---- --observe: SIX STATES, AND THE READ ORDER THAT MAKES THEM HONEST ----------------
# Three properties are on trial in this section, and each has its own asserts:
#   1. THE RECORD OUTRANKS THE LIVENESS PROBE. A verdict the child itself wrote can never be
#      overruled by a probe of the group it used to lead.
#   2. LIVENESS IS IDENTITY-CHECKED. A bare `kill -0` answers for whoever holds the pgid NOW, and
#      pgids are recycled; the run that recorded it may be long dead.
#   3. STDOUT IS THE PROTOCOL — exactly one line. The log tail a `died` prints is multiline and
#      arbitrary, so it can never share the channel a caller parses.

# ---- running / passed / failed --------------------------------------------------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
run_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
assert "a live child observes as running" '[ "$(gate_run --observe "$RD")" = "state=running" ]'
reap "$run_pgid"

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
assert "a clean exit observes as passed" '[ "$(gate_run --observe "$RD")" = "state=passed" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 1')"; await_terminal "$RD"
assert "a red exit observes as failed" '[ "$(gate_run --observe "$RD")" = "state=failed" ]'

# ---- PROPERTY 1: a present record outranks a group that answers ALIVE ------------------
# This is not a synthetic state. The wrapper writes `terminal` and only THEN exits, so between
# those two instants the record is present AND the recorded group is alive. An observer that
# probed liveness first would report `running` for a run that has already returned its verdict —
# and a caller keyed on `running` would keep waiting on a finished run.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
live_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
printf 'kind=exit code=0\n' >"$RD/terminal"
assert "a present record outranks a live group" '[ "$(gate_run --observe "$RD")" = "state=passed" ]'
assert "the fixture's group really was alive, so the assert above is not vacuous" \
  'kill -0 -"$live_pgid" 2>/dev/null'
reap "$live_pgid"

# ---- died cause=signal: the 0276 headline shape ----------------------------------------
# `died` is NEVER `failed`. A child killed by a signal never finished, and `failed` is the one
# state allowed to feed repair work — so collapsing these two mints integration-repair work for a
# suite that never ran.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
sig_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -TERM -"$sig_pgid" 2>/dev/null || true
await_terminal "$RD"
assert "a signal death observes as died cause=signal, never failed" \
  '[ "$(gate_run --observe "$RD")" = "state=died cause=signal" ]'
reap "$sig_pgid"

# ---- died cause=vanished: group gone, no record ever written ----------------------------
# A KILLed group cannot write anything, so the absence of a record IS the evidence. The child
# writes to its stderr first so the log tail below has real bytes to carry — a tail assert against
# an empty log would pass on the diagnostic prefix alone and prove nothing about property 3.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'echo diag-first >&2; echo diag-last >&2; sleep 30')"
van_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
for _ in $(seq 1 100); do
  stderr_so_far="$(cat "$RD/stderr.log" 2>/dev/null || true)"
  case "$stderr_so_far" in *diag-last*) break ;; esac
  sleep 0.1
done
kill -KILL -"$van_pgid" 2>/dev/null || true      # KILL: nothing survives to write a record
assert "the recorded group is verified gone, so the observation below is not racing a corpse" \
  'await_group_gone "$van_pgid"'
assert "a KILLed group left NO terminal record" '[ ! -f "$RD/terminal" ]'
obs="$(gate_run --observe "$RD" 2>"$SBX/obs.err")"
obs_first_line="${obs%%$'\n'*}"
# THE PROMPTNESS PROPERTY the whole change exists for: detected on the NEXT observation.
assert "a vanished group observes as died cause=vanished" '[ "$obs" = "state=died cause=vanished" ]'
assert "PROPERTY 3: stdout carried exactly one line" '[ "$obs" = "$obs_first_line" ]'
assert "the log tail goes to STDERR, never the protocol channel" '[ -s "$SBX/obs.err" ]'
assert "the tail carries the child's own last diagnostics" 'grep -qF -- "diag-last" "$SBX/obs.err"'
assert "and no part of the tail reached stdout" '! grep -qF -- "diag-last" <<<"$obs"'

# ---- unavailable: a malformed record, and an unreadable run dir -------------------------
# A verdict read out of garbage is fabricated. A malformed record means the SUPERVISOR did not
# finish cleanly, which is a different thing from any verdict about the child.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
printf 'garbage\n' >"$RD/terminal"
assert "a malformed record observes as unavailable" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=unavailable" ]'
assert "the malformed-record detail goes to stderr, not the protocol line" \
  '[ -n "$(gate_run --observe "$RD" 2>&1 >/dev/null)" ]'
# The kind is right but the payload is not a number — still unparseable, still not a verdict.
printf 'kind=exit code=oops\n' >"$RD/terminal"
assert "a record whose exit code is not a number observes as unavailable" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=unavailable" ]'
assert "a nonexistent run dir observes as unavailable" \
  '[ "$(gate_run --observe "$SBX/no-such-run" 2>/dev/null)" = "state=unavailable" ]'

# ---- PROPERTY 2, THE IDENTITY GUARD: a recycled pgid must never read alive ---------------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
recycled_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
recorded_ident="$(cat "$RD/identity")"
kill -KILL -"$recycled_pgid" 2>/dev/null || true
await_group_gone "$recycled_pgid"
# The one-second separation is LOAD-BEARING, not padding: `ps -o lstart=` resolves to whole
# seconds, so a bystander started inside the same second as the dead leader would carry an
# identical token and the mismatch this fixture exists to create would not exist.
sleep 1
foreign_pgid="$(start_foreign_group)"
foreign_ident="$(ps -o lstart= -p "$foreign_pgid" 2>/dev/null | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//')"
assert "the bystander really leads a live foreign group" \
  '[ -n "$foreign_pgid" ] && kill -0 -"$foreign_pgid" 2>/dev/null'
assert "the fixture really is an identity MISMATCH, or the guard below is vacuous" \
  '[ -n "$recorded_ident" ] && [ -n "$foreign_ident" ] && [ "$foreign_ident" != "$recorded_ident" ]'
# Substitute the live foreign group under the recorded pgid — the recycled-pgid state, reproduced.
sed -i.bak "s/^pgid=.*/pgid=$foreign_pgid/" "$RD/launch"
assert "an identity mismatch reads died, never running" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]'
reap "$foreign_pgid"

# ---- the identity token has TWO sources, and an empty one fails the conjunct CLOSED -------
# `launch` is written before `identity`, deliberately, so a crashed establishment can leave the
# token file absent while the launch record still carries its `identity=` field. Either source
# answers.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
fallback_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
rm -f "$RD/identity"
assert "the launch record's identity= field is a working fallback source" \
  '[ "$(gate_run --observe "$RD")" = "state=running" ]'
reap "$fallback_pgid"

# With BOTH sources empty there is nothing to compare — and "nothing to compare" is not agreement.
# A conjunct that read it as agreement would hand `running` to every run whose establishment died.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
blank_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
rm -f "$RD/identity"
sed -i.bak 's/^identity=.*/identity=/' "$RD/launch"
assert "no recorded identity token at all reads died, never running" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]'
assert "and that run's group really was alive, so the assert above is not vacuous" \
  'kill -0 -"$blank_pgid" 2>/dev/null'
reap "$blank_pgid"

exit "$fail"

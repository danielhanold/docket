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

# ---- THE FAILURE PATH SIGNALS NOTHING IT CANNOT PROVE IS OURS -----------------------
# This path is the only signalling site outside `--stop`, and it runs on a run dir NOBODY WILL EVER
# BE HANDED — so it is the one place where "kill it anyway" is most tempting and least checkable
# afterwards. It is identity-gated all the same, for the reason spec assumption 14 gives itself:
# what the path cannot verify it reports LOUDLY with the run-dir path for manual disposal, so
# refusing to signal an unprovable group routes the survivor to that report rather than abandoning
# it. The trade is also cheap HERE specifically: the group-directed branch is reachable only with
# `launch` present and `identity` absent, so by the ordering rule no command process has been forked
# and a live member under an unprovable leader is likelier a recycled pgid than an orphan of ours.
#
# THE FIXTURE IS THE RECYCLED-PGID ONE, built exactly as `--observe`'s and `--stop`'s are: a live
# BYSTANDER group is substituted under the recorded pgid while the wrapper is wedged, so the pgid
# the failure path is about to signal is alive and is provably not this run's.
bystander_pgid="$(start_foreign_group)"
bystander_ident="$(ps -o lstart= -p "$bystander_pgid" 2>/dev/null | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//')"
assert "the bystander really leads a live foreign group" \
  '[ -n "$bystander_pgid" ] && kill -0 -"$bystander_pgid" 2>/dev/null'
# The one-second separation is LOAD-BEARING, and it is why the bystander is started BEFORE the
# launch rather than during it: `ps -o lstart=` resolves to whole seconds, so a wrapper forked
# inside the same second as the bystander would carry an identical token and the mismatch this
# fixture exists to create would not exist. Held outside the establishment window so the window
# stays wide enough for the substitution below to land inside it.
sleep 1
( GATE_RUN_TEST_WEDGE=post-record GATE_RUN_ESTABLISH_SECS=2 \
    gate_run --launch --root "$SBX/runs" --run-name unprovable -- /bin/sh -c 'sleep 30' \
    >"$SBX/unprovable.out" 2>"$SBX/unprovable.err" ) &
unprov_job=$!
assert "the wedged launch recorded its group before the substitution" \
  'wait_for_file "$SBX/runs/unprovable/launch"'
unprov_pid="$(sed -n 's/^pid=//p' "$SBX/runs/unprovable/launch" 2>/dev/null || true)"
# Read BEFORE the substitution below overwrites it: the sandbox's EXIT reap is driven off the `pgid=`
# field of every `launch` record it can find, so once that field names the bystander this run's own
# group has no other way to be cleaned up if an assert below goes red.
unprov_pgid="$(sed -n 's/^pgid=//p' "$SBX/runs/unprovable/launch" 2>/dev/null || true)"
unprov_ident="$(sed -n 's/^identity=//p' "$SBX/runs/unprovable/launch" 2>/dev/null || true)"
assert "the fixture really is an identity MISMATCH, or the guard below is vacuous" \
  '[ -n "$unprov_ident" ] && [ -n "$bystander_ident" ] && [ "$unprov_ident" != "$bystander_ident" ]'
sed -i.bak "s/^pgid=.*/pgid=$bystander_pgid/" "$SBX/runs/unprovable/launch"
wait "$unprov_job" 2>/dev/null || true
assert "a launch whose recorded group cannot be proven ours still reports the failure token" \
  '[ "$(cat "$SBX/unprovable.out" 2>/dev/null)" = "launch-failed" ]'
# THE HEADLINE ASSERT. Drop the identity conjunct and this group takes a TERM and then a KILL while
# the failure path's own report still reads "terminated and verified gone" — the bystander is the
# only witness that can tell a proven kill from an unprovable one.
assert "the bystander group was NOT signalled — ownership was unprovable" \
  'kill -0 -"$bystander_pgid" 2>/dev/null'
assert "the refusal names ownership as the reason, on stderr" \
  'grep -qF -- "ownership-unprovable" "$SBX/unprovable.err"'
# FAIL-CLOSED, NOT FAIL-SILENT. Detection is possible where safe signalling is not, so the survivor
# is reported loudly with its run dir for manual disposal — assumption 14's own second clause, and
# the same posture `--stop` step 1 takes over live orphans.
assert "the unverified survivor is reported loudly, with its run dir" \
  'grep -qF -- "LAUNCH FAILED AND UNVERIFIED" "$SBX/unprovable.err" \
     && grep -qF -- "$SBX/runs/unprovable" "$SBX/unprovable.err"'
# AND THE RUN'S OWN PLUMBING IS STILL GONE. Refusing the group signal is not a licence to leak the
# process this launch actually started: the direct spawn child is signalled by pid under its own
# identity check, so the wedged wrapper dies whichever way the group branch goes.
assert "the wedged wrapper this launch really started is gone" \
  '[ -n "$unprov_pid" ] && ! kill -0 "$unprov_pid" 2>/dev/null'
reap "$bystander_pgid"
reap "$unprov_pgid"

# THE 0/1 FLOOR ON THE SAME PATH, ASSERTED STRUCTURALLY — and the hazard is itself the reason it is
# structural. `kill -TERM -0` means THIS CALLER'S OWN PROCESS GROUP, so the mutation that would
# redden a behavioural fixture here is one that TERMs the harness running it. The floor's BEHAVIOUR
# is pinned end-to-end on the `--stop` side (tests/test_gate_run_stop.sh's bogus-pgid loop, through
# this same `recorded_pgid`); what is left to prove is that this path is ROUTED through that floor
# rather than reading the recorded field raw, and that its group signal is gated on identity.
fail_stop_body="$(awk '/^launch_failure_stop\(\) \{/ {inblock=1} inblock {print} inblock && /^\}/ {exit}' "$GATE_RUN")"
fail_stop_lines="$(wc -l <<<"$fail_stop_body")"; fail_stop_lines="${fail_stop_lines//[[:space:]]/}"
assert "the failure-stop body really was extracted, or the guards below are vacuous" \
  '[ "$fail_stop_lines" -gt 20 ] && grep -qF -- "kill -TERM" <<<"$fail_stop_body"'
assert "the failure path reads its pgid through the 0/1 floor" \
  'grep -qF -- "recorded_pgid \"\$RUN_DIR\"" <<<"$fail_stop_body"'
assert "and never reads that field raw, which would bypass the floor" \
  '! grep -qE "record_field[^#]*pgid" <<<"$fail_stop_body"'
assert "and its group-directed signal is gated on the identity check" \
  'grep -qF -- "identity_matches \"\$RUN_DIR\"" <<<"$fail_stop_body"'
# THE PID-DIRECTED SIGNAL IS GATED TOO, and this one is structural for a different reason: its
# false positive needs a pid RECYCLED inside the launch window, which no fixture can construct on
# demand. It is a real hazard all the same and the measurement is what says so — Bash reaps a
# background child from its SIGCHLD handler well before the explicit `wait`, so `$!` names a
# reclaimable pid and not a held one (`ps -o state=` reports it gone, not `Z`, which is exactly what
# the handshake loop keys on). The token has to be captured at the fork, because that is the last
# instant the pid is known to still be ours.
assert "the spawn child's identity token is captured at the fork" \
  'grep -qF -- "SPAWN_IDENT=\"\$(identity_of \"\$SPAWN_PID\")\"" "$GATE_RUN"'
assert "and the pid-directed signal is gated on it" \
  'grep -qF -- "SPAWN_IDENT" <<<"$fail_stop_body"'

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

# ---- THE OBSERVE TOCTOU RACE, PINNED BY A DETERMINISTIC INTERLEAVING ---------------------
# The half of the record-outranks-liveness property no earlier assert can reach. "A present record
# outranks a live group" above pins step 1; NOTHING without a barrier can tell step 1's read apart
# from step 3's RE-READ, because in every unheld run the two see the same world.
#
# The race is real and it is the expensive one: an observer reads the record (absent), the child
# finishes and the wrapper writes `kind=exit code=0` and exits, and only THEN does the observer
# probe liveness — and finds the group gone. Without the re-read that observation reports `died` on
# a run that PASSED, and a call site keyed on `died` relaunches a suite that already went green.
#
# Held with a rendezvous, never a sleep: `barrier post-first-record` announces its arrival and then
# waits to be let go, so the interleaving is constructed rather than raced for.
RD="$(gate_run --launch --root "$SBX/runs" -- \
        /bin/sh -c "while [ ! -e '$SBX/race-go' ]; do sleep 0.1; done; exit 0")"
race_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
BAR="$SBX/observe-barrier"
( GATE_RUN_TEST_BARRIER=post-first-record GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --observe "$RD" >"$SBX/race-obs.out" 2>/dev/null ) &
obs_job=$!
assert "the observer really is held at the rendezvous, or the race below never happened" \
  'wait_for_file "$BAR.reached"'
# Read at the instant the observer is held: the child is still blocked on its gate, so the record
# the observer just looked for provably was NOT there. This is what makes the assert a re-read test
# and not a first-read test.
assert "the held observer's FIRST record read found nothing, so the re-read is what decides" \
  '[ ! -f "$RD/terminal" ]'
touch "$SBX/race-go"                                  # the child completes...
assert "the child completed and its verdict landed while the observer was held" \
  'await_terminal "$RD" && [ "$(cat "$RD/terminal")" = "kind=exit code=0" ]'
assert "and its group is verified gone, so the observer's probe WILL read dead" \
  'await_group_gone "$race_pgid"'
touch "$BAR.release"                                  # ...and only now is the observer let go
wait "$obs_job" 2>/dev/null || true
assert "an observer held across completion reports passed, never died" \
  '[ "$(cat "$SBX/race-obs.out" 2>/dev/null)" = "state=passed" ]'

# ---- THE BARRIER IS ENV-GATED AND INERT BY DEFAULT ----------------------------------------
# A fixture hook that could engage in production would be a hang site in the one helper whose whole
# job is to stop a wait from hanging. Both probes below run against a LIVE child, deliberately: a
# run with a terminal record returns at step 1 and never reaches the call site at all, so it would
# report a green inertness that proves nothing.
#
# Witnessed by the rendezvous file, not by a clock. `<file>.reached` is created only by an ENGAGED
# barrier and it outlives the process, so its absence is direct evidence the hook did not fire —
# where a stopwatch only ever says the stall was shorter than some number. The elapsed-seconds
# assert is kept as the second witness because an engaged barrier stalls for 30s against a bound of
# 5, a margin no scheduling noise closes; whole seconds, because `date +%s%N` is a GNU extension
# and a BSD `date` without it would make the arithmetic itself the failure.
INERT_RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 30')"
inert_pgid="$(sed -n 's/^pgid=//p' "$INERT_RD/launch")"
INERT_BAR="$SBX/inert-barrier"                        # no `.release` is ever created for this path

# The FILE variable alone must not arm anything — only the POINT variable does.
inert_start=$(date +%s)
inert_state="$(GATE_RUN_TEST_BARRIER_FILE="$INERT_BAR" gate_run --observe "$INERT_RD" 2>/dev/null)"
inert_elapsed=$(( $(date +%s) - inert_start ))
assert "an unarmed barrier leaves --observe reporting its real state" '[ "$inert_state" = "state=running" ]'
assert "an unarmed barrier never reaches its rendezvous" '[ ! -e "$INERT_BAR.reached" ]'
assert "an unarmed barrier does not stall the observation" '[ "$inert_elapsed" -lt 5 ]'

# Armed at a DIFFERENT point: the gate matches the point NAME, not the mere presence of the
# variable — or arming one rendezvous would hold every other call site too.
mismatch_start=$(date +%s)
mismatch_state="$(GATE_RUN_TEST_BARRIER=no-such-point GATE_RUN_TEST_BARRIER_FILE="$INERT_BAR" \
  gate_run --observe "$INERT_RD" 2>/dev/null)"
mismatch_elapsed=$(( $(date +%s) - mismatch_start ))
assert "a barrier armed at another point stays inert" '[ "$mismatch_state" = "state=running" ]'
assert "a barrier armed at another point never reaches its rendezvous" '[ ! -e "$INERT_BAR.reached" ]'
assert "a barrier armed at another point does not stall" '[ "$mismatch_elapsed" -lt 5 ]'
reap "$inert_pgid"

# ---- THE CONTRACT, THE FACADE WIRING, AND THE BUDGET ROWS ----------------------------------
# The contract page is the AUTHORITATIVE statement of this helper's vocabulary (spec assumption
# 10): the gate-execution posture points AT it rather than restating it, so a missing section or a
# vocabulary that drifted out of the page is an interface defect, not a documentation nicety.
# These are existence-and-shape asserts only — prose fidelity rests on co-location plus review, the
# same boundary tests/test_script_contracts_coverage.sh draws.
#
# EVERY assert below reads a SLICE of the page or a population DERIVED from one of its own tables,
# never the whole page — and that is mutation evidence rather than taste. Measured against this
# page: with whole-page haystacks, deleting the very section an assert names left TWELVE of them
# green. The six-state loop survived deleting the state table outright (`running` matched this
# file's own title, `failed` matched `launch-failed`, `passed` matched "a moment that has passed",
# `died` matched "the wrapper died alongside the command"); `stop-intent` matched the
# run-directory layout block; "per-platform capability" and "own process group" matched prose
# pointers outside the platform section; "only `running` is retryable" is stated twice, so either
# statement could go; the which-leg paragraph was covered by step 4's escalation bullet; and the
# facade assert was satisfied by the word `gate-run` in a comment. That is the same vacuity class
# repaired twice already in this change, and the idiom used against it here is
# tests/test_gate_execution_posture.sh's: locate a slice, anchor its non-vacuity, grep inside.

# The facade wiring is the OPS LIST, not a mention: `gate-run` named only in a comment is a helper
# no call site can reach through the facade.
assert "the facade wraps gate-run" \
  'grep -qE "^WRAPPED_OPS=.*[\" ]gate-run[\" ]" "$REPO/scripts/docket.sh"'
assert "gate-run has a co-located contract" '[ -f "$REPO/scripts/gate-run.md" ]'
contract="$(cat "$REPO/scripts/gate-run.md" 2>/dev/null || true)"
# Population floor FIRST: an unreadable page has to redden here, or every negative assert below
# passes by default and every positive one reports a deleted section it never actually looked for.
assert "the contract is non-vacuous (>= 200 lines)" '[ "$(grep -c . <<<"$contract")" -ge 200 ]'

# The slicers. `csection` runs a `## <name>` heading to the next `## `; `cfrom` runs a column-0
# lead-in to the next heading of any level. Same shape as tests/test_gate_execution_posture.sh's
# `para()`, with one deliberate difference: `cfrom` does NOT close on a following column-0 `**`,
# because this page hard-wraps bolded phrases to column 0 mid-paragraph (`**ordinary live child**`
# opens the which-leg paragraph's second line) and a `**`-terminated slice would end right there,
# one line in. Phrase asserts read a whitespace-FLATTENED slice, since grep matches within a line
# and a phrase-spanning pattern over hard-wrapped markdown otherwise doubles as a re-flow guard.
csection(){ awk -v h="## $1" '$0 == h {f=1;next} f && /^## /{f=0} f' <<<"$contract"; }
cfrom(){ awk -v pat="$1" 'index($0,pat)==1{f=1;print;next} f && /^#/{f=0} f' <<<"$contract"; }
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

# Headings are matched WHOLE — `^## .*<name>` would be satisfied by `## Named residuals (retired)`,
# which is the section's removal dressed as its presence.
for s in Purpose Usage "Run-directory layout" Behavior "Exit codes" \
         "Per-platform capability note" "Named residuals" Invariants; do
  assert "the contract has a $s section" 'grep -qxF -- "## '"$s"'" <<<"$contract"'
done

# THE SIX STATES, DERIVED FROM THE TABLE THAT DEFINES THEM. A caller keys its loop on these tokens,
# so the page has to carry every one as a ROW of the state table — not as a substring of whatever
# prose happens to be nearby. The first column is parsed the way test_gate_execution_posture.sh
# parses this same page's stop-token table; the `died cause=…` rows collapse onto `died`, which is
# the token an `--observe` line leads with.
states_tbl="$(awk -F'|' '
  /^\| *State *\| *Meaning *\| *Retryable *\|/ {f=1; next}
  !f {next}
  /^\|[ -]*\|[ -]*\|[ -]*\|/ {next}
  /^\|/ {print; next}
  {f=0}' <<<"$contract")"
n_state_rows="$(grep -c . <<<"$states_tbl")"
assert "the state table was located (non-vacuity anchor, got $n_state_rows rows)" \
  '[ "$n_state_rows" -ge 6 ]'
# First column, backticks and any `cause=…` qualifier stripped. With `want` set, only the rows
# whose Retryable cell says that.
state_col(){ awk -F'|' -v want="$1" '
  {t=$2; gsub(/`/,"",t); sub(/^[[:space:]]+/,"",t); sub(/[[:space:]].*$/,"",t)
   v=tolower($4); gsub(/[^a-z]/,"",v)
   if (t != "" && (want == "" || v == want)) print t}' <<<"$states_tbl"; }
state_names="$(state_col "" | sort -u)"
for st in running passed failed died stopped unavailable; do
  assert "the state table gives the $st state a row of its own" \
    'grep -qxF -- "'"$st"'" <<<"$state_names"'
done
# ...and the Retryable column agrees with the rule the prose states. Derived from the same table,
# so a flipped cell reddens here instead of waiting for a human to read the row.
retryable_rows="$(state_col yes | sort -u | tr '\n' ' ')"; retryable_rows="${retryable_rows% }"
assert "the table marks running — and only running — retryable (got '$retryable_rows')" \
  '[ "$retryable_rows" = "running" ]'

# The retryable rule is stated TWICE by design — once in Purpose as a property every caller may
# rely on, once beside the table where the states are defined — so each statement is pinned in its
# own slice. Measured: a whole-page grep let either one be deleted while the other kept it green.
purpose_blk="$(csection Purpose)"
observe_blk="$(cfrom '### `--observe`')"
assert "the Purpose section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$purpose_blk")" -ge 10 ]'
assert "the --observe section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$observe_blk")" -ge 18 ]'
assert "Purpose states that only running is retryable" \
  'grep -qiE "only .running. is retryable" <<<"$purpose_blk"'
assert "and the section that defines the states states it there too" \
  'grep -qiE "only .running. is retryable" <<<"$observe_blk"'

# THE PER-PLATFORM NOTE, and the two things that make it honest rather than aspirational: it must
# name the narrowing, and it must never claim a session it does not deliver. The section-located
# anchor below replaces a whole-page grep for "per-platform capability", which two prose pointers
# elsewhere on the page kept green through a deletion of the section itself.
platform_blk="$(csection "Per-platform capability note")"
assert "the per-platform capability section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$platform_blk")" -ge 20 ]'
assert "the contract never claims a new session unconditionally" \
  '! grep -qiE "always (creates|delivers) a new session" <<<"$contract"'
assert "the narrowed platform is described as own process group plus the handshake" \
  'grep -qiE "own process group" <<<"$platform_blk"'
# Cited by VERBATIM QUOTE, never by line number (AGENTS.md, ADR-0054) — a quoted clause is
# greppable, which is what makes this assert possible at all.
assert "the note quotes ADR-0080's measured clause verbatim" \
  'grep -qF -- "own process GROUP, not a new SESSION" <<<"$platform_blk"'
assert "the note quotes gate-execution-evidence.md's setsid-absent finding verbatim" \
  'grep -qF -- "is not installed on macOS" <<<"$platform_blk"'

# THE NAMED RESIDUALS. Each is a property the shipped helper does NOT have; a residual that is not
# written down is indistinguishable from a bug nobody has hit yet.
#
# Each anchor must occur in EXACTLY ONE residual paragraph, which is what makes these guards over
# the residuals rather than over the page. `stop-intent` is the proof: it occurs five times here
# (the layout block, `--stop`'s steps, the invariants), so the whole-page grep this replaces stayed
# green through a deletion of residual 3 outright. Exactly-one rather than at-least-one, so the
# same residual cannot be diluted into two half-statements either.
#
# NUMBERING IS DELIBERATELY NOT PART OF ANY ANCHOR. Retiring a residual renumbers the rest — this
# change did exactly that once already — and a guard keyed on the number would redden on that
# legitimate edit while still missing the deletion it exists to catch.
residuals_blk="$(csection "Named residuals")"
assert "the Named residuals section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$residuals_blk")" -ge 30 ]'
# The floor here — and on every other slice in this section — is a NON-VACUITY floor, not a lock on
# the count. It has one job: make an unlocated slice redden with a message that says so, instead of
# letting every grep below read a parse failure as a missing clause. Set loose enough that retiring
# one residual, or re-flowing a paragraph, does not redden a guard about something else.
n_residuals="$(grep -cE '^\*\*[0-9]+\. ' <<<"$residuals_blk")"
assert "the numbered residual paragraphs were located (got $n_residuals)" '[ "$n_residuals" -ge 3 ]'
# Count the residual paragraphs containing a fixed string. A paragraph runs from its `**<n>. `
# lead-in to the next one, so trailing prose under the last residual belongs to that residual.
residual_hits(){ awk -v pat="$1" '
  /^\*\*[0-9]+\. / { if (buf != "" && index(buf, pat)) n++; buf=""; inr=1 }
  inr { buf = buf $0 "\n" }
  END { if (buf != "" && index(buf, pat)) n++; print n+0 }' <<<"$residuals_blk"; }
assert "exactly one residual records the 129..192 shell floor" \
  '[ "$(residual_hits "129..192")" = "1" ]'
assert "exactly one residual names the non-shell helper a real fix would need" \
  '[ "$(residual_hits "non-shell helper")" = "1" ]'
assert "exactly one residual records the escaped-group survivor" \
  '[ "$(residual_hits "escaped the recorded group")" = "1" ]'
assert "exactly one residual records the external signal read as deliberate" \
  '[ "$(residual_hits "stop-intent")" = "1" ]'
assert "exactly one residual names the human-gated perl supersede option" \
  '[ "$(residual_hits "POSIX::setsid")" = "1" ]'
assert "exactly one residual records the unprovable launch-failure group as detected, not signalled" \
  '[ "$(residual_hits "detected, not signalled")" = "1" ]'

# THE INVARIANTS AND THE CODE MUST AGREE ABOUT WHO MAY BE SIGNALLED. A contract that states an
# absolute the shipped script does not hold is worse than one that states the narrower truth: the
# absolute is what a later reader trusts instead of reading the code. Both clauses below are quoted
# verbatim from the page (AGENTS.md, ADR-0054), so drift is greppable — and each is read in the
# section that owns it, so neither can be satisfied by the other's restatement.
launch_blk="$(cfrom '### `--launch`')"
invariants_blk="$(csection Invariants)"
assert "the --launch section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$launch_blk")" -ge 18 ]'
assert "the Invariants section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$invariants_blk")" -ge 30 ]'
assert "the contract states the launch failure path's refusal to signal an unprovable group" \
  'grep -qF -- "Where ownership cannot be proven" <<<"$launch_blk"'
assert "and it states the bare-probe rule by FAIL DIRECTION, not as a count of sites" \
  'grep -qF -- "about fail direction, not a syscall ban" <<<"$invariants_blk"'

# THE DELIBERATE DEVIATIONS FROM THE PLAN'S SKETCH. Each was measured by the task that shipped it,
# and a contract that documents the sketch instead of the script is worse than none.
trap_blk="$(cfrom "### The terminal record")"
layout_blk="$(csection "Run-directory layout")"
which_leg_blk="$(cfrom '**Which leg produces which token')"
which_leg_flat="$(flat "$which_leg_blk")"
exit_blk="$(csection "Exit codes")"
assert "the terminal-record section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$trap_blk")" -ge 12 ]'
assert "the run-directory layout section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$layout_blk")" -ge 15 ]'
assert "the which-leg paragraph and its token table were located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$which_leg_blk")" -ge 8 ]'
assert "the exit-codes section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$exit_blk")" -ge 8 ]'
TRAP_LIT="trap '' TERM"
assert "the contract records the wrapper's ignored-TERM disposition" \
  'grep -qF -- "$TRAP_LIT" <<<"$trap_blk"'
# Scoped to the paragraph that owns the claim: at page scope this was satisfied by `--stop` step
# 4's own "**The KILL escalation deliberately does not re-check identity**" bullet, so the
# which-leg paragraph and its whole token table could be deleted while it stayed green.
assert "the contract records which leg actually produces the stopped token" \
  'grep -qiE "KILL[ -]escalation" <<<"$which_leg_blk"'
assert "and it names already-terminal as the ORDINARY live-child stop" \
  'grep -qiE "ordinary live child[^.]{0,80}already-terminal" <<<"$which_leg_flat"'
assert "the contract records that launch is written before identity" \
  'grep -qiE "launch. is written (first|before)" <<<"$layout_blk"'
assert "the exit-code section defers to the report line" \
  'grep -qiE "key on the stdout report line" <<<"$exit_blk"'

# Budgets: a new test file with no budget row is how the suite silently grows. Keyed on the ROW
# SHAPE the file documents — path, seconds, lane — because a commented-out row still carries the
# path, and a commented-out row is exactly the state this assert exists to catch.
budgets="$(cat "$REPO/tests/runtime-budgets.tsv")"
for f in tests/test_gate_run.sh tests/test_gate_run_stop.sh; do
  f_pat="${f//./\\.}"
  assert "$f has a budget row" \
    'grep -qE "^'"$f_pat"'[[:space:]]+[0-9]+[[:space:]]+(parallel|serial)$" <<<"$budgets"'
done

exit "$fail"

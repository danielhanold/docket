#!/usr/bin/env bash
# tests/test_gate_run_stop.sh — scripts/gate-run.sh's `--stop` verb (change 0282).
#
# WHAT THIS FILE IS FOR: `--stop` is the one verb that SIGNALS, so its asserts are about two things
# that a launch or an observation never has to answer for —
#   1. WHO gets the signal. Identity is checked before anything is signalled, and the one bare probe
#      in the whole script (step 1's orphan probe) sits where the leader is known dead, so no match
#      is possible and an alive result can only move the outcome fail-closed.
#   2. WHAT MAY BE CLAIMED AFTERWARDS. The terminal record outranks the stop at steps 1, 3 and 6;
#      `--stop` never writes a terminal record of its own; and the completed `stopped` marker is
#      written only after termination is VERIFIED and only when the group was actually signalled.
#
# Split from tests/test_gate_run.sh because one file carrying both would exceed its wall-clock
# budget and blur two review surfaces. Task 6 appends the five deterministic interleaving fixtures
# to this file; the two barrier points they consume — `pre-term` and `post-kill-pre-annotate` — are
# proven placed by the last section here.
#
# Contract: scripts/gate-run.md. Prologue and sandbox: tests/lib/gate_run_common.sh.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/gate_run_common.sh"

# ---- STOPPING A LIVE CHILD: the annotation path, and it is the ORDINARY one -------------
# The wrapper IGNORES TERM and outlives the group-directed signal just long enough to reap the
# command and record `kind=signal` — so on every child that dies of the TERM, a terminal record is
# already on disk by the time step 5 can verify the group empty (the wrapper leads that group, so it
# cannot be gone before the wrapper exited, and it writes the record before it exits). The report is
# therefore `already-terminal` — the record outranking the stop, exactly as at steps 1 and 3 — and
# the marker is written as an ANNOTATION so the cancellation is not read as an accidental death.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
live_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
stop_out="$(gate_run --stop "$RD" --reason 'budget exhausted' 2>"$SBX/stop1.err")"; stop_rc=$?
assert "stopping a live child reports already-terminal — the wrapper recorded the signal death" \
  '[ "$stop_out" = "already-terminal" ]'
assert "a determined outcome exits 0" '[ "$stop_rc" = "0" ]'
assert "the recorded group is verified gone before --stop reports" '! kill -0 -"$live_pgid" 2>/dev/null'
assert "a completed stop marker exists" '[ -f "$RD/stopped" ]'
assert "the stop-intent record was written on the signalling path" '[ -f "$RD/stop-intent" ]'
intent="$(cat "$RD/stop-intent" 2>/dev/null || true)"
# THE INTENT CLAIMS ONLY THAT A SIGNAL IS IMMINENT. It is written before the kill precisely so a
# `--stop` that dies mid-flight still leaves evidence the death was deliberate — but it must never
# be readable as a claim that termination HAPPENED, or the marker-before-verification invariant
# would be defeated by the very record that exists to survive a crash.
assert "the intent never claims termination happened" '! grep -qE "stopped_at=|^kind=" <<<"$intent"'
assert "the intent carries the caller's reason" 'grep -qF -- "budget exhausted" <<<"$intent"'
term_rec="$(cat "$RD/terminal" 2>/dev/null || true)"
assert "the terminal record here is the WRAPPER's signal record, not a --stop synthesis" \
  '[ "${term_rec%% *}" = "kind=signal" ]'
assert "a stopped run observes as stopped — never died, never passed" \
  '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

# `--reason` is caller-supplied free text and the intent is a KEY=value record every reader parses
# with a line-oriented `sed -n 's/^key=//p'` — so the flattening happens unconditionally at the
# WRITE boundary, not by trusting the shape of what arrives. An embedded newline would otherwise
# forge a field, and the field worth forging is the one every signal in this file is keyed on.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
forge_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
gate_run --stop "$RD" --reason "$(printf 'abandoned\npgid=999999')" >/dev/null 2>&1
intent_lines="$(wc -l <"$RD/stop-intent" 2>/dev/null || echo 0)"; intent_lines="${intent_lines//[[:space:]]/}"
assert "free text in --reason cannot forge a second record field" '[ "$intent_lines" = "1" ]'
assert "and no forged pgid field reached the intent record" '! grep -q "^pgid=" "$RD/stop-intent"'
reap "$forge_pgid"

# ---- KILL ESCALATION: a child that ignores TERM is still gone after the grace -------------
# `trap "" TERM` in the command sets the disposition to IGNORE, which is inherited across fork and
# exec — so the command, its `sleep`, AND the wrapper (which ignores TERM for its own reasons) all
# survive the signal. Only the escalation removes them, and a KILLed group leaves NO record, which
# is why this is also the case where `--stop` must produce its `stopped` token on its own evidence.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'trap "" TERM; sleep 60')"
kill_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
# Non-vacuity, and it doubles as the settle: TERM the group ourselves first. A group that died here
# would have left a terminal record well inside the bounded wait, and the escalation below would
# never be exercised at all.
kill -TERM -"$kill_pgid" 2>/dev/null || true
assert "the fixture really ignores TERM, so the escalation below is exercised" \
  '! await_terminal "$RD" 10 && kill -0 -"$kill_pgid" 2>/dev/null'
kill_out="$(gate_run --stop "$RD" 2>/dev/null)"
assert "a TERM-ignoring child is still stopped" '[ "$kill_out" = "stopped" ]'
assert "KILL escalation removed the group" '! kill -0 -"$kill_pgid" 2>/dev/null'
assert "the completed marker records the verified stop" '[ -f "$RD/stopped" ]'
assert "--stop wrote NO terminal record — a KILLed group leaves none and --stop synthesizes none" \
  '[ ! -f "$RD/terminal" ]'
assert "a KILL-escalated stop observes as stopped" '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

# ---- IDEMPOTENCE: a second call, and a call on a run that finished on its own --------------
second_out="$(gate_run --stop "$RD" 2>/dev/null)"; second_rc=$?
assert "a second --stop reports already-terminal" '[ "$second_out" = "already-terminal" ]'
assert "a second --stop is not an error" '[ "$second_rc" = "0" ]'
assert "a second --stop leaves the completed marker alone" '[ -f "$RD/stopped" ]'
assert "and the run still observes as stopped" '[ "$(gate_run --observe "$RD")" = "state=stopped" ]'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'exit 0')"; await_terminal "$RD"
passed_out="$(gate_run --stop "$RD" 2>/dev/null)"; passed_rc=$?
assert "--stop on an already-passed run reports already-terminal" '[ "$passed_out" = "already-terminal" ]'
assert "and that is not an error either" '[ "$passed_rc" = "0" ]'
# THE STOP MAY NOT ANNOTATE A RUN IT DID NOT KILL. A marker here would be a claim of deliberate
# termination over a verdict the child reached by itself.
assert "and it writes no marker over a run that finished on its own" '[ ! -f "$RD/stopped" ]'
assert "and no intent either — nothing was signalled" '[ ! -f "$RD/stop-intent" ]'
assert "and it observes as passed still — the stop did not reclassify it" \
  '[ "$(gate_run --observe "$RD")" = "state=passed" ]'

# ---- --stop NEVER WRITES A TERMINAL RECORD, and that is a property of the CODE --------------
# The behavioural half is the KILL-escalation assert above (`[ ! -f "$RD/terminal" ]` on a run whose
# wrapper was KILLed before it could write one). This is the structural half: synthesizing a
# terminal record from the stop path would report a run that never finished as one that did — the
# exact conflation `died` exists to prevent — so the stop body is checked for a WRITE of it. Reads
# are expected and required: steps 1, 3 and 6 all read that file.
stop_body="$(awk '/^stop_run\(\) \{/ {inblock=1} inblock {print} inblock && /^\}/ {exit}' "$GATE_RUN")"
stop_body_lines="$(wc -l <<<"$stop_body")"; stop_body_lines="${stop_body_lines//[[:space:]]/}"
assert "the stop body really was extracted, or the guard below is vacuous" \
  '[ "$stop_body_lines" -gt 30 ] && grep -qF -- "already-terminal" <<<"$stop_body"'
stop_terminal_writes="$(grep -nE '^[^#]*(atomic_write|>)[^#]*terminal' <<<"$stop_body" || true)"
assert "--stop never WRITES a terminal record — only the wrapper may" '[ -z "$stop_terminal_writes" ]'

# ---- THE IDENTITY GUARD ON THE SIGNAL PATH: a recycled pgid is signalled NOT AT ALL ---------
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
dead_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
recorded_ident="$(cat "$RD/identity" 2>/dev/null || true)"
kill -KILL -"$dead_pgid" 2>/dev/null || true
assert "the run's own group is verified gone before the bystander is planted" \
  'await_group_gone "$dead_pgid"'
# The one-second separation is LOAD-BEARING: `ps -o lstart=` resolves to whole seconds, so a
# bystander started inside the same second as the dead leader would carry an identical token and the
# mismatch this fixture exists to create would not exist.
sleep 1
foreign_pgid="$(start_foreign_group)"
foreign_ident="$(ps -o lstart= -p "$foreign_pgid" 2>/dev/null | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//')"
assert "the bystander really leads a live foreign group" \
  '[ -n "$foreign_pgid" ] && kill -0 -"$foreign_pgid" 2>/dev/null'
assert "the fixture really is an identity MISMATCH, or the guard below is vacuous" \
  '[ -n "$recorded_ident" ] && [ -n "$foreign_ident" ] && [ "$foreign_ident" != "$recorded_ident" ]'
sed -i.bak "s/^pgid=.*/pgid=$foreign_pgid/" "$RD/launch"
recycle_out="$(gate_run --stop "$RD" 2>"$SBX/recycle.err")"; recycle_rc=$?
assert "a recycled group reports unavailable" '[ "$recycle_out" = "unavailable" ]'
assert "unavailable exits non-zero — no outcome could be determined" '[ "$recycle_rc" != "0" ]'
assert "the ownership-unprovable sub-reason goes to stderr" \
  'grep -qF -- "ownership-unprovable" "$SBX/recycle.err"'
assert "and no sub-reason reached the protocol channel" \
  '! grep -qF -- "ownership-unprovable" <<<"$recycle_out"'
assert "and the bystander group was NOT signalled" 'kill -0 -"$foreign_pgid" 2>/dev/null'
assert "and nothing was written — no intent, no marker" \
  '[ ! -f "$RD/stop-intent" ] && [ ! -f "$RD/stopped" ]'
reap "$foreign_pgid"

# ---- THE VANISHED-GROUP GATE: no record, group absent ⇒ NO MARKER ----------------------------
# Step 7 is gated on the group having actually been SIGNALLED. Without that gate a `--stop` on an
# already-vanished group mints a `stopped` for a signal never sent — the report and the marker then
# disagree, and the vanished-death relaunch leg becomes unreachable because the re-observe reads
# `stopped` and declines the relaunch.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
van_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -KILL -"$van_pgid" 2>/dev/null || true
assert "the group is verified gone, or the branch under test is not the one taken" \
  'await_group_gone "$van_pgid"'
assert "a KILLed group left no terminal record" '[ ! -f "$RD/terminal" ]'
van_out="$(gate_run --stop "$RD" 2>/dev/null)"; van_rc=$?
assert "a vanished group reports already-terminal" '[ "$van_out" = "already-terminal" ]'
assert "and that is not an error" '[ "$van_rc" = "0" ]'
assert "and writes NO stop marker — the vanished death must stay relaunchable" '[ ! -f "$RD/stopped" ]'
assert "and no stop-intent either — nothing was ever signalled" '[ ! -f "$RD/stop-intent" ]'
assert "so it still observes as died cause=vanished" \
  '[ "$(gate_run --observe "$RD" 2>/dev/null)" = "state=died cause=vanished" ]'

# ---- THE ORPHAN PROBE on the record-present path ---------------------------------------------
# The leader is dead, so ownership of anything still under the recorded pgid is unprovable — but
# DETECTION is possible where safe signalling is not, and reporting `already-terminal` over live
# orphans is what makes the relaunch leg race a suite that is still running.
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
orph_run_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
kill -TERM -"$orph_run_pgid" 2>/dev/null || true
assert "the wrapper recorded the signal death, so the record-present path is the one taken" \
  'await_terminal "$RD" && grep -q "^kind=signal" "$RD/terminal"'
assert "the run's own group is gone before the orphan is planted" 'await_group_gone "$orph_run_pgid"'
orphan_pgid="$(start_foreign_group)"
assert "the planted orphan really leads a live group" \
  '[ -n "$orphan_pgid" ] && kill -0 -"$orphan_pgid" 2>/dev/null'
sed -i.bak "s/^pgid=.*/pgid=$orphan_pgid/" "$RD/launch"
orph_out="$(gate_run --stop "$RD" 2>"$SBX/orphan.err")"; orph_rc=$?
assert "live orphans under a present record report unavailable" '[ "$orph_out" = "unavailable" ]'
assert "and that is non-zero" '[ "$orph_rc" != "0" ]'
assert "the orphans-detected sub-reason goes to stderr" 'grep -qF -- "orphans-detected" "$SBX/orphan.err"'
assert "and nothing was signalled — ownership is unprovable, detection is not" \
  'kill -0 -"$orphan_pgid" 2>/dev/null'
assert "and no marker was written over the record" '[ ! -f "$RD/stopped" ]'
reap "$orphan_pgid"

# ---- A RUN DIR OR A RECORD THAT CANNOT NAME A GROUP IS unavailable, NEVER already-terminal ----
# `already-terminal` is a claim about the run; a stop that cannot even name the group has no grounds
# to make one. Every leg here is also a signalling precondition: `kill … -0` means the CALLER'S OWN
# process group and `kill … -1` means every process the user can signal, so neither may ever be
# treated as a recorded run's group — as a probe each answers for a bystander, and as a signal each
# takes the caller (or the machine) down with it.
missing_out="$(gate_run --stop "$SBX/no-such-run" 2>"$SBX/missing.err")"; missing_rc=$?
assert "a nonexistent run dir reports unavailable" '[ "$missing_out" = "unavailable" ]'
assert "and exits non-zero" '[ "$missing_rc" != "0" ]'
assert "with rundir-unreadable on stderr, never on stdout" \
  'grep -qF -- "rundir-unreadable" "$SBX/missing.err" && ! grep -qF -- "rundir-unreadable" <<<"$missing_out"'

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
guard_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
for bogus in "" 0 1 notanumber; do
  sed -i.bak "s/^pgid=.*/pgid=$bogus/" "$RD/launch"
  bogus_out="$(gate_run --stop "$RD" 2>"$SBX/bogus.err")"
  assert "a recorded pgid of '${bogus:-<empty>}' reports unavailable, never already-terminal" \
    '[ "$bogus_out" = "unavailable" ]'
  # AND THE REFUSAL IS THE UNUSABLE-RECORD ONE — the record is rejected before anything is probed
  # with it. Keyed on the sub-reason rather than the token because the token alone cannot tell this
  # guard from the identity check downstream: `kill … -0` and `kill … -1` both SUCCEED as probes
  # (the caller's own group, and every process the user can signal), so without the syntactic floor
  # those two are probed at all and only their identity mismatch saves the caller.
  assert "and the refusal names the unusable record, before any probe is made with it" \
    'grep -qF -- "names no usable process group" "$SBX/bogus.err"'
done
own_pgid="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
sed -i.bak "s/^pgid=.*/pgid=$own_pgid/" "$RD/launch"
self_out="$(gate_run --stop "$RD" 2>"$SBX/self.err")"
assert "a record naming the CALLER'S OWN process group reports unavailable" '[ "$self_out" = "unavailable" ]'
assert "and it is refused AS the caller's own group, not merely caught by the identity check" \
  'grep -qF -- "it is this process'"'"'s own group" "$SBX/self.err"'
assert "and none of those probes signalled anything — the real group is untouched" \
  'kill -0 -"$guard_pgid" 2>/dev/null'
reap "$guard_pgid"

# ---- THE TWO RENDEZVOUS POINTS, AND THE ORDERING EACH ONE PINS --------------------------------
# Task 6's five interleaving fixtures are held at exactly these two points, so a point placed one
# step off would silently weaken every one of them. Both are proven placed HERE, and each placement
# is itself an ordering assert this task owes: nothing is signalled before `pre-term`, and
# termination is verified before `post-kill-pre-annotate` — which is before any marker is written.

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
bar_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
BAR="$SBX/pre-term-barrier"
( GATE_RUN_TEST_BARRIER=pre-term GATE_RUN_TEST_BARRIER_FILE="$BAR" \
    gate_run --stop "$RD" >"$SBX/pre-term.out" 2>/dev/null ) &
bar_job=$!
assert "the stop really is held at the pre-term rendezvous" 'wait_for_file "$BAR.reached"'
assert "NOTHING has been signalled at pre-term — the recorded group is still alive" \
  'kill -0 -"$bar_pgid" 2>/dev/null'
assert "and the stop-intent is unwritten — it belongs immediately before the signal, not before the probe" \
  '[ ! -f "$RD/stop-intent" ]'
touch "$BAR.release"
wait "$bar_job" 2>/dev/null || true
assert "released, the held stop completes normally" \
  '[ "$(cat "$SBX/pre-term.out" 2>/dev/null)" = "already-terminal" ]'
assert "and the group it was held over is gone" '! kill -0 -"$bar_pgid" 2>/dev/null'

# STEP 4 READS BOTH CONJUNCTS ON THE KILL'S SIDE OF THE FENCE. The step-2 validation and the signal
# are separated by a window, and a group recycled inside it must be refused — which nothing carried
# down from step 2 can see, because the stale value is precisely what the TERM would be aimed at.
# `pre-term` is exactly that boundary, so this is the only place the re-read is observable at all:
# in every unheld run the two reads see the same world. (Task 6's fixture 5 is this interleaving.)
RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
window_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
BAR3="$SBX/stop-window-barrier"
( GATE_RUN_TEST_BARRIER=pre-term GATE_RUN_TEST_BARRIER_FILE="$BAR3" \
    gate_run --stop "$RD" >"$SBX/window.out" 2>/dev/null ) &
win_job=$!
assert "the stop is held between its step-2 validation and its step-4 probe" \
  'wait_for_file "$BAR3.reached"'
kill -KILL -"$window_pgid" 2>/dev/null || true
assert "the run's own group dies while the stop is held" 'await_group_gone "$window_pgid"'
sleep 1                                               # whole-second lstart resolution, as above
window_foreign="$(start_foreign_group)"
assert "a live bystander is planted under the recorded pgid while the stop is held" \
  '[ -n "$window_foreign" ] && kill -0 -"$window_foreign" 2>/dev/null'
sed -i.bak "s/^pgid=.*/pgid=$window_foreign/" "$RD/launch"
touch "$BAR3.release"
wait "$win_job" 2>/dev/null || true
assert "a group recycled inside the stop window reports unavailable" \
  '[ "$(cat "$SBX/window.out" 2>/dev/null)" = "unavailable" ]'
assert "and the bystander was never signalled" 'kill -0 -"$window_foreign" 2>/dev/null'
assert "and nothing was written — no intent, no marker" \
  '[ ! -f "$RD/stop-intent" ] && [ ! -f "$RD/stopped" ]'
reap "$window_foreign"

RD="$(gate_run --launch --root "$SBX/runs" -- /bin/sh -c 'sleep 60')"
ann_pgid="$(sed -n 's/^pgid=//p' "$RD/launch")"
BAR2="$SBX/post-kill-barrier"
( GATE_RUN_TEST_BARRIER=post-kill-pre-annotate GATE_RUN_TEST_BARRIER_FILE="$BAR2" \
    gate_run --stop "$RD" >"$SBX/post-kill.out" 2>/dev/null ) &
ann_job=$!
assert "the stop really is held at the post-kill rendezvous" 'wait_for_file "$BAR2.reached"'
assert "termination was VERIFIED before the rendezvous — the group is already gone" \
  '! kill -0 -"$ann_pgid" 2>/dev/null'
assert "and no completed marker exists yet: verification precedes the marker, never the reverse" \
  '[ ! -f "$RD/stopped" ]'
assert "while the stop-intent, written before the signal, is already durable" '[ -f "$RD/stop-intent" ]'
touch "$BAR2.release"
wait "$ann_job" 2>/dev/null || true
assert "released, the annotation path completes" \
  '[ "$(cat "$SBX/post-kill.out" 2>/dev/null)" = "already-terminal" ]'
assert "and only now does the completed marker exist" '[ -f "$RD/stopped" ]'

exit "$fail"

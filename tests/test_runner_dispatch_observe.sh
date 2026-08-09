#!/usr/bin/env bash
# tests/test_runner_dispatch_observe.sh — the OBSERVE half of the change-0271 detachment posture:
# the still-running / complete / failed dispositions, the stdout relay, durability across
# `git worktree remove`, the observation budget, and the identity-checked group kill it ends in.
# Run: bash tests/test_runner_dispatch_observe.sh
#
# Sharded out of tests/test_runner_dispatch_detach.sh, which keeps the launch half; the build
# verdict family and implement-next's run gate live in tests/test_runner_dispatch_build_gate.sh.
# tests/lib/runner_dispatch_detach_common.sh carries the shared prologue, fixtures and helpers,
# and its header records why the file was cut into three.
# shellcheck source=lib/runner_dispatch_detach_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/runner_dispatch_detach_common.sh"

# ---- observe: still running -> 4, terminal -> 0 ---------------------------------

make_fixture
FAKE_SLEEP=6 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "observe on a live child exits 4 (still running)" '[ "$rc" = "4" ]'
assert "observe says still running" 'grep -qi "still running" <<<"$out"'
# The observation must be SHORT — it may not become the long foreground call all over again.
start=$(date +%s)
observe "$KEY" >/dev/null 2>&1
assert "an observation is short-lived" '[ $(( $(date +%s) - start )) -lt 10 ]'

# An unparseable budget is NOT "spent". Read as one it would kill a healthy child over a typo in
# an environment variable, so the value is normalized to the shipped default instead.
BUDGET="not-a-number"
observe "$KEY" >/dev/null 2>&1; rc=$?
BUDGET=60
assert "an unparseable budget keeps observing rather than killing" '[ "$rc" = "4" ]'

for _ in $(seq 1 40); do
  observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1
done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "observe after a clean child exits 0" '[ "$rc" = "0" ]'

# IDEMPOTENCE: same inputs, same verdict and code, every time.
out2="$(observe "$KEY" 2>&1)"; rc2=$?
assert "observe is idempotent in code" '[ "$rc2" = "$rc" ]'
assert "observe is idempotent in output" '[ "$out2" = "$out" ]'

# ---- THE RELAY: the child's stdout reaches observe's OWN stdout ------------------
# `--launch` redirects the adapter's streams into the dispatch dir, so this is the ONLY channel by
# which an in-context-report agent's result (a build worker's COMPLETE line, a reviewer's findings)
# can reach its caller — the shim is told to "Relay that observe call's stdout as your result", and
# without this that instruction is unsatisfiable. Captured with stderr DISCARDED, so nothing a
# diagnostic prints can satisfy these asserts.
rout="$(observe "$KEY" 2>/dev/null)"
assert "0271: observe relays the child's stdout on ITS OWN stdout" \
  'grep -qF "fake adapter stdout" <<<"$rout"'
assert "0271: the relay carries the child's stdout only, never its stderr" \
  '! grep -qF "fake adapter stderr" <<<"$rout"'
# Diagnostics are all `runner-dispatch: …`-prefixed, so their absence is checked on the prefix
# rather than on any one message's wording.
assert "0271: no diagnostic leaks onto the relay stream" \
  '! grep -qF "runner-dispatch:" <<<"$rout"'
# VERBATIM: not merely "contains". A prefix, a banner, or a reformat would corrupt a report the
# caller parses, so the relayed bytes must equal the child's stdout exactly.
assert "0271: the relay is verbatim — no prefix, no reformat" '[ "$rout" = "fake adapter stdout" ]'

# ---- the recorded result OUTLIVES `git worktree remove` -------------------------
# The dispatch dir lives under the git COMMON dir so a result survives the removal of the worktree
# it was launched against. Storing it there is only half the claim: the facade's own reader anchored
# on the worktree, so once that directory was gone every observation died on "not a directory" and
# the recorded result was reachable by a human with a shell and by nothing else. The root is
# repo-wide, so an observation falls back to the main worktree and still reports the result.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
WTGONE="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach-wt.XXXXXX")"; FIXTURES+=("$WTGONE"); rmdir "$WTGONE"
git -C "$SBX" worktree add -q -b feat/gone "$WTGONE" 2>/dev/null
KEY="$( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0 \
    bash "$FACADE" --launch --runner fake --agent status --worktree "$WTGONE" )"
DDIR="$(ddir_for "$KEY")"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "0271: fixture sanity — the dispatch finished while its worktree still existed" '[ -f "$DDIR/done" ]'
git -C "$SBX" worktree remove --force "$WTGONE" 2>/dev/null
assert "0271: fixture sanity — the anchor worktree is really gone" '[ ! -d "$WTGONE" ]'
gone_out="$( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" \
    --runner fake --agent status --worktree "$WTGONE" 2>&1 )"; gone_rc=$?
assert "0271: an observation still reports its recorded result after the worktree was removed" \
  '[ "$gone_rc" = "0" ]'
assert "0271: and says why it read the root from the main worktree instead" \
  'grep -qi "no longer exists" <<<"$gone_out"'
gone_rout="$( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" \
    --runner fake --agent status --worktree "$WTGONE" 2>/dev/null )"
assert "0271: the relay survives the removal too" 'grep -qF "fake adapter stdout" <<<"$gone_rout"'
# The FALLBACK IS SCOPED: it fires on a MISSING anchor only. A path that exists but is not this
# repository's worktree is still refused, so the durability leg widens nothing.
FOREIGN="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach-foreign.XXXXXX")"; FIXTURES+=("$FOREIGN")
fgn_out="$( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" \
    --runner fake --agent status --worktree "$FOREIGN" 2>&1 )"; fgn_rc=$?
assert "0271: a foreign anchor is still refused, not silently redirected" '[ "$fgn_rc" != "0" ]'
assert "0271: and the refusal is the worktree-of-this-repository gate" \
  'grep -qi "not a worktree of this repository" <<<"$fgn_out"'

# ---- a failed child -> 1 --------------------------------------------------------
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=9
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "a non-zero adapter code observes as failed (1)" '[ "$rc" = "1" ]'
assert "the failure diagnostic reports the child's code" 'grep -qF "exited 9" <<<"$out"'
# A failing child still said something, and that is what a caller has to report. The relay is a
# terminal-path obligation, not a success-path one.
rout="$(observe "$KEY" 2>/dev/null)"
assert "0271: a failed child's stdout is relayed too" 'grep -qF "fake adapter stdout" <<<"$rout"'
assert "0271: and the failure diagnostic still stays off stdout" \
  '! grep -qF "runner-dispatch:" <<<"$rout"'

# ---- a STILL-RUNNING observation relays NOTHING ---------------------------------
# The shim observes repeatedly, so a partial relay on the `4` path would hand the caller the same
# prefix once per pass. The fake is shaped to make that a real measurement: it writes its stdout
# IMMEDIATELY and then lingers with no sentinel, so `stdout.log` is non-empty at the moment of the
# still-running observation — an unconditional relay would emit it here and redden the assert.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=10 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
live_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
for _ in $(seq 1 40); do [ -s "$DDIR/stdout.log" ] && break; sleep 0.2; done
assert "0271: fixture sanity — the live child has already written stdout" '[ -s "$DDIR/stdout.log" ]'
live_out="$(observe "$KEY" 2>/dev/null)"; live_rc=$?
assert "0271: fixture sanity — that observation took the still-running path" '[ "$live_rc" = "4" ]'
assert "0271: a still-running observation emits nothing on stdout" '[ -z "$live_out" ]'
reap "$live_pgid"
FAKE_TAIL=0

# ---- a key that is not a live mint is a usage error, never a verdict ------------
out="$(observe "no-such-key-0000" 2>&1)"; rc=$?
assert "an unknown dispatch key aborts" '[ "$rc" != "0" ]'
assert "the unknown-key diagnostic names the key" 'grep -qF "no-such-key-0000" <<<"$out"'
# The key becomes a path component, so it earns --runner's traversal refusal. `..` is the
# discriminating case precisely because it names a directory that EXISTS: without a shape gate it
# is observed as "still running" forever, a verdict manufactured out of a typo.
out="$(observe ".." 2>&1)"; rc=$?
assert "a traversing dispatch key is refused" '[ "$rc" = "1" ]'
assert "the traversal refusal calls the key invalid" 'grep -qi "invalid dispatch key" <<<"$out"'

# ---- budget exhaustion kills the GROUP and reports unavailable ------------------
# The orphan policy (honors change 0231): no unwatched agent keeps working after the run was
# declared failed. The fake is shaped so the ORPHAN HAZARD IS MEASURABLE — it writes its marker
# 4s in and then lingers, so a signal aimed at the launcher PID instead of the GROUP leaves the
# adapter alive to finish, and the marker assert below turns red.
make_fixture
FAKE_SLEEP=4 FAKE_TAIL=60 FAKE_RC=0
BUDGET=0                       # legal, and buys exactly ONE observation
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "budget exhaustion exits 1" '[ "$rc" = "1" ]'
assert "the diagnostic distinguishes unavailable from failed" 'grep -qi "unavailable" <<<"$out"'
assert "the detached process group was killed" '! kill -0 -"$lpgid" 2>/dev/null'
assert "a killed marker was recorded" '[ -f "$DDIR/killed" ]'
# Waited PAST the instant a surviving adapter would have written its marker, which is what makes
# the next assert a measurement of the group kill rather than of the clock.
sleep 6
assert "the adapter never completed its work" '[ ! -f "$SBX/marker" ]'
# Deterministic re-observation AFTER the terminal kill.
out2="$(observe "$KEY" 2>&1)"; rc2=$?
assert "re-observing a killed dispatch stays unavailable (1)" '[ "$rc2" = "1" ]'
assert "re-observation after the kill is deterministic" 'grep -qi "unavailable" <<<"$out2"'
reap "$lpgid"
BUDGET=60
FAKE_TAIL=0

# ---- the budget kill signals only a group PROVEN to be the launched child's -------
# The arm above is the positive half — the group really is the child's, and it dies. This is the
# negative half: a pgid is a REUSABLE name, and the budget path is reached only when no sentinel
# exists, which includes a child that died an hour earlier without one. By then the OS may have
# handed that id to an unrelated tree, and a group-directed TERM/KILL reaches all of it.
#
# THE BYSTANDER stands in for that unrelated tree: a sleeper isolated into ITS OWN process group
# under `set -m`, so a group-directed signal aimed at it can never reach the harness running this
# file. The record is then rewritten to name the BYSTANDER's group while `child_pid` still names
# the launched child — exactly the shape of a recycled pgid — and the measurement is that the
# bystander is STILL ALIVE afterwards.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
real_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
set -m
( sleep 20 ) &
by_pid=$!
set +m
by_pgid="$(ps -o pgid= -p "$by_pid" 2>/dev/null | tr -d ' ')"
mypgid="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
assert "0271: fixture sanity — the bystander leads its OWN group, not the test's" \
  '[ -n "$by_pgid" ] && [ "$by_pgid" = "$by_pid" ] && [ "$by_pgid" != "$mypgid" ]'
rec="$(sed "s/^pgid=.*/pgid=$by_pgid/" "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
BUDGET=0
out="$(observe "$KEY" 2>&1)"; rc=$?
BUDGET=60
assert "0271: an unconfirmable group still reports unavailable (1)" '[ "$rc" = "1" ]'
assert "0271: and says nothing was signalled" 'grep -qi "not signalling" <<<"$out"'
# THE ASSERT THE GUARD EXISTS FOR. With the identity check removed, the bystander's whole group
# takes TERM and then KILL, and this goes red.
assert "0271: THE UNRELATED GROUP SURVIVED — no signal was sent to a pgid we could not confirm" \
  'kill -0 "$by_pid" 2>/dev/null'
assert "0271: the dispatch is still recorded terminal" '[ -f "$DDIR/killed" ]'
assert "0271: the marker records that nothing was signalled, not that we killed it" \
  'grep -qxF "reason=group-already-gone" "$DDIR/killed"'
# IDEMPOTENCE across the terminal transition: the re-report is unavailable, deterministic, and
# still sends nothing.
BUDGET=0
out2="$(observe "$KEY" 2>&1)"; rc2=$?
out3="$(observe "$KEY" 2>&1)"; rc3=$?
BUDGET=60
assert "0271: re-observing an unsignalled kill stays unavailable (1)" '[ "$rc2" = "1" ] && [ "$rc3" = "1" ]'
assert "0271: and re-reports identically forever" '[ "$out3" = "$out2" ]'
assert "0271: the re-report does not claim a kill that never happened" \
  '! grep -qi "was killed at budget exhaustion" <<<"$out2"'
assert "0271: the bystander survives re-observation too" 'kill -0 "$by_pid" 2>/dev/null'
reap "$by_pgid"
reap "$real_pgid"

# ---- pid->group agreement is not enough: the START TIME is checked too -----------
# A recycled pid that leads a group of the SAME id satisfies the pid->group conjunct on its own —
# pid reuse is precisely what makes the hazard reachable. The launch therefore records the OS's own
# start time for the child as an opaque token, and the observation compares it verbatim. Driven
# here by rewriting the token (the pid and the group are left REAL), so the surviving process is
# the launched child itself.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
real_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
assert "0271: the launch record carries the child's start-time token" \
  'grep -qE "^child_lstart=.+" "$DDIR/launch"'
rec="$(sed 's/^child_lstart=.*/child_lstart=Thu Jan  1 00:00:00 1970/' "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
BUDGET=0
out="$(observe "$KEY" 2>&1)"; rc=$?
BUDGET=60
assert "0271: a start time that does not match the launch's is unavailable (1)" '[ "$rc" = "1" ]'
assert "0271: the diagnostic names pid recycling" 'grep -qi "recycled" <<<"$out"'
assert "0271: and the group whose leader looks recycled is NOT signalled" \
  'kill -0 -"$real_pgid" 2>/dev/null'
reap "$real_pgid"

# ---- observe REFUSES to signal its OWN process group -----------------------------
# Defense in depth: `--launch` already fails closed when the child did not separate, so a launch
# record it wrote can only name a foreign group. This covers the record being wrong anyway — a
# hand-edited record, a pgid reused after the group died — because a group-directed signal aimed
# at the observer's own group takes down the harness that ran it, the one failure this facade
# must never cause.
# The observation runs as a background job under `set -m`, so it LEADS ITS OWN GROUP: if this
# guard is ever removed, the self-kill reaps only that job and never this test file.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
real_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
set -m
( sleep 2
  BUDGET=0 observe "$KEY" >"$SBX/self.out" 2>&1
  printf '%s\n' "$?" > "$SBX/self.rc" ) &
self_pid=$!
set +m
self_pgid="$(ps -o pgid= -p "$self_pid" 2>/dev/null | tr -d ' ')"
rec="$(sed "s/^pgid=.*/pgid=${self_pgid:-0}/" "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
wait "$self_pid" 2>/dev/null
self_rc="$(sed -n 1p "$SBX/self.rc" 2>/dev/null)"
self_out="$(sed -n '1,20p' "$SBX/self.out" 2>/dev/null)"
assert "the observer's own group is never signalled (it refuses, code 1)" '[ "$self_rc" = "1" ]'
assert "and it says so instead of dying of its own signal" 'grep -qi "own process group" <<<"$self_out"'
assert "and nothing is recorded as killed, because nothing was killed" '[ ! -f "$DDIR/killed" ]'
reap "$real_pgid"

# ---- a malformed sentinel with a dead child is UNAVAILABLE, never a fake failure -
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
printf 'garbage-not-a-schema\n' > "$DDIR/done"
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "a malformed sentinel is unavailable, not a synthesized failure" '[ "$rc" = "1" ]'
assert "the malformed-sentinel diagnostic says unavailable" 'grep -qi "unavailable" <<<"$out"'
assert "no exit code was read out of garbage" '! grep -qi "exited garbage" <<<"$out"'

# ---- an UNENFORCEABLE budget must still TERMINATE, not return 4 forever ----------
# `4` is the caller's loop condition, so a state that returns it unconditionally is a state the
# loop can never leave: the facade's "no positive evidence, so do not enforce" posture used to make
# an unreadable clock, an unreadable `started_at`, or a missing launch record permanent. Driven
# hermetically by corrupting `started_at` — the record's own field, so no clock or `ps` is faked.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
un_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
rec="$(sed 's/^started_at=.*/started_at=not-a-timestamp/' "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
un1="$(observe "$KEY" 2>&1)"; un_rc1=$?
un2="$(observe "$KEY" 2>&1)"; un_rc2=$?
un3="$(observe "$KEY" 2>&1)"; un_rc3=$?
assert "0271: an unenforceable pass still observes as still-running (4)" '[ "$un_rc1" = "4" ] && [ "$un_rc2" = "4" ]'
assert "0271: and says the budget was not enforced this pass" 'grep -qi "budget not enforced" <<<"$un1"'
# THE ASSERT THIS ARM EXISTS FOR: the Nth consecutive unenforceable pass is TERMINAL.
assert "0271: the 3rd consecutive unenforceable observation terminates (1)" '[ "$un_rc3" = "1" ]'
assert "0271: the terminating diagnostic reports the result unavailable" 'grep -qi "unavailable" <<<"$un3"'
assert "0271: and names WHY the budget could not be enforced" \
  'grep -qi "could not be enforced" <<<"$un3" && grep -qi "start time" <<<"$un3"'
# The give-up reuses the identity-checked kill path, so an identifiable group still dies rather
# than being orphaned — the same obligation the exhausted-budget arm above measures.
assert "0271: the unenforceable give-up still kills the detached group" '! kill -0 -"$un_pgid" 2>/dev/null'
assert "0271: and records the dispatch as terminal" '[ -f "$DDIR/killed" ]'
assert "0271: the marker names the CAUSE, not just whether a signal went out" \
  'grep -qxF "cause=budget-unenforceable" "$DDIR/killed"'
# IDEMPOTENCE ACROSS THE NEW TERMINAL TRANSITION: once given up on, it re-reports identically.
un4="$(observe "$KEY" 2>&1)"; un_rc4=$?
un5="$(observe "$KEY" 2>&1)"; un_rc5=$?
assert "0271: re-observing an unenforceable give-up stays terminal (1)" '[ "$un_rc4" = "1" ] && [ "$un_rc5" = "1" ]'
assert "0271: and re-reports identically forever" '[ "$un4" = "$un5" ]'
assert "0271: the re-report still names the unenforceable cause" 'grep -qi "could not be enforced" <<<"$un4"'
reap "$un_pgid"

# ---- an ENFORCEABLE pass RESETS the counter --------------------------------------
# A transient unreadable clock must not accumulate toward termination across an otherwise healthy
# run. Without the reset the two passes after the good one would be the 3rd and 4th and the first
# of them would terminate; with it they are the 1st and 2nd and both keep observing.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
rs_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
good_start="$(sed -n 's/^started_at=//p' "$DDIR/launch")"
assert "0271: fixture sanity — the launch recorded a start time to restore" '[ -n "$good_start" ]'
corrupt_start(){ local r; r="$(sed 's/^started_at=.*/started_at=not-a-timestamp/' "$DDIR/launch")"; printf '%s\n' "$r" > "$DDIR/launch"; }
restore_start(){ local r; r="$(sed "s/^started_at=.*/started_at=$good_start/" "$DDIR/launch")"; printf '%s\n' "$r" > "$DDIR/launch"; }
corrupt_start
observe "$KEY" >/dev/null 2>&1; rs_rc1=$?
observe "$KEY" >/dev/null 2>&1; rs_rc2=$?
assert "0271: fixture sanity — two unenforceable passes are still non-terminal" '[ "$rs_rc1" = "4" ] && [ "$rs_rc2" = "4" ]'
restore_start
rs_ok="$(observe "$KEY" 2>&1)"; rs_rc3=$?
assert "0271: an enforceable pass observes against the real budget" \
  '[ "$rs_rc3" = "4" ] && grep -qF "of 60m budget" <<<"$rs_ok"'
assert "0271: and clears the consecutive-unenforceable counter" '[ ! -f "$DDIR/unenforceable" ]'
corrupt_start
observe "$KEY" >/dev/null 2>&1; rs_rc4=$?
observe "$KEY" >/dev/null 2>&1; rs_rc5=$?
assert "0271: the count restarts after an enforceable pass" '[ "$rs_rc4" = "4" ] && [ "$rs_rc5" = "4" ]'
assert "0271: and the dispatch was NOT given up on" '[ ! -f "$DDIR/killed" ]'
reap "$rs_pgid"

# ---- the counter never touches a TERMINAL state's idempotence --------------------
# The counter is mutable state written by an observation, which is exactly why its scope matters:
# a completed dispatch must re-report identically forever, however many times it is observed.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
tid1="$(observe "$KEY" 2>&1)"; tid_rc1=$?
tid2="$(observe "$KEY" 2>&1)"; tid_rc2=$?
tid3="$(observe "$KEY" 2>&1)"; tid_rc3=$?
assert "0271: a completed dispatch stays 0 across repeated observations" \
  '[ "$tid_rc1" = "0" ] && [ "$tid_rc2" = "0" ] && [ "$tid_rc3" = "0" ]'
assert "0271: and re-reports byte-identically" '[ "$tid1" = "$tid2" ] && [ "$tid2" = "$tid3" ]'
assert "0271: a terminal state never writes the unenforceable counter" '[ ! -f "$DDIR/unenforceable" ]'

# ---- THE KILL WINDOW: a sentinel that lands between the "no sentinel" read and the kill ----
# The window runs from the `[ -f done ]` read to the signal, and it spans a `date`, a SUBPROCESS
# (`verify-run --iso-to-epoch`, a fork+exec of bash plus a source) and a `ps` — tens of
# milliseconds, not microseconds. A child that lands its work inside it used to be masked FOREVER
# by the `killed` marker and re-reported as RESULT UNAVAILABLE, sending a human to hunt for work
# that is in fact committed.
#
# Driven DETERMINISTICALLY rather than by luck, through the VERIFY_RUN mock seam: the stub is
# invoked for `--iso-to-epoch` at exactly one point on this path — AFTER the sentinel read, BEFORE
# anything can be signalled — so writing the sentinel from inside it plants the child's completion
# in the middle of the window on every single run. The epoch it prints is `0`, so the budget reads
# as exhausted and the give-up path is entered.
race_fixture(){  # sets SBX/RDIR/KEY/DDIR/RPGID: a live child plus a stub that finishes it mid-window
  make_fixture
  FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
  KEY="$(launch status)"
  DDIR="$(ddir_for "$KEY")"
  RPGID="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
  cat > "$SBX/race-vr.sh" <<VRE
#!/usr/bin/env bash
case "\$1" in
  --iso-to-epoch)
    # The child "finishes" HERE — written the way the wrapper writes it, atomically.
    printf 'exit_code=0\nstarted_at=x\nfinished_at=y\npid=1\ndispatch_key=$KEY\n' > "$DDIR/done.partial"
    mv -f "$DDIR/done.partial" "$DDIR/done"
    printf '0\n'; exit 0 ;;
esac
exit 0
VRE
  chmod +x "$SBX/race-vr.sh"
}
race_observe(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    VERIFY_RUN="$SBX/race-vr.sh" bash "$FACADE" --observe "$1" --runner fake --agent status ); }

# (1) the re-read IMMEDIATELY BEFORE THE SIGNAL, on the path where a signal really would go out.
race_fixture
rc_out="$(race_observe "$KEY" 2>&1)"; rc_rc=$?
assert "0271: fixture sanity — the stub really planted a sentinel inside the kill window" \
  '[ -f "$DDIR/done" ]'
assert "0271: a sentinel landing inside the kill window observes as COMPLETE (0), not unavailable" \
  '[ "$rc_rc" = "0" ]'
assert "0271: and no killed marker is written over a run that completed" '[ ! -f "$DDIR/killed" ]'
# THE ASSERT THE PRE-SIGNAL RE-READ EXISTS FOR: the post-kill read alone would still report `0`
# here, but only after needlessly signalling the group of a run that had already finished.
assert "0271: a completed run's process group is never signalled" 'kill -0 -"$RPGID" 2>/dev/null'
rc_out2="$(race_observe "$KEY" 2>&1)"; rc_rc2=$?
assert "0271: and the completed verdict is terminal from there on" '[ "$rc_rc2" = "0" ]'
assert "0271: re-reporting it identically" '[ "$rc_out2" = "$rc_out" ]'
reap "$RPGID"

# (2) the re-read AFTER THE KILL and BEFORE THE MARKER, driven on the path where no signal is sent:
#     a start-time token that does not match makes the group unconfirmable, so the pre-signal read
#     is unreachable and this second read stands alone between the sentinel and the marker write.
race_fixture
rec="$(sed 's/^child_lstart=.*/child_lstart=Thu Jan  1 00:00:00 1970/' "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
rc_out="$(race_observe "$KEY" 2>&1)"; rc_rc=$?
assert "0271: an UNSIGNALLED give-up yields to a sentinel that landed in the window too (0)" \
  '[ "$rc_rc" = "0" ]'
assert "0271: and writes no killed marker over that completed run either" '[ ! -f "$DDIR/killed" ]'
reap "$RPGID"
FAKE_SLEEP=0

# (3) ORDERING: the sentinel is read AHEAD of the `killed` marker. The correctness argument is that
#     a group TERM reaches the untrapped wrapper subshell, which cannot then write `done` — so a
#     sentinel present alongside a marker means the child completed BEFORE the signal, and the
#     completed disposition is the true one. Idempotence is unharmed: the sentinel never disappears,
#     so the verdict read from it is the same forever after.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "0271: fixture sanity — the child completed and left a sentinel" '[ -f "$DDIR/done" ]'
printf 'killed_at=1970-01-01T00:00:00Z\nreason=group-already-gone\ncause=budget-exhausted\ndetail=\nbudget_minutes=60\n' \
  > "$DDIR/killed"
ord_out="$(observe "$KEY" 2>&1)"; ord_rc=$?
assert "0271: a sentinel outranks a killed marker — a completed run is never reported unavailable" \
  '[ "$ord_rc" = "0" ]'
assert "0271: and the verdict spoken is the sentinel's, not the marker's" \
  'grep -qi "complete" <<<"$ord_out" && ! grep -qi "unavailable" <<<"$ord_out"'
ord_out2="$(observe "$KEY" 2>&1)"; ord_rc2=$?
assert "0271: that precedence is itself idempotent" \
  '[ "$ord_rc2" = "0" ] && [ "$ord_out2" = "$ord_out" ]'

exit "$fail"

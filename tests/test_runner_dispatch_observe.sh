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

# The identity-token reader, from the same lib the facade itself consults (change 0284). Three arms
# below must PIN a launch record's `child_lstart` to a LIVE process's real token: since 0284 the
# observation probes liveness BEFORE the clock, so a fixture that rewrites `pgid` to name some other
# live group and leaves the recorded token alone is decided by whether the two processes happened to
# start inside the same `ps -o lstart=` second — a coin flip, and a green one only by luck. Pinning
# the token makes those fixtures reach the give-up path they were written to measure. Read through
# the facade's own function rather than a second local copy, so a change to the normalization moves
# both sides at once.
# shellcheck source=../scripts/lib/docket-liveness.sh
. "$ROOT/scripts/lib/docket-liveness.sh"

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

# 0208: THE FALLBACK SURVIVES THE MAIN-TREE REJECTION, for a FEATURE-SCOPED agent. Change 0208 added
# gate 3b — a feature-scoped agent may not anchor at the main worktree — and the fallback above
# reassigns the anchor to exactly that. Unconditional, gate 3b would `die` here and convert this
# durability guarantee into a failed observation for precisely the agents that use it: a delegated
# `review-*` / `rebase-resolver` / `integration-repair` dispatch whose worktree finalize has since
# removed. It rides on the dispatch this block already launched (no second launch, no sleep) — the
# recorded agent does not gate the read, and what is under test is the ANCHOR gate, not the verdict.
# The declaration sanity assert is not decoration: with `worktree-scope:` absent from the source the
# agent would fall to the tolerant metadata default, gate 3b would never apply, and this leg would
# be green with the exemption deleted.
assert "0208: fixture sanity — review-lean really declares feature scope" \
  'grep -qx "worktree-scope: feature" "$ROOT/agents/docket-review-lean.md"'
fs_out="$( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$KEY" \
    --runner fake --agent review-lean --worktree "$WTGONE" 2>&1 )"; fs_rc=$?
assert "0208: a FEATURE-SCOPED observation still reports its result after the worktree was removed" \
  '[ "$fs_rc" = "0" ]'
# The negative leg is pinned on gate 3b's own clause `must run in a linked feature worktree`, which
# appears in no other message the facade can emit here — notably not in the fallback's own "under
# the main worktree <path>" line, which a bare "main worktree" match would collide with.
assert "0208: and it is the anchor FALLBACK it took, not the main-tree rejection" \
  'grep -qi "no longer exists" <<<"$fs_out" &&
   ! grep -qF -- "must run in a linked feature worktree" <<<"$fs_out"'

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
# The recorded TOKEN is pinned to the bystander's own start time as well as its pgid (change 0284).
# Without the pin, the liveness probe that now runs BEFORE the budget arithmetic decides this
# fixture on a one-second `ps -o lstart=` boundary: the bystander is spawned within a fraction of a
# second of the launched child, so the two tokens agree or disagree by luck, and on a disagreement
# the observation is disposed as `child-vanished` and never reaches the give-up path this arm
# exists to measure. Pinned, the group is provably ALIVE and provably the token's, so the only thing
# left to refuse the signal is `terminate_dispatch`'s own pid->group conjunct — which is the guard
# under test.
by_lstart="$(docket_identity_of "$by_pid")"
assert "0271: fixture sanity — the bystander has a readable start-time token to pin" '[ -n "$by_lstart" ]'
rec="$(sed -e "s/^pgid=.*/pgid=$by_pgid/" -e "s|^child_lstart=.*|child_lstart=$by_lstart|" "$DDIR/launch")"
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
#
# A MISMATCH IS `unprovable`, NEVER `gone` (change 0284 review, finding 1). The group is demonstrably
# ALIVE on this leg — `kill -0` answered for it — and what failed is a COMPARISON, which a token
# rendered by an older, unpinned build of `docket_identity_of` fails for a reason that has nothing to
# do with the process. So the first pass may not dispose the dispatch: it is counted, and the third
# consecutive one converts to the bounded terminal the unenforceable family already owns. THE BUDGET
# IS LEFT AT 60 — nowhere near spent — so nothing but the liveness leg can be driving this arm.
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
real_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
assert "0271: the launch record carries the child's start-time token" \
  'grep -qE "^child_lstart=.+" "$DDIR/launch"'
rec="$(sed 's/^child_lstart=.*/child_lstart=Thu Jan  1 00:00:00 1970/' "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
mm1="$(observe "$KEY" 2>&1)"; mm_rc1=$?
assert "0284-f1: a token that cannot be compared is NOT terminal on the first pass" '[ "$mm_rc1" = "4" ]'
assert "0284-f1: and no terminal marker is written over a child never proven dead" \
  '[ ! -f "$DDIR/killed" ]'
assert "0284-f1: the unprovable pass is COUNTED, which is what bounds it" \
  '[ "$(sed -n 1p "$DDIR/unenforceable" 2>/dev/null)" = "1" ]'
# FINDING 7: the wording may not assert a death that was never established.
assert "0284-f7: and it never claims the child died" '! grep -qi "died" <<<"$mm1"'
assert "0284-f7: it says only that the child could not be proven alive" \
  'grep -qi "could not be proven alive" <<<"$mm1"'
# THE OTHER HALF OF THE SAME REFUSAL, and it needs its own assert: the reason string alone carries
# the words "could not be proven alive", so the assert above is satisfied by a headline that ALSO
# announces the child as still running — a claim nothing on this leg established either. Pinned on
# the headline the clock family uses, which is a fact THERE (liveness was proven one step earlier)
# and an assertion here.
assert "0284-f7: nor that it is still running, which is equally unestablished" \
  '! grep -qi "still running" <<<"$mm1"'
assert "0284-f1: naming the comparison that failed" 'grep -qi "recycled" <<<"$mm1"'
mm2="$(observe "$KEY" 2>&1)"; mm_rc2=$?
assert "0284-f1: the second unprovable pass is still non-terminal" '[ "$mm_rc2" = "4" ]'
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0271: a start time that does not match the launch's is unavailable (1)" '[ "$rc" = "1" ]'
assert "0284-f1: the 3rd consecutive unprovable pass terminates on the BOUNDED cause" \
  'grep -qxF "cause=budget-unenforceable" "$DDIR/killed"'
assert "0284-f1: never as a vanishing, which is a claim nothing here established" \
  '! grep -qxF "cause=child-vanished" "$DDIR/killed"'
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
# The token is pinned to the observer's own group leader for the same reason as the bystander arm
# above (change 0284): the liveness probe now runs ahead of the budget arithmetic, and this guard
# lives INSIDE `terminate_dispatch`, one phase past it. With the token left alone the probe decides
# on a one-second boundary whether the recorded group looks like ours, and on the unlucky half the
# observation is disposed as `child-vanished` — never reaching the own-group refusal, and reddening
# both its wording and its no-marker assert. Pinned, the record describes a group that is alive and
# provably its own, so the own-group refusal is the first thing that can stop the signal.
self_lstart="$(docket_identity_of "${self_pgid:-0}")"
assert "0271: fixture sanity — the observer's own group leader has a readable token to pin" \
  '[ -n "$self_pgid" ] && [ -n "$self_lstart" ]'
rec="$(sed -e "s/^pgid=.*/pgid=${self_pgid:-0}/" -e "s|^child_lstart=.*|child_lstart=$self_lstart|" "$DDIR/launch")"
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
#     a recorded CHILD PID that no longer leads the recorded group makes the group unconfirmable, so
#     the pre-signal read is unreachable and this second read stands alone between the sentinel and
#     the marker write.
#     DRIVEN ON `child_pid`, NOT ON THE TOKEN (change 0284). Blanking the token was the original
#     driver, but the liveness leg now consults exactly that token BEFORE the clock read — and the
#     clock read is where this fixture's stub plants the sentinel. A mismatched token would dispose
#     the dispatch as `child-vanished` one step too early, with the sentinel not yet written and this
#     re-read never reached. `child_pid` is read only by `terminate_dispatch`'s own pid->group
#     conjunct, so it drives the same no-signal give-up while leaving the earlier probe's inputs
#     untouched. `999999` is above the platform's pid ceiling, so no live process can answer for it.
race_fixture
rec="$(sed 's/^child_pid=.*/child_pid=999999/' "$DDIR/launch")"
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

# ---- 0284: a child that DIED without a sentinel is detected on THIS observation ----
# THE HEADLINE. Before change 0284 the predicate was "no sentinel ⇒ still running", so this exact
# fixture returned 4 on every pass until the 60-minute budget expired. The assert is therefore a
# pair: not-4, AND reached while the budget it would otherwise have waited out is still unspent —
# see the second half's own note for why that is measured off the RECORD's clock and not off the
# call's wall-clock duration.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
assert "0284: fixture sanity — the launch recorded a usable pgid" '[ -n "$lpgid" ] && [ "$lpgid" -gt 1 ]'
assert "0284: fixture sanity — the child is alive before we kill it" 'kill -0 -"$lpgid" 2>/dev/null'
# Kill the GROUP without letting the wrapper write `done`: SIGKILL is untrappable, so the untrapped
# wrapper subshell dies with it and no sentinel can ever appear. That is precisely the state the
# sentinel-only predicate could not see.
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — the group is gone" '! kill -0 -"$lpgid" 2>/dev/null'
assert "0284: fixture sanity — and no sentinel was ever written" '[ ! -f "$DDIR/done" ]'
BUDGET=60
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0284: a vanished child does NOT observe as still running" '[ "$rc" != "4" ]'
assert "0284: it is terminal — result unavailable (1)" '[ "$rc" = "1" ]'
# THE PAIRED HALF (change 0284 review, finding 4). It used to be wall-clock elapsed `< 30`, which
# measured nothing: `--observe` is by contract ONE short read that never blocks, so the DEFECTIVE
# pre-0284 predicate ("no sentinel ⇒ still running") returned its wrong `4` in a fraction of a
# second too. What the headline actually claims is that the TERMINAL verdict was reached WITHOUT
# the budget having been spent — a statement about the clock THE FACADE READS, which is the launch
# record's own `started_at` measured against the budget in force. Converted through
# `verify-run --iso-to-epoch`, the converter the facade's own budget arithmetic uses, so no second
# parser can disagree with it. It reddens on a fixture whose budget HAS lapsed (drop `BUDGET` to 0
# above and watch it), which is exactly the drift that would quietly turn this arm into a
# re-measurement of budget exhaustion and leave the `cause=child-vanished` assert below reading as
# luck rather than as the liveness leg's own verdict.
vstarted="$("$DOCKET_BASH_PATH" "$ROOT/scripts/verify-run.sh" --iso-to-epoch \
  "$(sed -n 's/^started_at=//p' "$DDIR/launch")" 2>/dev/null)"
assert "0284: fixture sanity — the launch's start time converts, so the bound below is measurable" \
  '[ -n "$vstarted" ]'
assert "0284-f4: the terminal verdict was reached with the ${BUDGET}m budget demonstrably UNSPENT" \
  '[ $(( $(date -u +%s) - vstarted )) -lt $(( BUDGET * 60 )) ]'
assert "0284: the diagnostic says the child died without a sentinel" \
  'grep -qiE "without .*sentinel" <<<"$out"'
assert "0284: and it names the dispatch dir so the orphans it did not reap can be found" \
  'grep -qF "$DDIR" <<<"$out"'
# NO FABRICATED EXIT CODE (spec § Testing case 5, shape-keyed rather than wording-enumerated): the
# child said nothing at all, so an "exited <n>" phrase would assert a code that was never read.
assert "0284: the dead path never claims an exit code it did not read" \
  '! grep -qE "exited [0-9]+" <<<"$out"'
# The give-up path's own diagnostic is NOT what was spoken: the budget was nowhere near spent, and
# borrowing that wording would tell a reader a budget expired when none did.
assert "0284: and it never claims a budget it did not spend" \
  '! grep -qiE "budget of [0-9]+m exhausted|budget was exhausted" <<<"$out"'

# ---- 0284: the terminal marker is the EXISTING one, with the new cause -------------
assert "0284: a killed marker was recorded (no second terminal file was minted)" '[ -f "$DDIR/killed" ]'
assert "0284: its cause is child-vanished" 'grep -qx "cause=child-vanished" "$DDIR/killed"'
assert "0284: its reason says nothing was signalled" 'grep -qx "reason=group-already-gone" "$DDIR/killed"'
# THE CLASS IS RECORDED, because the replay below is worded off it and a marker cannot re-derive
# what a probe established seconds ago (change 0284 review, finding 1).
assert "0284-f1: and it records the evidence class that justified disposing at all" \
  'grep -qx "liveness_class=gone" "$DDIR/killed"'

# ---- 0284: idempotence — every later observation short-circuits at step 2 -----------
# The FIRST observation is the terminal TRANSITION, not a re-report, and it speaks from the leg that
# made the transition: it relays the child's stdout, it names the dispatch dir, and it may carry a
# git verdict the marker does not record. What idempotence obliges is that every
# observation AFTER it is identical, which is the same shape the 0271 give-up arms above assert.
out2="$(observe "$KEY" 2>&1)"; rc2=$?
out3="$(observe "$KEY" 2>&1)"; rc3=$?
assert "0284: re-observing a vanished dispatch stays terminal (1)" '[ "$rc2" = "1" ] && [ "$rc3" = "1" ]'
assert "0284: and re-reports identically forever" '[ "$out3" = "$out2" ]'
# THE ASSERT THE `child-vanished` READER ARM EXISTS FOR: without it `cause` falls to the default and
# the re-report tells a reader the budget ran out on a dispatch that never spent a minute of it.
assert "0284: the re-report names the vanishing, not a budget the dispatch never spent" \
  'grep -qi "died without writing a sentinel" <<<"$out2" && ! grep -qi "budget was exhausted" <<<"$out2"'

# ---- 0284 review, finding 7: the wording is keyed on the EVIDENCE, not on the cause ----
# `cause=child-vanished` says the facade gave up because it stopped seeing the child; only the
# CLASS says whether a death was ever established. A marker written by an older build carries no
# class at all, and wording that one as a death asserts exactly what the code two lines below
# refuses to assert about an exit code it never read. Driven by rewriting THIS already-terminal
# dispatch's marker — no second fixture, and what is under test is the wording alone.
class_marker(){  # $1 = the liveness_class value to carry ('' = the field is absent entirely)
  { grep -v '^liveness_class=' "$DDIR/killed"
    [ -n "$1" ] && printf 'liveness_class=%s\n' "$1"
    :; } > "$DDIR/killed.tmp"
  mv -f "$DDIR/killed.tmp" "$DDIR/killed"
}
class_marker ""
assert "0284-f7: fixture sanity — the marker really carries no class now" \
  '! grep -q "^liveness_class=" "$DDIR/killed"'
cls_out="$(observe "$KEY" 2>&1)"; cls_rc=$?
assert "0284-f7: a classless vanished marker still replays its recorded code" '[ "$cls_rc" = "1" ]'
assert "0284-f7: but it never asserts a death the record does not carry" \
  '! grep -qi "died" <<<"$cls_out"'
assert "0284-f7: it says only that the child can no longer be proven alive" \
  'grep -qi "can no longer be proven alive" <<<"$cls_out"'
class_marker gone
gone_out="$(observe "$KEY" 2>&1)"; gone_rc=$?
assert "0284-f7: and a marker that DOES carry the proof is worded as a death" \
  '[ "$gone_rc" = "1" ] && grep -qi "died without writing a sentinel" <<<"$gone_out"'

# ---- 0284 review, finding 2: the RELAY replays too, not only the wording ----------
# A vanished dispatch was the ONE terminal state whose STDOUT differed between the transition and
# every later observation: the transition relayed, the step-2 marker replay did not. So a caller
# that observed a second time got a terminal code with an EMPTY relay — indistinguishable from a run
# that produced no output — and, since the marker's `mv -f` is what makes the state terminal, a
# crash between that rename and the relay lost the child's only surviving words permanently.
# `report_done_disposition` relays on EVERY observation while `done` exists; this pins the same
# promise on the vanished path. The "same bytes once per pass" objection does not apply: a polling
# caller stops at the first terminal code, exactly as it already does on the `done` path.
#
# The fixture must carry stdout the child actually WROTE before dying, which the `gone` fixture
# above cannot: it was SIGKILLed mid-`sleep`, before the adapter printed anything. So this one
# prints FIRST and then lingers, and is killed during the linger.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=300 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
waited=0
while [ ! -s "$DDIR/stdout.log" ] && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284-f2: fixture sanity — the child wrote stdout before it was killed" '[ -s "$DDIR/stdout.log" ]'
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284-f2: fixture sanity — the group is gone and no sentinel was ever written" \
  '! kill -0 -"$lpgid" 2>/dev/null && [ ! -f "$DDIR/done" ]'
# STDERR DISCARDED on all three passes, so nothing a diagnostic prints can satisfy these asserts.
BUDGET=60 rl1="$(observe "$KEY" 2>/dev/null)"; rl_rc1=$?
BUDGET=60 rl2="$(observe "$KEY" 2>/dev/null)"; rl_rc2=$?
BUDGET=60 rl3="$(observe "$KEY" 2>/dev/null)"; rl_rc3=$?
assert "0284-f2: fixture sanity — the transition disposed it as vanished" \
  '[ "$rl_rc1" = "1" ] && grep -qx "cause=child-vanished" "$DDIR/killed"'
assert "0284-f2: the transition relays the child's stdout" '[ "$rl1" = "fake adapter stdout" ]'
# THE FINDING. Each later pass is asserted against the BYTES, never only against its predecessor:
# `rl3 = rl2` is satisfied vacuously by two empty relays, which is exactly the defect.
assert "0284-f2: and so does the SECOND observation, replayed off the marker" \
  '[ "$rl2" = "fake adapter stdout" ]'
assert "0284-f2: and the THIRD" '[ "$rl3" = "fake adapter stdout" ]'
assert "0284-f2: the replayed relay carries the child's stdout only, never its stderr" \
  '! grep -qF "fake adapter stderr" <<<"$rl2$rl3"'
assert "0284-f2: and no diagnostic leaks onto it" '! grep -qF "runner-dispatch:" <<<"$rl2$rl3"'
assert "0284-f2: the code stays terminal on every pass" '[ "$rl_rc2" = "1" ] && [ "$rl_rc3" = "1" ]'

# ---- 0284: NOTHING IS SIGNALLED when the recorded group cannot be proven ours -----
# The no-signal promise needs a discriminating fixture: a live process that would die if the facade
# signalled the recorded group. The canary leads its own group and the DEAD dispatch's record is
# rewritten to name it — the "a pgid is a reusable name" state exactly, and the state the identity
# conjunct exists to catch. The measurement is that the canary is still there.
#
# SINCE THE 0284 REVIEW (finding 1) THIS IS ALSO THE CLASS SPLIT'S DISCRIMINATING CASE, and it could
# not be otherwise: `kill -0` SUCCEEDS here (the canary's group answers), so nothing at all was
# established about the launched child and the dispatch may not be disposed as vanished on this
# pass. It takes the bounded unprovable route instead — three passes, then the honest terminal — and
# the canary must live through every one of them. The `gone` leg cannot host this fixture: there,
# `kill -0` failed, so by construction no process in that group is left to signal or to survive.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
# The canary leads its OWN group: `set -m` inside the subshell makes the background job a
# process-group leader, and the pid is RECORDED rather than marked in an argv comment — a single
# simple command under `sh -c` is EXEC'd and a comment marker vanishes from the argv the kernel
# shows (learnings: exec-optimization-erases-the-process-marker). Verified by the sanity asserts
# below: it must lead its own group, that group must not be this file's, and it must still be alive.
( set -m; sleep 60 & echo $! > "$SBX/canary.pgid" )
canary="$(cat "$SBX/canary.pgid")"
canary_mypgid="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
assert "0284: fixture sanity — the canary leads its own group, and not this file's" \
  '[ -n "$canary" ] && [ "$(ps -o pgid= -p "$canary" 2>/dev/null | tr -d " ")" = "$canary" ] &&
   [ "$canary" != "$canary_mypgid" ]'
assert "0284: fixture sanity — and it is alive to be killed" 'kill -0 -"$canary" 2>/dev/null'
# Point the DEAD dispatch's record at the LIVE canary group. The recorded TOKEN is left as the dead
# child's, which is what makes the live group unprovable as ours — and it is blanked rather than
# left to chance, because the canary can start inside the same `ps -o lstart=` second as the child
# it replaces and an accidental match would report the dispatch as still running.
rec="$(sed -e "s/^pgid=.*/pgid=$canary/" \
           -e "s|^child_lstart=.*|child_lstart=Thu Jan  1 00:00:00 1970|" "$DDIR/launch")"
printf '%s\n' "$rec" > "$DDIR/launch"
assert "0284: fixture sanity — the record now names the canary's group" \
  '[ "$(sed -n "s/^pgid=//p" "$DDIR/launch")" = "$canary" ]'
BUDGET=60 cy1="$(observe "$KEY" 2>&1)"; cy_rc1=$?
assert "0284-f1: a LIVE group we cannot prove is ours is not disposed on the first pass" \
  '[ "$cy_rc1" = "4" ] && [ ! -f "$DDIR/killed" ]'
assert "0284-f7: and nothing claims the child died" '! grep -qi "died" <<<"$cy1"'
assert "0284-f7: nor that it is still running" '! grep -qi "still running" <<<"$cy1"'
assert "0284: the canary survives the first pass" 'kill -0 "$canary" 2>/dev/null'
BUDGET=60 observe "$KEY" >/dev/null 2>&1; cy_rc2=$?
assert "0284-f1: nor on the second" '[ "$cy_rc2" = "4" ]'
BUDGET=60 out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0284: a record naming a group that is not ours is still terminal (1)" '[ "$rc" = "1" ]'
assert "0284: NOTHING was signalled — the canary is still running" 'kill -0 "$canary" 2>/dev/null'
assert "0284: and the diagnostic still points a human at the dispatch dir" 'grep -qF "$DDIR" <<<"$out"'
reap "$canary"

# ---- 0284 review, finding 3: the pattern that means "disposed as a VANISHING" ------
# The two negatives below used to key on the word "vanished", which `--observe` NEVER prints on any
# path: it lives only inside the `killed` FILE as `cause=child-vanished`, which neither assert
# reads. So the negative half was green on the shipped code and on a mutant that took the wrong
# branch alike — decoration, not a measurement.
#
# What the vanished disposition actually speaks is `say_vanished`'s class-keyed `fate` clause, and
# BOTH of its legs are covered here (change 0284's review split the classes, so keying on one leg
# would go vacuous the moment the other were taken). The headline is NOT usable as the key: a
# vanished dispatch whose work landed in git prints `COMPLETE` too, so only the fate clause
# separates the two dispositions.
VANISHED_FATE_RE='died without writing a sentinel|wrote no sentinel'
# AND THE PATTERN IS PINNED TO THE SOURCE, so a reworded `fate` reddens HERE rather than silently
# re-hollowing every negative keyed on it — the same failure this finding is repairing. Read out of
# `say_vanished`'s own case arms, so a THIRD class added later is covered or reddens.
say_fates="$(sed -n '/say_vanished()/,/^  }/p' "$FACADE" | sed -n 's/.*fate="\([^"]*\)".*/\1/p')"
assert "0284-f3: contract — say_vanished speaks at least the two evidence classes' fates" \
  '[ "$(grep -c . <<<"$say_fates")" -ge 2 ]'
assert "0284-f3: contract — every fate it can speak is matched by the negatives' pattern" \
  '[ -n "$say_fates" ] && ! grep -qvE "$VANISHED_FATE_RE" <<<"$say_fates"'

# ---- 0284: the SENTINEL OUTRANKS liveness -----------------------------------------
# Pins step 1's precedence against the new step 3: a dead child WITH a `done` sentinel takes the
# sentinel disposition, unchanged. Without this the new leg could silently capture completed runs.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
waited=0
while [ ! -f "$DDIR/done" ] && [ "$waited" -lt 300 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — the child finished and wrote its sentinel" '[ -f "$DDIR/done" ]'
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
# The sentinel is written by the wrapper's second-to-last act, so the group can outlive it by
# milliseconds. Wait it out rather than asserting on the instant — the discriminating condition is
# that liveness ALONE would say vanished, and a half-torn-down group would not measure that.
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
assert "0284: fixture sanity — and its group is gone (so liveness alone would say vanished)" \
  '! kill -0 -"$lpgid" 2>/dev/null'
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0284: a dead child WITH a sentinel takes the sentinel disposition (0)" '[ "$rc" = "0" ]'
assert "0284: and it is worded as a completion, not as a vanishing" \
  'grep -qi "complete" <<<"$out" && ! grep -qiE "$VANISHED_FATE_RE" <<<"$out"'
assert "0284: no killed marker is written over a completed run" '[ ! -f "$DDIR/killed" ]'

# ---- 0284: the step-4 RE-READ closes the probe's own TOCTOU window ----------------
# Steps 1-3 span a `ps` call and a `kill -0`; the child has every chance to finish inside that
# window, and without the re-read a run that PASSED is disposed as dead. The barrier holds the
# observer at exactly the pre-probe point so the interleaving is deterministic rather than
# sleep-tuned. Inert unless armed, so no other arm in this file is affected.
make_fixture
FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
kill -KILL -"$lpgid" 2>/dev/null
waited=0
while kill -0 -"$lpgid" 2>/dev/null && [ "$waited" -lt 100 ]; do sleep 0.1; waited=$(( waited + 1 )); done
BFILE="$SBX/barrier"
( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    DOCKET_RUNNER_DISPATCH_TEST_BARRIER=pre-liveness-probe \
    DOCKET_RUNNER_DISPATCH_TEST_BARRIER_FILE="$BFILE" \
    bash "$FACADE" --observe "$KEY" --runner fake --agent status ) > "$SBX/bout" 2> "$SBX/berr" &
bpid=$!
waited=0
while [ ! -e "$BFILE.reached" ] && [ "$waited" -lt 200 ]; do sleep 0.1; waited=$(( waited + 1 )); done
# NOT DECORATION: everything below is a claim about an INTERLEAVING, and an interleaving that never
# happened would be asserted just as green by a barrier that was never reached.
assert "0284: fixture sanity — the observer is held at the pre-probe barrier" '[ -e "$BFILE.reached" ]'
# The child "finishes" INSIDE the window: write the sentinel the wrapper would have written, in the
# wrapper's own atomic shape.
printf 'exit_code=0\nstarted_at=x\nfinished_at=y\npid=1\ndispatch_key=%s\n' "$KEY" > "$DDIR/done.partial"
mv -f "$DDIR/done.partial" "$DDIR/done"
: > "$BFILE.release"
wait "$bpid"; rc=$?
berr="$(cat "$SBX/berr")"
assert "0284: a sentinel that lands inside the probe window is honoured, not disposed as dead" '[ "$rc" = "0" ]'
assert "0284: and it reports the COMPLETED disposition, never a vanishing" \
  'grep -qi "complete" <<<"$berr" && ! grep -qiE "$VANISHED_FATE_RE" <<<"$berr"'
assert "0284: no killed marker was written over the sentinel that arrived" '[ ! -f "$DDIR/killed" ]'

# ---- 0284: the barrier is inert unless armed on ITS OWN point name ----------------
# An always-on hang site is the one way a test hook can damage production, and arming point A must
# never hold point B. The fixture is a LIVE child with no terminal file, so the observation really
# does reach the barrier's call site — armed on a different name it must sail straight through it,
# through the probe, and out at the still-running verdict. (A finished child would take the sentinel
# disposition before the barrier and assert nothing at all.)
make_fixture
FAKE_SLEEP=30 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
DDIR="$(ddir_for "$KEY")"
in_pgid="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
istart=$(date +%s)
( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    DOCKET_RUNNER_DISPATCH_TEST_BARRIER=some-other-point \
    DOCKET_RUNNER_DISPATCH_TEST_BARRIER_FILE="$SBX/never" \
    bash "$FACADE" --observe "$KEY" --runner fake --agent status ) >/dev/null 2>&1; irc=$?
assert "0284: arming a DIFFERENT barrier point does not hold this one" \
  '[ $(( $(date +%s) - istart )) -lt 10 ]'
assert "0284: and an unarmed barrier creates no rendezvous files" '[ ! -e "$SBX/never.reached" ]'
# The same observation is the live-child control for the probe itself: a group that IS alive and IS
# ours must still observe as still-running, or the new leg would dispose every healthy dispatch.
assert "0284: a LIVE child that is provably ours still observes as still running (4)" '[ "$irc" = "4" ]'
assert "0284: and no terminal marker is written over it" '[ ! -f "$DDIR/killed" ]'
reap "$in_pgid"

# ---- 0284 case 4: GIT DECIDES the dead child's disposition ------------------------
# A dead child is NOT automatically "no result": a delegated run can commit its work, push its
# branch and open its PR and THEN be killed before the wrapper's `mv -f` lands. Reporting
# `unavailable` over evidence sitting in git sends a human hunting for work that is already
# committed — change 0258's failure, inverted.
#
# THE STUB SEAM is `VERIFY_RUN`, the same one tests/test_runner_dispatch_build_gate.sh drives its
# gate arms through, and the stub answers the same three shapes that file's `mkgatefixture` answers
# (`--build`, `--in-progress-ids[ --with-claimed-at]`, a bare change id) plus `--iso-to-epoch`,
# which it delegates to the real script. It is NOT lifted into tests/lib/: that copy is welded into
# `mkgatefixture`, which also builds the live gate's snapshots and its counting adapter, and
# extracting it would rewrite a passing shard for no assert in this one. What this copy adds is the
# CALL LOG — the only way to assert that a re-observation reads git zero more times.
#
# `DOCKET_FACADE` is stubbed alongside it because an implement-next launch and observation both
# re-sync metadata: unstubbed, that is the REAL `docket.sh preflight` against the developer's own
# repository, from inside a test.
VFUT=$(( $(date -u +%s) + 600 ))          # a claim stamp inside this dispatch's attribution window
vanish_stub(){   # $SBX must exist; sets VRLOG and SNAP, and writes both stubs
  VRLOG="$SBX/vr.log"; : > "$VRLOG"
  SNAP="$SBX/snap"; mkdir -p "$SNAP"
  printf '' > "$SNAP/before"                     # nothing claimed at the handoff
  printf '%s %s\n' 7 "$VFUT" > "$SNAP/after"     # one claim, attributable to this dispatch
  : > "$SNAP/verdict.7"
  cat > "$SBX/vanish-vr.sh" <<VRE
#!/usr/bin/env bash
printf 'vr %s\n' "\$*" >> "$VRLOG"
case "\$1" in --iso-to-epoch) exec "$ROOT/scripts/verify-run.sh" "\$@" ;; esac
for a in "\$@"; do [ "\$a" = "--build" ] && { cat "$SNAP/verdict.7"; exit 0; }; done
withca=0
for a in "\$@"; do [ "\$a" = "--with-claimed-at" ] && withca=1; done
for a in "\$@"; do
  [ "\$a" = "--in-progress-ids" ] || continue
  if [ "\$withca" = 1 ]; then cat "$SNAP/after"; else cat "$SNAP/before"; fi
  exit 0
done
id=""
for a in "\$@"; do case "\$a" in [0-9]*) id="\$a" ;; esac; done
cat "$SNAP/verdict.\$id" 2>/dev/null
exit 0
VRE
  chmod +x "$SBX/vanish-vr.sh"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$SBX/vanish-facade.sh"
  chmod +x "$SBX/vanish-facade.sh"
}
# Launch a child and kill its GROUP without letting the wrapper write `done` — SIGKILL is
# untrappable, so the untrapped wrapper subshell dies with it and no sentinel can ever appear. The
# recorded `pgid` is left ALONE (no rewrite, no borrowed group), so the probe's verdict rests on a
# group that is genuinely gone rather than on whether two processes happened to start inside the
# same `ps -o lstart=` second. Both preconditions are ASSERTED here, once per arm, because every
# claim below is a claim about the dead path and a live child would take a different one entirely.
vlaunch(){   # $1 = agent, rest = extra facade args -> sets KEY, DDIR, VPGID
  local agent="$1" w=0; shift
  KEY="$( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
      FAKE_SLEEP=300 FAKE_TAIL=0 FAKE_RC=0 \
      VERIFY_RUN="$SBX/vanish-vr.sh" DOCKET_FACADE="$SBX/vanish-facade.sh" \
      bash "$FACADE" --launch --runner fake --agent "$agent" "$@" )"
  DDIR="$(ddir_for "$KEY")"
  VPGID="$(sed -n 's/^pgid=//p' "$DDIR/launch")"
  kill -KILL -"$VPGID" 2>/dev/null
  while kill -0 -"$VPGID" 2>/dev/null && [ "$w" -lt 100 ]; do sleep 0.1; w=$(( w + 1 )); done
  assert "0284 case 4: fixture sanity — the $agent child's group is really gone" \
    '! kill -0 -"$VPGID" 2>/dev/null'
  assert "0284 case 4: fixture sanity — and it never wrote a sentinel" '[ ! -f "$DDIR/done" ]'
}
vobserve(){  # $1 = key, $2 = agent, rest = extra facade args
  local k="$1" ag="$2"; shift 2
  ( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    VERIFY_RUN="$SBX/vanish-vr.sh" DOCKET_FACADE="$SBX/vanish-facade.sh" \
    bash "$FACADE" --observe "$k" --runner fake --agent "$ag" "$@" )
}
vrbuilds(){ grep -cF -- "--build" "$VRLOG" | tr -d ' '; }   # how many git BUILD reads so far
vrlines(){ wc -l < "$VRLOG" | tr -d ' '; }                  # how many verify-run calls at all

# One repo serves every main-tree arm: each `vlaunch` mints its OWN dispatch (a disposed one is
# terminal forever, so no arm can reuse another's), and the fixture cost is paid once.
make_fixture
vanish_stub

# --- implement-next + run-complete => 0 -------------------------------------------
printf 'run-complete 7\n' > "$SNAP/verdict.7"
vlaunch implement-next
assert "0284: fixture sanity — the launch really armed the run gate" \
  'grep -qE "^dispatch_epoch=[0-9]+$" "$DDIR/launch" && [ -f "$DDIR/gate-before" ]'
vr_n="$(vrlines)"
out="$(vobserve "$KEY" implement-next 2>&1)"; rc=$?
assert "0284: a vanished implement-next whose work LANDED exits 0" '[ "$rc" = "0" ]'
assert "0284: and the wording states the death FIRST, then the git verdict" \
  'grep -qi "died without writing a sentinel" <<<"$out" && grep -qF "run-complete 7" <<<"$out"'
assert "0284: it never claims an exit code it did not read" '! grep -qE "exited [0-9]+" <<<"$out"'
# IDEMPOTENCE, AND THE POINT OF RECORDING THE VERDICT (spec § Testing case 6). The git verdict is
# written into the marker at the transition precisely so that every later observation replays the
# same code and the same wording from it — WITHOUT a second `verify-run` call, which would be a
# differently-timed answer to a question already answered. Compared across the SECOND and THIRD
# observations, the file's established idiom: the first is the terminal TRANSITION, which also
# relays the child's stdout.
assert "0284: THE TRANSITION really read git (so the no-second-read assert below is not vacuous)" \
  '[ "$(vrlines)" -gt "$vr_n" ]'
assert "0284: and recorded the verdict it read in the marker" \
  'grep -qxF "git_verdict=run-complete 7" "$DDIR/killed"'
vr_n="$(vrlines)"
out2="$(vobserve "$KEY" implement-next 2>&1)"; rc2=$?
out3="$(vobserve "$KEY" implement-next 2>&1)"; rc3=$?
assert "0284: re-observing replays the SAME exit code, not a generic failure" \
  '[ "$rc2" = "0" ] && [ "$rc3" = "0" ]'
assert "0284: and re-reports identically forever" '[ "$out3" = "$out2" ]'
assert "0284: the re-report reads git ZERO more times (the marker short-circuits at step 2)" \
  '[ "$(vrlines)" = "$vr_n" ]'
assert "0284: and still names the git verdict it replayed" 'grep -qF "run-complete 7" <<<"$out2"'

# --- implement-next + run-halted => the HALT disposition, wording preserved --------
# THE EXIT CODE IS 3, NOT 1, and this is a deliberate reading of the spec against itself. Its §3
# table names `observe_implement_next` as this leg's reader and says "wording preserved", and that
# function's halt disposition is `3` — the code change 0271's synthesized-exit table pins
# normatively for a halt reached under detachment, and the code CLAUDE.md's run gate keys on for
# "never re-dispatch a halt". The same table's "⇒ exit 1" summary would collapse a stop-for-a-human
# into an ordinary failure, which is the prose-level failure change 0237 exists to eliminate. What
# a dead child changes is how the facade LEARNED the run stopped, never what the run's own state is.
printf 'run-halted 7\n' > "$SNAP/verdict.7"
vlaunch implement-next
out="$(vobserve "$KEY" implement-next 2>&1)"; rc=$?
assert "0284: a vanished implement-next that HALTED is never reported as complete" '[ "$rc" != "0" ]'
assert "0284: it takes the HALT disposition (3), not a generic failure" '[ "$rc" = "3" ]'
assert "0284: and the halted wording is preserved" \
  'grep -qi "halted" <<<"$out" && grep -qi "needs a human" <<<"$out"'
assert "0284: the halt still states the death first" 'grep -qi "died without writing a sentinel" <<<"$out"'
out2="$(vobserve "$KEY" implement-next 2>&1)"; rc2=$?
out3="$(vobserve "$KEY" implement-next 2>&1)"; rc3=$?
assert "0284: a replayed halt is still a halt, not a downgraded failure" \
  '[ "$rc2" = "3" ] && [ "$rc3" = "3" ]'
assert "0284: and it replays identically" '[ "$out3" = "$out2" ]'

# --- any other agent => 1, and git is never asked ---------------------------------
# The stub is left holding a POSITIVE verdict, so an implementation that read git for every agent
# would exit 0 here rather than merely printing an extra word.
printf 'task-committed feat/thing\n' > "$SNAP/verdict.7"
vlaunch status
vr_n="$(vrlines)"
out="$(vobserve "$KEY" status 2>&1)"; rc=$?
assert "0284: a vanished status dispatch is unavailable (1)" '[ "$rc" = "1" ]'
assert "0284: and claims no git verdict" '! grep -qF "task-committed" <<<"$out"'
assert "0284: nor reads one — no verify-run call is made for an agent with no git question" \
  '[ "$(vrlines)" = "$vr_n" ]'
assert "0284: it still points a human at the dispatch dir" 'grep -qF "$DDIR" <<<"$out"'

# --- ORDERING: deadness is knowable without a readable clock (spec §2) ------------
# Placed AFTER the clock reads, a dispatch with an unreadable `started_at` would take the
# `note_unenforceable` path for three more observations and then terminate on the WRONG cause. So a
# vanished child with a BLANKED start time must still be disposed `child-vanished` on the FIRST
# pass, and the unenforceable counter must never be touched.
vlaunch status
rec="$(sed 's/^started_at=.*/started_at=/' "$DDIR/launch")"; printf '%s\n' "$rec" > "$DDIR/launch"
assert "0284: fixture sanity — the launch record now carries no start time" \
  '[ -z "$(sed -n "s/^started_at=//p" "$DDIR/launch")" ]'
out="$(vobserve "$KEY" status 2>&1)"; rc=$?
assert "0284: an unreadable clock does not delay the vanished verdict (1, first pass)" '[ "$rc" = "1" ]'
assert "0284: and the cause recorded is the vanishing, not an unenforceable budget" \
  'grep -qx "cause=child-vanished" "$DDIR/killed"'
assert "0284: the unenforceable counter was never touched" '[ ! -f "$DDIR/unenforceable" ]'
assert "0284: and the diagnostic never claims the budget could not be enforced" \
  '! grep -qi "budget not enforced\|could not be enforced" <<<"$out"'

# --- the build-* family: a REAL linked worktree, because build-* is feature-scoped -
make_fixture
vanish_stub
git -C "$SBX" worktree add -q -b feat/thing "$SBX/.worktrees/build" >/dev/null 2>&1
VWT="$SBX/.worktrees/build"
assert "0284: fixture sanity — the build arms anchor on a REAL linked worktree, not the main tree" \
  '[ -f "$VWT/.git" ] && [ "$VWT" != "$SBX" ]'

# --- build-* + task-committed => 0 -------------------------------------------------
printf 'task-committed feat/thing\n' > "$SNAP/verdict.7"
vlaunch build-standard --worktree "$VWT" -- "build task"
out="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc=$?
assert "0284: a vanished build task whose commit LANDED exits 0" '[ "$rc" = "0" ]'
assert "0284: and echoes the git verdict after the death" \
  'grep -qi "died without writing a sentinel" <<<"$out" && grep -qF "task-committed" <<<"$out"'
assert "0284: it never claims an exit code it did not read" '! grep -qE "exited [0-9]+" <<<"$out"'
# The branch asked about is the LAUNCH-RECORDED one, never the anchor's HEAD now — the same
# conjunct 0271 made non-vacuous on the sentinel path.
assert "0284: the build verdict is asked against the branch the LAUNCH recorded" \
  'grep -qF -- "--branch feat/thing" "$VRLOG"'
vr_b="$(vrbuilds)"
out2="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc2=$?
out3="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc3=$?
assert "0284: a landed build verdict replays as 0, not as unavailable" \
  '[ "$rc2" = "0" ] && [ "$rc3" = "0" ]'
assert "0284: and replays identically" '[ "$out3" = "$out2" ]'
assert "0284: without a second build read" '[ "$(vrbuilds)" = "$vr_b" ]'

# --- build-* + no evidence => 1 unavailable ---------------------------------------
printf 'task-incomplete feat/thing tree\n' > "$SNAP/verdict.7"
vlaunch build-standard --worktree "$VWT" -- "build task"
out="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc=$?
assert "0284: a vanished build task with NO git evidence exits 1" '[ "$rc" = "1" ]'
assert "0284: and says the result is unavailable" 'grep -qi "unavailable\|no result" <<<"$out"'
assert "0284: naming the verdict that failed to establish it" 'grep -qF "task-incomplete" <<<"$out"'

# --- 0208 IS NOT REGRESSED (1/2): a launch record with no branch is a non-verdict --
# Falling back to the observation-time branch would reinstate exactly the vacuity 0271 removed, so
# the absence is NO POSITIVE EVIDENCE and git is not asked at all.
printf 'task-committed feat/thing\n' > "$SNAP/verdict.7"
vlaunch build-standard --worktree "$VWT" -- "build task"
rec="$(sed 's/^branch=.*/branch=/' "$DDIR/launch")"; printf '%s\n' "$rec" > "$DDIR/launch"
assert "0284: fixture sanity — the launch record now names no branch" \
  '[ -z "$(sed -n "s/^branch=//p" "$DDIR/launch")" ]'
vr_b="$(vrbuilds)"
out="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc=$?
assert "0284: a branchless launch record is not verified against the observation-time branch" \
  '[ "$rc" = "1" ]'
assert "0284: and it says so honestly" 'grep -qF "launch-branch-missing" <<<"$out"'
assert "0284: git was never asked, so no verdict could be manufactured" \
  '[ "$(vrbuilds)" = "$vr_b" ]'
assert "0284: the non-verdict is what the marker replays" \
  'grep -qxF "git_verdict=task-unverifiable launch-branch-missing" "$DDIR/killed"'

# --- 0208 IS NOT REGRESSED (2/2): a removed worktree is an honest non-verdict ------
# `--observe` on a removed worktree deliberately reassigns ANCHOR to the repo root, so verifying the
# build THERE would answer a question nobody asked. This arm is LAST in this fixture: it removes the
# worktree the arms above anchor on.
printf 'task-committed feat/thing\n' > "$SNAP/verdict.7"
vlaunch build-standard --worktree "$VWT" -- "build task"
git -C "$SBX" worktree remove --force "$VWT" 2>/dev/null
assert "0284: fixture sanity — the anchor worktree is really gone" '[ ! -d "$VWT" ]'
vr_b="$(vrbuilds)"
out="$(vobserve "$KEY" build-standard --worktree "$VWT" 2>&1)"; rc=$?
assert "0284: fixture sanity — the observation really took the anchor FALLBACK" \
  'grep -qi "no longer exists" <<<"$out"'
assert "0284: a vanished build task whose WORKTREE is gone is not verified against the main tree" \
  '[ "$rc" = "1" ]'
assert "0284: and it says so honestly" 'grep -qF "worktree-removed" <<<"$out"'
assert "0284: git was never asked against the main worktree" '[ "$(vrbuilds)" = "$vr_b" ]'

exit "$fail"

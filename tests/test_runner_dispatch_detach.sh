#!/usr/bin/env bash
# tests/test_runner_dispatch_detach.sh — the launch/observe detachment posture (change 0271).
# Run: bash tests/test_runner_dispatch_detach.sh
# Hermetic: a FAKE adapter script stands in for every runner, so nothing here needs a child
# harness CLI. The fake sleeps for a caller-controlled duration and writes a marker, which is
# what makes "survived the group teardown" observable rather than asserted.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
  cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
sleep "${FAKE_SLEEP:-0}"
printf 'adapter-ran\n' > "$FAKE_MARKER"
printf 'fake adapter stdout\n'
printf 'fake adapter stderr\n' >&2
sleep "${FAKE_TAIL:-0}"
exit "${FAKE_RC:-0}"
FAKE
  chmod +x "$RDIR/fake.sh"
}
launch(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_SLEEP="${FAKE_SLEEP:-0}" FAKE_TAIL="${FAKE_TAIL:-0}" FAKE_RC="${FAKE_RC:-0}" \
    bash "$FACADE" --launch --runner fake --agent "${1:-status}" ); }
# The per-dispatch dir for KEY, resolved the way an outside reader must: from the repo's git
# COMMON dir, never from the worktree.
ddir_for(){ local c; c="$(cd "$SBX" && git rev-parse --git-common-dir)"
  printf '%s/docket/dispatch/%s' "$(cd "$SBX/$c" 2>/dev/null || cd "$c"; pwd -P)" "$1"; }

# ---- launch returns immediately with a key -------------------------------------
make_fixture
FAKE_SLEEP=5
start=$(date +%s)
KEY="$(launch status)"; rc=$?
elapsed=$(( $(date +%s) - start ))
assert "launch exits 0" '[ "$rc" = "0" ]'
assert "launch prints a dispatch key" '[ -n "$KEY" ]'
assert "the key names the agent" '[[ "$KEY" == status-* ]]'
# THE POINT OF THE CHANGE: launch must NOT block for the child's duration.
assert "launch returned well before the child finished" '[ "$elapsed" -lt 3 ]'

DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"

# The child's OWN process group — the whole detachment property, asserted on the MECHANISM
# and not merely on an outcome that an unrelated pass could satisfy. Read the group from the OS
# WHILE THE CHILD IS STILL RUNNING (the fake sleeps 5s and launch returned in under 3), never from
# the launch record alone: the record's pgid falls back to the child's pid when `ps` cannot see the
# process, and a pid never equals this test's pgid, so a record-only comparison passes vacuously
# even with the detachment removed. Measured on the live process, it cannot.
lcpid="$(sed -n 's/^child_pid=//p' "$DDIR/launch" 2>/dev/null)"
livepgid="$(ps -o pgid= -p "${lcpid:-0}" 2>/dev/null | tr -d ' ')"
mypgid="$(ps -o pgid= -p $$ | tr -d ' ')"
lpgid="$(sed -n 's/^pgid=//p' "$DDIR/launch" 2>/dev/null)"

assert "the per-dispatch dir exists" '[ -d "$DDIR" ]'
assert "a launch record was written" '[ -f "$DDIR/launch" ]'
assert "the launch record carries a pgid" 'grep -qE "^pgid=[0-9]+$" "$DDIR/launch"'
assert "the launch record carries started_at" 'grep -qE "^started_at=[0-9TZ:-]+$" "$DDIR/launch"'
assert "the child is in its OWN process group, not the test's" '[ -n "$livepgid" ] && [ "$livepgid" != "$mypgid" ]'
assert "the child LEADS that group (its pgid is its own pid)" '[ "$livepgid" = "$lcpid" ]'
assert "the launch record reports the group the OS actually shows" '[ "$lpgid" = "$livepgid" ]'

# ---- the sentinel is written as the LAST act, by the wrapper, not the agent -----
assert "no sentinel while the child is still running" '[ ! -f "$DDIR/done" ]'
for _ in $(seq 1 40); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "the sentinel appears after the child finishes" '[ -f "$DDIR/done" ]'
assert "the adapter actually ran" '[ -f "$SBX/marker" ]'
assert "sentinel carries exit_code" 'grep -qE "^exit_code=0$" "$DDIR/done"'
assert "sentinel carries finished_at" 'grep -qE "^finished_at=[0-9TZ:-]+$" "$DDIR/done"'
assert "sentinel carries the dispatch key" 'grep -qxF "dispatch_key=$KEY" "$DDIR/done"'
# The sentinel's `pid` is the LAUNCHER SUBSHELL's own pid ($BASHPID), i.e. the same process the
# launch record names as child_pid — never the facade's `$$`, which is a long-exited process by
# the time a human debugs from the sentinel.
assert "sentinel pid is the launched subshell, not the facade" \
  '[ -n "$lcpid" ] && grep -qxF "pid=$lcpid" "$DDIR/done"'

# ---- every stream redirected to a durable location -----------------------------
assert "stdout was captured durably" 'grep -qF "fake adapter stdout" "$DDIR/stdout.log"'
assert "stderr was captured durably" 'grep -qF "fake adapter stderr" "$DDIR/stderr.log"'

# ---- a failing adapter records its code, and the sentinel is still well-formed --
make_fixture
FAKE_SLEEP=0 FAKE_RC=7
KEY="$(launch status)"
DDIR="$(cd "$SBX" && git rev-parse --git-common-dir)"
DDIR="$(cd "$SBX/$DDIR" 2>/dev/null || cd "$DDIR"; pwd -P)/docket/dispatch/$KEY"
for _ in $(seq 1 30); do [ -f "$DDIR/done" ] && break; sleep 1; done
assert "a failing adapter still writes a sentinel" '[ -f "$DDIR/done" ]'
assert "the sentinel records the adapter's real code" 'grep -qxF "exit_code=7" "$DDIR/done"'

# ---- the dispatch root resolver refuses rather than guesses ---------------------
# Asserted on the LIBRARY directly: the facade's anchor gates make a non-repo anchor unreachable
# through a dispatch, which is precisely why the resolver has to hold this line by itself — the
# next caller need not sit behind those gates. It must refuse, print no path (a path inside the
# handed directory would put the dispatch area where `git worktree remove` takes it), and stay
# quiet: resolving through `cd "$(git rev-parse …)"` lets the empty answer reach `cd`, which
# reports "null directory" on the caller's stderr.
NOTREPO="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach-norepo.XXXXXX")"
root_out="$( . "$ROOT/scripts/lib/docket-dispatch-dir.sh"; docket_dispatch_root "$NOTREPO" 2>/dev/null )"; root_rc=$?
root_err="$( . "$ROOT/scripts/lib/docket-dispatch-dir.sh"; docket_dispatch_root "$NOTREPO" 2>&1 >/dev/null )"
assert "docket_dispatch_root refuses a non-repo path" '[ "$root_rc" != "0" ]'
assert "and prints no path rather than one inside that directory" '[ -z "$root_out" ]'
assert "and refuses quietly, without a stray cd diagnostic" '[ -z "$root_err" ]'
rm -rf "$NOTREPO"

# ---- build-* still requires --worktree at launch time --------------------------
make_fixture
err="$( ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake --agent build-standard ) 2>&1 >/dev/null )"; rc=$?
assert "build-* without --worktree is still a loud abort at launch" '[ "$rc" != "0" ]'
assert "the diagnostic still names --worktree" 'grep -qF -- "--worktree" <<<"$err"'

# ---- a value-taking verb in FINAL position validates rather than spinning -------
# bash's `shift 2` FAILS (it does not truncate) when only one argument remains, and this parser
# has no trailing shift — so the house form loops forever on a trailing value-taking flag, which
# would make `--observe`'s own missing-key refusal unreachable. Timed with a background run and a
# liveness poll rather than `timeout`, which is not a BSD base-system tool.
( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe >/dev/null 2>&1 ) &
bare_pid=$!
bare_done=0
for _ in $(seq 1 50); do
  kill -0 "$bare_pid" 2>/dev/null || { bare_done=1; break; }
  sleep 0.1
done
[ "$bare_done" = 1 ] || kill -9 "$bare_pid" 2>/dev/null
wait "$bare_pid" 2>/dev/null; bare_rc=$?
assert "a trailing --observe terminates instead of spinning in the parser" '[ "$bare_done" = "1" ]'
assert "and it terminates by refusing, not by succeeding" '[ "$bare_rc" != "0" ]'

# ---- observe: still running -> 4, terminal -> 0 ---------------------------------
observe(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" \
    DELEGATION_OBSERVATION_BUDGET="${BUDGET:-60}" \
    bash "$FACADE" --observe "$1" --runner fake --agent "${2:-status}" ); }

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

# ---- the gate now covers build-*, via the BUILD verdict family ------------------
mkbuildrepo(){   # a repo + feature worktree the fake adapter can commit into
  local outside
  make_fixture
  # These are the only arms whose verdict reads `git status` on the fixture repo, so they are the
  # only ones for which make_fixture's RUNNERS_DIR placement matters: it sits INSIDE $SBX, where it
  # is untracked scaffolding that would leave the `tree` conjunct unmet no matter what the child
  # did — turning "clean build task" into a permanent failure and "stranded work" into a green
  # assert that measures the fixture rather than the child. Move it out; nothing else about the
  # dispatch depends on where the adapter lives.
  outside="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach-runners.XXXXXX")"
  FIXTURES+=("$outside")
  mv -f "$RDIR/fake.sh" "$outside/fake.sh"
  rmdir "$RDIR"
  RDIR="$outside"
  git -C "$SBX" checkout -q -b feat/thing
  WT="$SBX"     # the fake adapter runs with --worktree $SBX
}
# The build arms drive the facade directly rather than through `launch`/`observe`: those helpers
# hard-code `--agent status` shape and omit `--worktree`, which gate 1 requires for build-*.
blaunch(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake \
    --agent build-standard --worktree "$WT" ); }
bobserve(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$1" --runner fake \
    --agent build-standard --worktree "$WT" ); }
bsettle(){ local _ ; for _ in $(seq 1 30); do bobserve "$1" >/dev/null 2>&1
    [ "$?" != "4" ] && return 0; sleep 1; done; return 0; }

# (a) the child commits cleanly -> task-committed -> observe 0
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
git commit --allow-empty -qm "task work"
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$(blaunch)"
bsettle "$KEY"
out="$(bobserve "$KEY" 2>&1)"; rc=$?
assert "0271: a clean build task observes as complete (0)" '[ "$rc" = "0" ]'
assert "0271: the build gate reports task-committed" 'grep -qF "task-committed" <<<"$out"'

# (b) THE DISAGREEMENT RULE — the child exits 0 but strands uncommitted work.
#     This is change 0258's exact failure, and the sentinel alone would call it success.
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
printf 'stranded work\n' > stranded.txt   # never committed
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$(blaunch)"
bsettle "$KEY"
out="$(bobserve "$KEY" 2>&1)"; rc=$?
assert "0271: a sentinel-success with stranded work FAILS (correctness wins)" '[ "$rc" = "1" ]'
assert "0271: the disagreement diagnostic names the git verdict" 'grep -qF "task-incomplete" <<<"$out"'
assert "0271: the stranded file is still there for a human" '[ -f "$SBX/stranded.txt" ]'
# The verdict is TERMINAL and re-reports identically — a disagreement must not oscillate between
# a failure and a success across two reads of the same unchanged dispatch.
out2="$(bobserve "$KEY" 2>&1)"; rc2=$?
assert "0271: the disagreement verdict is idempotent in code" '[ "$rc2" = "$rc" ]'
assert "0271: the disagreement verdict is idempotent in output" '[ "$out2" = "$out" ]'

# (c) build-* is OBSERVE-ONLY: never a second adapter run on top of partial commits.
assert "0271: no re-dispatch happened for a build agent" \
  '[ "$(git -C "$SBX" rev-list --count HEAD)" -le 2 ]'

# (c2) THE BRANCH CONJUNCT BINDS ON THIS SEAM. The branch is recorded AT LAUNCH, before the child
#      can move HEAD, so a child that ends somewhere else is caught. Read at observation time it
#      compared the anchor's HEAD to itself and could never be unmet — one of three conjuncts dead
#      in production while its unit test stayed green.
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
git checkout -q -b feat/elsewhere
git commit --allow-empty -qm "task work, on the wrong branch"
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$(blaunch)"
assert "0271: the launch record carries the branch captured at launch" \
  'grep -qxF "branch=feat/thing" "$(ddir_for "$KEY")/launch"'
bsettle "$KEY"
out="$(bobserve "$KEY" 2>&1)"; rc=$?
assert "0271: a child that ends on a DIFFERENT branch observes as failed (1)" '[ "$rc" = "1" ]'
assert "0271: and the verdict names the launched branch with branch unmet" \
  'grep -qF "task-incomplete feat/thing branch" <<<"$out"'

# (c3) a DETACHED HEAD at the end is likewise not the launched branch — the other half of the
#      failure this conjunct exists for, and the half a wrong-branch-only test would miss.
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
git commit --allow-empty -qm "task work"
git checkout -q --detach
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$(blaunch)"
bsettle "$KEY"
out="$(bobserve "$KEY" 2>&1)"; rc=$?
assert "0271: a child that ends on a DETACHED HEAD observes as failed (1)" '[ "$rc" = "1" ]'
assert "0271: and branch is the unmet conjunct there too" \
  'grep -qF "task-incomplete feat/thing branch" <<<"$out"'

# (c4) NO SILENT FALLBACK. An older dispatch — or a detached HEAD at launch — records no branch.
#      Re-reading the anchor's HEAD instead would reinstate the vacuity, so the absence is
#      surfaced as no positive evidence, the same posture an empty verdict already gets.
mkbuildrepo
cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
cd "$DOCKET_REPO_ROOT" || exit 1
git commit --allow-empty -qm "task work"
exit 0
FAKE
chmod +x "$RDIR/fake.sh"
KEY="$(blaunch)"
bsettle "$KEY"
DDIR="$(ddir_for "$KEY")"
rec="$(grep -v '^branch=' "$DDIR/launch")"; printf '%s\n' "$rec" > "$DDIR/launch"
out="$(bobserve "$KEY" 2>&1)"; rc=$?
assert "0271: a launch record with no branch is NOT verified against the current HEAD" '[ "$rc" = "1" ]'
assert "0271: and the missing branch is surfaced as unverifiable" \
  'grep -qF "task-unverifiable launch-branch-missing" <<<"$out"'

# (d) a non-build, non-implement-next agent keeps the sentinel-only disposition
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0271: a status agent still observes as complete on the sentinel alone" '[ "$rc" = "0" ]'
assert "0271: no build verdict is claimed for a status agent" '! grep -qF "task-committed" <<<"$out"'

# ---- the observe seam carries implement-next's RUN GATE (change 0237, reached detached) ----
# Once the shim ALWAYS launches, the synchronous fence at the bottom of the facade is unreachable
# for every delegated implement-next run. Without a disposition on this seam a run that HALTED, or
# that stopped before its PR, exits 0 at the adapter and observes as `complete (child exited 0)` —
# the shim then reports success, which is exactly the prose-level failure change 0237 exists to
# eliminate. These arms drive the verdict through the VERIFY_RUN mock seam, so nothing here needs a
# docket metadata tree.
#
# The claim stamp is in the FUTURE relative to the launch, because attribution keeps only a claim
# stamped at or after the dispatch epoch; a stamp in the past is a different (already-covered)
# property and would make every arm below vacuously green.
GFUT=$(( $(date -u +%s) + 600 ))

mkgatefixture(){   # $1 = the verdict line the stub reports for change 7
  make_fixture
  SNAP="$SBX/snap"; mkdir -p "$SNAP"
  ORDER="$SBX/order.log"; : > "$ORDER"
  : > "$SBX/ad.log"
  printf '' > "$SNAP/before"                       # nothing claimed at the handoff
  printf '%s %s\n' 7 "$GFUT" > "$SNAP/after"       # one claim, stamped inside this dispatch's window
  printf '%s\n' "$1" > "$SNAP/verdict.7"
  # The stub reader. It serves the two snapshot shapes from files the fixture controls and the
  # verdict from a third, and it delegates ONLY `--iso-to-epoch` to the real script (pure, needs no
  # config) so the still-running budget path behaves as it does in production.
  cat > "$SBX/fake-vr.sh" <<VRE
#!/usr/bin/env bash
printf 'vr %s\n' "\$*" >> "${ORDER:?}"
case "\$1" in --iso-to-epoch) exec "$ROOT/scripts/verify-run.sh" "\$@" ;; esac
for a in "\$@"; do [ "\$a" = "--build" ] && { printf 'task-committed test-branch\n'; exit 0; }; done
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
  chmod +x "$SBX/fake-vr.sh"
  # The re-sync seam, logged so the ORDER of re-sync vs snapshot read is assertable: both reads
  # must be of FRESH ORIGIN state, and an unsynced one attributes an abandoned claim to this run.
  cat > "$SBX/fake-facade.sh" <<'FF'
#!/usr/bin/env bash
printf 'facade %s\n' "$*" >> "${ORDER_LOG:?}"
exit 0
FF
  chmod +x "$SBX/fake-facade.sh"
  # An adapter that APPENDS, so "no re-dispatch happened" is a count and not an overwrite.
  cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
printf 'ran\n' >> "${AD_LOG:?}"
printf 'fake adapter stdout\n'
exit "${FAKE_RC:-0}"
FAKE
  chmod +x "$RDIR/fake.sh"
}
gate_env(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" DELEGATION_OBSERVATION_BUDGET=60 \
    ORDER_LOG="$ORDER" AD_LOG="$SBX/ad.log" FAKE_RC="${FAKE_RC:-0}" \
    VERIFY_RUN="$SBX/fake-vr.sh" DOCKET_FACADE="$SBX/fake-facade.sh" \
    bash "$FACADE" "$@" ); }
# `shift`-then-"$@" rather than a `"${@:2}"` slice: the slice's behavior on an empty positional set
# is the kind of thing that differs across the bash versions this suite is run under, and every arm
# below calls these with no trailing flags at all.
glaunch(){ local ag="${1:-implement-next}"; [ $# -gt 0 ] && shift
  gate_env --launch --runner fake --agent "$ag" "$@"; }
gobserve(){ local k="$1" ag; shift; ag="${1:-implement-next}"; [ $# -gt 0 ] && shift
  gate_env --observe "$k" --runner fake --agent "$ag" "$@"; }
gsettle(){ local _; for _ in $(seq 1 40); do [ -f "$(ddir_for "$1")/done" ] && return 0; sleep 0.5; done; return 0; }

# (e) HALTED — the disposition the design spec pins normatively under detachment: exit 3, never 0.
mkgatefixture "run-halted 7"
KEY="$(glaunch)"
DDIR="$(ddir_for "$KEY")"
assert "0271-gate: the launch records the dispatch epoch" 'grep -qE "^dispatch_epoch=[0-9]+$" "$DDIR/launch"'
assert "0271-gate: the launch records the before-snapshot" '[ -f "$DDIR/gate-before" ]'
assert "0271-gate: the before-snapshot is preceded by a metadata re-sync" \
  '[ "$(sed -n 1p "$ORDER")" = "facade preflight" ]'
assert "0271-gate: and the before-read is the bare id form" \
  '[ "$(sed -n 2p "$ORDER")" = "vr --in-progress-ids" ]'
gsettle "$KEY"
out="$(gobserve "$KEY" 2>&1)"; rc=$?
assert "0271-gate: a HALTED delegated implement-next run observes as 3, not 0" '[ "$rc" = "3" ]'
# Keyed on the phrase that separates a HALT from a failure, not on the verdict token: the verdict
# line is echoed on every disposition, so `halted` alone stays green even with the halt mapping
# removed — the assert would measure the echo rather than the disposition.
assert "0271-gate: the halt diagnostic distinguishes stop-for-a-human from failed" \
  'grep -qi "needs a human" <<<"$out"'
assert "0271-gate: the after-read is re-synced too (fresh origin on BOTH sides)" \
  '[ "$(grep -c "^facade preflight$" "$ORDER" | tr -d " ")" -ge "2" ]'
assert "0271-gate: the after-read carries the claim stamps" \
  'grep -qxF "vr --in-progress-ids --with-claimed-at" "$ORDER"'
assert "0271-gate: a halt is never re-dispatched from an observation" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
rout="$(gobserve "$KEY" 2>/dev/null)"
assert "0271-gate: a halted run still relays the child's stdout" 'grep -qF "fake adapter stdout" <<<"$rout"'
out2="$(gobserve "$KEY" 2>&1)"; rc2=$?
assert "0271-gate: the halt verdict is idempotent in code" '[ "$rc2" = "$rc" ]'

# (f) COMPLETE — a positive green verdict observes as 0.
mkgatefixture "run-complete 7"
KEY="$(glaunch)"
gsettle "$KEY"
out="$(gobserve "$KEY" 2>&1)"; rc=$?
assert "0271-gate: a completed delegated implement-next run observes as 0" '[ "$rc" = "0" ]'
assert "0271-gate: the completion diagnostic names the verdict" 'grep -qF "run-complete 7" <<<"$out"'

# (g) INCOMPLETE — stopped before its PR. Reported as a failure (1), and NEVER re-dispatched from
#     an observation: the synchronous gate's one bounded re-dispatch is deliberately not recreated
#     at this seam, because re-launching a detached child out of a repeated short read would race
#     the very run being observed.
mkgatefixture "run-incomplete 7 status pr branch"
KEY="$(glaunch)"
gsettle "$KEY"
out="$(gobserve "$KEY" 2>&1)"; rc=$?
assert "0271-gate: an unfinished delegated implement-next run observes as 1" '[ "$rc" = "1" ]'
assert "0271-gate: the failure diagnostic names the unmet conjuncts" \
  'grep -qF "run-incomplete 7 status pr branch" <<<"$out"'
assert "0271-gate: an observation never re-dispatches the adapter" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
gobserve "$KEY" >/dev/null 2>&1
assert "0271-gate: and re-observing still never re-dispatches" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'

# (h) attribution — a claim that predates the dispatch epoch is another session's abandoned one and
#     must not be verified. Same three filters as the synchronous gate, so the same fixture shape.
mkgatefixture "run-halted 7"
printf '%s %s\n' 7 "$(( $(date -u +%s) - 100000 ))" > "$SNAP/after"
KEY="$(glaunch)"
gsettle "$KEY"
out="$(gobserve "$KEY" 2>&1)"; rc=$?
assert "0271-gate: a claim stamped before the dispatch is not attributed to it" '[ "$rc" = "0" ]'
assert "0271-gate: and its verdict is never read" '! grep -qxF "vr 7" "$ORDER"'
assert "0271-gate: the skip is announced, not silent" 'grep -qi "run gate" <<<"$out"'

# (i) attribution — two fresh claims cannot be told apart, so the gate stands down rather than
#     reporting one run's disposition for another's change.
mkgatefixture "run-halted 7"
printf '%s %s\n' 7 "$GFUT" 9 "$GFUT" > "$SNAP/after"
KEY="$(glaunch)"
gsettle "$KEY"
out="$(gobserve "$KEY" 2>&1)"; rc=$?
assert "0271-gate: an ambiguous claim set stands down to the sentinel disposition (0)" '[ "$rc" = "0" ]'
assert "0271-gate: an ambiguous claim set verifies nothing" '! grep -qxF "vr 7" "$ORDER"'

# (j) a build-* observation is UNAFFECTED by the new leg. Non-vacuous: the same fixture would
#     report a halt (3) if the disposition were keyed on the wrong agent family, and the
#     implement-next leg is the only reader of `--with-claimed-at` at this seam.
mkgatefixture "run-halted 7"
git -C "$SBX" checkout -q -b feat/thing
KEY="$(glaunch build-standard --worktree "$SBX")"
gsettle "$KEY"
out="$(gobserve "$KEY" build-standard --worktree "$SBX" 2>&1)"; rc=$?
assert "0271-gate: a build-* observation never takes the implement-next disposition" '[ "$rc" != "3" ]'
assert "0271-gate: a build-* observation reads the build verdict instead" \
  'grep -qF "task-committed" <<<"$out"'
assert "0271-gate: a build-* observation never reads the claim snapshot" \
  '! grep -qF -- "--with-claimed-at" "$ORDER"'
assert "0271-gate: a build-* launch records no dispatch epoch" \
  '! grep -qE "^dispatch_epoch=[0-9]+$" "$(ddir_for "$KEY")/launch"'

# ---- the posture cites gate-execution.md, never restates the six capabilities ----
DOC="$ROOT/scripts/runner-dispatch.md"
DEL="$ROOT/skills/docket-build/references/delegation-execution.md"
assert "0271: the delegation verdicts reference exists" '[ -f "$DEL" ]'
assert "0271: runner-dispatch.md states a delegation execution posture" \
  'grep -qi "delegation execution posture" "$DOC"'
# CITES rather than RESTATES (the change-0154 discipline): the posture must point at the
# quarantine file, and must NOT grow its own copy of the numbered capability list. The citation
# itself is the one permitted mention, which is what makes the ceiling non-vacuous — a pasted
# copy of gate-execution.md's own section carries a second one and reddens this.
assert "0271: the posture cites gate-execution.md" 'grep -qF "gate-execution.md" "$DOC"'
assert "0271: the posture does not restate the six capabilities" \
  '[ "$(grep -ciE "six (required )?capabilities" "$DOC")" -le 1 ]'
assert "0271: the posture says a delegated run may outlive its launching call" \
  'grep -qiE "outlive (the|its) call" "$DOC"'

# Per-harness rows are DERIVED from the shipped roster, never hand-listed — and the population
# is floored so an extractor returning nothing cannot read as parity holding
# (LEARNINGS: marker-scoped-guard-needs-a-population-floor). The roster is read from its own
# definition site, `HD_SHIPPED_HARNESSES` in scripts/lib/harness-defaults.sh, by sourcing that
# reader the way every other harness-population guard in this suite does.
# shellcheck source=/dev/null
. "$ROOT/scripts/lib/harness-defaults.sh"
n_h=0
for h in $HD_SHIPPED_HARNESSES; do
  # Read THIS harness's table row, not the file at large: a bare file-wide grep for the token
  # would be satisfied by any other harness's row. `[|]` rather than `\|` so BSD ERE and ugrep
  # agree the pipe is a literal and not an alternation.
  assert "0271: delegation verdicts carry a row for harness '$h'" \
    'row="$(grep -iE "^[|][[:space:]]*'"$h"'[[:space:]|]" "$DEL")"; [ -n "$row" ]'
  # Honesty: an unmeasured adapter launch shape must read `unverified`, never inherit the gate's
  # `supported`. gate-execution.md's own rule — a verdict is version- and scope-scoped.
  assert "0271: harness '$h' ships unverified, never supported" \
    'row="$(grep -iE "^[|][[:space:]]*'"$h"'[[:space:]|]" "$DEL")";
     grep -qF "unverified" <<<"$row" && ! grep -qF "supported" <<<"$row"'
  n_h=$((n_h+1))
done
assert "0271: the harness loop actually enumerated the roster (got $n_h)" '[ "$n_h" -ge 4 ]'
assert "0271: the reference says the gate verdicts do NOT transfer" \
  'grep -qiE "do(es)? not transfer|never inherit" "$DEL"'
# The mechanism measurement and the per-harness gap are SEPARATE claims: a reader must not be able
# to read the hermetic process-group result as evidence about any child CLI.
assert "0271: the reference records the hermetic mechanism measurement separately" \
  'grep -qiE "measured hermetically" "$DEL"'

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

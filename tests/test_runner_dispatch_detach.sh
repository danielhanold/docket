#!/usr/bin/env bash
# tests/test_runner_dispatch_detach.sh — the LAUNCH half of the change-0271 detachment posture:
# the child's own process group, the launch record, the sentinel, the durable streams, the
# retention rule, and the argument gates that guard a launch.
# Run: bash tests/test_runner_dispatch_detach.sh
#
# Its siblings — tests/test_runner_dispatch_observe.sh and
# tests/test_runner_dispatch_build_gate.sh — cover the observation seam. The shared prologue and
# fixtures live in tests/lib/runner_dispatch_detach_common.sh, whose header records why the file
# was cut into three.
# shellcheck source=lib/runner_dispatch_detach_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/runner_dispatch_detach_common.sh"

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

# ---- RETENTION: old terminal dispatches are pruned, live ones never are ---------
# Every launch mints a dir holding a whole agent run's logs and nothing else in docket removes one,
# so .git grows without bound under the autonomous drainer. The prune runs at the top of `--launch`
# and is deliberately conservative: only a TERMINAL dispatch (a `done` sentinel or a `killed`
# marker) whose terminal file is older than the retention window is eligible.
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
OLDKEY="$(launch status)"
OLDD="$(ddir_for "$OLDKEY")"
for _ in $(seq 1 30); do [ -f "$OLDD/done" ] && break; sleep 1; done
FRESHKEY="$(launch status)"
FRESHD="$(ddir_for "$FRESHKEY")"
for _ in $(seq 1 30); do [ -f "$FRESHD/done" ] && break; sleep 1; done
# A LIVE child: no sentinel, no marker, and its launch record BACKDATED — so "age alone never
# prunes a live dispatch" is a measurement and not an assumption.
FAKE_SLEEP=30
LIVEKEY="$(launch status)"
LIVED="$(ddir_for "$LIVEKEY")"
pr_pgid="$(sed -n 's/^pgid=//p' "$LIVED/launch")"
FAKE_SLEEP=0
DROOT_T="$(dirname "$OLDD")"
# Two synthetic dispatches, so both remaining classes are covered without waiting on a real one: an
# old KILLED dispatch (terminal by the observer's marker rather than by the launcher's sentinel),
# and an old dispatch with NO terminal file at all, which must survive forever.
SYN_KILLED="$DROOT_T/status-19700101T000000Z-11111"; mkdir -p "$SYN_KILLED"
printf 'killed_at=1970-01-01T00:00:00Z\nreason=budget-exhausted\n' > "$SYN_KILLED/killed"
SYN_LIVE="$DROOT_T/status-19700101T000000Z-22222"; mkdir -p "$SYN_LIVE"
printf 'pgid=1\nstarted_at=1970-01-01T00:00:00Z\n' > "$SYN_LIVE/launch"
touch -t 200001010000 "$OLDD/done" "$LIVED/launch" "$SYN_KILLED/killed" "$SYN_LIVE/launch"
NEWKEY="$(launch status)"     # the mint that runs the prune
assert "0271: a terminal dispatch older than the retention window is pruned" '[ ! -d "$OLDD" ]'
assert "0271: an old KILLED dispatch is pruned too — both terminal writes count" '[ ! -d "$SYN_KILLED" ]'
assert "0271: a terminal dispatch inside the window is KEPT — a caller may still observe it" \
  '[ -d "$FRESHD" ]'
assert "0271: a LIVE dispatch is never pruned, however old its record looks" '[ -d "$LIVED" ]'
assert "0271: nor is an old dispatch that never went terminal" '[ -d "$SYN_LIVE" ]'
assert "0271: and the dispatch being minted is untouched" '[ -d "$(ddir_for "$NEWKEY")" ]'
# The kept dispatch is still OBSERVABLE, which is the property the window exists to protect.
assert "0271: a kept terminal dispatch still observes as complete" \
  '( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --observe "$FRESHKEY" --runner fake --agent status ) >/dev/null 2>&1'
reap "$pr_pgid"

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

# ---- 0277: --launch spools the brief into the dispatch dir ---------------------------
# The brief becomes part of the dispatch's audit record, beside `launch`, `stdout.log`, and
# `done` — and the adapter is handed the DURABLE copy, so a detached child no longer depends on
# the caller's temp file outliving the call that started it.
make_fixture
FAKE_SLEEP=0
BF="$SBX/caller-brief.txt"
printf 'spooled-line-one\nspooled-line-two\n' > "$BF"
FAKE_ARGV_LOG="$SBX/argv.log"
KEY="$(launch status --brief-file "$BF")"; rc=$?
assert "0277 launch: a brief-file launch exits 0" '[ "$rc" = "0" ]'
DDIR="$(ddir_for "$KEY")"
# Wait for the (instant) child so its argv log is complete before reading it.
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -f "$DDIR/done" ] && break; sleep 0.3; done
assert "0277 launch: the brief was spooled into the dispatch dir" '[ -f "$DDIR/brief" ]'
assert "0277 launch: the spooled brief is byte-identical to the caller's" 'cmp -s "$BF" "$DDIR/brief"'
assert "0277 launch: no partial file is left behind" '[ ! -e "$DDIR/brief.partial" ]'
argv="$(cat "$SBX/argv.log" 2>/dev/null)"
assert "0277 launch: the adapter was handed the DURABLE copy" 'grep -qxF -- "$DDIR/brief" <<<"$argv"'
assert "0277 launch: the adapter was NOT handed the caller's path" '! grep -qxF -- "$BF" <<<"$argv"'
unset FAKE_ARGV_LOG
rm -rf "$SBX"

# The exclusion and the build-* payload gate are pre-verb, so they refuse BEFORE anything is
# minted — a refused dispatch leaves no dispatch dir behind.
make_fixture
printf 'a brief\n' > "$SBX/b.txt"
before="$(ls "$(ddir_for "" )" 2>/dev/null | wc -l | tr -d ' ')"
err="$( launch status --brief-file "$SBX/b.txt" -- "argv too" 2>&1 >/dev/null )"; rc=$?
after="$(ls "$(ddir_for "" )" 2>/dev/null | wc -l | tr -d ' ')"
assert "0277 launch: both channels are refused" '[ "$rc" != "0" ]'
assert "0277 launch: the refusal says never both" 'grep -qiF "never both" <<<"$err"'
assert "0277 launch: the refusal minted no dispatch dir" '[ "$before" = "$after" ]'
rm -rf "$SBX"

exit "$fail"

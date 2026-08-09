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

make_fixture(){  # sets SBX (repo), RDIR (fake runners dir)
  SBX="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach.XXXXXX")"; SBX="$(cd "$SBX" && pwd -P)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  RDIR="$SBX/fake-runners"; mkdir -p "$RDIR"
  # The fake adapter: sleeps FAKE_SLEEP, then writes a marker and exits FAKE_RC.
  cat > "$RDIR/fake.sh" <<'FAKE'
#!/usr/bin/env bash
sleep "${FAKE_SLEEP:-0}"
printf 'adapter-ran\n' > "$FAKE_MARKER"
printf 'fake adapter stdout\n'
printf 'fake adapter stderr\n' >&2
exit "${FAKE_RC:-0}"
FAKE
  chmod +x "$RDIR/fake.sh"
}
launch(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_SLEEP="${FAKE_SLEEP:-0}" FAKE_RC="${FAKE_RC:-0}" \
    bash "$FACADE" --launch --runner fake --agent "${1:-status}" ); }

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

rm -rf "$SBX"
exit "$fail"

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

# ---- a failed child -> 1 --------------------------------------------------------
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=9
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "a non-zero adapter code observes as failed (1)" '[ "$rc" = "1" ]'
assert "the failure diagnostic reports the child's code" 'grep -qF "exited 9" <<<"$out"'

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

# (d) a non-build, non-implement-next agent keeps the sentinel-only disposition
make_fixture
FAKE_SLEEP=0 FAKE_TAIL=0 FAKE_RC=0
KEY="$(launch status)"
for _ in $(seq 1 30); do observe "$KEY" >/dev/null 2>&1; [ "$?" != "4" ] && break; sleep 1; done
out="$(observe "$KEY" 2>&1)"; rc=$?
assert "0271: a status agent still observes as complete on the sentinel alone" '[ "$rc" = "0" ]'
assert "0271: no build verdict is claimed for a status agent" '! grep -qF "task-committed" <<<"$out"'

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

exit "$fail"

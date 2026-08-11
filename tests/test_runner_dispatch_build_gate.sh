#!/usr/bin/env bash
# tests/test_runner_dispatch_build_gate.sh — the GIT-READ dispositions on change 0271's observe
# seam: the `build-*` verdict family (the disagreement rule and the branch conjunct), change 0237's
# implement-next run gate reached through detachment, and the posture-doc asserts that keep
# runner-dispatch.md citing gate-execution.md rather than restating it.
# Run: bash tests/test_runner_dispatch_build_gate.sh
#
# Sharded out of tests/test_runner_dispatch_detach.sh, which keeps the launch half; the observation
# dispositions and the budget live in tests/test_runner_dispatch_observe.sh.
# tests/lib/runner_dispatch_detach_common.sh carries the shared prologue, fixtures and helpers,
# and its header records why the file was cut into three.
# shellcheck source=lib/runner_dispatch_detach_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/runner_dispatch_detach_common.sh"

# ---- the gate now covers build-*, via the BUILD verdict family ------------------
mkbuildrepo(){   # a repo + feature worktree the fake adapter can commit into
  local outside
  make_fixture
  # These are the only arms whose verdict reads `git status` on the anchor, so they are the only
  # ones for which make_fixture's RUNNERS_DIR placement ever mattered: it sits INSIDE $SBX, where it
  # is untracked scaffolding that would leave the `tree` conjunct unmet no matter what the child
  # did — turning "clean build task" into a permanent failure and "stranded work" into a green
  # assert that measures the fixture rather than the child. Since 0208 the anchor is the LINKED
  # WORKTREE below, which does not contain $RDIR, so that hazard is gone by construction; the move
  # is kept because it makes the independence STRUCTURAL rather than a property of where the
  # worktree happens to be placed. Nothing else about the dispatch depends on where the adapter lives.
  outside="$(mktemp -d "${TMPDIR:-/tmp}/docket-detach-runners.XXXXXX")"
  FIXTURES+=("$outside")
  mv -f "$RDIR/fake.sh" "$outside/fake.sh"
  rmdir "$RDIR"
  RDIR="$outside"
  # A REAL LINKED WORKTREE, not $SBX itself. `build-standard` declares `worktree-scope: feature`, and
  # since 0208 the facade refuses a feature-scoped dispatch anchored at the MAIN worktree — the
  # primary checkout on the integration branch is the precise value that gate exists to reject. With
  # `WT="$SBX"` every arm below would abort before the adapter ran and their asserts would measure a
  # refusal. `feat/thing` lives on the worktree rather than on the main tree, so the branch conjunct
  # (c2)/(c3) still read the same recorded value.
  git -C "$SBX" worktree add -q -b feat/thing "$SBX/.worktrees/build" >/dev/null 2>&1
  WT="$SBX/.worktrees/build"     # the fake adapter runs with --worktree $WT
  assert "0208: fixture sanity — the build arm's anchor is a REAL linked worktree, not the main tree" \
    '[ -f "$WT/.git" ] && [ "$WT" != "$SBX" ]'
}
# The build arms drive the facade directly rather than through `launch`/`observe`: those helpers
# hard-code `--agent status` shape and omit `--worktree`, which gate 1 requires for build-*.
# The trailing payload satisfies change 0277's build-* empty-payload gate, which refuses a
# task-less build-* dispatch before anything is launched. `bobserve` needs none: the gate exempts
# `--observe`, which starts no child.
blaunch(){ ( cd "$SBX" && RUNNERS_DIR="$RDIR" bash "$FACADE" --launch --runner fake \
    --agent build-standard --worktree "$WT" -- "build task" ); }
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
# COUNTS ITS OWN INVOCATIONS, beside the adapter rather than inside $SBX: the build verdict reads
# `git status` on the fixture repo, so a counter file in the worktree would change what the (b)
# arm measures. Read by the observe-only assert below.
printf 'ran\n' >> "$(dirname "$0")/adapter-runs"
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
assert "0271: the stranded file is still there for a human" '[ -f "$WT/stranded.txt" ]'
# The verdict is TERMINAL and re-reports identically — a disagreement must not oscillate between
# a failure and a success across two reads of the same unchanged dispatch.
out2="$(bobserve "$KEY" 2>&1)"; rc2=$?
assert "0271: the disagreement verdict is idempotent in code" '[ "$rc2" = "$rc" ]'
assert "0271: the disagreement verdict is idempotent in output" '[ "$out2" = "$out" ]'

# (c) build-* is OBSERVE-ONLY: never a second adapter run on top of partial commits.
# MEASURED ON A COUNTER THE ADAPTER ITSELF APPENDS TO, never on the repo's commit count: this
# fixture's adapter never commits, so `rev-list --count HEAD` is 1 whether the adapter ran once or
# ten times — and a `-le 2` bound would have tolerated a re-dispatch's commit on top of that. The
# count is taken after SEVERAL observations (bsettle's poll, (b)'s two, and the two below), so a
# re-dispatch on any observation pass reddens it.
bobserve "$KEY" >/dev/null 2>&1
bobserve "$KEY" >/dev/null 2>&1
assert "0271: fixture sanity — the adapter recorded its own launch in the counter" \
  '[ -s "$RDIR/adapter-runs" ]'
assert "0271: no re-dispatch happened for a build agent — the adapter ran EXACTLY once" \
  '[ "$(wc -l < "$RDIR/adapter-runs" | tr -d " ")" = "1" ]'

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
# A REAL LINKED WORKTREE for the same reason mkbuildrepo builds one: `build-standard` declares
# `worktree-scope: feature`, and since 0208 a feature-scoped dispatch anchored at the MAIN worktree
# is refused before anything launches — which would leave "never takes the implement-next
# disposition" green for the wrong reason (no observation happened at all).
git -C "$SBX" worktree add -q -b feat/thing "$SBX/.worktrees/build" >/dev/null 2>&1
JWT="$SBX/.worktrees/build"
assert "0208: fixture sanity — the (j) anchor is a REAL linked worktree, not the main tree" \
  '[ -f "$JWT/.git" ] && [ "$JWT" != "$SBX" ]'
KEY="$(glaunch build-standard --worktree "$JWT" -- "build task")"  # payload: change 0277's build-* gate
gsettle "$KEY"
out="$(gobserve "$KEY" build-standard --worktree "$JWT" 2>&1)"; rc=$?
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

exit "$fail"

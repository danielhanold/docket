#!/usr/bin/env bash
# tests/test_docket_preflight.sh — hermetic tests for scripts/lib/docket-preflight.sh (change 0068).
# Sources the lib and drives docket_preflight against stubbed config exports + temp repos. No network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-preflight.sh"
SCRIPTS="$REPO/scripts"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# A fixture config-export command: prints the given lines. $1 = a file with KEY=value lines.
mkexport(){ printf '#!/usr/bin/env bash\ncat %q\n' "$1" > "$2"; chmod +x "$2"; }

# --- (A) non-PROCEED verdicts fail closed -----------------------------------
printf 'BOOTSTRAP=STOP_MIGRATE\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/stop.env"
mkexport "$tmp/stop.env" "$tmp/stop-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/stop-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/stop.err"; rc=$?
assert "STOP_MIGRATE returns non-zero" '[ "$rc" -ne 0 ]'
assert "STOP_MIGRATE names migrate-to-docket" 'grep -qi "migrate" "$tmp/stop.err"'

printf 'BOOTSTRAP=CREATE_ORPHAN\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/orphan.env"
mkexport "$tmp/orphan.env" "$tmp/orphan-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/orphan-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/orphan.err"; rc=$?
assert "CREATE_ORPHAN returns non-zero" '[ "$rc" -ne 0 ]'

printf 'BOOTSTRAP=WAT\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/wat.env"
mkexport "$tmp/wat.env" "$tmp/wat-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/wat-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/wat.err"; rc=$?
assert "unknown verdict returns non-zero" '[ "$rc" -ne 0 ]'

# --- (B) docket-mode PROCEED creates + syncs the metadata worktree ----------
# Build a repo with a real `docket` branch on a bare origin.
bare="$tmp/dk.git"; work="$tmp/dk"
git init --quiet --bare "$bare"
git clone --quiet "$bare" "$work" 2>/dev/null
git -C "$work" config user.email t@t.test; git -C "$work" config user.name Test
git -C "$work" checkout --quiet -b main; : > "$work/README.md"
git -C "$work" add README.md; git -C "$work" commit --quiet -m init; git -C "$work" push --quiet -u origin main
git -C "$work" push --quiet origin "$(git -C "$work" commit-tree "$(git -C "$work" mktree </dev/null)" -m orphan):refs/heads/docket"
git -C "$work" fetch --quiet origin docket
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\nINTEGRATION_BRANCH=main\nCHANGES_DIR=docs/changes\n' > "$tmp/ok.env"
mkexport "$tmp/ok.env" "$tmp/ok-export.sh"
assert "metadata worktree absent before preflight" '[ ! -d "$work/.docket" ]'
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/ok.err"; rc=$?
assert "docket-mode PROCEED returns zero" '[ "$rc" -eq 0 ]'
assert "docket-mode PROCEED created the metadata worktree" '[ -d "$work/.docket" ]'

# --- (C) PROCEED sets config vars in the caller's scope, METADATA_WORKTREE ABSOLUTE (0075) ------
work_abs="$(cd "$work" && pwd -P)"
DOCKET_MODE=""; METADATA_WORKTREE=""
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" >/dev/null 2>&1 \
  && [ "$DOCKET_MODE" = docket ] && [ "$METADATA_WORKTREE" = "$work_abs/.docket" ] ); rc=$?
assert "PROCEED exposes resolved config vars, with METADATA_WORKTREE anchored ABSOLUTE (0075)" '[ "$rc" -eq 0 ]'

# --- (D) change 0075 / defect D2: preflight from INSIDE the metadata worktree -------------------
# Pre-0075 this created a real <repo>/.docket/.docket worktree and still exited 0. The metadata
# worktree path must be built from the MAIN worktree, so running preflight from a linked worktree
# is a no-op with respect to the worktree set.
before="$(git -C "$work" worktree list --porcelain | grep -c '^worktree ')"
( cd "$work/.docket" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/d2.err"; rc=$?
after="$(git -C "$work" worktree list --porcelain | grep -c '^worktree ')"
assert "D2: preflight from inside .docket/ returns zero" '[ "$rc" -eq 0 ]'
assert "D2: preflight from inside .docket/ creates NO second worktree" '[ "$before" = "$after" ]'
assert "D2: no nested <repo>/.docket/.docket directory was minted" '[ ! -d "$work/.docket/.docket" ]'
assert "D2: the worktree list contains no nested .docket/.docket entry" \
  '! git -C "$work" worktree list --porcelain | grep >/dev/null "^worktree .*/\.docket/\.docket$"'

# --- (E) D2, the harder shape: the target does not yet exist under the caller's CWD -------------
# A fresh clone whose .docket/ has NOT been created yet, with the caller standing in a linked
# feature worktree. The relative ".docket" would resolve under THAT worktree.
work2="$tmp/dk2"
git clone --quiet "$bare" "$work2" 2>/dev/null
git -C "$work2" config user.email t@t.test; git -C "$work2" config user.name Test
git -C "$work2" fetch --quiet origin docket
git -C "$work2" branch --quiet feat/y
git -C "$work2" worktree add --quiet "$work2/.worktrees/feat-y" feat/y >/dev/null 2>&1
work2_abs="$(cd "$work2" && pwd -P)"
( cd "$work2/.worktrees/feat-y" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/d2b.err"; rc=$?
assert "D2b: preflight from a feature worktree returns zero" '[ "$rc" -eq 0 ]'
assert "D2b: the metadata worktree was created at the MAIN root" '[ -d "$work2_abs/.docket" ]'
assert "D2b: NOT under the feature worktree" '[ ! -d "$work2/.worktrees/feat-y/.docket" ]'

# --- (F) the nested-target guard refuses rather than creating debris ----------------------------
# Force the pathological target directly: a metadata worktree path INSIDE an existing LINKED
# worktree is never legitimate, so preflight must refuse (non-zero) and create nothing.
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=%s\nINTEGRATION_BRANCH=main\nCHANGES_DIR=docs/changes\n' \
  "$work2_abs/.worktrees/feat-y/.docket" > "$tmp/nested.env"
mkexport "$tmp/nested.env" "$tmp/nested-export.sh"
( cd "$work2" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/nested-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/nested.err"; rc=$?
assert "D2 guard: a metadata target inside a LINKED worktree is refused (non-zero)" '[ "$rc" -ne 0 ]'
assert "D2 guard: the refusal explains itself on stderr" 'grep -qi "inside an existing worktree" "$tmp/nested.err"'
assert "D2 guard: nothing was created at the refused target" '[ ! -d "$work2_abs/.worktrees/feat-y/.docket" ]'

# --- (G) main-mode PROCEED anchors METADATA_WORKTREE to the absolute repo root (review finding) ---
# Sections C-F above only ever exercise DOCKET_MODE=docket; main-mode's anchor (config value "."
# mapped by docket_anchor_path to the root) had NO test at all, so a regression that moved the
# anchor line inside the docket-mode branch would ship undetected. Reuses $bare (already a real
# repo with `main` tracking `origin/main`, built in section B) for a fresh clone, since main-mode's
# sync needs a real remote to fetch `main` from. Section P below extends this same $work3.
work3="$tmp/dk3"
git clone --quiet "$bare" "$work3" 2>/dev/null
git -C "$work3" config user.email t@t.test; git -C "$work3" config user.name Test
work3_abs="$(cd "$work3" && pwd -P)"
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=main\nMETADATA_BRANCH=main\nMETADATA_WORKTREE=.\nINTEGRATION_BRANCH=main\nCHANGES_DIR=docs/changes\n' > "$tmp/main.env"
mkexport "$tmp/main.env" "$tmp/main-export.sh"

( cd "$work3" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/main.err"; rc=$?
assert "main-mode PROCEED returns zero" '[ "$rc" -eq 0 ]'

DOCKET_MODE=""; METADATA_WORKTREE=""
( cd "$work3" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" docket_preflight "$SCRIPTS" >/dev/null 2>&1 \
  && [ "$DOCKET_MODE" = main ] && [ "$METADATA_WORKTREE" = "$work3_abs" ] ); rc=$?
assert "main-mode PROCEED anchors METADATA_WORKTREE to the absolute repo root, not \".\" (review finding)" '[ "$rc" -eq 0 ]'

# Same check from a SUBDIRECTORY of the main worktree — where a CWD-relative "." would silently
# differ from the anchored root. This is exactly the mutation shape this guard exists to catch.
mkdir -p "$work3/sub"
DOCKET_MODE=""; METADATA_WORKTREE=""
( cd "$work3/sub" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" docket_preflight "$SCRIPTS" >/dev/null 2>&1 \
  && [ "$DOCKET_MODE" = main ] && [ "$METADATA_WORKTREE" = "$work3_abs" ] ); rc=$?
assert "main-mode PROCEED from a subdirectory still anchors METADATA_WORKTREE to the repo root, not the subdirectory (review finding)" '[ "$rc" -eq 0 ]'

# --- (H)-(N) change 0247: the bounded, discriminating metadata sync ------------------------------
# Sections H-N drive _docket_sync_metadata through the shared-worktree collision classes. They all
# run against $work/.docket (created in section B, a real linked worktree on branch `docket`
# tracking origin/docket in the bare $bare) and against $other, a second clone standing in for the
# concurrent agent. Every one of them injects DOCKET_PREFLIGHT_TEST_SLEEP_CMD, so the ~22s real
# backoff budget costs zero wall-clock here — the suite's per-file budgets make real sleeping in a
# fixture a defect, not a style choice.

# mkcounter COUNTFILE SCRIPT — a fixture backoff command that counts its own invocations into
# COUNTFILE. Counting the backoffs is how a test can tell "took the fast path" from "retried and
# eventually succeeded", and how the bound on the retry loop becomes observable at all.
mkcounter(){
  rm -f "$1"
  printf '#!/usr/bin/env bash\nn=$(cat %q 2>/dev/null || echo 0); echo "$((n+1))" > %q\nexit 0\n' \
    "$1" "$1" > "$2"
  chmod +x "$2"
}
# backoffs COUNTFILE — the number of backoffs a run spent (0 when the file was never written).
backoffs(){ cat "$1" 2>/dev/null || echo 0; }

# --- (H) a dirty tree with NOTHING to pull must never fail the sync -----------------------------
# The most common collision by far: the other agent is mid-edit and has not pushed. There is
# nothing to integrate, so no rebase is needed, so a dirty tree is irrelevant. Pre-0247 this was a
# hard failure — `pull --rebase` refuses on unstaged changes regardless of whether it has work.
printf 'tracked\n' > "$work/.docket/tracked.txt"
git -C "$work/.docket" add tracked.txt >/dev/null 2>&1
git -C "$work/.docket" commit -q -m tracked >/dev/null 2>&1
git -C "$work/.docket" push -q origin HEAD:docket >/dev/null 2>&1
printf 'modified\n' >> "$work/.docket/tracked.txt"          # tracked + dirty, remote NOT moved
mkcounter "$tmp/h.count" "$tmp/h-sleep.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/h-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/h.err"; rc=$?
assert "H: dirty tracked tree with no remote movement syncs successfully" '[ "$rc" -eq 0 ]'
assert "H: the dirty edit was NOT stashed, reset, or committed away" \
  'grep -q "^modified$" "$work/.docket/tracked.txt"'
assert "H: it took the fast path — no retry budget spent" '[ "$(backoffs "$tmp/h.count")" -eq 0 ]'

# --- (I) untracked-only files never count as dirty ----------------------------------------------
# The remote MUST have moved here. With the remote standing still the fast path returns before the
# dirty predicate is ever consulted, so an untracked-only fixture would assert nothing about how
# untracked files are classified — it would stay green with the predicate counting them as dirty.
# Zero backoffs is the load-bearing assert: it is what separates "untracked is not dirty" from
# "the sync happened to succeed anyway".
git -C "$work/.docket" checkout -- tracked.txt
other="$tmp/other"; git clone --quiet "$bare" "$other" 2>/dev/null
git -C "$other" config user.email t@t.test; git -C "$other" config user.name Test
git -C "$other" checkout --quiet -B docket origin/docket
printf 'remote moved\n' > "$other/remote-moved.txt"
git -C "$other" add remote-moved.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
: > "$work/.docket/stray-untracked.txt"
mkcounter "$tmp/i.count" "$tmp/i-sleep.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/i-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/i.err"; rc=$?
assert "I: an untracked-only file never fails the sync" '[ "$rc" -eq 0 ]'
assert "I: the untracked-only tree was NOT classified dirty — zero retry budget spent" \
  '[ "$(backoffs "$tmp/i.count")" -eq 0 ]'
assert "I: the remote commit was integrated" '[ -f "$work/.docket/remote-moved.txt" ]'
assert "I: the untracked file survives the sync" '[ -f "$work/.docket/stray-untracked.txt" ]'
rm -f "$work/.docket/stray-untracked.txt"

# --- (J) dirty tree + remote MOVED: retries, then succeeds once the other agent finishes --------
# The sleep seam is the ONLY point in the loop where a second actor could have acted, so the
# fixture models "the other agent committed and the tree went clean" from inside it. This also
# proves the retry loop actually re-evaluates state between attempts rather than re-running a
# decision it made once.
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'remote moved again\n' > "$other/remote-moved-2.txt"
git -C "$other" add remote-moved-2.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent 2" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
printf 'mid-edit\n' >> "$work/.docket/tracked.txt"          # our tree is dirty AND remote moved
rm -f "$tmp/heal.count"
{ printf '#!/usr/bin/env bash\n'
  printf 'n=$(cat %q 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > %q\n' "$tmp/heal.count" "$tmp/heal.count"
  # On the 2nd backoff, the "other agent" finishes: our tree goes clean.
  printf '[ "$n" -ge 2 ] && git -C %q checkout -- tracked.txt\n' "$work/.docket"
  printf 'exit 0\n'
} > "$tmp/heal.sh"
chmod +x "$tmp/heal.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/heal.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/j.err"; rc=$?
assert "J: a dirty tree with remote movement retries and then succeeds" '[ "$rc" -eq 0 ]'
assert "J: it actually spent at least two backoffs before succeeding" \
  '[ "$(backoffs "$tmp/heal.count")" -ge 2 ]'
assert "J: the remote commit was integrated" '[ -f "$work/.docket/remote-moved-2.txt" ]'

# --- (K) exhaustion: non-zero, and the diagnostic NAMES the last failure class ------------------
# The point of a discriminating retry is that the caller learns WHAT blocked it, not merely that
# five attempts died. Keeping the tree dirty for every attempt forces the `dirty` class.
printf 'still mid-edit\n' >> "$work/.docket/tracked.txt"
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'remote moved a third time\n' > "$other/remote-moved-3.txt"
git -C "$other" add remote-moved-3.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent 3" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
mkcounter "$tmp/exh.count" "$tmp/count.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/k.err"; rc=$?
assert "K: exhaustion returns non-zero (fail-closed, as before)" '[ "$rc" -ne 0 ]'
assert "K: the exhaustion diagnostic names the dirty-tracked-tree class" \
  'grep -qi "dirty" "$tmp/k.err"'
assert "K: the exhaustion diagnostic distinguishes it from a wedged tree" \
  '! grep -qi "rebase or merge is in progress" "$tmp/k.err"'
assert "K: the budget is bounded — exactly 4 backoffs across 5 attempts" \
  '[ "$(backoffs "$tmp/exh.count")" -eq 4 ]'
assert "K: it never integrated the remote behind a dirty tree" \
  '[ ! -f "$work/.docket/remote-moved-3.txt" ]'
git -C "$work/.docket" checkout -- tracked.txt

# --- (L) a conflicting local commit aborts IMMEDIATELY without burning the retry budget ---------
# A content conflict raised by THIS attempt's own rebase is deterministic: it fails identically on
# every retry, so spending budget on it is pure latency. The tree must be restored to its
# pre-attempt state (rebase --abort), never left mid-rebase for the next agent to find.
git -C "$work/.docket" pull -q --rebase origin docket >/dev/null 2>&1
printf 'ours\n' > "$work/.docket/conflict.txt"
git -C "$work/.docket" add conflict.txt >/dev/null 2>&1
git -C "$work/.docket" commit -q -m ours >/dev/null 2>&1
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'theirs\n' > "$other/conflict.txt"
git -C "$other" add conflict.txt >/dev/null 2>&1
git -C "$other" commit -q -m theirs >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
mkcounter "$tmp/conf.count" "$tmp/count2.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count2.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/l.err"; rc=$?
assert "L: a content conflict fails immediately (non-zero)" '[ "$rc" -ne 0 ]'
assert "L: it spent NO retry budget (zero backoffs)" '[ "$(backoffs "$tmp/conf.count")" -eq 0 ]'
assert "L: the diagnostic names the conflict class" 'grep -qi "conflict" "$tmp/l.err"'
# The rebase state lives under the LINKED worktree's own git dir, which is
# <main>/.git/worktrees/<name> — NOT "$work/.docket/.git", which is a gitdir POINTER FILE. A
# `[ ! -d "$work/.docket/.git/rebase-merge" ]` test can therefore never be false, and would pass
# against an implementation that left the rebase in progress. Resolve the real dir, and assert it
# resolved at all so the check cannot go vacuous a second way.
docket_gitdir="$(git -C "$work/.docket" rev-parse --absolute-git-dir 2>/dev/null)"
l_status="$(git -C "$work/.docket" status --porcelain 2>/dev/null)"
assert "L: the rebase-state probe resolved a real git dir (guards against a vacuous assert)" \
  '[ -n "$docket_gitdir" ] && [ -d "$docket_gitdir" ]'
assert "L: the tree was restored — no rebase left in progress" \
  '[ ! -d "$docket_gitdir/rebase-merge" ] && [ ! -d "$docket_gitdir/rebase-apply" ] && [ ! -f "$docket_gitdir/MERGE_HEAD" ] && ! grep -q "^UU" <<<"$l_status"'
assert "L: our own commit survived the abort" \
  '[ "$(cat "$work/.docket/conflict.txt" 2>/dev/null)" = ours ]'

# --- (M) the never-autostash rule, asserted by repo grep ----------------------------------------
# A shared tree makes the autostash flag a data-loss bug: it stashes ANOTHER agent's in-flight
# edits. Shape-keyed over the whole sync library, not an enumerated list of call sites.
#
# EXECUTABLE lines only. Only executable text can violate the ban, and the sync function's own
# INVARIANT comment has to be able to NAME the forbidden flag — that comment exists so a reviewer
# reads the rule at the code it binds. A whole-file literal grep makes those two requirements
# contradictory; sorting the file into prose and executable resolves it the way AGENTS.md does.
# Full-line comments are dropped by SHAPE, never by an enumerated allowlist, and a flag smuggled
# into a trailing comment on a code line still trips the gate.
lib_code="$(grep -v '^[[:space:]]*#' "$LIB")"
assert "M: the never-autostash grep has executable text to look at (guards a vacuous assert)" \
  '[ "$(grep -c "rebase" <<<"$lib_code")" -ge 1 ]'
assert "M: no autostash flag on any executable line of the metadata sync library" \
  '! grep -qF -- "--autostash" <<<"$lib_code"'

# --- (N) a PRE-EXISTING wedge spends the budget and is NAMED as such on exhaustion --------------
# _docket_tree_wedged is the dependency Task 2 consumes by name, and K only asserts the wedged
# wording is ABSENT — so its true branch needs a test of its own or the whole wedged arm is
# decoration. A rebase/merge that predates the attempt is another agent mid-sync: retryable, per
# the spec, but only the exhaustion diagnostic gets to call it wedged.
git -C "$work/.docket" fetch -q origin docket >/dev/null 2>&1
git -C "$work/.docket" rebase FETCH_HEAD >/dev/null 2>&1   # conflicts on purpose; LEFT in progress
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'remote moved a fourth time\n' > "$other/remote-moved-4.txt"
git -C "$other" add remote-moved-4.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent 4" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
assert "N: fixture precondition — the shared tree really is mid-rebase" \
  '[ -d "$docket_gitdir/rebase-merge" ] || [ -d "$docket_gitdir/rebase-apply" ]'
mkcounter "$tmp/wedge.count" "$tmp/count3.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count3.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/n.err"; rc=$?
assert "N: a pre-existing wedge returns non-zero" '[ "$rc" -ne 0 ]'
assert "N: the exhaustion diagnostic names the wedged class" \
  'grep -qi "rebase or merge is in progress" "$tmp/n.err"'
assert "N: a wedge is retryable — it spent the full backoff budget" \
  '[ "$(backoffs "$tmp/wedge.count")" -eq 4 ]'
git -C "$work/.docket" rebase --abort >/dev/null 2>&1 || :

# --- (O) HEAD's STATE gates SUCCESS, not merely the integrate path ------------------------------
# The regression pinned here: probing the wedge only AFTER the fast path returned made preflight
# green-light the exact state it exists to survive. A conflicted rebase leaves HEAD DETACHED at the
# rebase's `onto` commit, so with the remote standing still `rev-parse HEAD` == `rev-parse
# FETCH_HEAD`, the ancestor test was trivially true, and the sync returned 0 without ever consulting
# _docket_tree_wedged. An agent's correctly scoped commit then lands on that detached HEAD — the
# branch ref never moves — and the next `rebase --abort` destroys it. Preflight is the only
# mechanical gate the agent channel has, so a green verdict there is the whole ballgame.
#
# Section N cannot catch this: its fixture MOVES the remote, which is precisely what carries it past
# the fast path. The distinguishing fixture is a wedge with the remote STANDING STILL.
#
# The vacuous-fixture trap this surface has already sprung twice: in a linked worktree <dir>/.git is
# a POINTER FILE, not a directory, so `mkdir -p "$wt/.git/rebase-merge"` silently plants nothing and
# every assert resting on it is permanently green. These fixtures build a REAL interrupted rebase
# and resolve the real git dir through git — and assert both, plus the ancestor relation that makes
# the old fast path fire, so a fixture that stops reproducing the bug fails loudly instead of
# passing quietly.
o_gitdir="$(git -C "$work/.docket" rev-parse --absolute-git-dir 2>/dev/null)"
git -C "$work/.docket" fetch -q origin docket >/dev/null 2>&1
git -C "$work/.docket" rebase FETCH_HEAD >/dev/null 2>&1   # conflicts on purpose; LEFT in progress
o_head="$(git -C "$work/.docket" rev-parse HEAD 2>/dev/null)"
o_fetch="$(git -C "$work/.docket" rev-parse FETCH_HEAD 2>/dev/null)"
assert "O: fixture precondition — the real git dir resolved (guards against a vacuous assert)" \
  '[ -n "$o_gitdir" ] && [ -d "$o_gitdir" ]'
assert "O: fixture precondition — a REAL rebase is in progress, not a planted marker directory" \
  '[ -d "$o_gitdir/rebase-merge" ] || [ -d "$o_gitdir/rebase-apply" ]'
assert "O: fixture precondition — HEAD is detached at the rebase's onto commit, EQUAL to FETCH_HEAD" \
  '[ -n "$o_head" ] && [ "$o_head" = "$o_fetch" ] && ! git -C "$work/.docket" symbolic-ref -q HEAD >/dev/null'
assert "O: fixture precondition — the remote is an ancestor of HEAD, so the fast path is reachable" \
  'git -C "$work/.docket" merge-base --is-ancestor "$o_fetch" "$o_head"'

# (O1) wedged + remote UNMOVED: retryable, and NAMED as wedged on exhaustion.
mkcounter "$tmp/o1.count" "$tmp/count4.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count4.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/o1.err"; rc=$?
assert "O1: a wedged tree with the remote UNMOVED must NOT report success" '[ "$rc" -ne 0 ]'
assert "O1: the exhaustion diagnostic names the wedged class" \
  'grep -qi "rebase or merge is in progress" "$tmp/o1.err"'
assert "O1: a wedge is retryable whether or not the remote moved — full backoff budget spent" \
  '[ "$(backoffs "$tmp/o1.count")" -eq 4 ]'
assert "O1: the rebase was left for its owner, never aborted out from under them" \
  '[ -d "$o_gitdir/rebase-merge" ] || [ -d "$o_gitdir/rebase-apply" ]'
git -C "$work/.docket" rebase --abort >/dev/null 2>&1 || :

# (O2) a plain detached HEAD — no operation in progress — with the remote UNMOVED. Same lost-commit
# shape without the rebase: a commit lands on no branch. Terminal, not retryable: nothing but a
# human clears a `git checkout <sha>`, and this file spends budget only on classes that self-heal.
git -C "$work/.docket" checkout -q --detach "$o_fetch" >/dev/null 2>&1
o2_head="$(git -C "$work/.docket" rev-parse HEAD 2>/dev/null)"
assert "O2: fixture precondition — HEAD is detached with NO git operation in progress" \
  '! git -C "$work/.docket" symbolic-ref -q HEAD >/dev/null && [ ! -d "$o_gitdir/rebase-merge" ] && [ ! -d "$o_gitdir/rebase-apply" ] && [ ! -f "$o_gitdir/MERGE_HEAD" ]'
assert "O2: fixture precondition — the remote is an ancestor of HEAD, so the fast path is reachable" \
  '[ -n "$o2_head" ] && git -C "$work/.docket" merge-base --is-ancestor "$o_fetch" "$o2_head"'
mkcounter "$tmp/o2.count" "$tmp/count5.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count5.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/o2.err"; rc=$?
assert "O2: a detached HEAD must NOT report success" '[ "$rc" -ne 0 ]'
assert "O2: the diagnostic names the detached class" 'grep -qi "detached" "$tmp/o2.err"'
assert "O2: detached is not misreported as the wedged class" \
  '! grep -qi "rebase or merge is in progress" "$tmp/o2.err"'
assert "O2: nothing self-heals a detached HEAD — no retry budget spent" \
  '[ "$(backoffs "$tmp/o2.count")" -eq 0 ]'

# (O3) the same detached HEAD once the remote MOVES. Refusing on the fast path is not enough: the
# integrate arm below it would otherwise rebase the detached HEAD and report success for a sync
# that moved no branch at all.
git -C "$other" pull -q --rebase origin docket >/dev/null 2>&1
printf 'remote moved a fifth time\n' > "$other/remote-moved-5.txt"
git -C "$other" add remote-moved-5.txt >/dev/null 2>&1
git -C "$other" commit -q -m "other agent 5" >/dev/null 2>&1
git -C "$other" push -q origin HEAD:docket >/dev/null 2>&1
mkcounter "$tmp/o3.count" "$tmp/count6.sh"
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/count6.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/o3.err"; rc=$?
assert "O3: a detached HEAD with the remote MOVED still refuses" '[ "$rc" -ne 0 ]'
assert "O3: the detached HEAD was never rebased — it did not move" \
  '[ "$(git -C "$work/.docket" rev-parse HEAD 2>/dev/null)" = "$o2_head" ]'
assert "O3: the remote commit was NOT integrated onto the detached HEAD" \
  '[ ! -f "$work/.docket/remote-moved-5.txt" ]'
assert "O3: it spent no retry budget" '[ "$(backoffs "$tmp/o3.count")" -eq 0 ]'
git -C "$work/.docket" checkout -q docket >/dev/null 2>&1 || :

# --- (P) the sync NEVER rewrites a branch that is not the one it was asked to sync --------------
# The regression pinned here (review finding): main-mode stopped being upstream-relative. The old
# line was `git -C "$root" pull --rebase` — whatever branch the PRIMARY tree had checked out, synced
# against its OWN upstream — and it became `fetch origin <integration_branch>` plus a rebase onto
# that tip. Main-mode is exactly the configuration chosen by people who do not want extra worktrees,
# so a topic branch checked out in the primary tree is an ORDINARY state, and preflight is Step 0 of
# every docket skill. Measured against the pre-fix behaviour on this very fixture: rc=0, the topic
# branch's HEAD moved, origin/main's commit appeared in the tree, and the branch ended diverged from
# its own remote — a silent rewrite of the user's history, reported as success.
#
# P1-P3 and P7 EXTEND section G's main-mode fixture ($work3, a clone of $bare on `main`) rather than
# minting a parallel one; P4-P5 need an integration branch that is not named `main`, which no
# existing fixture has. In main-mode $dir is the PRIMARY worktree, where .git IS a directory — the
# opposite of the linked-worktree shape that made sections L/N/O's probes go vacuous twice. The
# asserts below therefore key on branch tips and file presence, which mean the same thing in both,
# and each fixture asserts its own precondition so a fixture that stops reproducing the bug fails
# loudly instead of passing quietly.

# mover — a throwaway clone standing in for whoever advances origin/main. $other is on the docket
# branch and section B's $work is the docket-mode fixture, so neither can play this part.
mover="$tmp/mover"; git clone --quiet "$bare" "$mover" 2>/dev/null
git -C "$mover" config user.email t@t.test; git -C "$mover" config user.name Test
git -C "$mover" checkout --quiet -B main origin/main
printf 'integration moved\n' > "$mover/integration-moved.txt"
git -C "$mover" add integration-moved.txt; git -C "$mover" commit --quiet -m "main moved"
git -C "$mover" push --quiet origin HEAD:main

# (P1) main-mode, primary tree on a TOPIC branch, origin/<integration> has MOVED.
git -C "$work3" checkout --quiet -b topic
printf 'my work\n' > "$work3/topic.txt"
git -C "$work3" add topic.txt; git -C "$work3" commit --quiet -m "the human's own commit"
git -C "$work3" push --quiet -u origin topic
# Refresh origin/main so the preconditions read the real remote tip: this clone predates the mover's
# push, and a stale remote-tracking ref would make "the remote really moved" false in the wrong
# direction — a precondition that lies green.
git -C "$work3" fetch --quiet origin main
p_head="$(git -C "$work3" rev-parse HEAD)"
assert "P1: fixture precondition — the primary tree is on a topic branch, level with its OWN upstream" \
  '[ "$(git -C "$work3" symbolic-ref --short -q HEAD)" = topic ] && [ "$(git -C "$work3" rev-list --count "origin/topic..HEAD")" -eq 0 ]'
assert "P1: fixture precondition — origin/main really has moved, so the rebase arm is reachable" \
  '! git -C "$work3" merge-base --is-ancestor "$(git -C "$work3" rev-parse origin/main)" HEAD'
mkcounter "$tmp/p1.count" "$tmp/p1-sleep.sh"
( cd "$work3" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p1-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p1.err"; rc=$?
assert "P1: main-mode on a branch that is not the integration branch must NOT report success" '[ "$rc" -ne 0 ]'
assert "P1: the human's topic branch was NOT rewritten — its tip is untouched" \
  '[ "$(git -C "$work3" rev-parse HEAD)" = "$p_head" ]'
assert "P1: the integration branch's commit was NOT grafted onto the topic branch" \
  '[ ! -f "$work3/integration-moved.txt" ]'
assert "P1: the topic branch did not diverge from its own upstream" \
  '[ "$(git -C "$work3" rev-list --count "origin/topic..HEAD")" -eq 0 ]'
assert "P1: the diagnostic names both the branch found and the branch expected" \
  'grep -q "on branch .topic., not .main." "$tmp/p1.err"'
assert "P1: a wrong branch is a human's state — no retry budget spent" \
  '[ "$(backoffs "$tmp/p1.count")" -eq 0 ]'
assert "P1: it is not misreported as the detached or the wedged class" \
  '! grep -qi "is DETACHED" "$tmp/p1.err" && ! grep -qi "rebase or merge is in progress" "$tmp/p1.err"'

# (P2) the same wrong branch with the integration branch STANDING STILL. Refusing only the rebase is
# not enough: the fast path sits above it and would return 0 here, and the caller's next act is a
# metadata commit onto that topic branch. A real merge of origin/main INTO the topic branch makes
# origin/main an ancestor of HEAD without moving the topic branch onto it — precisely the
# "nothing to integrate" shape the fast path fires on. Non-zero rc plus zero backoffs is what
# separates "refused ahead of the fast path" from "the rebase happened to have nothing to do".
git -C "$work3" merge --quiet --no-edit origin/main >/dev/null 2>&1
p2_head="$(git -C "$work3" rev-parse HEAD)"
assert "P2: fixture precondition — still on the topic branch, and origin/main is now an ANCESTOR of HEAD (the fast path is reachable)" \
  '[ "$(git -C "$work3" symbolic-ref --short -q HEAD)" = topic ] && git -C "$work3" merge-base --is-ancestor "$(git -C "$work3" rev-parse origin/main)" HEAD'
mkcounter "$tmp/p2.count" "$tmp/p2-sleep.sh"
( cd "$work3" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p2-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p2.err"; rc=$?
assert "P2: the fast path does not green-light a tree on the wrong branch" '[ "$rc" -ne 0 ]'
assert "P2: HEAD is untouched" '[ "$(git -C "$work3" rev-parse HEAD)" = "$p2_head" ]'
assert "P2: the diagnostic still names the wrong-branch class" \
  'grep -q "on branch .topic., not .main." "$tmp/p2.err"'
assert "P2: no retry budget spent" '[ "$(backoffs "$tmp/p2.count")" -eq 0 ]'

# (P3) the SAME primary tree back on the integration branch still syncs — the guard refuses the
# wrong branch, it does not break main-mode. Without this, P1/P2 would pass against a "fix" that
# simply broke main-mode outright. Section G asserted main-mode returns zero against a remote that
# had never moved; this one has to integrate real remote movement.
git -C "$work3" checkout --quiet main
mkcounter "$tmp/p3.count" "$tmp/p3-sleep.sh"
( cd "$work3" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p3-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p3.err"; rc=$?
assert "P3: main-mode ON the integration branch still returns zero" '[ "$rc" -eq 0 ]'
assert "P3: and it actually integrated origin/main" '[ -f "$work3/integration-moved.txt" ]'

# (P4) an UNSET INTEGRATION_BRANCH fails closed instead of falling back to METADATA_BRANCH. That
# fallback resolves to the mode KEYWORD "main" (docket-config.sh accepts only 'docket'/'main' for
# that key), which is not a branch name — the very wrong-ref case the adjacent comment claims this
# change fixed. This fixture's integration branch is 'trunk', so the fallback fetches a ref that does
# not exist. Asserting the refusal is PRE-FETCH (git never printed its own failure, zero backoffs) is
# what distinguishes failing closed from failing only after burning the whole ~22s budget.
bare_t="$tmp/t.git"; workt="$tmp/t"
git init --quiet --bare "$bare_t"
git clone --quiet "$bare_t" "$workt" 2>/dev/null
git -C "$workt" config user.email t@t.test; git -C "$workt" config user.name Test
git -C "$workt" checkout --quiet -b trunk; : > "$workt/README.md"
git -C "$workt" add README.md; git -C "$workt" commit --quiet -m init
git -C "$workt" push --quiet -u origin trunk
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=main\nMETADATA_BRANCH=main\nMETADATA_WORKTREE=.\nCHANGES_DIR=docs/changes\n' > "$tmp/noib.env"
mkexport "$tmp/noib.env" "$tmp/noib-export.sh"
mkcounter "$tmp/p4.count" "$tmp/p4-sleep.sh"
( cd "$workt" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/noib-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p4-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p4.err"; rc=$?
assert "P4: an unset INTEGRATION_BRANCH fails closed in main-mode" '[ "$rc" -ne 0 ]'
assert "P4: the diagnostic names INTEGRATION_BRANCH, not the network" \
  'grep -q "INTEGRATION_BRANCH" "$tmp/p4.err"'
assert "P4: it never fell back to the mode keyword and fetched a nonexistent 'main'" \
  '! grep -q "couldn.t find remote ref main" "$tmp/p4.err"'
assert "P4: it refused before spending any retry budget" '[ "$(backoffs "$tmp/p4.count")" -eq 0 ]'

# (P5) a repo whose integration branch is NOT literally "main" fetches and integrates the RIGHT ref.
# P4 proves the fallback is gone; this proves the replacement resolves a non-"main" branch correctly
# rather than merely refusing everything.
othert="$tmp/t-other"; git clone --quiet "$bare_t" "$othert" 2>/dev/null
git -C "$othert" config user.email t@t.test; git -C "$othert" config user.name Test
# $bare_t's own HEAD names a branch that was never pushed (only `trunk` was), so the clone lands on
# an UNBORN branch. Without this checkout the "other agent" would build a disconnected root commit
# and its push would be rejected as non-fast-forward — leaving P5 asserting against a remote that
# never moved.
git -C "$othert" checkout --quiet -B trunk origin/trunk
printf 'trunk moved\n' > "$othert/trunk-moved.txt"
git -C "$othert" add trunk-moved.txt; git -C "$othert" commit --quiet -m "trunk moved"
git -C "$othert" push --quiet origin HEAD:trunk
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=main\nMETADATA_BRANCH=main\nMETADATA_WORKTREE=.\nINTEGRATION_BRANCH=trunk\nCHANGES_DIR=docs/changes\n' > "$tmp/trunk.env"
mkexport "$tmp/trunk.env" "$tmp/trunk-export.sh"
mkcounter "$tmp/p5.count" "$tmp/p5-sleep.sh"
( cd "$workt" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/trunk-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p5-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p5.err"; rc=$?
assert "P5: an integration branch named 'trunk' syncs successfully" '[ "$rc" -eq 0 ]'
assert "P5: it fetched and integrated origin/trunk, not a ref named after the mode keyword" \
  '[ -f "$workt/trunk-moved.txt" ]'
assert "P5: no wrong-ref fetch failure was printed" \
  '! grep -q "couldn.t find remote ref" "$tmp/p5.err"'

# (P6) the guard is UNIFORM across both arms of the sync — docket-mode gets it too. A shared helper
# that polices its target in one caller and not the other is how the two arms drift apart, which is
# the spec's own "both branches of the sync function must behave identically" clause (Half 1 item 1).
#
# Runs against $work2's metadata worktree, NOT $work's: sections L-O left conflicting commits on the
# latter, so a stray branch there would be refused by the CONFLICT arm and every assert below would
# pass without the wrong-branch arm existing at all (confirmed by mutation — the vacuity is not
# hypothetical). $work2/.docket has never been written to, so a wrong-branch rebase would SUCCEED
# there, which is what makes refusing it observable.
git -C "$work2/.docket" checkout -q -b stray-branch >/dev/null 2>&1
stray_head="$(git -C "$work2/.docket" rev-parse HEAD)"
assert "P6: fixture precondition — the metadata worktree is on a branch that is not METADATA_BRANCH" \
  '[ "$(git -C "$work2/.docket" symbolic-ref --short -q HEAD)" = stray-branch ]'
assert "P6: fixture precondition — origin/docket has moved past it, so the rebase arm is reachable" \
  'git -C "$work2/.docket" fetch -q origin docket && ! git -C "$work2/.docket" merge-base --is-ancestor "$(git -C "$work2/.docket" rev-parse origin/docket)" HEAD'
mkcounter "$tmp/p6.count" "$tmp/p6-sleep.sh"
( cd "$work2" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p6-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p6.err"; rc=$?
assert "P6: docket-mode refuses a metadata worktree parked on the wrong branch" '[ "$rc" -ne 0 ]'
assert "P6: that branch was not rewritten either" \
  '[ "$(git -C "$work2/.docket" rev-parse HEAD)" = "$stray_head" ]'
assert "P6: the diagnostic names both branches" \
  'grep -q "on branch .stray-branch., not .docket." "$tmp/p6.err"'
assert "P6: no retry budget spent" '[ "$(backoffs "$tmp/p6.count")" -eq 0 ]'
git -C "$work2/.docket" checkout -q docket >/dev/null 2>&1 || :
git -C "$work2/.docket" branch -q -D stray-branch >/dev/null 2>&1 || :

# (P7) the remote tip comes from the REMOTE-TRACKING REF, not FETCH_HEAD — for both arms, since the
# resolution lives in the shared helper. FETCH_HEAD is per-worktree but not per-PROCESS, and in
# main-mode $dir is the PRIMARY worktree, shared with every other docket helper that fetches there
# (sync-integration-branch.sh, terminal-publish.sh). One of those landing between the sync's fetch
# and its read of the tip hands the rebase a DIFFERENT branch's tip.
#
# The fixture makes that interleaving deterministic through the GIT seam rather than hoping to lose a
# race: a wrapper that forwards every call to real git and, after any `fetch`, fetches the METADATA
# branch into the same tree — which is exactly what those helpers do. Against a FETCH_HEAD read the
# sync then rebases the integration branch onto the metadata branch's tip and reports success.
git -C "$mover" pull -q --rebase origin main >/dev/null 2>&1
printf 'integration moved again\n' > "$mover/integration-moved-2.txt"
git -C "$mover" add integration-moved-2.txt; git -C "$mover" commit --quiet -m "main moved again"
git -C "$mover" push --quiet origin HEAD:main
{ printf '#!/usr/bin/env bash\n'
  printf 'real=%q\n' "$(command -v git)"
  printf '"$real" "$@"; rc=$?\n'
  printf 'for a in "$@"; do\n'
  printf '  [ "$a" = fetch ] || continue\n'
  printf '  "$real" -C %q fetch origin docket >/dev/null 2>&1\n' "$work3"
  printf '  break\n'
  printf 'done\n'
  printf 'exit "$rc"\n'
} > "$tmp/p7-git.sh"
chmod +x "$tmp/p7-git.sh"
mkcounter "$tmp/p7.count" "$tmp/p7-sleep.sh"
( cd "$work3" && . "$LIB" && GIT="$tmp/p7-git.sh" CONFIG_EXPORT_CMD="bash $tmp/main-export.sh" \
    DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p7-sleep.sh" docket_preflight "$SCRIPTS" ) \
    >/dev/null 2>"$tmp/p7.err"; rc=$?
p7_other="$(git -C "$work3" rev-parse origin/docket 2>/dev/null)"
assert "P7: fixture precondition — the interloping fetch really did clobber FETCH_HEAD (guards a vacuous assert)" \
  '[ -n "$p7_other" ] && [ "$(git -C "$work3" rev-parse FETCH_HEAD 2>/dev/null)" = "$p7_other" ]'
assert "P7: fixture precondition — that tip is NOT the integration branch tip" \
  '[ "$p7_other" != "$(git -C "$work3" rev-parse origin/main 2>/dev/null)" ]'
assert "P7: the sync still succeeds under a concurrent fetch of another ref" '[ "$rc" -eq 0 ]'
assert "P7: it integrated origin/main — the ref it was asked to sync" \
  '[ -f "$work3/integration-moved-2.txt" ]'
assert "P7: it did NOT rebase onto the concurrently-fetched branch's tip" \
  '[ ! -f "$work3/tracked.txt" ]'
assert "P7: the integration branch ended exactly at origin/main" \
  '[ "$(git -C "$work3" rev-parse HEAD)" = "$(git -C "$work3" rev-parse origin/main)" ]'

# (P8) an EMPTY branch argument is a precondition failure, not something to classify downstream.
# Driven at the helper directly because no caller can produce one any more (P4 closed the only one
# that could) — and a precondition with no reachable caller is exactly the kind that rots into
# decoration. `git fetch origin ""` does NOT fail: measured on git 2.55 against a real remote it
# fetches that remote's HEAD and returns 0, so without this guard the empty branch falls all the way
# through to the wrong-branch arm and the human is told to "check out ''" — a diagnostic blaming
# their checkout for the caller's bug. The WORDING assert is the load-bearing one here; the non-zero
# return and the zero backoffs both survive the guard's removal (mutation-measured, not assumed).
mkcounter "$tmp/p8.count" "$tmp/p8-sleep.sh"
( . "$LIB" && DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/p8-sleep.sh" \
    _docket_sync_metadata git "$work3" origin "" ) >/dev/null 2>"$tmp/p8.err"; rc=$?
assert "P8: an empty branch argument fails closed" '[ "$rc" -ne 0 ]'
assert "P8: it refused before spending any retry budget" '[ "$(backoffs "$tmp/p8.count")" -eq 0 ]'
assert "P8: the diagnostic names the missing branch, not a fetch failure" \
  'grep -q "no branch was named" "$tmp/p8.err" && ! grep -q "after 5 attempts" "$tmp/p8.err"'

# --- (Q) an UNCONFIGURED remote fails fast; a merely unreachable one still retries ---------------
# The fetch arm retries undiscriminated on purpose, and that stays true — but "there is no remote
# named origin" is not a transient network failure, and paying the full ~22s backoff for it on
# EVERY skill's Step 0 (and every CAS re-sync) is latency bought with nothing. `remote get-url`
# answers from local config, deterministically, so this is the one sub-class that can be carved out
# without the locale- and version-fragile stderr matching the spec declines.
#
# Q2 is the other half of the same rule and is what keeps Q1 from being read as "classify fetch
# failures": a remote that IS configured but points nowhere is indistinguishable from a dropped
# packet, so it must still spend the whole budget. Without Q2, widening the guard into a general
# fetch-failure fast-fail would leave every assert green.
noremote="$tmp/noremote"
git init --quiet "$noremote"
git -C "$noremote" config user.email t@t.test; git -C "$noremote" config user.name Test
git -C "$noremote" checkout --quiet -b docket
: > "$noremote/README.md"
git -C "$noremote" add README.md; git -C "$noremote" commit --quiet -m init
assert "Q: fixture precondition — the tree really has no remote configured" \
  '[ -z "$(git -C "$noremote" remote)" ]'
assert "Q: fixture precondition — it is clean, on the branch being synced, and not wedged" \
  '[ "$(git -C "$noremote" symbolic-ref --short -q HEAD)" = docket ] \
   && [ -z "$(git -C "$noremote" status --porcelain --untracked-files=no)" ] \
   && [ ! -d "$(git -C "$noremote" rev-parse --absolute-git-dir)/rebase-merge" ]'
mkcounter "$tmp/q1.count" "$tmp/q1-sleep.sh"
( . "$LIB" && DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/q1-sleep.sh" \
    _docket_sync_metadata git "$noremote" origin docket ) >/dev/null 2>"$tmp/q1.err"; rc=$?
assert "Q1: a missing remote fails closed" '[ "$rc" -ne 0 ]'
assert "Q1: it spent ZERO backoff — an absent remote is not transient" \
  '[ "$(backoffs "$tmp/q1.count")" -eq 0 ]'
assert "Q1: the diagnostic names the missing remote, not an exhausted fetch" \
  'grep -q "no remote named .origin. is configured" "$tmp/q1.err" && ! grep -q "after 5 attempts" "$tmp/q1.err"'

# Q2: configured but unreachable — the undiscriminated fetch retry the spec specifies, unchanged.
git -C "$noremote" remote add origin "$tmp/does-not-exist.git"
mkcounter "$tmp/q2.count" "$tmp/q2-sleep.sh"
( . "$LIB" && DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/q2-sleep.sh" \
    _docket_sync_metadata git "$noremote" origin docket ) >/dev/null 2>"$tmp/q2.err"; rc=$?
assert "Q2: an unreachable-but-configured remote still fails closed" '[ "$rc" -ne 0 ]'
assert "Q2: it still spends the full retry budget — fetch stays undiscriminated" \
  '[ "$(backoffs "$tmp/q2.count")" -eq 4 ]'
assert "Q2: and it exhausts on the fetch class, never the missing-remote one" \
  'grep -q "the last failure was fetching" "$tmp/q2.err" && ! grep -q "no remote named" "$tmp/q2.err"'

# Q3: the ordered arms below the guard are undisturbed — the wrong-branch refusal still answers for
# a tree on another branch, rather than the new check short-circuiting ahead of it on a shape it was
# never meant to own. The remote has to be REACHABLE here, not merely configured: every arm below
# the fetch lives in its `else`, so an unreachable URL would exhaust on the fetch class and this
# would pass for the wrong reason.
git init --quiet --bare "$tmp/q.git"
git -C "$noremote" remote set-url origin "$tmp/q.git"
git -C "$noremote" push --quiet origin docket
git -C "$noremote" checkout --quiet -b stray-branch
mkcounter "$tmp/q3.count" "$tmp/q3-sleep.sh"
( . "$LIB" && DOCKET_PREFLIGHT_TEST_SLEEP_CMD="bash $tmp/q3-sleep.sh" \
    _docket_sync_metadata git "$noremote" origin docket ) >/dev/null 2>"$tmp/q3.err"; rc=$?
assert "Q3: a wrong-branch tree still fails closed" '[ "$rc" -ne 0 ]'
assert "Q3: ... naming the branch mismatch, not the remote" \
  'grep -q "on branch .stray-branch., not .docket." "$tmp/q3.err"'
git -C "$noremote" checkout --quiet docket

exit $fail

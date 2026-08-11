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
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
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
# `git -C "$root" pull --rebase` needs a real upstream to pull against.
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

exit $fail

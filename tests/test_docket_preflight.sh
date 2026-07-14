#!/usr/bin/env bash
# tests/test_docket_preflight.sh — hermetic tests for scripts/lib/docket-preflight.sh (change 0068).
# Sources the lib and drives docket_preflight against stubbed config exports + temp repos. No network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-preflight.sh"
SCRIPTS="$REPO/scripts"
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
  '! git -C "$work" worktree list --porcelain | grep -q "^worktree .*/\.docket/\.docket$"'

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

exit $fail

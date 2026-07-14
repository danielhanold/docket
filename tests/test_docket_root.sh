#!/usr/bin/env bash
# tests/test_docket_root.sh — hermetic tests for scripts/lib/docket-root.sh (change 0075).
# The main-worktree anchor: every docket script must resolve the SAME primary root no matter which
# worktree (or subdirectory) the caller stands in. No network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-root.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; tmp="$(cd "$tmp" && pwd -P)"; trap 'rm -rf "$tmp"' EXIT

# shellcheck source=/dev/null
. "$LIB"

# --- fixture: a repo with BOTH docket worktree shapes -------------------------
# work/            <- the main worktree
# work/.docket/    <- a linked worktree (the metadata worktree)
# work/.worktrees/feat-x/  <- a linked worktree (a feature worktree)
# work/sub/        <- a plain subdirectory of the main worktree
work="$tmp/work"
git init --quiet "$work"
git -C "$work" config user.email t@t.test
git -C "$work" config user.name  Test
: > "$work/README.md"
git -C "$work" add README.md
git -C "$work" commit --quiet -m init
git -C "$work" branch --quiet docket
git -C "$work" branch --quiet feat/x
git -C "$work" worktree add --quiet "$work/.docket" docket >/dev/null 2>&1
git -C "$work" worktree add --quiet "$work/.worktrees/feat-x" feat/x >/dev/null 2>&1
mkdir -p "$work/sub"

# --- (A) docket_main_worktree: the SAME root from all four CWDs ---------------
assert "main worktree from the main root" \
  '[ "$( cd "$work" && docket_main_worktree )" = "$work" ]'
assert "main worktree from the .docket/ metadata worktree (NOT .docket itself)" \
  '[ "$( cd "$work/.docket" && docket_main_worktree )" = "$work" ]'
assert "main worktree from a .worktrees/<slug> feature worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && docket_main_worktree )" = "$work" ]'
assert "main worktree from a plain subdirectory" \
  '[ "$( cd "$work/sub" && docket_main_worktree )" = "$work" ]'

# The contrast that names the bug: --show-toplevel returns the LINKED worktree.
assert "CONTRAST: git rev-parse --show-toplevel returns the linked worktree, which is the defect" \
  '[ "$( cd "$work/.docket" && git rev-parse --show-toplevel )" != "$work" ]'

# --- (B) not a git repo => empty, never an error ------------------------------
outside="$tmp/outside"; mkdir -p "$outside"
assert "outside a git repo: empty output" \
  '[ -z "$( cd "$outside" && docket_main_worktree )" ]'
assert "outside a git repo: exit 0 (soft, never fatal)" \
  '( cd "$outside" && docket_main_worktree >/dev/null )'

# --- (C) explicit dir argument ------------------------------------------------
assert "explicit dir argument resolves that repo's main worktree" \
  '[ "$( cd "$outside" && docket_main_worktree "$work/.docket" )" = "$work" ]'

# --- (D) docket_anchor_path ---------------------------------------------------
assert "anchor: relative path joins the main worktree, from a linked worktree" \
  '[ "$( cd "$work/.docket" && docket_anchor_path .docket )" = "$work/.docket" ]'
assert "anchor: nested relative path joins the main worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && docket_anchor_path .worktrees/feat-x )" = "$work/.worktrees/feat-x" ]'
assert "anchor: '.' resolves to the main worktree itself (main-mode shape)" \
  '[ "$( cd "$work/sub" && docket_anchor_path . )" = "$work" ]'
assert "anchor: empty string resolves to the main worktree from a linked worktree" \
  '[ "$( cd "$work/.docket" && docket_anchor_path "" )" = "$work" ]'
assert "anchor: './x' does not produce a doubled slash-dot" \
  '[ "$( cd "$work/sub" && docket_anchor_path ./docs )" = "$work/docs" ]'
assert "anchor: an ABSOLUTE path passes through untouched" \
  '[ "$( cd "$work/.docket" && docket_anchor_path /somewhere/else )" = "/somewhere/else" ]'
assert "anchor: outside a repo, the path passes through unchanged (soft fallback)" \
  '[ "$( cd "$outside" && docket_anchor_path .docket )" = ".docket" ]'

# --- (E) docket_metadata_worktree, from the config vars in scope --------------
assert "metadata worktree: docket-mode => <root>/.docket, resolved from a linked worktree" \
  '[ "$( cd "$work/.worktrees/feat-x" && DOCKET_MODE=docket METADATA_WORKTREE=.docket docket_metadata_worktree )" = "$work/.docket" ]'
assert "metadata worktree: main-mode ('.') => the repo root itself, never its parent" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=main METADATA_WORKTREE=. docket_metadata_worktree )" = "$work" ]'
assert "metadata worktree: docket-mode default when METADATA_WORKTREE is unset" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=docket docket_metadata_worktree )" = "$work/.docket" ]'
assert "metadata worktree: an already-absolute METADATA_WORKTREE is not re-anchored" \
  '[ "$( cd "$work/sub" && DOCKET_MODE=docket METADATA_WORKTREE=/abs/mw docket_metadata_worktree )" = "/abs/mw" ]'

# --- (F) the GIT mock seam is honored -----------------------------------------
assert "honors the GIT seam (a git that prints nothing => empty resolution)" \
  '[ -z "$( cd "$work" && GIT=true docket_main_worktree )" ]'

exit $fail

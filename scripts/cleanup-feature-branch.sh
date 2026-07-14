#!/usr/bin/env bash
# scripts/cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature
# branch + worktree (change 0025). Removes the worktree ONLY if it resolves under the repo root's
# .worktrees/ (never the .docket/ metadata worktree, never an out-of-tree path), then deletes the
# local and remote feat/<slug> branch. Fail-closed: self-verifies both are gone.
#
# The repo root is the MAIN worktree and the target is ABSOLUTE (change 0075), so the script means
# the same thing from every CWD; and it REFUSES, before any destructive step, when the caller's CWD
# is at or inside the target worktree.
#
# Usage: cleanup-feature-branch.sh --slug S [--worktrees-dir DIR] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
SLUG="" WORKTREES_DIR=".worktrees" REMOTE="origin"

die(){ printf '%s\n' "cleanup-feature-branch: $*" >&2; exit 1; }
log(){ printf '%s\n' "cleanup-feature-branch: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --slug) SLUG="$2"; shift ;;
    --worktrees-dir) WORKTREES_DIR="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$SLUG" ] || die "missing --slug"

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR"/lib/docket-root.sh

# Capture the caller's CWD BEFORE anything cd's (change 0075). The guard below compares it against
# the target; a `cd` first would compare $root against itself and the guard could never fire.
caller_pwd="$(pwd -P)"

# The repo root is the MAIN worktree, never `git rev-parse --show-toplevel` — from the .docket/
# metadata worktree or a .worktrees/<slug> feature worktree that returns the LINKED worktree, which
# made `target` (below) resolve to nothing: the worktree removal was skipped, `git branch -D` fell
# into `|| true`, and execution still reached `git push --delete`, which SUCCEEDED. Partial,
# irreversible destruction that reported failure (defect D1).
root="$(docket_main_worktree)"
[ -n "$root" ] || die "not in a git repo"

canon(){ ( cd "$1" 2>/dev/null && pwd -P ); }   # realpath of an existing dir, else empty

# target is ABSOLUTE, anchored to the main worktree — so the removal block, the guards, and the
# postcondition below all mean the same thing from every CWD. An absolute --worktrees-dir is
# honored verbatim (the provenance guard still governs whether it may be removed).
case "$WORKTREES_DIR" in
  /*) target="$WORKTREES_DIR/$SLUG" ;;
  *)  target="$root/$WORKTREES_DIR/$SLUG" ;;
esac
allowed_root="$root/.worktrees"

# FAIL-CLOSED CWD GUARD (change 0075, defects D1+D3) — refuse when the caller stands AT or INSIDE
# the target. Placed before BOTH destructive steps (the worktree removal AND the remote delete):
# `git worktree remove --force` succeeds with a process CWD inside the target (the process merely
# orphans its CWD) and the caller's NEXT command then cannot start, so the only safe answer is to
# do nothing at all. Refusing takes away nothing that worked: from this CWD the pre-0075 script
# destroyed the remote branch and failed anyway.
target_rp="$(canon "$target")"
if [ -n "$target_rp" ]; then
  case "$caller_pwd/" in
    "$target_rp/"*)
      die "refusing to clean up feat/$SLUG: the caller's CWD is at or inside the target worktree ($caller_pwd) — cd to the repo root ($root) and re-run" ;;
  esac
fi

# provenance guard: the worktree, if present, must resolve under <root>/.worktrees/
if [ -e "$target" ]; then
  rp="$(canon "$target")"
  case "$rp/" in
    "$allowed_root/"*) ;;   # under .worktrees/ — allowed
    *) die "refusing to remove worktree outside .worktrees/: $rp" ;;
  esac
  $GIT -C "$root" worktree remove --force "$target" >/dev/null 2>&1 || die "worktree remove failed: $target"
fi

# delete local + remote feat/<slug> — anchored at the main worktree, never the caller's CWD
$GIT -C "$root" branch -D "feat/$SLUG" >/dev/null 2>&1 || true

# FAIL-CLOSED REMOTE-DELETE GUARD (review finding 2, change 0075) — never destroy the REMOTE
# branch while the LOCAL branch still exists. If `git branch -D` above didn't actually remove it
# (typically: still checked out in another worktree — e.g. a hand-passed --worktrees-dir that
# doesn't match where the branch is really checked out), refuse BEFORE the remote delete: the
# remote stays intact and the operator can re-run once the local branch is actually gone. This is
# a separate, later ordering guard — it does not broaden the .worktrees/ provenance guard above.
if $GIT -C "$root" rev-parse --verify -q "feat/$SLUG" >/dev/null 2>&1; then
  die "refusing to delete remote feat/$SLUG: the local branch still exists (likely still checked out in another worktree) — resolve that first and re-run"
fi

if $GIT -C "$root" ls-remote --exit-code "$REMOTE" "feat/$SLUG" >/dev/null 2>&1; then
  $GIT -C "$root" push "$REMOTE" --delete "feat/$SLUG" >/dev/null 2>&1 || die "remote branch delete failed"
fi

# fail-closed self-verification
[ ! -e "$target" ] || die "postcondition: worktree still present"
$GIT -C "$root" rev-parse --verify -q "feat/$SLUG" >/dev/null && die "postcondition: local branch still present"
log "cleaned up feat/$SLUG"
exit 0

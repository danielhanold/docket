#!/usr/bin/env bash
# scripts/cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature
# branch + worktree (change 0025). Removes the worktree ONLY if it resolves under the repo root's
# .worktrees/ (never the .docket/ metadata worktree, never an out-of-tree path), then deletes the
# local and remote feat/<slug> branch. Fail-closed: self-verifies both are gone.
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

root="$($GIT rev-parse --show-toplevel)" || die "not in a git repo"
canon(){ ( cd "$1" 2>/dev/null && pwd -P ); }   # realpath of an existing dir, else empty

target="$WORKTREES_DIR/$SLUG"
allowed_root="$(canon "$root")/.worktrees"

# provenance guard: the worktree, if present, must resolve under <root>/.worktrees/
if [ -e "$target" ]; then
  rp="$(canon "$target")"
  case "$rp/" in
    "$allowed_root/"*) ;;   # under .worktrees/ — allowed
    *) die "refusing to remove worktree outside .worktrees/: $rp" ;;
  esac
  $GIT worktree remove --force "$target" >/dev/null 2>&1 || die "worktree remove failed: $target"
fi

# delete local + remote feat/<slug>
$GIT branch -D "feat/$SLUG" >/dev/null 2>&1 || true
if $GIT ls-remote --exit-code "$REMOTE" "feat/$SLUG" >/dev/null 2>&1; then
  $GIT push "$REMOTE" --delete "feat/$SLUG" >/dev/null 2>&1 || die "remote branch delete failed"
fi

# fail-closed self-verification
[ ! -e "$target" ] || die "postcondition: worktree still present"
$GIT rev-parse --verify -q "feat/$SLUG" >/dev/null && die "postcondition: local branch still present"
log "cleaned up feat/$SLUG"
exit 0

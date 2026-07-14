#!/usr/bin/env bash
# scripts/lib/docket-root.sh — the repo-root anchor (change 0075). The ONE implementation of
# "which tree am I operating on", so that no docket script derives its root from the caller's CWD.
#
# git lists the MAIN worktree FIRST in `worktree list --porcelain`, and the list is reachable from
# EVERY worktree in the set — so this resolves the repo's primary checkout even when the caller
# stands in the .docket/ metadata worktree or a .worktrees/<slug> feature worktree.
# `git rev-parse --show-toplevel` (and a bare `cd "$dir" && pwd -P`) instead return the LINKED
# worktree the caller happens to be in, which is the root cause of D1 (cleanup deleted the remote
# branch and then failed) and D2 (preflight minted a nested <repo>/.docket/.docket).
# See docs/superpowers/specs/2026-07-14-cwd-independent-repo-root-anchor-design.md.
#
#   docket_main_worktree [dir]       absolute path of the main worktree of the repo containing
#                                    <dir> (default $PWD); EMPTY when <dir> is not in a git repo.
#   docket_anchor_path <path> [dir]  <path> made absolute against that main worktree. Absolute
#                                    passes through; "." (or empty) becomes the root; a relative
#                                    path is joined to it. Not a repo => <path> unchanged, so the
#                                    caller's own not-a-repo gate still fires as before.
#   docket_metadata_worktree         the metadata worktree, ABSOLUTE, from the DOCKET_MODE /
#                                    METADATA_WORKTREE vars already in the caller's scope.
#
# Mock seam: GIT="${GIT:-git}".
# This file is a sourced helper: it is documented within its callers' contracts (docket-config.md,
# docket.md, docket-status.md, cleanup-feature-branch.md), not by a co-located .md
# (test_script_contracts_coverage.sh scopes lib/ out).

docket_main_worktree(){
  local dir="${1:-$PWD}" git="${GIT:-git}"
  "$git" -C "$dir" worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p'
  return 0   # under caller's `set -o pipefail`, git's non-zero (not-a-repo) must not leak out —
             # this is a SOFT fallback (empty output, exit 0), never a hard error.
}

docket_anchor_path(){
  local path="$1" dir="${2:-$PWD}" root
  case "$path" in
    /*) printf '%s\n' "$path"; return 0 ;;   # already absolute — never re-anchor
  esac
  root="$(docket_main_worktree "$dir")"
  if [ -z "$root" ]; then
    printf '%s\n' "$path"                    # not a repo: soft fallback, caller's gate reports it
    return 0
  fi
  case "$path" in
    ""|.) printf '%s\n' "$root" ;;
    ./*)  printf '%s\n' "$root/${path#./}" ;;
    *)    printf '%s\n' "$root/$path" ;;
  esac
}

docket_metadata_worktree(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then
    mw="${METADATA_WORKTREE:-.docket}"
  else
    mw="${METADATA_WORKTREE:-.}"
  fi
  docket_anchor_path "$mw"
}

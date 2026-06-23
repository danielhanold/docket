#!/usr/bin/env bash
# scripts/sync-integration-branch.sh — best-effort, FF-only sync of a clone's local
# integration-branch checkout to its remote after a docket merge (change 0029). Runs at docket's
# two merge sites (docket-finalize-change, the docket-status merge sweep) so the skills symlinked
# from the primary checkout stop drifting behind origin/<integration_branch>.
#
# Best-effort like github-mirror.sh (NOT fail-closed like archive-change.sh): the merge has
# already landed, so this is downstream housekeeping. Every runtime skip — wrong branch, dirty
# tree, non-FF divergence, fetch failure, not-a-repo — is a normal exit 0 with a one-line note.
# It never aborts or alters the close-out. Only a usage error (missing --integration-branch,
# unknown flag) exits non-zero.
#
# Triple gate (acts only when ALL hold): on <integration-branch> AND clean tree AND origin/<branch>
# strictly ahead with the local tip an ancestor (a true fast-forward). Then: git merge --ff-only.
#
# Usage: sync-integration-branch.sh --integration-branch BR [--clone-dir DIR] [--remote R]
#   --clone-dir defaults to the main worktree of the invoking repo (CWD).  --remote defaults to origin.
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
BRANCH="" CLONE_DIR="" REMOTE="origin"

note(){ printf '%s\n' "sync-integration-branch: $*" >&2; }
die(){  printf '%s\n' "sync-integration-branch: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --integration-branch) BRANCH="$2"; shift ;;
    --clone-dir) CLONE_DIR="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$BRANCH" ] || die "missing --integration-branch"

# --clone-dir defaults to the MAIN worktree of the repo this script was invoked from (CWD), NOT
# the repo the script physically lives in. git lists the main worktree first and it is reachable
# from any linked worktree in the set, so this resolves the consuming repo's primary checkout even
# when the caller's shell sits in a linked worktree (the sync site runs from the .docket/ metadata
# worktree on the docket branch). `git rev-parse --show-toplevel` would instead return that linked
# worktree (on the docket branch) and gate 1 would skip it — so main-worktree resolution is
# load-bearing. An explicit --clone-dir still overrides. If CWD is not inside a git repo the
# resolution is empty and we fall back to CWD so the not-a-repo gate below emits the standard skip.
if [ -z "$CLONE_DIR" ]; then
  CLONE_DIR="$("$GIT" -C "$PWD" worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p')"
  [ -n "$CLONE_DIR" ] || CLONE_DIR="$PWD"
fi

# not-a-repo → best-effort skip (never abort the close-out).
if ! "$GIT" -C "$CLONE_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  note "not a git work tree: $CLONE_DIR — skipping"; exit 0
fi

# Gate 1: on the integration branch? (detached HEAD → empty → skip)
cur="$("$GIT" -C "$CLONE_DIR" symbolic-ref --short -q HEAD || true)"
if [ "$cur" != "$BRANCH" ]; then
  note "checkout is on '${cur:-(detached)}', not '$BRANCH' — skipping"; exit 0
fi

# Gate 2: clean working tree? (any porcelain output — tracked OR untracked-non-ignored — blocks).
# Condition unchanged; the note is explicit so an untracked-only tree is a diagnosable skip, not a
# silent drift (change 0041).
porcelain="$("$GIT" -C "$CLONE_DIR" status --porcelain 2>/dev/null)"
if [ -n "$porcelain" ]; then
  count="$(printf '%s\n' "$porcelain" | wc -l | tr -d ' ')"
  note "working tree not clean — skipping (best-effort; never fast-forwards onto a non-pristine tree)."
  note "  Untracked (non-ignored) files also block the fast-forward, not only tracked edits."
  note "  Remedy: commit or stash tracked changes, and remove or .gitignore untracked paths, then re-run."
  note "  ${count} offending path(s) (git status --porcelain):"
  printf '%s\n' "$porcelain" | head -5 | sed 's/^/    /' >&2
  exit 0
fi

# Fetch the branch (cheap/no-op for the merge sites, which already fetched). Swallow git's own
# stderr; on failure emit our own note and skip.
if ! "$GIT" -C "$CLONE_DIR" fetch "$REMOTE" "$BRANCH" >/dev/null 2>&1; then
  note "fetch of $REMOTE/$BRANCH failed — skipping (best-effort)"; exit 0
fi

local_tip="$("$GIT" -C "$CLONE_DIR" rev-parse HEAD)"
remote_tip="$("$GIT" -C "$CLONE_DIR" rev-parse FETCH_HEAD)"

# Already current?
if [ "$local_tip" = "$remote_tip" ]; then
  note "$BRANCH already current ($local_tip) — nothing to fast-forward"; exit 0
fi

# Gate 3: true fast-forward? (local tip must be an ancestor of the fetched tip)
if ! "$GIT" -C "$CLONE_DIR" merge-base --is-ancestor "$local_tip" "$remote_tip"; then
  note "$REMOTE/$BRANCH has diverged from local (not a fast-forward) — skipping"; exit 0
fi

# All gates pass: fast-forward only.
if "$GIT" -C "$CLONE_DIR" merge --ff-only FETCH_HEAD >/dev/null 2>&1; then
  note "fast-forwarded $BRANCH ${local_tip:0:9}..${remote_tip:0:9}"
else
  note "fast-forward merge failed unexpectedly — skipping (best-effort)"
fi
exit 0

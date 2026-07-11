#!/usr/bin/env bash
# scripts/disable-worktree-hooks.sh — disable git hooks on a docket-owned worktree, idempotently, so
# docket's bookkeeping commits skip the repo's shared hook framework (pre-commit/husky/lefthook).
# Change 0063. Contract: scripts/disable-worktree-hooks.md. Mock seam: GIT="${GIT:-git}".
set -uo pipefail
GIT="${GIT:-git}"
die(){ echo "disable-worktree-hooks: $1" >&2; exit "${2:-1}"; }
usage(){ echo "usage: disable-worktree-hooks.sh --worktree DIR" >&2; }

WT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --worktree) WT="${2:-}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "disable-worktree-hooks: unknown argument: $1" >&2; usage; exit 2 ;;
  esac; shift
done
[ -n "$WT" ] || { usage; exit 2; }
[ -d "$WT" ] || die "worktree dir not found: $WT"

# Absolute empty, docket-owned hooks dir inside the common git dir. Under .git/, never tracked,
# never leaks into a commit. Absolute (via pwd -P) so core.hooksPath never resolves relative to a
# worktree root; a real (empty) dir avoids "hooksPath does not exist" surprises.
common="$(cd "$WT" && cd "$("$GIT" rev-parse --git-common-dir 2>/dev/null)" && pwd -P)" \
  || die "cannot resolve git common dir for $WT"
empty="$common/docket/empty-hooks"
mkdir -p "$empty"

# worktreeConfig safety (git >=2.20): once enabled, core.worktree/core.bare read per-worktree, so a
# value in the COMMON config would silently stop applying to linked worktrees. Relocate any such
# value to the MAIN worktree's per-worktree config (git's guidance); if that cannot be done safely,
# warn loudly, roll back the enable, and fail closed rather than leave it enabled blindly.
if [ "$("$GIT" -C "$WT" config --local --get extensions.worktreeConfig 2>/dev/null || true)" != "true" ]; then
  main_wt="$("$GIT" -C "$WT" worktree list --porcelain 2>/dev/null | awk '/^worktree /{print $2; exit}')"
  # git requires extensions.worktreeConfig enabled BEFORE any --worktree write, so enable it once
  # up front. If a needed relocation then fails, roll this back so the repo is never left with the
  # extension enabled but a core.worktree/core.bare value stranded (and now ignored) in common config.
  "$GIT" -C "$WT" config extensions.worktreeConfig true
  for key in core.worktree core.bare; do
    val="$("$GIT" -C "$WT" config --local --get "$key" 2>/dev/null || true)"
    [ -n "$val" ] || continue
    if [ -n "$main_wt" ] \
       && "$GIT" -C "$main_wt" config --worktree "$key" "$val" \
       && "$GIT" -C "$WT" config --local --unset "$key"; then
      echo "disable-worktree-hooks: relocated common $key='$val' to $main_wt (worktreeConfig safety)" >&2
    else
      "$GIT" -C "$WT" config --local --unset extensions.worktreeConfig 2>/dev/null || true
      die "refusing to enable worktreeConfig — common $key='$val' present and could not be relocated safely; set core.hooksPath per-invocation instead"
    fi
  done
fi

# Point THIS worktree's hook lookup at the empty dir (worktree-scoped). Idempotent: a repeat write is
# the same value, and --worktree replaces rather than appends, so there is never a duplicate entry.
"$GIT" -C "$WT" config --worktree core.hooksPath "$empty"

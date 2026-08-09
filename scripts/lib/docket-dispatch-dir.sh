#!/usr/bin/env bash
# scripts/lib/docket-dispatch-dir.sh — the durable per-dispatch result area (change 0271).
# Sourced by runner-dispatch.sh; never executed directly.
#
# WHERE: <git-common-dir>/docket/dispatch/<key>/ — the same family as
# disable-worktree-hooks.sh's docket-owned dir inside the common git dir. Under .git/, so it is
# never tracked, never leaks into a commit, and needs no .gitignore entry. It is NOT in the
# feature worktree: a dispatch result must outlive `git worktree remove`, and the whole point of
# this change is that the child's work can be inspected after the run was declared over.
#
# KEY: <agent>-<UTC timestamp>-<pid>. Keyed on agent + a mint rather than on change id or
# worktree, so two concurrent dispatches FOR THE SAME CHANGE never collide.
#
# This file is a sourced helper: it is documented within its caller's contract
# (scripts/runner-dispatch.md), not by a co-located .md (test_script_contracts_coverage.sh
# scopes lib/ out).
#
# Mock seam: GIT="${GIT:-git}".

docket_dispatch_root(){  # $1 = a path inside the repo -> absolute dispatch root; 1 when unresolvable
  # PURE: resolves, never creates. The launcher creates; an observer must be able to ask where the
  # root is without minting one as a side effect of looking.
  local wt="${1:-.}" common gitdir
  gitdir="$(cd "$wt" 2>/dev/null && "${GIT:-git}" rev-parse --git-common-dir 2>/dev/null)" || return 1
  # An empty answer must be refused rather than passed on: `cd ""` is a no-op that SUCCEEDS, so
  # falling through would silently resolve the root inside the WORKTREE instead of under .git/ —
  # a dispatch area that `git worktree remove` then takes with it, which is the one property this
  # location exists to guarantee.
  [ -n "$gitdir" ] || return 1
  common="$(cd "$wt" && cd "$gitdir" && pwd -P)" || return 1
  printf '%s/docket/dispatch' "$common"
}

docket_dispatch_mint(){  # $1 = root, $2 = agent -> prints a fresh unique key, creates its dir
  local root="$1" agent="$2" stamp key
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  key="$agent-$stamp-$$"
  # A same-second, same-pid collision is not reachable in practice, but a silent reuse would
  # overwrite a live dispatch's sentinel — so refuse rather than clobber.
  [ -e "$root/$key" ] && return 1
  mkdir -p "$root/$key" || return 1
  printf '%s' "$key"
}

docket_dispatch_dir(){  # $1 = root, $2 = key -> prints the dir, 1 when absent
  local d="$1/$2"
  [ -d "$d" ] || return 1
  printf '%s' "$d"
}

docket_sentinel_field(){  # $1 = dispatch dir, $2 = field -> value, empty when absent/malformed
  [ -f "$1/done" ] || return 0
  sed -n "s/^$2=//p" "$1/done" | sed -n 1p
}

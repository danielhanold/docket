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

# RETENTION. Every launch mints a directory holding a whole agent run's stdout.log and stderr.log,
# and nothing else in docket ever removes one — not the facade, not cleanup-feature-branch.sh, not
# docket-status. Under the autonomous drainer, the intended heavy user, .git grows without bound.
#
# WHAT THE RULE GUARANTEES, stated as a guarantee rather than as a heuristic:
#   1. A dispatch with NO terminal file (`done` from the launcher wrapper, or `killed` from the
#      observer's give-up) is NEVER considered — so a LIVE child, a child whose observer has not yet
#      given up, and a child nothing ever observed are all untouchable, whatever their age. Liveness
#      is never inferred from a clock here; only the two terminal writes make a dispatch eligible.
#   2. An eligible dispatch is removed only once its TERMINAL FILE is older than the retention
#      window — the file that is written last, so the window is measured from the end of the run and
#      not from its launch. A caller therefore has the full window to observe a finished dispatch,
#      and re-observation stays idempotent throughout it.
# The accepted residual, deliberately: a dispatch that never went terminal is retained forever. That
# is the conservative direction — the evidence this change exists to preserve is never destroyed —
# and such a dispatch is visible to a human under .git/docket/dispatch/ for manual removal.
DOCKET_DISPATCH_RETENTION_DAYS="${DOCKET_DISPATCH_RETENTION_DAYS:-7}"
docket_dispatch_prune(){  # $1 = root -> best effort; always 0, a prune never fails a dispatch
  local root="$1" days="${DOCKET_DISPATCH_RETENTION_DAYS:-7}" d term
  [ -d "$root" ] || return 0
  case "$days" in ''|*[!0-9]*) return 0 ;; esac
  for d in "$root"/*; do
    [ -d "$d" ] || continue
    term=""
    if [ -f "$d/done" ]; then term="$d/done"
    elif [ -f "$d/killed" ]; then term="$d/killed"
    fi
    [ -n "$term" ] || continue
    # `find -mtime +N` — "last modified more than N*24h ago" — on the TERMINAL FILE, and the empty
    # answer is the skip. Captured into a variable rather than piped into a test: a producer feeding
    # an early-exiting consumer takes SIGPIPE under `pipefail` (AGENTS.md, "Shell").
    [ -n "$(find "$term" -maxdepth 0 -mtime +"$days" 2>/dev/null)" ] || continue
    rm -rf "$d"
  done
  return 0
}

docket_sentinel_field(){  # $1 = dispatch dir, $2 = field -> value, empty when absent/malformed
  [ -f "$1/done" ] || return 0
  sed -n "s/^$2=//p" "$1/done" | sed -n 1p
}

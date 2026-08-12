#!/usr/bin/env bash
# scripts/stack-base.sh — print the EFFECTIVE BASE BRANCH for one change (change 0298). A change
# that declares `stacked_on: <parent id>` is built on its parent's unmerged feature branch; every
# other change is built on the integration branch. This is the single answer both
# docket-implement-next (branch cut, PR base) and docket-finalize-change (rebase target) ask for, so
# it is safe to invoke UNCONDITIONALLY: a non-stacked change resolves to the integration branch at
# exit 0 rather than erroring.
#
# Pure read: no writes, no network, no commits. The only external call is a read-only
# `git -C <changes dir> show-ref --verify` under the GIT mock seam — addressed at --changes-dir's
# repo by the resolver itself, so the answer does not depend on the cwd this script is run from.
#
# Usage: stack-base.sh --changes-dir DIR --id N --integration-branch BR [--remote R]
#   --changes-dir        the docket changes directory (the parent of active/ and archive/); required
#   --id                 the change id, padded or bare (0298 and 298 are the same change); required
#   --integration-branch the repo's integration branch (main/develop); required
#   --remote             remote whose refs decide whether a parent branch is pushed (default origin)
#   Mock seam: GIT="${GIT:-git}".
#
# Exit codes: 0 resolved (branch name on stdout) · 2 usage · 3 the chain reaches a KILLED parent ·
# 4 the chain is invalid (missing parent, cycle, or a parent branch with no remote ref). Full
# contract: scripts/stack-base.md.
set -uo pipefail

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT="${GIT:-git}"

die(){ printf 'stack-base: %s\n' "$1" >&2; exit 2; }

CHANGES_DIR=""
ID=""
INTEGRATION_BRANCH=""
REMOTE="origin"
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) [ $# -ge 2 ] || die "--changes-dir needs a value"; CHANGES_DIR="$2"; shift ;;
    --id) [ $# -ge 2 ] || die "--id needs a value"; ID="$2"; shift ;;
    --integration-branch) [ $# -ge 2 ] || die "--integration-branch needs a value"; INTEGRATION_BRANCH="$2"; shift ;;
    --remote) [ $# -ge 2 ] || die "--remote needs a value"; REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

# Validate the WHOLE flag set before doing any work, so a caller fixing one usage error is not sent
# back for the next one a call later.
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$ID" ] || die "missing --id"
[ -n "$INTEGRATION_BRANCH" ] || die "missing --integration-branch"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
case "$ID" in (*[!0-9]*) die "--id must be a change id, got: $ID" ;; esac
# Canonicalize at the ARGUMENT boundary: ids arrive zero-padded from every docket surface, and bash
# reads a leading `0` as an octal prefix — `0237` would silently become 159 and `0008` would not
# parse at all. Same precedent as scripts/board-checks.sh and scripts/adr-checks.sh.
ID=$(( 10#$ID ))

# shellcheck source=lib/docket-frontmatter.sh
source "$SCRIPTDIR/lib/docket-frontmatter.sh"
# shellcheck source=lib/docket-stack.sh
source "$SCRIPTDIR/lib/docket-stack.sh"

base="$(stack_effective_base "$CHANGES_DIR" "$ID" "$INTEGRATION_BRANCH" "$REMOTE")"
rc=$?
case "$rc" in
  0) printf '%s\n' "$base" ;;
  # A POINTER, never an assertion about the immediate parent: exit 3 can come from an ancestor
  # several `stacked-merged` hops up, and this call site cannot tell which — the resolver publishes
  # the id in STACK_KILLED_ANCESTOR, and the `$(…)` that captures the base discards the global with
  # the subshell. board-checks.sh calls the resolver directly and so can name the change; here the
  # honest form is the chain. Naming a "parent" would send a human to a change that is not killed.
  3) printf 'stack-base: change %04d has no resolvable base — its stacked_on chain reaches a KILLED change; a human must rescope or unstack it, never silently fall back to %s\n' "$ID" "$INTEGRATION_BRANCH" >&2 ;;
  4) printf 'stack-base: change %04d has no resolvable base — its stacked_on chain names a missing parent, closes a cycle, or reaches a parent whose branch is not on %s\n' "$ID" "$REMOTE" >&2 ;;
  *) printf 'stack-base: unexpected resolver status %s for change %04d\n' "$rc" "$ID" >&2 ;;
esac
exit "$rc"

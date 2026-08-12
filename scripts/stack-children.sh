#!/usr/bin/env bash
# scripts/stack-children.sh — list the STACKED DESCENDANTS of one change (change 0298), one per
# line, parents before children: `<padded id> <status> <pr-or-dash>`. This is the oracle the
# finalize open-children gate and the step 3.5 close-out gate read.
#
# WHY A CLI AND NOT THE RENDERED ROW: spec §11 makes the child's `stacked_on:` the sole source of
# truth and requires the gate to "derive the child set by scanning … never by reading a parent-side
# list". The parent's derived `## Stacked children` row is a VIEW of that scan taken at the parent's
# last render, and `render-change-links.sh` runs on a link-bearing write to THE PARENT — so a child
# stacked on an already-`implemented` parent is created after the parent's last such write and is
# absent from the row until something writes the parent again. A gate keyed on the row therefore
# reads "no children" for exactly the case the gate exists to catch, and the parent's branch is
# deleted with open child PRs hanging off it. Every gate reads this scan; the row stays a human view.
#
# Pure read: no writes, no network, no git calls at all — unlike stack-base.sh, nothing here turns
# on a remote ref.
#
# Usage: stack-children.sh --changes-dir DIR --id N [--open-only]
#   --changes-dir  the docket changes directory (the parent of active/ and archive/); required
#   --id           the change id, padded or bare (0298 and 298 are the same change); required
#   --open-only    keep only descendants a merge of this change would STRAND: everything that is
#                  neither terminal (DOCKET_STATUSES_TERMINAL) nor `stacked-merged`. This is spec
#                  §8's gate set — a `stacked-merged` child's code is already in this branch and
#                  rides the merge, and a `done` or `killed` child is settled.
#
# Exit codes: 0 scan completed (an empty stdout means NO descendants) · 2 usage · 4 --id names no
# change in this tree. Full contract: scripts/stack-children.md.
set -uo pipefail

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die(){ printf 'stack-children: %s\n' "$1" >&2; exit 2; }

CHANGES_DIR=""
ID=""
OPEN_ONLY=0
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) [ $# -ge 2 ] || die "--changes-dir needs a value"; CHANGES_DIR="$2"; shift ;;
    --id) [ $# -ge 2 ] || die "--id needs a value"; ID="$2"; shift ;;
    --open-only) OPEN_ONLY=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

# Validate the WHOLE flag set before doing any work, as stack-base.sh does: a caller fixing one
# usage error is not sent back for the next one a call later.
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$ID" ] || die "missing --id"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
case "$ID" in (*[!0-9]*) die "--id must be a change id, got: $ID" ;; esac
# Canonicalize at the ARGUMENT boundary: ids arrive zero-padded from every docket surface, and bash
# reads a leading `0` as an octal prefix — `0030` would silently become 24 and `0008` would not
# parse at all. Same precedent as scripts/stack-base.sh and scripts/board-checks.sh.
ID=$(( 10#$ID ))

# shellcheck source=lib/docket-frontmatter.sh
source "$SCRIPTDIR/lib/docket-frontmatter.sh"
# shellcheck source=lib/docket-stack.sh
source "$SCRIPTDIR/lib/docket-stack.sh"

# THE ROOT MUST EXIST. `stack_descendants` answers an unknown root with the same empty stdout as a
# childless one — correct for a library a renderer calls on every file, wrong for the oracle a gate
# keys on: a mistyped id would read as "nothing to block on" and let the merge through for a reason
# that has nothing to do with the stack. Exit 4 is the same "repair the data, never fall back"
# family stack-base.sh uses.
if ! stack_find_file "$CHANGES_DIR" "$ID" >/dev/null; then
  printf 'stack-children: no change %04d in %s — check the id before reading this as "no children"\n' \
    "$ID" "$CHANGES_DIR" >&2
  exit 4
fi

while IFS= read -r cid; do
  [ -n "$cid" ] || continue
  f="$(stack_find_file "$CHANGES_DIR" "$cid")" || continue
  status="$(field "$f" status)"
  # `status:` is guaranteed by the change template, so the unanchored `field` is correct for it;
  # `pr:` is an OPTIONAL key and takes the anchored `fm_field` — an unanchored read of an absent key
  # runs past the closing `---` into body prose that discusses it. See the selection rule in
  # scripts/lib/docket-frontmatter.sh.
  pr="$(fm_field "$f" pr)"
  if [ "$OPEN_ONLY" = 1 ]; then
    # Keyed on the shared vocabulary, never a hand-listed set of statuses: a status added to
    # DOCKET_STATUSES_TERMINAL is dropped here the day it lands. `stacked-merged` is the one named
    # exception — it is an ACTIVE status, so "not terminal" alone would leave it in and hard-block
    # every finalize of a stack root, which is the merge the close-out exists to let through.
    docket_status_is_terminal "$status" && continue
    [ "$status" = stacked-merged ] && continue
  fi
  printf '%04d %s %s\n' "$cid" "${status:--}" "${pr:--}"
done < <(stack_descendants "$CHANGES_DIR" "$ID")

# EXPLICIT. A `while` loop returns the status of the last command its body ran, so a final iteration
# that ends on a filtered-out `[ … = stacked-merged ]` would exit 1 — "the scan failed" — for a root
# whose last descendant simply did not match. The scan's success is not the filter's last verdict.
exit 0

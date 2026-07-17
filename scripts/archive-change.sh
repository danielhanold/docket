#!/usr/bin/env bash
# scripts/archive-change.sh — the shared terminal-transition archive primitive (change 0025).
# Moves a change from active/ to a dated archive/ name on the metadata branch, sets its terminal
# frontmatter, commits CHANGE-FILE-ONLY, and pushes with a rebase-retry loop. `done` and `killed`
# are unified here; finalize, the docket-status sweep, and the two kill paths all invoke this.
# Fail-closed: self-verifies its postconditions and exits non-zero with a diagnostic on deviation.
# Idempotent: a reuse-existing-archive probe makes a racing/resumed run a safe no-op.
#
# CONCURRENCY PRECONDITION: callers MUST derive --date (the UTC merge/kill date) and --results
# deterministically from the manifest, never from now(). That is what keeps the change-file-only
# commit tree-identical across two concurrent archivers (finalize racing the docket-status sweep):
# they stage the same rename to the same dated path with the same fields, so neither clobbers the
# other and the CAS push resolves to identical bytes.
#
# Usage:
#   archive-change.sh --changes-dir DIR --id N --outcome done|killed --date YYYY-MM-DD
#                     [--message MSG] [--results PATH] [--reason TEXT] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

. "$(dirname "$0")/lib/docket-frontmatter.sh"

GIT="${GIT:-git}"
CHANGES_DIR="" ID="" OUTCOME="" DATE="" MESSAGE="" RESULTS="" REASON="" REMOTE="origin"

die(){ printf '%s\n' "archive-change: $*" >&2; exit 1; }
log(){ printf '%s\n' "archive-change: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --id) ID="$2"; shift ;;
    --outcome) OUTCOME="$2"; shift ;;
    --date) DATE="$2"; shift ;;
    --message) MESSAGE="$2"; shift ;;
    --results) RESULTS="$2"; shift ;;
    --reason) REASON="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$ID" ]          || die "missing --id"
case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome (done|killed)" ;; esac
[ -n "$DATE" ]        || die "missing --date"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"

pad="$(printf '%04d' "$ID")"
WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
# changes-dir path relative to the worktree root (git mv/commit want worktree-relative paths).
# Use pwd -P to resolve symlinks: macOS mktemp gives /var/... but git rev-parse gives /private/var/...
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"
REL="${REL_ABS#"$WT"/}"

# cas_push BRANCH: push current HEAD to REMOTE/BRANCH, rebasing on non-fast-forward.
cas_push(){
  local br="$1"
  until $GIT -C "$WT" push "$REMOTE" "$br"; do
    $GIT -C "$WT" pull --rebase "$REMOTE" "$br" || die "rebase during push failed for $br"
  done
}

# set_field FILE KEY VALUE — replace a top-level frontmatter scalar in place (portable sed).
# Only touches the first ---...--- frontmatter block; never rewrites matching prose in the body.
set_field(){
  local f="$1" k="$2" v="$3" t; t="$(mktemp)"
  sed -E "/^---$/,/^---$/ s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv "$t" "$f"
}

branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"
shopt -s nullglob
active_matches=("$WT/$REL/active/$pad-"*.md)
archive_matches=("$WT/$REL/archive/"*"-$pad-"*.md)
shopt -u nullglob

# (1) reuse-existing-archive probe — already archived => idempotent no-op.
if [ "${#archive_matches[@]}" -gt 0 ]; then
  log "already archived (${archive_matches[0]##*/}); no-op"
  exit 0
fi
[ "${#active_matches[@]}" -eq 1 ] || die "expected exactly one active/$pad-*.md, found ${#active_matches[@]}"

active_file="${active_matches[0]}"
base="$(basename "$active_file")"           # <pad>-<slug>.md
slug="${base#"$pad-"}"; slug="${slug%.md}"
dest_rel="$REL/archive/$DATE-$pad-$slug.md"
src_rel="$REL/active/$base"

# (2) dated move
mkdir -p "$WT/$REL/archive"
$GIT -C "$WT" mv "$src_rel" "$dest_rel" || die "git mv failed"

# (3) frontmatter
dest="$WT/$dest_rel"
set_field "$dest" status "$OUTCOME"
set_field "$dest" updated "$DATE"
set_field "$dest" claimed_at ""   # presence-encoded-state: drop the lease on the terminal transition
if [ "$OUTCOME" = done ] && [ -n "$RESULTS" ]; then
  set_field "$dest" results "$RESULTS"
fi
if [ "$OUTCOME" = killed ]; then
  { printf '\n## Why killed\n\n'; printf '%s\n' "${REASON:-Killed.}"; } >> "$dest"
fi

# (4) commit CHANGE-FILE-ONLY + push
[ -n "$MESSAGE" ] || MESSAGE="docket($pad): $OUTCOME — archived (status $OUTCOME, $DATE)"
# git mv pre-staged both halves of the rename; -- pins the commit to the change file only.
$GIT -C "$WT" commit -m "$MESSAGE" -- "$src_rel" "$dest_rel" >/dev/null || die "commit failed"
cas_push "$branch"

# (5) fail-closed self-verification
[ ! -e "$WT/$src_rel" ]                                   || die "postcondition: active file still present"
[ -e "$dest" ]                                            || die "postcondition: archive file missing"
[ "$(field "$dest" status)"  = "$OUTCOME" ]              || die "postcondition: status not $OUTCOME"
[ "$(field "$dest" updated)" = "$DATE" ]                 || die "postcondition: updated not $DATE"
[ -z "$(field "$dest" claimed_at)" ]                     || die "postcondition: claimed_at not cleared"
if [ "$OUTCOME" = done ] && [ -n "$RESULTS" ]; then
  [ "$(field "$dest" results)" = "$RESULTS" ] || die "postcondition: results not set to $RESULTS"
fi
[ "$($GIT -C "$WT" rev-parse @)" = "$($GIT -C "$WT" rev-parse "$REMOTE/$branch")" ] \
  || die "postcondition: push did not land on $REMOTE/$branch"
log "archived $base -> $DATE-$pad-$slug.md ($OUTCOME)"

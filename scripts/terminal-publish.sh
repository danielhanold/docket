#!/usr/bin/env bash
# scripts/terminal-publish.sh — the shared "Terminal publish (docket-mode)" procedure (change 0025).
# Copies a change's terminal records (archived change file + its spec + its Accepted ADRs) from
# origin/<metadata-branch> onto the integration branch, via a transient worktree, with a CAS push.
# docket-mode only: a no-op in main-mode (metadata-branch == integration-branch). Fail-closed:
# re-fetches and asserts the full copy-set landed before exiting 0. Idempotent and re-run safe.
#
# Usage:
#   terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

. "$(dirname "$0")/lib/docket-frontmatter.sh"

GIT="${GIT:-git}"
ID="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"

die(){ printf '%s\n' "terminal-publish: $*" >&2; exit 1; }
log(){ printf '%s\n' "terminal-publish: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --id) ID="$2"; shift ;;
    --outcome) OUTCOME="$2"; shift ;;
    --integration-branch) INT_BRANCH="$2"; shift ;;
    --metadata-branch) META_BRANCH="$2"; shift ;;
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --message) MESSAGE="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

[ -n "$ID" ] || die "missing --id"
case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome" ;; esac
[ -n "$INT_BRANCH" ] && [ -n "$META_BRANCH" ] || die "missing --integration-branch/--metadata-branch"
[ -n "$CHANGES_DIR" ] && [ -n "$ADRS_DIR" ]   || die "missing --changes-dir/--adrs-dir"

# Mode guard: main-mode has no docket branch to copy from.
if [ "$META_BRANCH" = "$INT_BRANCH" ]; then
  log "main-mode (metadata-branch == integration-branch); no-op"
  exit 0
fi

pad="$(printf '%04d' "$ID")"
[ -n "$MESSAGE" ] || MESSAGE="docket($pad): publish terminal record ($OUTCOME)"

# --- build the copy-set from origin/<metadata-branch> (authoritative remote bytes) ---
$GIT fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$META_BRANCH failed"
metaref="$REMOTE/$META_BRANCH"

# locate the archived change file path on the metadata branch by id
tree="$($GIT ls-tree -r --name-only "$metaref" -- "$CHANGES_DIR/archive")"
change_path="$(printf '%s\n' "$tree" | grep -m1 -E "/[0-9]{4}-[0-9]{2}-[0-9]{2}-$pad-[^/]*\.md$")"
[ -n "$change_path" ] || die "no archived change file for id $ID on $metaref"

# read its frontmatter via a temp dump (field operates on files)
tmpd="$(mktemp -d)"
trap 'rm -rf "$tmpd"' EXIT
$GIT show "$metaref:$change_path" > "$tmpd/change.md" || die "cannot read $change_path"
spec_path="$(field "$tmpd/change.md" spec)"
adr_ids="$(list_field "$tmpd/change.md" adrs)"

copyset=("$change_path")
[ -n "$spec_path" ] && copyset+=("$spec_path")

# Accepted gate: include an ADR only if its status: is Accepted on the metadata branch
adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
for aid in $adr_ids; do
  apad="$(printf '%04d' "$aid")"
  apath="$(printf '%s\n' "$adr_tree" | grep -m1 -E "/$apad-[^/]*\.md$")"
  [ -n "$apath" ] || { log "adr $aid: file not found on $metaref; skipping"; continue; }
  $GIT show "$metaref:$apath" > "$tmpd/adr.md" || { log "adr $aid: unreadable; skipping"; continue; }
  if [ "$(field "$tmpd/adr.md" status)" = "Accepted" ]; then
    copyset+=("$apath")
  else
    log "adr $aid: not Accepted; skipped by gate"
  fi
done

# --- provision a transient integration checkout on a throwaway branch ---
pub="$(mktemp -d)/pub"
$GIT worktree prune
$GIT worktree add -B "pub-$ID" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$ID worktree"

teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1
  $GIT worktree remove --force "$pub" >/dev/null 2>&1
  $GIT branch -D "pub-$ID" >/dev/null 2>&1 || true
  rm -rf "$(dirname "$pub")" "$tmpd"
}

# --- copy the terminal records from the metadata remote tip and CAS-push ---
$GIT -C "$pub" fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || { teardown; die "fetch in pub failed"; }
$GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}" || { teardown; die "checkout copyset failed"; }
if ! $GIT -C "$pub" diff --cached --quiet; then
  $GIT -C "$pub" commit -m "$MESSAGE" >/dev/null || { teardown; die "publish commit failed"; }
fi
# push HEAD explicitly (a bare push resolves the stale local <integration> ref); CAS retry loop
until $GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"; do
  if $GIT -C "$pub" pull --rebase "$REMOTE" "$INT_BRANCH"; then :; else
    $GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}"
    $GIT -C "$pub" rebase --continue || { teardown; die "CAS rebase --continue failed"; }
  fi
done

# --- fail-closed: re-fetch and assert the full copy-set landed on origin/<integration> ---
$GIT fetch "$REMOTE" "$INT_BRANCH" >/dev/null 2>&1 || { teardown; die "post-push fetch failed"; }
landed="$($GIT ls-tree -r --name-only "$REMOTE/$INT_BRANCH")"
for p in "${copyset[@]}"; do
  printf '%s\n' "$landed" | grep -qxF "$p" || { teardown; die "postcondition: $p missing on $REMOTE/$INT_BRANCH"; }
done

teardown
# teardown removed the worktree; assert it is gone (registration pruned)
wt_list="$($GIT worktree list)"
printf '%s\n' "$wt_list" | grep -q "pub-$ID" && die "postcondition: pub-$ID worktree survived"
log "published ${#copyset[@]} record(s) for id $ID onto $INT_BRANCH"
exit 0

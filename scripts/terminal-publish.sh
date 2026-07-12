#!/usr/bin/env bash
# scripts/terminal-publish.sh — the shared "Terminal publish (docket-mode)" procedure (change 0025;
# --adr mode added in 0030). The single executor of BOTH publish shapes. In change mode it copies a
# change's terminal records (archived change file + its spec + its Accepted ADRs); in ADR mode it
# copies a single ADR file (no Accepted gate; archive step skipped) — both from origin/<metadata-branch>
# onto the integration branch, via a transient worktree, with a CAS push. docket-mode only: a no-op in
# main-mode (metadata-branch == integration-branch). Fail-closed: re-fetches and asserts the full
# copy-set landed before exiting 0. Idempotent and re-run safe.
#
# Usage (exactly one of --id / --adr):
#   terminal-publish.sh --id N --outcome done|killed --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#                       [--enabled true|false]
#   terminal-publish.sh --adr NN --integration-branch B --metadata-branch M
#                       --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]
#                       [--enabled true|false]
#
# --enabled false (change 0064: the per-repo `terminal_publish` knob) makes this script a no-op:
# the record stays on the metadata branch and nothing is committed onto the integration branch.
# Default true — omitting the flag behaves exactly as before the knob existed. The guard sits
# BEFORE the --id/--adr mode dispatch, so one guard covers BOTH publish shapes.
#
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

. "$(dirname "$0")/lib/docket-frontmatter.sh"

GIT="${GIT:-git}"
ID="" ADR="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"
ENABLED="true"   # change 0064: default true == today's behavior

die(){ printf '%s\n' "terminal-publish: $*" >&2; exit 1; }
log(){ printf '%s\n' "terminal-publish: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --id) ID="$2"; shift ;;
    --adr) ADR="$2"; shift ;;
    --outcome) OUTCOME="$2"; shift ;;
    --integration-branch) INT_BRANCH="$2"; shift ;;
    --metadata-branch) META_BRANCH="$2"; shift ;;
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --message) MESSAGE="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    --enabled) ENABLED="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

# exactly one of --id / --adr
if [ -n "$ID" ] && [ -n "$ADR" ]; then die "--id and --adr are mutually exclusive"; fi
if [ -z "$ID" ] && [ -z "$ADR" ]; then die "exactly one of --id / --adr is required"; fi
# fail closed on a non-integer id (CLI arg, never frontmatter) — a publish must hard-stop, not skip
case "$ID"  in (''|*[!0-9]*) [ -z "$ID" ]  || die "non-integer --id: '$ID'"  ;; esac
case "$ADR" in (''|*[!0-9]*) [ -z "$ADR" ] || die "non-integer --adr: '$ADR'" ;; esac
# --outcome is required (and validated) only in change (--id) mode
if [ -n "$ID" ]; then
  case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome" ;; esac
fi
[ -n "$INT_BRANCH" ] && [ -n "$META_BRANCH" ] || die "missing --integration-branch/--metadata-branch"
[ -n "$CHANGES_DIR" ] && [ -n "$ADRS_DIR" ]   || die "missing --changes-dir/--adrs-dir"
# change 0064: fail closed on an unparseable value — never silently coerce to true, which would
# publish onto the integration branch against the repo's stated intent.
case "$ENABLED" in true|false) ;; *) die "invalid --enabled: '$ENABLED' (expected true|false)" ;; esac

# Mode guard: main-mode has no docket branch to copy from.
if [ "$META_BRANCH" = "$INT_BRANCH" ]; then
  log "main-mode (metadata-branch == integration-branch); no-op"
  exit 0
fi

# Knob guard (change 0064): terminal_publish: false. A second no-op guard beside the mode guard,
# placed BEFORE the --id/--adr dispatch so it covers BOTH publish shapes. A suppressed publish is
# SUCCESS (exit 0) — callers trust the exit code, and close-out steps 4-5 (cleanup, board) still run.
if [ "$ENABLED" = false ]; then
  log "terminal_publish: false — skipping publish onto $INT_BRANCH; the record stays on $META_BRANCH"
  exit 0
fi

# --- fetch the authoritative metadata remote tip ---
$GIT fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$META_BRANCH failed"
metaref="$REMOTE/$META_BRANCH"

tmpd="$(mktemp -d)"
trap 'rm -rf "$tmpd"' EXIT

# Tracks whether the copy-set includes ≥1 ADR file (change 0040). When true, the publish also
# regenerates the integration-branch ADR index from the branch's own ADR set (see render block below).
adr_published=false

if [ -n "$ADR" ]; then
  # ----- ADR-only publish: copy-set = the single ADR file; step-1 archive skipped; no Accepted gate -----
  apad="$(printf '%04d' "$ADR")"
  T="adr-$apad"
  [ -n "$MESSAGE" ] || MESSAGE="docket(adr-$apad): publish ADR-$apad"
  adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
  apath="$(printf '%s\n' "$adr_tree" | grep -E "/$apad-[^/]*\.md$")"
  apath="${apath%%$'\n'*}"
  [ -n "$apath" ] || die "no ADR file for id $ADR on $metaref"
  copyset=("$apath")
  adr_published=true   # the lone copy-set entry is an ADR
else
  # ----- change publish: token = the id; build copy-set from the archived change manifest -----
  pad="$(printf '%04d' "$ID")"
  T="$ID"
  [ -n "$MESSAGE" ] || MESSAGE="docket($pad): publish terminal record ($OUTCOME)"
  tree="$($GIT ls-tree -r --name-only "$metaref" -- "$CHANGES_DIR/archive")"
  change_path="$(printf '%s\n' "$tree" | grep -E "/[0-9]{4}-[0-9]{2}-[0-9]{2}-$pad-[^/]*\.md$")"
  change_path="${change_path%%$'\n'*}"
  [ -n "$change_path" ] || die "no archived change file for id $ID on $metaref"
  $GIT show "$metaref:$change_path" > "$tmpd/change.md" || die "cannot read $change_path"
  spec_path="$(field "$tmpd/change.md" spec)"
  adr_ids="$(list_field "$tmpd/change.md" adrs)"
  copyset=("$change_path")
  [ -n "$spec_path" ] && copyset+=("$spec_path")
  # Accepted gate: include an ADR only if its status: is Accepted on the metadata branch
  adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
  for aid in $adr_ids; do
    apad="$(printf '%04d' "$aid")"
    apath="$(printf '%s\n' "$adr_tree" | grep -E "/$apad-[^/]*\.md$")"
    apath="${apath%%$'\n'*}"
    [ -n "$apath" ] || { log "adr $aid: file not found on $metaref; skipping"; continue; }
    $GIT show "$metaref:$apath" > "$tmpd/adr.md" || { log "adr $aid: unreadable; skipping"; continue; }
    if [ "$(field "$tmpd/adr.md" status)" = "Accepted" ]; then
      copyset+=("$apath")
      adr_published=true   # ≥1 Accepted ADR in the copy-set
    else
      log "adr $aid: not Accepted; skipped by gate"
    fi
  done
fi

# --- provision a transient integration checkout on a throwaway branch ---
pub="$(mktemp -d)/pub"
$GIT worktree prune
$GIT worktree add -B "pub-$T" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$T worktree"
# Change 0063: this is docket's own doc-publish commit (its archived change/spec/ADRs), not the
# team's code — skip the integration branch's shared hooks on it. Covers the publish commit AND the
# CAS rebase --continue replay below (worktree-scoped, torn down with the worktree). Best-effort.
"$(dirname "$0")/disable-worktree-hooks.sh" --worktree "$pub" >/dev/null 2>&1 || true

teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1
  $GIT worktree remove --force "$pub" >/dev/null 2>&1
  $GIT branch -D "pub-$T" >/dev/null 2>&1 || true
  rm -rf "$(dirname "$pub")" "$tmpd"
}

# change 0040: regenerate the integration-branch ADR index, staged into the same publish commit.
# Fires only when this publish copies ≥1 ADR ($adr_published). Renders from pub's OWN <adrs_dir>
# — the integration branch's ADR files with this publish's ADR(s) overlaid (the copy-set was just
# checked out) — never the metadata superset, so every index link resolves (no dangling rows). The
# dir is guaranteed present (an ADR was just checked out into it). Idempotent: a byte-identical
# re-render leaves nothing for the guarded commit to capture.
refresh_adr_index(){
  [ "$adr_published" = true ] || return 0
  "$(dirname "$0")/render-adr-index.sh" --adrs-dir "$pub/$ADRS_DIR" > "$pub/$ADRS_DIR/README.md" \
    || { teardown; die "adr index render failed"; }
  $GIT -C "$pub" add "$ADRS_DIR/README.md"
}

# --- copy the terminal records from the metadata remote tip and CAS-push ---
$GIT -C "$pub" fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || { teardown; die "fetch in pub failed"; }
$GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}" || { teardown; die "checkout copyset failed"; }
refresh_adr_index
if ! $GIT -C "$pub" diff --cached --quiet; then
  $GIT -C "$pub" commit -m "$MESSAGE" >/dev/null || { teardown; die "publish commit failed"; }
fi
# push HEAD explicitly (a bare push resolves the stale local <integration> ref); CAS retry loop
until $GIT -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"; do
  if $GIT -C "$pub" pull --rebase "$REMOTE" "$INT_BRANCH"; then :; else
    $GIT -C "$pub" checkout "$metaref" -- "${copyset[@]}"
    refresh_adr_index   # regenerate deterministically on conflict — never a 3-way-merge of the index
    GIT_EDITOR=true $GIT -C "$pub" rebase --continue \
      || { teardown; die "CAS rebase --continue failed"; }
  fi
done

# --- fail-closed: re-fetch and assert the full copy-set landed on origin/<integration> ---
$GIT fetch "$REMOTE" "$INT_BRANCH" >/dev/null 2>&1 || { teardown; die "post-push fetch failed"; }
landed="$($GIT ls-tree -r --name-only "$REMOTE/$INT_BRANCH")"
for p in "${copyset[@]}"; do
  printf '%s\n' "$landed" | grep -qxF "$p" || { teardown; die "postcondition: $p missing on $REMOTE/$INT_BRANCH"; }
done
# change 0040: the rendered ADR index is not in the copy-set, so assert it separately (fail-closed).
if [ "$adr_published" = true ]; then
  printf '%s\n' "$landed" | grep -qxF "$ADRS_DIR/README.md" \
    || { teardown; die "postcondition: $ADRS_DIR/README.md missing on $REMOTE/$INT_BRANCH"; }
fi

teardown
# teardown removed the worktree; assert it is gone (registration pruned)
wt_list="$($GIT worktree list)"
printf '%s\n' "$wt_list" | grep -q "pub-$T" && die "postcondition: pub-$T worktree survived"
log "published ${#copyset[@]} record(s) for $T onto $INT_BRANCH"
exit 0

#!/usr/bin/env bash
# scripts/docket-status.sh — deterministic orchestrator for the docket-status pass (change 0058).
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
#
# Usage: docket-status.sh [--board-only] [--repo OWNER/REPO] [--project OWNER/NUMBER]
#                          [--auto-create-project] [--project-owner OWNER]
#   --board-only           only regenerate the board surfaces; skip sweep/health passes
#   --repo OWNER/REPO      GitHub repo for PR-link resolution (defaults to origin remote)
#   --project OWNER/NUMBER GitHub Project to sync (later task)
#   --auto-create-project  create the GitHub Project if --project doesn't resolve (later task)
#   --project-owner OWNER  owner to create the project under (later task)
#
# Contract: scripts/docket-status.md.
# Mock seams: GIT="${GIT:-git}", GH="${GH:-gh}", CONFIG_EXPORT_CMD (config export override).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT="${GIT:-git}"
GH="${GH:-gh}"
# shellcheck source=lib/docket-frontmatter.sh
. "$SELF_DIR"/lib/docket-frontmatter.sh

BOARD_ONLY=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
usage(){ sed -n '2,12p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done

# Config export mock seam: CONFIG_EXPORT_CMD lets tests inject a stub export.
config_export(){ ${CONFIG_EXPORT_CMD:-"$SELF_DIR"/docket-config.sh --export}; }

ensure_and_sync_worktree(){
  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    if [ ! -d "$wt" ]; then
      "$GIT" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$GIT" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-status: cannot create metadata worktree $wt" >&2; exit 1; }
    fi
    "$GIT" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$GIT" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-status: metadata worktree sync failed" >&2; exit 1; }
  else
    "$GIT" pull --rebase >&2 || { echo "docket-status: metadata sync failed" >&2; exit 1; }
  fi
}

board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  [ -n "$surfaces" ] || return 0
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  local tok
  for tok in $surfaces; do
    case "$tok" in
      inline) board_pass_inline "$mw" "$cd_dir" ;;
      github) board_pass_github "$cd_dir" ;;
      *) echo "docket-status: unknown board surface '$tok'" >&2 ;;
    esac
  done
}

board_pass_inline(){
  local mw="$1" cd_dir="$2"
  local board="$cd_dir/BOARD.md" tmp="$cd_dir/BOARD.md.tmp"
  if ! "$SELF_DIR"/render-board.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} > "$tmp" 2>&2 || [ ! -s "$tmp" ]; then
    echo "docket-status: board render failed; keeping existing BOARD.md" >&2
    rm -f "$tmp"
    return 0
  fi
  if [ -f "$board" ] && cmp -s "$tmp" "$board"; then
    rm -f "$tmp"
    echo "board inline clean"
    return 0
  fi
  mv "$tmp" "$board"
  "$GIT" -C "$mw" add "$(basename "$board")" >&2 2>/dev/null || "$GIT" -C "$mw" add "$board" >&2
  "$GIT" -C "$mw" commit -q -m "docket: board refresh" >&2 || true

  local attempt=0 pushed=0
  while [ $attempt -lt 5 ]; do
    attempt=$((attempt + 1))
    if "$GIT" -C "$mw" push >&2 2>&1; then
      pushed=1
      break
    fi
    if ! "$GIT" -C "$mw" pull --rebase >&2 2>&1; then
      if "$GIT" -C "$mw" status --porcelain 2>/dev/null | grep -q "BOARD.md"; then
        if ! "$SELF_DIR"/render-board.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} > "$tmp" 2>&2 || [ ! -s "$tmp" ]; then
          echo "docket-status: board regeneration during rebase failed; aborting rebase" >&2
          rm -f "$tmp"
          "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true
          pushed=-1
          break
        fi
        mv "$tmp" "$board"
        "$GIT" -C "$mw" add "$(basename "$board")" >&2 2>/dev/null || "$GIT" -C "$mw" add "$board" >&2
        "$GIT" -C "$mw" rebase --continue >&2 2>&1 || { "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true; pushed=-1; break; }
      else
        "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true
        break
      fi
    fi
  done
  if [ $pushed -eq 1 ]; then
    echo "board inline changed pushed"
  else
    echo "board inline changed push-failed"
  fi
}

board_pass_github(){
  local cd_dir="$1"
  local out rc
  out="$("$SELF_DIR"/github-mirror.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} ${PROJECT_FLAG:+--project "$PROJECT_FLAG"} $([ "$AUTO_CREATE_PROJECT" = 1 ] && echo --auto-create-project) ${PROJECT_OWNER:+--project-owner "$PROJECT_OWNER"} 2>&2)"
  rc=$?
  echo "$out" | while IFS= read -r line; do
    case "$line" in
      "issue-minted "*) set -- $line; echo "minted issue $2 $3" ;;
      "project-minted "*) set -- $line; echo "minted project $2 $3" ;;
    esac
  done
  if [ $rc -eq 0 ]; then
    echo "board github ok"
  else
    echo "board github failed"
  fi
}

# detect_merged — batched sweep detection (change 0058, task 4). Prints TAB-separated
# "<id>\t<slug>\t<pr>\t<merged-date>" for every `implemented` change under $CD/active whose
# PR has merged, using ONE batched gh call (an aliased graphql query keyed by pr number, plus a
# per-change `gh pr list` fallback only for changes with no `pr:` set). merged-date is the UTC
# date portion of GitHub's mergedAt (already Zulu/UTC) — never now()/local time. Best-effort:
# any gh/network/parse failure emits "sweep-skipped <reason>" and returns 0 (never aborts the pass).
detect_merged(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"

  local -a files
  mapfile -t files < <(find "$cd_dir/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  [ ${#files[@]} -gt 0 ] || return 0

  local -a ids slugs prs
  local f id slug status pr
  for f in "${files[@]}"; do
    status="$(field "$f" status)"
    [ "$status" = implemented ] || continue
    id="$(int_field "$f" id)"
    [ -n "$id" ] || continue
    slug="$(field "$f" slug)"
    pr="$(int_field "$f" pr)"
    ids+=("$id"); slugs+=("$slug"); prs+=("$pr")
  done
  [ ${#ids[@]} -gt 0 ] || return 0

  local repo="${REPO_FLAG:-}"
  if [ -z "$repo" ]; then
    repo="$("$GH" repo view --json owner,name -q '(.owner.login)+"/"+(.name)' 2>/dev/null)" \
      || { echo "sweep-skipped gh-unavailable"; return 0; }
  fi
  local owner="${repo%%/*}" name="${repo#*/}"
  if [ -z "$owner" ] || [ -z "$name" ] || [ "$owner" = "$repo" ]; then
    echo "sweep-skipped repo-unresolved"
    return 0
  fi

  # Build one aliased graphql query for every change with a known pr: number.
  local query="query {" i has_pr=0
  for i in "${!ids[@]}"; do
    [ -n "${prs[$i]}" ] || continue
    query="$query p${ids[$i]}: repository(owner: \"$owner\", name: \"$name\") { pullRequest(number: ${prs[$i]}) { number mergedAt state } }"
    has_pr=1
  done
  query="$query }"

  local gql_json="" gql_rc=0
  if [ "$has_pr" -eq 1 ]; then
    gql_json="$("$GH" api graphql -f query="$query" 2>/dev/null)"; gql_rc=$?
    if [ $gql_rc -ne 0 ] || [ -z "$gql_json" ] || ! printf '%s' "$gql_json" | jq -e . >/dev/null 2>&1; then
      echo "sweep-skipped gh-unavailable"
      return 0
    fi
  fi

  local merged_at state date pl_json pl_num pl_merged
  for i in "${!ids[@]}"; do
    id="${ids[$i]}"; slug="${slugs[$i]}"; pr="${prs[$i]}"
    if [ -n "$pr" ]; then
      merged_at="$(printf '%s' "$gql_json" | jq -r ".data.p${id}.mergedAt // empty" 2>/dev/null)"
      state="$(printf '%s' "$gql_json" | jq -r ".data.p${id}.state // empty" 2>/dev/null)"
      if [ "$state" = MERGED ] && [ -n "$merged_at" ]; then
        date="${merged_at:0:10}"
        printf '%s\t%s\t%s\t%s\n' "$id" "$slug" "$pr" "$date"
      fi
    else
      pl_json="$("$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
      if [ $? -ne 0 ] || ! printf '%s' "$pl_json" | jq -e . >/dev/null 2>&1; then
        continue
      fi
      pl_num="$(printf '%s' "$pl_json" | jq -r '.[0].number // empty')"
      pl_merged="$(printf '%s' "$pl_json" | jq -r '.[0].mergedAt // empty')"
      if [ -n "$pl_num" ] && [ -n "$pl_merged" ]; then
        date="${pl_merged:0:10}"
        printf '%s\t%s\t%s\t%s\n' "$id" "$slug" "$pl_num" "$date"
      fi
    fi
  done
  return 0
}

main(){
  local cfg; cfg="$(config_export)" || { echo "docket-status: config export failed" >&2; exit 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE) echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; exit 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — bootstrap is opt-in; run a docket skill to create the docket branch" >&2; exit 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; exit 1 ;;
  esac
  ensure_and_sync_worktree
  board_pass
  # Steps 4..7 wired in later tasks.
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi

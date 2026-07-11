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
        "$SELF_DIR"/render-board.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} > "$board" 2>&2
        "$GIT" -C "$mw" add "$(basename "$board")" >&2 2>/dev/null || "$GIT" -C "$mw" add "$board" >&2
        "$GIT" -C "$mw" rebase --continue >&2 2>&1 || break
      else
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
main "$@"

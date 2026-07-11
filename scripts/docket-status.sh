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

main(){
  local cfg; cfg="$(config_export)" || { echo "docket-status: config export failed" >&2; exit 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE) echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; exit 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — bootstrap is opt-in; run a docket skill to create the docket branch" >&2; exit 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; exit 1 ;;
  esac
  # Steps 2..7 wired in later tasks.
}
main "$@"

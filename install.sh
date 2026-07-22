#!/usr/bin/env bash
# install.sh — set up docket on this machine, in one command.
#
# Runs the four install primitives in order:
#   1. ensure-global-config.sh — discover/persist a machine-local Bash 4+ runtime before any
#                                runtime-dependent primitive
#   2. link-skills.sh  — symlink the skills into each present harness's skill dir (live; edit-once)
#   3. sync-agents.sh  — generate the model/effort-pinned agent wrappers into each present harness
#                        (generated copies; re-run after editing a config layer)
#   4. ensure-docket-env.sh — export DOCKET_SCRIPTS_DIR and DOCKET_BASH_PATH for consuming repos
#                             (re-run back-fills already-migrated clones)
# All are idempotent, so install.sh is safe to re-run any time (e.g. after adding a harness or
# editing ~/.config/docket/config.yml).
#
# NOT part of install: migrate-to-docket.sh — that migrates an existing repo to docket-mode and is
# run from INSIDE the repo you are migrating, not as machine setup.
#
# Test seam: DOCKET_HARNESS_ROOT is passed through to all four sub-scripts (overrides $HOME).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> ensure-global-config.sh (configure Bash runtime)"
bash "$SCRIPT_DIR/scripts/ensure-global-config.sh"

CONFIG_ROOT="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}"
DOCKET_BASH_PATH="$(awk '
  { line=$0; sub(/[[:space:]]*#.*/, "", line) }
  line ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
  in_runtime && line ~ /^[^[:space:]]/ { in_runtime=0 }
  in_runtime && line ~ /^[[:space:]]+bash[[:space:]]*:/ {
    sub(/^[[:space:]]+bash[[:space:]]*:[[:space:]]*/, "", line)
    sub(/[[:space:]]+$/, "", line)
    if (line ~ /^".*"$/ || line ~ /^'\''.*'\''$/) line=substr(line,2,length(line)-2)
    print line; exit
  }
' "$CONFIG_ROOT/docket/config.yml")"
export DOCKET_BASH_PATH

echo "==> link-skills.sh (install skills)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/link-skills.sh"

echo "==> sync-agents.sh (generate agent wrappers)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/sync-agents.sh"

echo "==> ensure-docket-env.sh (export Docket runtime environment)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/scripts/ensure-docket-env.sh"

echo "docket: install complete"

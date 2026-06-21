#!/usr/bin/env bash
# install.sh — set up docket on this machine, in one command.
#
# Runs the three install primitives in order:
#   1. link-skills.sh  — symlink the skills into each present harness's skill dir (live; edit-once)
#   2. sync-agents.sh  — generate the model/effort-pinned agent wrappers into each present harness
#                        (generated copies; re-run after editing a config layer)
#   3. ensure-docket-env.sh — export DOCKET_SCRIPTS_DIR so the skills can reach scripts/ from any
#                             consuming repo (re-run back-fills already-migrated clones)
# All are idempotent, so install.sh is safe to re-run any time (e.g. after adding a harness or
# editing ~/.config/docket/agents.yaml).
#
# NOT part of install: migrate-to-docket.sh — that migrates an existing repo to docket-mode and is
# run from INSIDE the repo you are migrating, not as machine setup.
#
# Test seam: DOCKET_HARNESS_ROOT is passed through to both sub-scripts (overrides $HOME).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> link-skills.sh (install skills)"
bash "$SCRIPT_DIR/link-skills.sh"

echo "==> sync-agents.sh (generate agent wrappers)"
bash "$SCRIPT_DIR/sync-agents.sh"

echo "==> ensure-docket-env.sh (export DOCKET_SCRIPTS_DIR)"
bash "$SCRIPT_DIR/scripts/ensure-docket-env.sh"

echo "docket: install complete"

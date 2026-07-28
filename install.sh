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

# change 0133: the shared runtime.bash reader. Sourced here rather than re-implemented so a fix to
# the scalar grammar reaches install, the installer, and the resolver together. Bootstrap-
# compatible by requirement — this runs under the system Bash, before DOCKET_BASH_PATH exists.
# shellcheck source=scripts/lib/docket-runtime.sh
. "$SCRIPT_DIR/scripts/lib/docket-runtime.sh"

echo "==> ensure-global-config.sh (configure Bash runtime)"
DOCKET_BOOTSTRAP_LAUNCH=1 bash "$SCRIPT_DIR/scripts/ensure-global-config.sh"

CONFIG_ROOT="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}"
# No markers and no duplicate handling: ensure-global-config.sh has just guaranteed exactly one
# authoritative declaration or exited non-zero, so this reads the value it settled on — managed or
# hand-authored alike.
DOCKET_BASH_PATH="$(docket_runtime_first "$CONFIG_ROOT/docket/config.yml")"
export DOCKET_BASH_PATH

echo "==> link-skills.sh (install skills)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/link-skills.sh"

echo "==> sync-agents.sh (generate agent wrappers)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/sync-agents.sh"

echo "==> ensure-docket-env.sh (export Docket runtime environment)"
"$DOCKET_BASH_PATH" "$SCRIPT_DIR/scripts/ensure-docket-env.sh"

echo "docket: install complete"

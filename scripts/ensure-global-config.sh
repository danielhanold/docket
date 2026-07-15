#!/usr/bin/env bash
# ensure-global-config.sh — scaffold the global docket config on first run.
#
# Copies the committed config.yml.example (repo root) to the user's global docket config
# at ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — but ONLY if that file does not
# already exist. Never overwrites, never merges, never edits an existing file. Idempotent:
# safe to re-run any number of times. Run by install.sh BEFORE sync-agents.sh so the first
# generator pass reads the just-written global config.
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for the config root (matching sync-agents.sh),
# and it is only consulted when XDG_CONFIG_HOME is unset (a set XDG_CONFIG_HOME wins).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$REPO_ROOT/config.yml.example"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
DEST_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
DEST="$DEST_DIR/config.yml"

if [ ! -f "$SRC" ]; then
  echo "docket: ensure-global-config: source $SRC not found — skipping" >&2
  exit 0
fi

if [ -e "$DEST" ]; then
  echo "docket: $DEST already exists — left untouched"
  exit 0
fi

mkdir -p "$DEST_DIR"
cp "$SRC" "$DEST"
echo "docket: wrote $DEST from config.yml.example (edit to enable harnesses / tune models)"
exit 0

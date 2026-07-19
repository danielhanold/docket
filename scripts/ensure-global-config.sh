#!/usr/bin/env bash
# ensure-global-config.sh — scaffold the global docket config on first run.
#
# Writes a MINIMAL, pointer-only ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — a header
# comment naming .docket.yml.example as the reference, and ZERO active keys — but ONLY if that
# file does not already exist. Never overwrites, never merges, never edits an existing file.
# Idempotent: safe to re-run any number of times. Run by install.sh BEFORE sync-agents.sh.
#
# Why pointer-only (change 0101): the previous version COPIED the repo's root-level example
# config file, so a user who installed once carried a frozen snapshot of that day's defaults
# forever — every later default change was silently pinned by their stale copy. A file with no
# active keys cannot pin anything.
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for the config root (matching sync-agents.sh),
# and it is only consulted when XDG_CONFIG_HOME is unset (a set XDG_CONFIG_HOME wins).
set -euo pipefail

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
DEST_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
DEST="$DEST_DIR/config.yml"

if [ -e "$DEST" ]; then
  echo "docket: $DEST already exists — left untouched"
  exit 0
fi

mkdir -p "$DEST_DIR"
cat > "$DEST" <<'EOF'
# ~/.config/docket/config.yml — docket's GLOBAL (per-machine, every-repo) configuration.
#
# This file is intentionally EMPTY: every key is unset, so docket runs its shipped defaults.
# Add only the keys you want to change on this machine.
#
# Configuration resolves PER KEY, precedence highest to lowest:
#   1. repo-local     <repo>/.docket.local.yml   (this machine, this repo; gitignored)
#   2. repo-committed <repo>/.docket.yml         (every clone)
#   3. global         this file                  (this machine, every repo)
#   4. built-in       docket's defaults
#
# FOR EVERY KEY, ITS DEFAULT, AND WHICH LAYERS MAY SET IT, SEE:
#   .docket.yml.example  in the docket repo — the canonical, all-comprehensive reference.
#
# Keys tagged "scope: repo-only (coordination-fenced, ADR-0019)" there are NOT settable here:
# a value for one of those in this file is loudly warned-and-ignored. Everything else is fair game.
#
# Common things to set on this machine:
#   agent_harnesses: [claude, cursor]   # enable another harness (then also set agents: below)
#   agents:                             # per-skill model/effort overrides
#   auto_capture: true                  # mint discovered follow-up work as stubs
#   reclaim: {auto: true}               # let expired claims self-heal
EOF
echo "docket: wrote $DEST (empty pointer config — see .docket.yml.example for every key)"
exit 0

#!/usr/bin/env bash
# scripts/ensure-docket-env.sh — make docket's helper scripts reachable from any consuming
# repo by exporting DOCKET_SCRIPTS_DIR (absolute path to THIS scripts/ dir) into the user's
# shell profile (primary, re-sourced on every Bash-tool call) and Claude Code's user-level
# settings.json env (reinforcement, read at session start). Idempotent + standalone:
# install.sh runs it; re-running back-fills already-migrated clones (change 0034).
#
# DOCKET_SCRIPTS_DIR points at the live docket clone the skills are symlinked from -> zero
# drift. Skills resolve every helper as "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh,
# so a missing/incomplete install fails loud at the first call instead of silently degrading.
#
# Usage: bash scripts/ensure-docket-env.sh
# Seams (tests): HOME (profile target), DOCKET_HARNESS_ROOT (settings.json root; default $HOME),
#   DOCKET_TARGET_SHELL (force the profile flavor; default = basename "$SHELL").
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"   # this dir IS the value we export
VALUE="$HERE"
NAME="DOCKET_SCRIPTS_DIR"
MARK_OPEN="# >>> docket (DOCKET_SCRIPTS_DIR) >>>"
MARK_CLOSE="# <<< docket (DOCKET_SCRIPTS_DIR) <<<"
say(){ printf 'ensure-docket-env: %s\n' "$*"; }
# Portably read a file's permission bits (octal); falls back to 644 when file is
# brand-new or stat is unavailable (macOS BSD stat vs GNU stat).
file_mode(){ stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1" 2>/dev/null || echo 644; }

# --- 1. shell-profile export (primary) ---------------------------------------
shell="${DOCKET_TARGET_SHELL:-$(basename "${SHELL:-sh}")}"
case "$shell" in
  zsh)  prof="$HOME/.zshenv";                  line="export $NAME=\"$VALUE\"" ;;
  bash) prof="$HOME/.bashrc";                  line="export $NAME=\"$VALUE\"" ;;
  fish) prof="$HOME/.config/fish/config.fish"; line="set -gx $NAME \"$VALUE\"" ;;
  *)    prof="$HOME/.profile";                 line="export $NAME=\"$VALUE\"" ;;   # POSIX fallback
esac
mkdir -p "$(dirname "$prof")"; touch "$prof"
# Idempotent marker block: strip any existing docket block, then append a fresh one
# (a moved clone updates the exported path instead of duplicating the block).
_prof_mode="$(file_mode "$prof")"
tmp="$(mktemp)"
awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
  $0==o {skip=1; next} $0==c {skip=0; next} !skip {print}
' "$prof" > "$tmp"
printf '%s\n%s\n%s\n' "$MARK_OPEN" "$line" "$MARK_CLOSE" >> "$tmp"
mv "$tmp" "$prof"
chmod "$_prof_mode" "$prof"
say "wrote $NAME -> $prof ($shell)"

# --- 2. Claude Code user-level settings.json env (reinforcement) --------------
HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
settings="$HARNESS_ROOT/.claude/settings.json"
if command -v jq >/dev/null 2>&1; then
  mkdir -p "$(dirname "$settings")"
  [ -f "$settings" ] || printf '{}\n' > "$settings"
  if jq empty "$settings" 2>/dev/null; then
    _settings_mode="$(file_mode "$settings")"
    t="$(mktemp)"
    if jq --arg v "$VALUE" '.env //= {} | .env.DOCKET_SCRIPTS_DIR = $v' "$settings" > "$t"; then
      mv "$t" "$settings"; chmod "$_settings_mode" "$settings"
      say "set env.$NAME -> ${settings#"$HARNESS_ROOT"/}"
    else rm -f "$t"; say "warning: could not update $settings"; fi
  else say "warning: $settings is not valid JSON — left unchanged"; fi
else
  say "warning: jq not found — wrote profile export only (settings.json env skipped)"
fi

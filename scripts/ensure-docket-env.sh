#!/usr/bin/env bash
# Persist Docket's clone scripts path and validated Bash runtime in shell/Claude environments.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SCRIPTS_VALUE="$HERE"
BASH_VALUE="${DOCKET_BASH_PATH:-}"
MARK_OPEN="# >>> docket (DOCKET_SCRIPTS_DIR) >>>"
MARK_CLOSE="# <<< docket (DOCKET_SCRIPTS_DIR) <<<"
say(){ printf 'ensure-docket-env: %s\n' "$*"; }
die(){ say "$*" >&2; exit 1; }
file_mode(){ stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1" 2>/dev/null || echo 644; }

case "$SCRIPTS_VALUE" in /*) ;; *) die "DOCKET_SCRIPTS_DIR must be absolute" ;; esac
case "$BASH_VALUE" in /*) ;; *) die "DOCKET_BASH_PATH must be an absolute path" ;; esac
[ -x "$BASH_VALUE" ] || die "DOCKET_BASH_PATH is not executable: $BASH_VALUE"
_version="$(LC_ALL=C "$BASH_VALUE" --version 2>/dev/null)" || die "DOCKET_BASH_PATH cannot report its version"
_first="${_version%%$'\n'*}"
case "$_first" in 'GNU bash, version '*) ;; *) die "DOCKET_BASH_PATH is not GNU Bash" ;; esac
_major="$(sed -nE 's/^GNU bash, version ([0-9]+)\..*/\1/p' <<<"$_first")"
[[ "$_major" =~ ^[0-9]+$ ]] && [ "$_major" -ge 4 ] || die "DOCKET_BASH_PATH must be Bash 4 or newer"

shell="${DOCKET_TARGET_SHELL:-$(basename "${SHELL:-sh}")}"
case "$shell" in
  zsh)  prof="$HOME/.zshenv";                  script_line="export DOCKET_SCRIPTS_DIR=\"$SCRIPTS_VALUE\""; bash_line="export DOCKET_BASH_PATH=\"$BASH_VALUE\"" ;;
  bash) prof="$HOME/.bashrc";                  script_line="export DOCKET_SCRIPTS_DIR=\"$SCRIPTS_VALUE\""; bash_line="export DOCKET_BASH_PATH=\"$BASH_VALUE\"" ;;
  fish) prof="$HOME/.config/fish/config.fish"; script_line="set -gx DOCKET_SCRIPTS_DIR \"$SCRIPTS_VALUE\""; bash_line="set -gx DOCKET_BASH_PATH \"$BASH_VALUE\"" ;;
  *)    prof="$HOME/.profile";                 script_line="export DOCKET_SCRIPTS_DIR=\"$SCRIPTS_VALUE\""; bash_line="export DOCKET_BASH_PATH=\"$BASH_VALUE\"" ;;
esac
mkdir -p "$(dirname "$prof")"; touch "$prof"

awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
  $0==o { if (inside || seen) bad=1; inside=1; seen=1; next }
  $0==c { if (!inside) bad=1; inside=0; next }
  END { if (inside || bad) exit 1 }
' "$prof" || die "$prof has malformed managed-block markers — left unchanged"

_prof_mode="$(file_mode "$prof")"
tmp="$(mktemp "$(dirname "$prof")/.docket-env.tmp.XXXXXX")" || die "cannot create profile temporary file"
trap 'rm -f "$tmp"' EXIT HUP INT TERM
awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
  $0==o {skip=1; next} $0==c {skip=0; next} !skip {print}
' "$prof" > "$tmp" || die "cannot preserve $prof"
printf '%s\n%s\n%s\n%s\n' "$MARK_OPEN" "$script_line" "$bash_line" "$MARK_CLOSE" >> "$tmp" || die "cannot write profile block"
chmod "$_prof_mode" "$tmp" || die "cannot preserve profile permissions"
mv "$tmp" "$prof" || die "cannot atomically replace $prof"
trap - EXIT HUP INT TERM
say "wrote DOCKET_SCRIPTS_DIR and DOCKET_BASH_PATH -> $prof ($shell)"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
settings="$HARNESS_ROOT/.claude/settings.json"
if command -v jq >/dev/null 2>&1; then
  mkdir -p "$(dirname "$settings")"
  _settings_source="$settings"
  _seed=
  if [ ! -f "$settings" ]; then
    _settings_mode=644
    _seed="$(mktemp "$(dirname "$settings")/.settings-seed.tmp.XXXXXX")" || die "cannot create settings seed"
    printf '{}\n' > "$_seed" || die "cannot write settings seed"
    _settings_source="$_seed"
  else
    _settings_mode="$(file_mode "$settings")"
  fi
  if jq empty "$_settings_source" 2>/dev/null; then
    t="$(mktemp "$(dirname "$settings")/.settings.json.tmp.XXXXXX")" || die "cannot create settings temporary file"
    if jq --arg scripts "$SCRIPTS_VALUE" --arg bash "$BASH_VALUE" \
      '.env //= {} | .env.DOCKET_SCRIPTS_DIR = $scripts | .env.DOCKET_BASH_PATH = $bash' "$_settings_source" > "$t"; then
      chmod "$_settings_mode" "$t" && mv "$t" "$settings"
      say "set env.DOCKET_SCRIPTS_DIR and env.DOCKET_BASH_PATH -> ${settings#"$HARNESS_ROOT"/}"
    else rm -f "$t"; say "warning: could not update $settings"; fi
  else say "warning: $settings is not valid JSON — left unchanged"; fi
  [ -z "$_seed" ] || rm -f "$_seed"
else
  say "warning: jq not found — wrote profile exports only (settings.json env skipped)"
fi

#!/usr/bin/env bash
# ensure-global-config.sh — scaffold global config and install its machine-local Bash runtime.
# This bootstrap deliberately stays within Bash 3.2/POSIX syntax: install.sh runs it before any
# Docket script that may require the configured Bash 4+ runtime.
set -eu

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
DEST_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
DEST="$DEST_DIR/config.yml"
MARK_OPEN='# >>> docket (runtime.bash) >>>'
MARK_CLOSE='# <<< docket (runtime.bash) <<<'
REMEDY='install Bash 4+ (on macOS: brew install bash), then re-run docket/install.sh'

die(){ printf 'ensure-global-config: %s\n' "$*" >&2; exit 1; }
file_mode(){ stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1" 2>/dev/null || echo 644; }

validate_runtime(){
  _vr_path=$1
  case "$_vr_path" in /*) ;; *) return 1 ;; esac
  [ -x "$_vr_path" ] || return 1
  _vr_version="$(LC_ALL=C "$_vr_path" --version 2>/dev/null)" || return 1
  _vr_first="$(printf '%s\n' "$_vr_version" | sed -n '1p')"
  case "$_vr_first" in 'GNU bash, version '*) ;; *) return 1 ;; esac
  _vr_major="$(printf '%s\n' "$_vr_first" | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  case "$_vr_major" in ''|*[!0-9]*) return 1 ;; esac
  [ "$_vr_major" -ge 4 ]
}

validate_serializable_path(){
  # Paths are one-line YAML scalars. Apostrophes are doubled by write_runtime_block and
  # backslashes are literal in YAML single quotes; only record separators are unrepresentable.
  case "$1" in *$'\n'*|*$'\r'*) return 1 ;; esac
  return 0
}

markers_valid(){
  [ -f "$1" ] || return 0
  awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
    $0==o { if (inside || seen) bad=1; inside=1; seen=1; next }
    $0==c { if (!inside) bad=1; inside=0; next }
    END { if (inside || bad) exit 1 }
  ' "$1"
}

explicit_runtime(){
  [ -f "$1" ] || return 0
  awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
    function scalar(value, sq,out,i,ch,rest) {
      sq=sprintf("%c", 39)
      if (substr(value,1,1) == sq) {
        out=""
        for (i=2; i<=length(value); i++) {
          ch=substr(value,i,1)
          if (ch == sq) {
            if (substr(value,i+1,1) == sq) { out=out sq; i++; continue }
            rest=substr(value,i+1)
            if (rest ~ /^[[:space:]]*(#.*)?$/) return out
            return value
          }
          out=out ch
        }
        return value
      }
      if (value ~ /^"[^"]*"[[:space:]]*(#.*)?$/) {
        sub(/^"/, "", value); sub(/"[[:space:]]*(#.*)?$/, "", value)
      } else {
        sub(/[[:space:]]*#.*/, "", value); sub(/[[:space:]]+$/, "", value)
      }
      return value
    }
    $0==o { managed=1; next }
    $0==c { managed=0; next }
    managed { next }
    { raw=$0; structural=$0; sub(/[[:space:]]*#.*/, "", structural) }
    structural ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && structural ~ /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/ {
      value=raw; sub(/^[[:space:]]+bash[[:space:]]*:[[:space:]]*/, "", value)
      print scalar(value); exit
    }
  ' "$1"
}


explicit_runtime_count(){
  [ -f "$1" ] || { printf '0\n'; return; }
  awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
    $0==o { managed=1; next }
    $0==c { managed=0; next }
    managed { next }
    { line=$0; sub(/[[:space:]]*#.*/, "", line) }
    line ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && line ~ /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && line ~ /^[[:space:]]+bash[[:space:]]*:/ { count++ }
    END { print count+0 }
  ' "$1"
}

consider_candidate(){
  _cc_path=$1
  [ -n "$_cc_path" ] || return 1
  case "$_cc_path" in /*) ;; *) return 1 ;; esac
  case "${_seen_candidates-}" in *"|$_cc_path|"*) return 1 ;; esac
  _seen_candidates="${_seen_candidates-}|$_cc_path|"
  if validate_runtime "$_cc_path"; then
    DISCOVERED_RUNTIME=$_cc_path
    return 0
  fi
  return 1
}

discover_runtime(){
  DISCOVERED_RUNTIME=
  _seen_candidates='|'
  _brew_prefix=
  if command -v brew >/dev/null 2>&1; then
    _brew_prefix="$(brew --prefix 2>/dev/null)" || _brew_prefix=
  fi
  [ -z "$_brew_prefix" ] || consider_candidate "$_brew_prefix/bin/bash" || :

  _standard_root=${DOCKET_BASH_STANDARD_ROOT-}
  [ -n "$DISCOVERED_RUNTIME" ] || consider_candidate "$_standard_root/opt/homebrew/bin/bash" || :
  [ -n "$DISCOVERED_RUNTIME" ] || consider_candidate "$_standard_root/usr/local/bin/bash" || :

  if [ -z "$DISCOVERED_RUNTIME" ]; then
    _path_bash="$(command -v bash 2>/dev/null)" || _path_bash=
    consider_candidate "$_path_bash" || :
  fi
  [ -n "$DISCOVERED_RUNTIME" ] || die "no qualifying Bash runtime found — $REMEDY"
}

write_pointer(){
  cat <<'EOF'
# ~/.config/docket/config.yml — docket's GLOBAL (per-machine, every-repo) configuration.
#
# This file starts with no policy overrides; add only keys you want to change on this machine.
# Configuration resolves per key: repo-local, repo-committed, global, then built-in.
# See .docket.example.yml in the docket repo for every key, default, and allowed layer.
EOF
}

write_runtime_block(){
  validate_serializable_path "$DISCOVERED_RUNTIME" \
    || die "runtime.bash path contains unsupported line-break characters"
  _wr_yaml="$(printf '%s' "$DISCOVERED_RUNTIME" | sed "s/'/''/g")"
  printf "%s\nruntime:\n  bash: '%s'\n%s\n" "$MARK_OPEN" "$_wr_yaml" "$MARK_CLOSE"
}

strip_runtime_block(){
  _sr_inside=0
  while :; do
    _sr_line=
    if IFS= read -r _sr_line; then _sr_rc=0; else _sr_rc=$?; fi
    [ "$_sr_rc" -eq 0 ] || [ -n "$_sr_line" ] || break
    if [ "$_sr_line" = "$MARK_OPEN" ]; then
      _sr_inside=1
    elif [ "$_sr_line" = "$MARK_CLOSE" ]; then
      _sr_inside=0
    elif [ "$_sr_inside" -eq 0 ]; then
      printf '%s' "$_sr_line"
      [ "$_sr_rc" -ne 0 ] || printf '\n'
    fi
    [ "$_sr_rc" -eq 0 ] || break
  done < "$1"
}

markers_valid "$DEST" || die "$DEST has malformed $MARK_OPEN / $MARK_CLOSE markers — left unchanged"
_explicit="$(explicit_runtime "$DEST")"
_explicit_count="$(explicit_runtime_count "$DEST")"
[ "$_explicit_count" -le 1 ] \
  || die "$DEST contains multiple explicit runtime.bash declarations; keep exactly one — left unchanged"
if [ "$_explicit_count" -eq 1 ] && grep -qF -- "$MARK_OPEN" "$DEST"; then
  die "$DEST contains both managed and explicit runtime.bash declarations; remove one so exactly one runtime is authoritative — left unchanged"
fi
if [ "$_explicit_count" -eq 1 ] && [ -z "$_explicit" ]; then
  die "$DEST contains an empty explicit runtime.bash; set it to an absolute executable GNU Bash 4+ path or remove the declaration — left unchanged"
fi
if [ -n "$_explicit" ]; then
  validate_serializable_path "$_explicit" \
    || die "configured runtime.bash contains unsupported line-break characters — $REMEDY"
  validate_runtime "$_explicit" \
    || die "configured runtime.bash is not an absolute executable GNU Bash 4+: $_explicit — $REMEDY"
  printf 'docket: %s already has a valid explicit runtime.bash — left untouched\n' "$DEST"
  exit 0
fi

discover_runtime
mkdir -p "$DEST_DIR"
_tmp="$(mktemp "$DEST_DIR/.config.yml.tmp.XXXXXX")" || die "cannot create temporary config beside $DEST"
trap 'rm -f "$_tmp"' EXIT HUP INT TERM
if [ -f "$DEST" ]; then
  _dest_mode="$(file_mode "$DEST")"
  # Keep the owned block first. The remainder retains whether its final record had a newline, so
  # all user-owned bytes survive both first install and every re-run.
  write_runtime_block > "$_tmp" || die "cannot write runtime.bash to temporary config"
  if grep -qF -- "$MARK_OPEN" "$DEST"; then
    strip_runtime_block "$DEST" >> "$_tmp" || die "cannot preserve $DEST"
  else
    cat "$DEST" >> "$_tmp" || die "cannot preserve $DEST"
  fi
  printf 'docket: updating managed runtime.bash in %s\n' "$DEST"
else
  _dest_mode=644
  write_runtime_block > "$_tmp" || die "cannot write runtime.bash to temporary config"
  write_pointer >> "$_tmp" || die "cannot scaffold $DEST"
  printf 'docket: wrote %s (pointer config plus managed runtime.bash)\n' "$DEST"
fi
chmod "$_dest_mode" "$_tmp" || die "cannot preserve config permissions"
mv "$_tmp" "$DEST" || die "cannot atomically replace $DEST"
trap - EXIT HUP INT TERM
exit 0

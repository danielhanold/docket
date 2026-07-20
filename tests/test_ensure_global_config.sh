#!/usr/bin/env bash
# tests/test_ensure_global_config.sh — run: bash tests/test_ensure_global_config.sh
# Unit-tests the ensure-global-config.sh primitive: fresh (writes a pointer-only file with ZERO
# active keys, naming .docket.example.yml as the reference + logs "wrote"), existing (untouched +
# logs "left untouched"), idempotent, exit 0 both, XDG_CONFIG_HOME honored.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-global-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

# Fresh: empty sandbox home, no existing config.
SB="$(mktemp -d)"; _tmpdirs+=("$SB")
DEST="$SB/.config/docket/config.yml"
out="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc=$?
assert "fresh: exits 0" '[ "$rc" = "0" ]'
assert "fresh: creates the global config" '[ -f "$DEST" ]'
# Pointer-only: every non-blank line is a comment — zero active keys, so it can never pin a
# shipped default (change 0101).
assert "fresh: contains NO active keys (comment/blank lines only)" \
  '[ -z "$(grep -vE "^[[:space:]]*(#.*)?$" "$DEST" 2>/dev/null)" ]'
assert "fresh: points at .docket.example.yml" 'grep -qF ".docket.example.yml" "$DEST"'
assert "fresh: names the layer precedence" 'grep -qiE "repo-local|precedence" "$DEST"'
assert "fresh: logs a wrote line naming the dest" 'printf "%s" "$out" | grep -qF "wrote $DEST"'

# Existing: pre-seed a distinct sentinel; the script must NOT touch it.
SB2="$(mktemp -d)"; _tmpdirs+=("$SB2")
DEST2="$SB2/.config/docket/config.yml"
mkdir -p "$(dirname "$DEST2")"; printf 'sentinel: do-not-overwrite\n' > "$DEST2"
out2="$(HOME="$SB2" DOCKET_HARNESS_ROOT="$SB2" bash "$SCRIPT" 2>&1)"; rc2=$?
assert "existing: exits 0" '[ "$rc2" = "0" ]'
assert "existing: file is left untouched" '[ "$(cat "$DEST2")" = "sentinel: do-not-overwrite" ]'
assert "existing: logs a left-untouched line" 'printf "%s" "$out2" | grep -qF "left untouched"'

# Idempotent: a second fresh-run over the just-written file leaves it untouched (now existing).
before="$(cat "$DEST")"
out3="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc3=$?
assert "idempotent: second run exits 0" '[ "$rc3" = "0" ]'
assert "idempotent: second run reports left untouched" 'printf "%s" "$out3" | grep -qF "left untouched"'
assert "idempotent: second run left the file byte-untouched" '[ "$(cat "$DEST")" = "$before" ]'

# XDG_CONFIG_HOME wins when set.
SB3="$(mktemp -d)"; _tmpdirs+=("$SB3"); XDGDIR="$(mktemp -d)"; _tmpdirs+=("$XDGDIR")
out4="$(HOME="$SB3" DOCKET_HARNESS_ROOT="$SB3" XDG_CONFIG_HOME="$XDGDIR" bash "$SCRIPT" 2>&1)"; rc4=$?
assert "xdg: honors XDG_CONFIG_HOME" '[ -f "$XDGDIR/docket/config.yml" ] && [ "$rc4" = "0" ]'

# No source file is consulted or required (change 0101 deletes config.yml.example).
assert "script does not reference config.yml.example" \
  '! grep -qF "config.yml.example" "$SCRIPT"'

exit $fail

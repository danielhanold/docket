#!/usr/bin/env bash
# tests/test_ensure_global_config.sh — run: bash tests/test_ensure_global_config.sh
# Unit-tests the ensure-global-config.sh primitive: fresh pointer plus managed runtime, preservation
# of unrelated existing content, idempotency, and XDG_CONFIG_HOME handling.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-global-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

# Hermetic qualifying fixed-location candidate; never inspect the developer's Homebrew/PATH Bash.
RUNTIME_ROOT="$(mktemp -d)"; _tmpdirs+=("$RUNTIME_ROOT"); TEST_BIN="$(mktemp -d)"; _tmpdirs+=("$TEST_BIN")
mkdir -p "$RUNTIME_ROOT/opt/homebrew/bin"
cat > "$RUNTIME_ROOT/opt/homebrew/bin/bash" <<'EOF'
#!/bin/sh
[ "$#" -eq 1 ] && [ "$1" = --version ] || exit 42
printf 'GNU bash, version 5.2.0(1)-release (test)\n'
EOF
chmod +x "$RUNTIME_ROOT/opt/homebrew/bin/bash"
printf '#!/bin/sh\nexit 1\n' > "$TEST_BIN/brew"; chmod +x "$TEST_BIN/brew"
export DOCKET_BASH_STANDARD_ROOT="$RUNTIME_ROOT"
export PATH="$TEST_BIN:$PATH"

# Fresh: empty sandbox home, no existing config.
SB="$(mktemp -d)"; _tmpdirs+=("$SB")
DEST="$SB/.config/docket/config.yml"
out="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc=$?
assert "fresh: exits 0" '[ "$rc" = "0" ]'
assert "fresh: creates the global config" '[ -f "$DEST" ]'
# Only the machine-local runtime is active; the scaffold still pins no policy defaults.
assert "fresh: contains the managed runtime.bash and no policy keys" \
  'grep -qF "# >>> docket (runtime.bash) >>>" "$DEST" && grep -qF "  bash: '\''$RUNTIME_ROOT/opt/homebrew/bin/bash'\''" "$DEST" && ! grep -qF "agent_harnesses:" "$DEST"'
assert "fresh: points at .docket.example.yml" 'grep -qF ".docket.example.yml" "$DEST"'
assert "fresh: names the layer precedence" 'grep -qiE "repo-local|precedence" "$DEST"'
assert "fresh: logs a wrote line naming the dest" 'printf "%s" "$out" | grep -qF "wrote $DEST"'

# Existing without a runtime: preserve the sentinel and append only the managed runtime block.
SB2="$(mktemp -d)"; _tmpdirs+=("$SB2")
DEST2="$SB2/.config/docket/config.yml"
mkdir -p "$(dirname "$DEST2")"; printf 'sentinel: do-not-overwrite\n' > "$DEST2"
out2="$(HOME="$SB2" DOCKET_HARNESS_ROOT="$SB2" bash "$SCRIPT" 2>&1)"; rc2=$?
assert "existing: exits 0" '[ "$rc2" = "0" ]'
assert "existing: unrelated content is preserved" 'grep -qxF "sentinel: do-not-overwrite" "$DEST2"'
assert "existing: receives the managed runtime" 'grep -qF "  bash: '\''$RUNTIME_ROOT/opt/homebrew/bin/bash'\''" "$DEST2"'
assert "existing: logs a managed-runtime update" 'printf "%s" "$out2" | grep -qF "updating managed runtime.bash"'

# Idempotent: a second fresh-run over the just-written file leaves it untouched (now existing).
before="$(cat "$DEST")"
out3="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc3=$?
assert "idempotent: second run exits 0" '[ "$rc3" = "0" ]'
assert "idempotent: second run reports a managed-runtime update" 'printf "%s" "$out3" | grep -qF "updating managed runtime.bash"'
assert "idempotent: second run left the file byte-untouched" '[ "$(cat "$DEST")" = "$before" ]'

# XDG_CONFIG_HOME wins when set.
SB3="$(mktemp -d)"; _tmpdirs+=("$SB3"); XDGDIR="$(mktemp -d)"; _tmpdirs+=("$XDGDIR")
out4="$(HOME="$SB3" DOCKET_HARNESS_ROOT="$SB3" XDG_CONFIG_HOME="$XDGDIR" bash "$SCRIPT" 2>&1)"; rc4=$?
assert "xdg: honors XDG_CONFIG_HOME" '[ -f "$XDGDIR/docket/config.yml" ] && [ "$rc4" = "0" ]'

# No source file is consulted or required (change 0101 deletes config.yml.example).
assert "script does not reference config.yml.example" \
  '! grep -qF "config.yml.example" "$SCRIPT"'

# A present explicit declaration is authoritative even when its scalar is empty. Empty must not
# collapse into "absent" and trigger discovery/prepending; fail with a remediation and preserve
# every existing byte for all supported empty spellings.
for empty_case in bare quoted comment; do
  EMPTY_SB="$(mktemp -d)"; _tmpdirs+=("$EMPTY_SB")
  EMPTY_DEST="$EMPTY_SB/.config/docket/config.yml"
  mkdir -p "$(dirname "$EMPTY_DEST")"
  case "$empty_case" in
    bare)    printf 'before: keep\nruntime:\n  bash:\nafter: keep\n' > "$EMPTY_DEST" ;;
    quoted)  printf "before: keep\nruntime:\n  bash: ''\nafter: keep\n" > "$EMPTY_DEST" ;;
    comment) printf 'before: keep\nruntime:\n  bash: # choose this machine runtime\nafter: keep\n' > "$EMPTY_DEST" ;;
  esac
  cp "$EMPTY_DEST" "$EMPTY_DEST.before"
  empty_out="$(HOME="$EMPTY_SB" DOCKET_HARNESS_ROOT="$EMPTY_SB" bash "$SCRIPT" 2>&1)"; empty_rc=$?
  assert "empty explicit $empty_case: exits non-zero" '[ "$empty_rc" -ne 0 ]'
  assert "empty explicit $empty_case: config remains byte-identical" \
    'cmp -s "$EMPTY_DEST.before" "$EMPTY_DEST"'
  assert "empty explicit $empty_case: diagnostic is actionable" \
    'grep -qi "empty" <<<"$empty_out" && grep -Eqi "set|remove" <<<"$empty_out"'
done

# The managed YAML scalar must round-trip real executable paths containing both an apostrophe and
# a backslash. YAML single quotes double apostrophes and keep backslashes literal.
ODD_ROOT="$(mktemp -d)/runtime'quote\\slash"; _tmpdirs+=("${ODD_ROOT%%/runtime*}")
mkdir -p "$ODD_ROOT/opt/homebrew/bin"
cp "$RUNTIME_ROOT/opt/homebrew/bin/bash" "$ODD_ROOT/opt/homebrew/bin/bash"
chmod +x "$ODD_ROOT/opt/homebrew/bin/bash"
ODD_SB="$(mktemp -d)"; _tmpdirs+=("$ODD_SB")
odd_out="$(HOME="$ODD_SB" DOCKET_HARNESS_ROOT="$ODD_SB" DOCKET_BASH_STANDARD_ROOT="$ODD_ROOT" bash "$SCRIPT" 2>&1)"; odd_rc=$?
ODD_DEST="$ODD_SB/.config/docket/config.yml"
ODD_EXPECTED="${ODD_ROOT//\'/\'\'}/opt/homebrew/bin/bash"
assert "odd runtime path: bootstrap succeeds" '[ "$odd_rc" -eq 0 ]'
assert "odd runtime path: managed YAML doubles apostrophe and preserves backslash" \
  'grep -qxF "  bash: '\''$ODD_EXPECTED'\''" "$ODD_DEST"'
odd_before="$(cat "$ODD_DEST")"
odd_rerun_out="$(HOME="$ODD_SB" DOCKET_HARNESS_ROOT="$ODD_SB" DOCKET_BASH_STANDARD_ROOT="$ODD_ROOT" bash "$SCRIPT" 2>&1)"; odd_rerun_rc=$?
assert "odd runtime path: generated YAML parses on re-run" \
  '[ "$odd_rerun_rc" -eq 0 ] && [ "$(cat "$ODD_DEST")" = "$odd_before" ]'

# Newlines remain outside every supported scalar/profile grammar even when the underlying file is
# a real qualifying executable; reject before creating a destination.
NEWLINE_ROOT="$(mktemp -d)/runtime
newline"; _tmpdirs+=("${NEWLINE_ROOT%%/runtime*}")
mkdir -p "$NEWLINE_ROOT/opt/homebrew/bin"
cp "$RUNTIME_ROOT/opt/homebrew/bin/bash" "$NEWLINE_ROOT/opt/homebrew/bin/bash"
chmod +x "$NEWLINE_ROOT/opt/homebrew/bin/bash"
NEWLINE_SB="$(mktemp -d)"; _tmpdirs+=("$NEWLINE_SB")
newline_out="$(HOME="$NEWLINE_SB" DOCKET_HARNESS_ROOT="$NEWLINE_SB" DOCKET_BASH_STANDARD_ROOT="$NEWLINE_ROOT" bash "$SCRIPT" 2>&1)"; newline_rc=$?
assert "newline runtime path: bootstrap rejects real executable" \
  '[ "$newline_rc" -ne 0 ] && [ ! -e "$NEWLINE_SB/.config/docket/config.yml" ]'
assert "newline runtime path: diagnostic names line-break restriction" \
  'grep -qi "line-break" <<<"$newline_out"'

exit $fail

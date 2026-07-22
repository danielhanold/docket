#!/usr/bin/env bash
# tests/test_ensure_docket_env.sh — run: bash tests/test_ensure_docket_env.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-docket-env.sh"
EXPECTED="$REPO/scripts"               # the script exports its own dir
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Accumulate all sandbox dirs; clean them up on exit.
_tmpdirs=()
trap 'rm -rf "${_tmpdirs[@]}"' EXIT
RUNTIME_DIR="$(mktemp -d)"; _tmpdirs+=("$RUNTIME_DIR"); RUNTIME="$RUNTIME_DIR/bash"
cat > "$RUNTIME" <<'EOF'
#!/bin/sh
[ "$#" -eq 1 ] && [ "$1" = --version ] || exit 42
printf 'GNU bash, version 5.2.0(1)-release (test)\n'
EOF
chmod +x "$RUNTIME"

# Each case runs in a sandbox HOME so the real profile is never touched.
run(){ # run <target_shell>  -> sets $H to the sandbox home
  H="$(mktemp -d)"; _tmpdirs+=("$H")
  HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL="$1" DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
}

# zsh -> ~/.zshenv export
run zsh
assert "zsh: writes ~/.zshenv"            '[ -f "$H/.zshenv" ]'
assert "zsh: export line present"         'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "zsh: runtime export present"      'grep -qF "export DOCKET_BASH_PATH=\"$RUNTIME\"" "$H/.zshenv"'
assert "zsh: marker block present"        'grep -qF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv"'

# bash -> ~/.bashrc export
run bash
assert "bash: writes ~/.bashrc export"    'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.bashrc"'
assert "bash: writes runtime export"      'grep -qF "export DOCKET_BASH_PATH=\"$RUNTIME\"" "$H/.bashrc"'

# fish -> ~/.config/fish/config.fish set -gx
run fish
assert "fish: writes config.fish set -gx" 'grep -qF "set -gx DOCKET_SCRIPTS_DIR \"$EXPECTED\"" "$H/.config/fish/config.fish"'
assert "fish: writes runtime set -gx"     'grep -qF "set -gx DOCKET_BASH_PATH \"$RUNTIME\"" "$H/.config/fish/config.fish"'

# unknown shell -> ~/.profile POSIX export fallback
run ksh
assert "other: POSIX export to ~/.profile" 'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.profile"'
assert "other: runtime export to ~/.profile" 'grep -qF "export DOCKET_BASH_PATH=\"$RUNTIME\"" "$H/.profile"'

# Paths are data, never shell text. Exercise both persisted values with command substitution,
# semicolon, hash, colon-space, and whitespace, then source the POSIX profile to prove it neither
# executes the payload nor changes either value.
META_BASE="$(mktemp -d)/clone \$(touch injected); # colon: value"
mkdir -p "$META_BASE/scripts"; cp "$SCRIPT" "$META_BASE/scripts/ensure-docket-env.sh"
META_SCRIPTS_RESOLVED="$(cd "$META_BASE/scripts" && pwd -P)"
META_RUNTIME_DIR="$(mktemp -d)/runtime \$(touch injected-runtime); # colon: value"
mkdir -p "$META_RUNTIME_DIR"; META_RUNTIME="$META_RUNTIME_DIR/bash"
cp "$RUNTIME" "$META_RUNTIME"; chmod +x "$META_RUNTIME"
H="$(mktemp -d)"; _tmpdirs+=("$H" "${META_BASE%%/clone *}" "${META_RUNTIME_DIR%%/runtime *}")
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=ksh DOCKET_BASH_PATH="$META_RUNTIME" \
  bash "$META_BASE/scripts/ensure-docket-env.sh" >/dev/null 2>&1; meta_rc=$?
assert "serialization: metacharacter clone/runtime values persist" '[ "$meta_rc" -eq 0 ]'
( cd "$H" && unset DOCKET_SCRIPTS_DIR DOCKET_BASH_PATH && . "$H/.profile" && \
  printf '%s\n%s\n' "$DOCKET_SCRIPTS_DIR" "$DOCKET_BASH_PATH" > "$H/loaded" )
assert "serialization: POSIX profile reloads both values literally" \
  '[ "$(sed -n "1p" "$H/loaded")" = "$META_SCRIPTS_RESOLVED" ] && [ "$(sed -n "2p" "$H/loaded")" = "$META_RUNTIME" ]'
assert "serialization: profile evaluation executes no embedded command" \
  '[ ! -e "$H/injected" ] && [ ! -e "$H/injected-runtime" ]'
assert "serialization: fish bindings use literal single-quoted values" \
  'HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=fish DOCKET_BASH_PATH="$META_RUNTIME" bash "$META_BASE/scripts/ensure-docket-env.sh" >/dev/null 2>&1 && grep -qF "set -gx DOCKET_SCRIPTS_DIR '\''$META_SCRIPTS_RESOLVED'\''" "$H/.config/fish/config.fish" && grep -qF "set -gx DOCKET_BASH_PATH '\''$META_RUNTIME'\''" "$H/.config/fish/config.fish"'

# settings.json env (jq), preserving an existing key
H="$(mktemp -d)"; _tmpdirs+=("$H"); mkdir -p "$H/.claude"
printf '{"permissions":{"allow":["keep"]}}\n' > "$H/.claude/settings.json"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
assert "settings: env.DOCKET_SCRIPTS_DIR set" 'jq -e --arg v "$EXPECTED" ".env.DOCKET_SCRIPTS_DIR == \$v" "$H/.claude/settings.json" >/dev/null'
assert "settings: env.DOCKET_BASH_PATH set" 'jq -e --arg v "$RUNTIME" ".env.DOCKET_BASH_PATH == \$v" "$H/.claude/settings.json" >/dev/null'
assert "settings: pre-existing key preserved" 'jq -e ".permissions.allow | index(\"keep\")" "$H/.claude/settings.json" >/dev/null'
assert "settings: still valid JSON"           'jq empty "$H/.claude/settings.json"'

# invalid settings.json is left untouched (refuse to clobber)
H="$(mktemp -d)"; _tmpdirs+=("$H"); mkdir -p "$H/.claude"
printf 'not valid json\n' > "$H/.claude/settings.json"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
assert "invalid settings.json left unchanged" 'grep -qF "not valid json" "$H/.claude/settings.json"'

# idempotent: a second run leaves exactly one marker block + unchanged settings
H="$(mktemp -d)"; _tmpdirs+=("$H")
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
assert "idempotent: exactly one marker block" '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

# stale block (clone moved) is replaced, not duplicated
H="$(mktemp -d)"; _tmpdirs+=("$H")
printf '# >>> docket (DOCKET_SCRIPTS_DIR) >>>\nexport DOCKET_SCRIPTS_DIR="/old/path/scripts"\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n' > "$H/.zshenv"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1
assert "stale path replaced"               'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "stale path: old value gone"        '! grep -qF "/old/path/scripts" "$H/.zshenv"'
assert "stale path: still one block"       '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

# Invalid runtime input fails before either destination is touched.
H="$(mktemp -d)"; _tmpdirs+=("$H"); printf '# keep\n' > "$H/.zshenv"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH=relative bash "$SCRIPT" >/dev/null 2>&1; invalid_rc=$?
assert "invalid runtime: exits non-zero" '[ "$invalid_rc" -ne 0 ]'
assert "invalid runtime: profile left unchanged" '[ "$(cat "$H/.zshenv")" = "# keep" ]'

# Marker corruption is rejected byte-safely instead of consuming the profile tail.
H="$(mktemp -d)"; _tmpdirs+=("$H")
printf '# >>> docket (DOCKET_SCRIPTS_DIR) >>>\nold\n# trailing user data\n' > "$H/.zshenv"; cp "$H/.zshenv" "$H/before"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1; marker_rc=$?
assert "markers: dangling profile block is rejected" '[ "$marker_rc" -ne 0 ] && cmp -s "$H/before" "$H/.zshenv"'

H="$(mktemp -d)"; _tmpdirs+=("$H")
printf '# >>> docket (DOCKET_SCRIPTS_DIR) >>>\none\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n# >>> docket (DOCKET_SCRIPTS_DIR) >>>\ntwo\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n' > "$H/.zshenv"; cp "$H/.zshenv" "$H/before"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1; marker_rc=$?
assert "markers: duplicate profile blocks are rejected byte-safely" \
  '[ "$marker_rc" -ne 0 ] && cmp -s "$H/before" "$H/.zshenv"'

H="$(mktemp -d)"; _tmpdirs+=("$H")
printf '# <<< docket (DOCKET_SCRIPTS_DIR) <<<\nkeep\n# >>> docket (DOCKET_SCRIPTS_DIR) >>>\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n' > "$H/.zshenv"; cp "$H/.zshenv" "$H/before"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" bash "$SCRIPT" >/dev/null 2>&1; marker_rc=$?
assert "markers: profile close-before-open is rejected byte-safely" \
  '[ "$marker_rc" -ne 0 ] && cmp -s "$H/before" "$H/.zshenv"'

# Persistence failures in Claude settings are hard failures and never print the success line.
REAL_MV="$(command -v mv)"; REAL_CHMOD="$(command -v chmod)"
for fail_command in mv chmod; do
  H="$(mktemp -d)"; _tmpdirs+=("$H"); FAIL_BIN="$(mktemp -d)"; _tmpdirs+=("$FAIL_BIN")
  mkdir -p "$H/.claude"; printf '{"keep":true}\n' > "$H/.claude/settings.json"; cp "$H/.claude/settings.json" "$H/settings.before"
  real_command="$REAL_MV"; [ "$fail_command" = chmod ] && real_command="$REAL_CHMOD"
  cat > "$FAIL_BIN/$fail_command" <<EOF
#!/bin/sh
case "\$*" in *'.settings.json.tmp.'*) exit 73 ;; esac
exec "$real_command" "\$@"
EOF
  chmod +x "$FAIL_BIN/$fail_command"
  failure_out="$(HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$RUNTIME" PATH="$FAIL_BIN:$PATH" bash "$SCRIPT" 2>&1)"; failure_rc=$?
  assert "settings $fail_command failure: script exits non-zero" '[ "$failure_rc" -ne 0 ]'
  assert "settings $fail_command failure: destination remains byte-identical" 'cmp -s "$H/settings.before" "$H/.claude/settings.json"'
  assert "settings $fail_command failure: no false success line" '! grep -qF "set env.DOCKET_SCRIPTS_DIR and env.DOCKET_BASH_PATH" <<<"$failure_out"'
done

# migrate-to-docket.sh points the user at install.sh for script reachability (DOCKET_SCRIPTS_DIR)
MIG="$REPO/migrate-to-docket.sh"
assert "migrate next-steps names DOCKET_SCRIPTS_DIR"  'grep -qF "DOCKET_SCRIPTS_DIR" "$MIG"'
assert "migrate next-steps points at install.sh"      'grep -qE "install\.sh" "$MIG"'

exit $fail

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

# Each case runs in a sandbox HOME so the real profile is never touched.
run(){ # run <target_shell>  -> sets $H to the sandbox home
  H="$(mktemp -d)"; _tmpdirs+=("$H")
  HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL="$1" bash "$SCRIPT" >/dev/null 2>&1
}

# zsh -> ~/.zshenv export
run zsh
assert "zsh: writes ~/.zshenv"            '[ -f "$H/.zshenv" ]'
assert "zsh: export line present"         'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "zsh: marker block present"        'grep -qF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv"'

# bash -> ~/.bashrc export
run bash
assert "bash: writes ~/.bashrc export"    'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.bashrc"'

# fish -> ~/.config/fish/config.fish set -gx
run fish
assert "fish: writes config.fish set -gx" 'grep -qF "set -gx DOCKET_SCRIPTS_DIR \"$EXPECTED\"" "$H/.config/fish/config.fish"'

# unknown shell -> ~/.profile POSIX export fallback
run ksh
assert "other: POSIX export to ~/.profile" 'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.profile"'

# settings.json env (jq), preserving an existing key
H="$(mktemp -d)"; _tmpdirs+=("$H"); mkdir -p "$H/.claude"
printf '{"permissions":{"allow":["keep"]}}\n' > "$H/.claude/settings.json"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "settings: env.DOCKET_SCRIPTS_DIR set" 'jq -e --arg v "$EXPECTED" ".env.DOCKET_SCRIPTS_DIR == \$v" "$H/.claude/settings.json" >/dev/null'
assert "settings: pre-existing key preserved" 'jq -e ".permissions.allow | index(\"keep\")" "$H/.claude/settings.json" >/dev/null'
assert "settings: still valid JSON"           'jq empty "$H/.claude/settings.json"'

# invalid settings.json is left untouched (refuse to clobber)
H="$(mktemp -d)"; _tmpdirs+=("$H"); mkdir -p "$H/.claude"
printf 'not valid json\n' > "$H/.claude/settings.json"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "invalid settings.json left unchanged" 'grep -qF "not valid json" "$H/.claude/settings.json"'

# idempotent: a second run leaves exactly one marker block + unchanged settings
H="$(mktemp -d)"; _tmpdirs+=("$H")
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "idempotent: exactly one marker block" '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

# stale block (clone moved) is replaced, not duplicated
H="$(mktemp -d)"; _tmpdirs+=("$H")
printf '# >>> docket (DOCKET_SCRIPTS_DIR) >>>\nexport DOCKET_SCRIPTS_DIR="/old/path/scripts"\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n' > "$H/.zshenv"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "stale path replaced"               'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "stale path: old value gone"        '! grep -qF "/old/path/scripts" "$H/.zshenv"'
assert "stale path: still one block"       '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

exit $fail

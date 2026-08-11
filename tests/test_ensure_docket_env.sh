#!/usr/bin/env bash
# tests/test_ensure_docket_env.sh — run: bash tests/test_ensure_docket_env.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-docket-env.sh"
EXPECTED="$REPO/scripts"               # the script exports its own dir
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
META_BASE="$(mktemp -d)/clone'quote\\slash \$(touch injected); # colon: value"
mkdir -p "$META_BASE/scripts/lib"; cp "$SCRIPT" "$META_BASE/scripts/ensure-docket-env.sh"
cp "$REPO/scripts/lib/docket-runtime.sh" "$META_BASE/scripts/lib/docket-runtime.sh"
META_SCRIPTS_RESOLVED="$(cd "$META_BASE/scripts" && pwd -P)"
META_RUNTIME_DIR="$(mktemp -d)/runtime'quote\\slash \$(touch injected-runtime); # colon: value"
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
zsh -c 'unset DOCKET_SCRIPTS_DIR DOCKET_BASH_PATH; source "$1"; printf "%s\n%s\n" "$DOCKET_SCRIPTS_DIR" "$DOCKET_BASH_PATH"' \
  zsh "$H/.profile" > "$H/zsh-loaded"
assert "serialization: zsh sources apostrophe/backslash values literally" \
  '[ "$(sed -n "1p" "$H/zsh-loaded")" = "$META_SCRIPTS_RESOLVED" ] && [ "$(sed -n "2p" "$H/zsh-loaded")" = "$META_RUNTIME" ]'
assert "serialization: profile evaluation executes no embedded command" \
  '[ ! -e "$H/injected" ] && [ ! -e "$H/injected-runtime" ]'
FISH_SCRIPTS="${META_SCRIPTS_RESOLVED//\\/\\\\}"; FISH_SCRIPTS="${FISH_SCRIPTS//\'/\\\'}"
FISH_RUNTIME="${META_RUNTIME//\\/\\\\}"; FISH_RUNTIME="${FISH_RUNTIME//\'/\\\'}"
assert "serialization: fish bindings use literal single-quoted values" \
  'HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=fish DOCKET_BASH_PATH="$META_RUNTIME" bash "$META_BASE/scripts/ensure-docket-env.sh" >/dev/null 2>&1 && grep -qF "set -gx DOCKET_SCRIPTS_DIR '\''$FISH_SCRIPTS'\''" "$H/.config/fish/config.fish" && grep -qF "set -gx DOCKET_BASH_PATH '\''$FISH_RUNTIME'\''" "$H/.config/fish/config.fish"'

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

# A real executable whose path contains a newline is still not serializable as a one-line profile
# binding. Reject it before touching either destination.
NEWLINE_RUNTIME_DIR="$(mktemp -d)/runtime
newline"; _tmpdirs+=("${NEWLINE_RUNTIME_DIR%%/runtime*}")
mkdir -p "$NEWLINE_RUNTIME_DIR"; NEWLINE_RUNTIME="$NEWLINE_RUNTIME_DIR/bash"
cp "$RUNTIME" "$NEWLINE_RUNTIME"; chmod +x "$NEWLINE_RUNTIME"
H="$(mktemp -d)"; _tmpdirs+=("$H"); printf '# keep\n' > "$H/.zshenv"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$NEWLINE_RUNTIME" bash "$SCRIPT" >/dev/null 2>&1; newline_rc=$?
assert "newline runtime: exits non-zero" '[ "$newline_rc" -ne 0 ]'
assert "newline runtime: profile left unchanged" '[ "$(cat "$H/.zshenv")" = "# keep" ]'

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

# --- change 0152: pin all five runtime diagnostics BEFORE the detection moves to the shared
# library. The suite asserted NONE of them, which made "the consolidation is message-preserving"
# unfalsifiable exactly where it matters. These are CHARACTERIZATION tests: green before and after.
mkfake(){ # mkfake <path> <first --version line> [noexec]
  mkdir -p "$(dirname "$1")"
  cat > "$1" <<EOF
#!/bin/sh
[ "\$#" -eq 1 ] && [ "\$1" = --version ] || exit 42
printf '%s\n' '$2'
EOF
  if [ "${3-}" = noexec ]; then chmod -x "$1"; else chmod +x "$1"; fi
}
FAKE_BIN="$(mktemp -d)"; _tmpdirs+=("$FAKE_BIN")
mkfake "$FAKE_BIN/legacy"  'GNU bash, version 3.2.57(1)-release (fake-legacy)'
mkfake "$FAKE_BIN/notbash" 'zsh 5.9 (arm64-apple-darwin)'
mkfake "$FAKE_BIN/noexec"  'GNU bash, version 5.2.0(1)-release (test)' noexec
printf '#!/bin/sh\nexit 7\n' > "$FAKE_BIN/novers"; chmod +x "$FAKE_BIN/novers"

diag(){ # diag <DOCKET_BASH_PATH value> -> stderr+stdout of one rejected run
  local h; h="$(mktemp -d)"; _tmpdirs+=("$h")
  HOME="$h" DOCKET_HARNESS_ROOT="$h" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$1" \
    bash "$SCRIPT" 2>&1
}

assert "0152 diagnostic: a relative path names 'must be an absolute path'" \
  'grep -qF "DOCKET_BASH_PATH must be an absolute path" <<<"$(diag relative)"'
assert "0152 diagnostic: a missing file names 'is not executable' with the path" \
  'grep -qF "DOCKET_BASH_PATH is not executable: $FAKE_BIN/does-not-exist" <<<"$(diag "$FAKE_BIN/does-not-exist")"'
assert "0152 diagnostic: a non-executable file names 'is not executable' with the path" \
  'grep -qF "DOCKET_BASH_PATH is not executable: $FAKE_BIN/noexec" <<<"$(diag "$FAKE_BIN/noexec")"'
assert "0152 diagnostic: a binary that cannot report a version names 'cannot report its version'" \
  'grep -qF "DOCKET_BASH_PATH cannot report its version" <<<"$(diag "$FAKE_BIN/novers")"'
assert "0152 diagnostic: a non-GNU binary names 'is not GNU Bash'" \
  'grep -qF "DOCKET_BASH_PATH is not GNU Bash" <<<"$(diag "$FAKE_BIN/notbash")"'
assert "0152 diagnostic: a Bash 3 binary names 'must be Bash 4 or newer'" \
  'grep -qF "DOCKET_BASH_PATH must be Bash 4 or newer" <<<"$(diag "$FAKE_BIN/legacy")"'

# NEGATIVE FIXTURES — the coverage gap this change exists to close. Routing through the library does
# NOT by itself give this file coverage: without these two cases, breaking the library's major check
# would leave tests/test_ensure_docket_env.sh fully green (green-suite-untested-branch).
h152_legacy="$(mktemp -d)"; _tmpdirs+=("$h152_legacy")
assert "0152 negative: a Bash 3.2 runtime is rejected non-zero" \
  '! HOME="$h152_legacy" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/legacy" bash "$SCRIPT" >/dev/null 2>&1'
h152_notbash="$(mktemp -d)"; _tmpdirs+=("$h152_notbash")
assert "0152 negative: a non-GNU runtime is rejected non-zero" \
  '! HOME="$h152_notbash" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/notbash" bash "$SCRIPT" >/dev/null 2>&1'

# Neither rejection may touch the profile.
h152="$(mktemp -d)"; _tmpdirs+=("$h152"); printf '# keep\n' > "$h152/.zshenv"
HOME="$h152" DOCKET_HARNESS_ROOT="$h152" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/legacy" \
  bash "$SCRIPT" >/dev/null 2>&1
assert "0152 negative: a rejected legacy runtime leaves the profile untouched" \
  '[ "$(cat "$h152/.zshenv")" = "# keep" ]'

exit $fail

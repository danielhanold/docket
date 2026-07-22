#!/usr/bin/env bash
# tests/test_bash_runtime_install.sh — hermetic install-time Bash discovery/persistence coverage
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-global-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

new_case(){
  CASE="$(mktemp -d)"; _tmpdirs+=("$CASE")
  HOME_DIR="$CASE/home"; BIN="$CASE/bin"; STD="$CASE/std"; LOG="$CASE/calls"
  mkdir -p "$HOME_DIR" "$BIN" "$STD/opt/homebrew/bin" "$STD/usr/local/bin"
  : > "$LOG"
  CONFIG="$HOME_DIR/.config/docket/config.yml"
}

fake_bash(){ # fake_bash <path> <major>
  mkdir -p "$(dirname "$1")"
  cat > "$1" <<EOF
#!/bin/sh
printf '%s\n' "\$0 \$*" >> "$LOG"
[ "\$#" -eq 1 ] && [ "\$1" = --version ] || exit 42
printf 'GNU bash, version $2.0.0(1)-release (test)\n'
EOF
  chmod +x "$1"
}

run_ensure(){
  HOME="$HOME_DIR" DOCKET_HARNESS_ROOT="$HOME_DIR" DOCKET_BASH_STANDARD_ROOT="$STD" \
    PATH="$BIN:/usr/bin:/bin" /bin/bash "$SCRIPT" >"$CASE/out" 2>&1
}

runtime_value(){
  awk '
    /^runtime:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && /^[[:space:]]+bash:[[:space:]]*/ {
      sub(/^[[:space:]]+bash:[[:space:]]*/, "")
      if ($0 ~ /^'\''.*'\''$/ || $0 ~ /^".*"$/) $0=substr($0,2,length($0)-2)
      print; exit
    }
  ' "$CONFIG"
}

# Homebrew's formula prefix wins over both fixed locations and PATH.
new_case
fake_bash "$CASE/brew-prefix/bin/bash" 5
fake_bash "$STD/opt/homebrew/bin/bash" 5
fake_bash "$BIN/bash" 5
cat > "$BIN/brew" <<EOF
#!/bin/sh
[ "\$#" -eq 1 ] && [ "\$1" = --prefix ] || exit 43
printf '%s\n' "$CASE/brew-prefix"
EOF
chmod +x "$BIN/brew"
run_ensure; rc=$?
assert "brew: qualifying Homebrew formula candidate is persisted" \
  '[ "$rc" -eq 0 ] && [ "$(runtime_value)" = "$CASE/brew-prefix/bin/bash" ]'

# An old Homebrew Bash is skipped; /opt/homebrew is the first fixed fallback.
new_case
fake_bash "$CASE/brew-prefix/bin/bash" 3
fake_bash "$STD/opt/homebrew/bin/bash" 4
fake_bash "$STD/usr/local/bin/bash" 5
fake_bash "$BIN/bash" 5
cat > "$BIN/brew" <<EOF
#!/bin/sh
printf '%s\n' "$CASE/brew-prefix"
EOF
chmod +x "$BIN/brew"
run_ensure; rc=$?
assert "fixed: Bash 3 candidate is rejected and /opt/homebrew fallback wins" \
  '[ "$rc" -eq 0 ] && [ "$(runtime_value)" = "$STD/opt/homebrew/bin/bash" ]'

# With no brew/fixed candidate, an absolute command -v result from PATH is used.
new_case
fake_bash "$BIN/bash" 4
run_ensure; rc=$?
assert "PATH: absolute PATH-resolved Bash is persisted last" \
  '[ "$rc" -eq 0 ] && [ "$(runtime_value)" = "$BIN/bash" ]'
assert "validation: candidates are asked only for --version" \
  '! awk '\''$2 != "--version" || NF != 2 { bad=1 } END { exit !bad }'\'' "$LOG"'

# A machine with only Apple's legacy Bash fails closed with the documented remedy.
new_case
fake_bash "$BIN/bash" 3
run_ensure; rc=$?
assert "legacy-only: install fails" '[ "$rc" -ne 0 ]'
assert "legacy-only: remedy names brew install bash" 'grep -qF "brew install bash" "$CASE/out"'
assert "legacy-only: no config is written" '[ ! -e "$CONFIG" ]'

# No candidate at all is the same closed failure.
new_case
run_ensure; rc=$?
assert "none: install fails" '[ "$rc" -ne 0 ]'
assert "none: remedy names brew install bash" 'grep -qF "brew install bash" "$CASE/out"'

# A hand-authored valid runtime is authoritative and the whole config remains byte-identical.
new_case
fake_bash "$CASE/explicit/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf '# user preface\nagent_harnesses: [claude]\nruntime:\n  bash: %s\n# user tail\n' "$CASE/explicit/bash" > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "explicit valid: existing runtime is preserved" '[ "$rc" -eq 0 ] && cmp -s "$CASE/before" "$CONFIG"'

# A hand-authored invalid runtime is never silently replaced by discovery.
new_case
fake_bash "$BIN/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf 'keep: yes\nruntime:\n  bash: /missing/bash\n' > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "explicit invalid: install stops" '[ "$rc" -ne 0 ]'
assert "explicit invalid: config remains byte-identical" 'cmp -s "$CASE/before" "$CONFIG"'
assert "explicit invalid: diagnostic names runtime.bash" 'grep -qF "runtime.bash" "$CASE/out"'

# A managed declaration and a hand-authored declaration must never compete. In particular, a
# valid explicit value must not make install bless a stale managed value that the resolver reads
# first. Refuse the ambiguous file byte-safely with an actionable diagnosis.
new_case
fake_bash "$CASE/explicit/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf '# >>> docket (runtime.bash) >>>\nruntime:\n  bash: /stale/managed/bash\n# <<< docket (runtime.bash) <<<\nruntime:\n  bash: %s\n' "$CASE/explicit/bash" > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "authority: stale managed plus valid explicit runtime is rejected" '[ "$rc" -ne 0 ]'
assert "authority: ambiguous config remains byte-identical" 'cmp -s "$CASE/before" "$CONFIG"'
assert "authority: diagnosis tells the user to keep one runtime declaration" \
  'grep -Eqi "both managed and explicit|one runtime\\.bash|remove.*managed" "$CASE/out"'

# The managed YAML scalar must remain literal even when the executable path contains shell/YAML
# metacharacters. The fixture would create $CASE/pwned if either persistence or later parsing
# evaluated command substitution.
new_case
WEIRD_ROOT="$CASE/root \$(touch $CASE/pwned); # colon: value"
fake_bash "$WEIRD_ROOT/opt/homebrew/bin/bash" 5
HOME="$HOME_DIR" DOCKET_HARNESS_ROOT="$HOME_DIR" DOCKET_BASH_STANDARD_ROOT="$WEIRD_ROOT" \
  PATH="$BIN:/usr/bin:/bin" /bin/bash "$SCRIPT" >"$CASE/out" 2>&1; rc=$?
assert "serialization: metacharacter runtime is installed literally" \
  '[ "$rc" -eq 0 ] && [ ! -e "$CASE/pwned" ]'
assert "serialization: managed YAML quotes the entire runtime scalar" \
  'grep -qF "  bash: '\''$WEIRD_ROOT/opt/homebrew/bin/bash'\''" "$CONFIG"'

# The managed block preserves all unrelated bytes exactly, including a missing EOF newline.
# Compare the recovered user-owned byte tail directly; record-oriented tools and command
# substitution would normalize the very boundary under test.
new_case
fake_bash "$BIN/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf '# first\nagent_harnesses: [claude]\n\n# final user line' > "$CONFIG"
cp "$CONFIG" "$CASE/before"
before_size="$(wc -c < "$CASE/before" | tr -d '[:space:]')"
run_ensure; rc=$?
tail -c "$before_size" "$CONFIG" > "$CASE/unmanaged"
assert "preservation: unrelated existing config bytes survive" \
  '[ "$rc" -eq 0 ] && cmp -s "$CASE/before" "$CASE/unmanaged"'
assert "preservation: managed block remains structurally separate from unterminated user content" \
  'grep -qF "# final user line" "$CONFIG" && grep -qF "# >>> docket (runtime.bash) >>>" "$CONFIG"'
run_ensure; rerun_rc=$?
tail -c "$before_size" "$CONFIG" > "$CASE/unmanaged-rerun"
assert "preservation: re-run keeps unterminated user config byte-identical" \
  '[ "$rerun_rc" -eq 0 ] && cmp -s "$CASE/before" "$CASE/unmanaged-rerun"'

# Corrupt managed markers are refused before any rewrite.
new_case
fake_bash "$BIN/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf 'keep: yes\n# >>> docket (runtime.bash) >>>\nruntime:\n  bash: /old/bash\n' > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "markers: dangling block is rejected" '[ "$rc" -ne 0 ] && cmp -s "$CASE/before" "$CONFIG"'

new_case
fake_bash "$BIN/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf '# >>> docket (runtime.bash) >>>\nruntime:\n  bash: /one\n# <<< docket (runtime.bash) <<<\n# >>> docket (runtime.bash) >>>\nruntime:\n  bash: /two\n# <<< docket (runtime.bash) <<<\n' > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "markers: duplicate config blocks are rejected byte-safely" \
  '[ "$rc" -ne 0 ] && cmp -s "$CASE/before" "$CONFIG"'

new_case
fake_bash "$BIN/bash" 5
mkdir -p "$(dirname "$CONFIG")"
printf '# <<< docket (runtime.bash) <<<\nkeep: yes\n# >>> docket (runtime.bash) >>>\n# <<< docket (runtime.bash) <<<\n' > "$CONFIG"
cp "$CONFIG" "$CASE/before"
run_ensure; rc=$?
assert "markers: config close-before-open is rejected byte-safely" \
  '[ "$rc" -ne 0 ] && cmp -s "$CASE/before" "$CONFIG"'

exit $fail

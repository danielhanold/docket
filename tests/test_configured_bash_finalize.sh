#!/usr/bin/env bash
# tests/test_configured_bash_finalize.sh — hermetic executable guard for the
# finalize suite-command boundary (change 0132). Run: bash tests/test_configured_bash_finalize.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FIN="$ROOT/skills/docket-finalize-change/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Execute the exact shell fragment published by finalize. Marker count and order are
# checked before extraction so a dangling or duplicated marker cannot broaden the range.
start='<!-- configured-bash-finalize:start -->'
end='<!-- configured-bash-finalize:end -->'
start_count="$(grep -cF -- "$start" "$FIN" || true)"
end_count="$(grep -cF -- "$end" "$FIN" || true)"
start_line="$(grep -nF -- "$start" "$FIN" | cut -d: -f1 || true)"
end_line="$(grep -nF -- "$end" "$FIN" | cut -d: -f1 || true)"
assert "contract has one balanced, ordered marker pair" \
  '[ "$start_count" = 1 ] && [ "$end_count" = 1 ] && [ -n "$start_line" ] && [ "$start_line" -lt "$end_line" ]'

contract="$(awk -v start="$start" -v end="$end" '
  $0 == start { in_contract=1; next }
  $0 == end { in_contract=0; exit }
  in_contract && $0 !~ /^```(bash)?$/ { print }
' "$FIN")"
assert "contract extraction is non-empty" '[ -n "$contract" ]'

fixture="$TMP/repo"
mkdir -p "$fixture/tests" "$TMP/bin"
runtime_log="$TMP/runtime.log"
execution_log="$TMP/execution.log"

cat > "$TMP/bin/configured-bash" <<'SH'
#!/bin/sh
printf '%s\n' "$1" >> "$RUNTIME_LOG"
exec /bin/bash "$@"
SH
chmod +x "$TMP/bin/configured-bash"

for name in test_alpha.sh test_beta.sh; do
  cat > "$fixture/tests/$name" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$(basename "$0")" >> "$EXECUTION_LOG"
SH
  chmod +x "$fixture/tests/$name"
done

export DOCKET_BASH_PATH="$TMP/bin/configured-bash"
export RUNTIME_LOG="$runtime_log"
export EXECUTION_LOG="$execution_log"
export FINALIZE_TEST_COMMAND=
(
  cd "$fixture" || exit 1
  /bin/bash -c "$contract"
)

assert "auto-detect executes both shell tests" \
  '[ "$(sort "$execution_log")" = "test_alpha.sh
test_beta.sh" ]'
assert "auto-detect routes both test paths through configured Bash" \
  '[ "$(cat "$runtime_log")" = "tests/test_alpha.sh
tests/test_beta.sh" ]'

recorder="$TMP/bin/record-command"
argv_log="$TMP/argv.log"
env_log="$TMP/env.log"
cat > "$recorder" <<'SH'
#!/bin/sh
printf '%s\n' "$#" "$@" > "$ARGV_LOG"
printf '%s\n' "$DOCKET_BASH_PATH" > "$ENV_LOG"
SH
chmod +x "$recorder"
export ARGV_LOG="$argv_log"
export ENV_LOG="$env_log"
export FINALIZE_TEST_COMMAND='"'"$recorder"'" "arg one" "literal;value"'
(
  cd "$fixture" || exit 1
  /bin/bash -c "$contract"
)

assert "explicit command text reaches the shell without interpreter rewriting" \
  '[ "$(cat "$argv_log")" = "2
arg one
literal;value" ]'
assert "explicit command receives configured Bash in its environment" \
  '[ "$(cat "$env_log")" = "$DOCKET_BASH_PATH" ]'
assert "explicit command does not traverse the configured runtime" \
  '[ "$(wc -l < "$runtime_log" | tr -d "[:space:]")" = 2 ]'

exit $fail

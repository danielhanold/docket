#!/usr/bin/env bash
# tests/test_configured_bash_finalize.sh — hermetic executable guard for the
# finalize suite-command boundary (change 0132). Run: bash tests/test_configured_bash_finalize.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FIN="$ROOT/skills/docket-finalize-change/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
green_status=0
(
  cd "$fixture" || exit 1
  /bin/bash -c "$contract"
) || green_status=$?

# The success direction of the accumulator's trailing exit predicate. Without it, an always-red
# auto-detect branch (a non-zero initializer, a negated predicate) keeps every -ne 0 assert green
# while turning every green suite into a red gate at both consumers.
assert "auto-detect reports zero when every test passes" \
  '[ "$green_status" -eq 0 ]'
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

# --- keep-going accumulator: a NON-FINAL red must still report non-zero (change 0228) ---------
# Own fixture dir and own log paths on purpose: the asserts above pin the SHARED runtime_log at
# exactly 2 lines and the explicit-command case exports a non-empty FINALIZE_TEST_COMMAND, so
# reusing either resource would redden a passing guard or skip the auto-detect branch entirely.
acc_fixture="$TMP/repo-accumulator"
mkdir -p "$acc_fixture/tests"
acc_runtime_log="$TMP/runtime-accumulator.log"
acc_execution_log="$TMP/execution-accumulator.log"

# test_bravo.sh is 2nd of 3 and exits 1; test_charlie.sh is last and passes. Without an
# accumulator the loop's status is the LAST test's, so the block reads green on a red suite.
for name in test_alpha.sh test_bravo.sh test_charlie.sh; do
  cat > "$acc_fixture/tests/$name" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$(basename "$0")" >> "$EXECUTION_LOG"
[ "$(basename "$0")" != "test_bravo.sh" ]
SH
  chmod +x "$acc_fixture/tests/$name"
done

acc_status=0
(
  cd "$acc_fixture" || exit 1
  RUNTIME_LOG="$acc_runtime_log" EXECUTION_LOG="$acc_execution_log" FINALIZE_TEST_COMMAND= \
    /bin/bash -c "$contract"
) || acc_status=$?

assert "auto-detect reports non-zero when a NON-FINAL test fails" \
  '[ "$acc_status" -ne 0 ]'
assert "auto-detect keeps going past the failure so every test still runs" \
  '[ "$(sort "$acc_execution_log")" = "test_alpha.sh
test_bravo.sh
test_charlie.sh" ]'

# --- empty suite: the literal glob must FAIL, never exit 0 having run zero tests (change 0228) -
# No nullglob and no [ -e "$test" ] guard, deliberately: with no matching files the glob stays
# literal, the invocation fails, and the block reports non-zero. nullglob would instead exit 0
# with zero tests run — a green gate certifying nothing.
#
# The pair below pins that property behaviorally, by effect rather than by spelling: the outcome
# assert pins the non-zero exit, and the mechanism assert pins WHY it is non-zero — the
# `configured-bash` wrapper records its "$1", so an unexpanded "tests/test_*.sh" in the runtime log
# is positive evidence the literal glob actually reached the runtime and failed there, not that the
# subshell died early on a bad cd or a broken contract. Any suppression of the literal glob —
# `shopt -s nullglob`, a `[ -e "$test" ] || continue`, a `compgen`/array rewrite — empties that log
# and reddens the mechanism assert, whatever the spelling. That is why there is no text-shape grep
# for the word "nullglob" here: it could only enumerate spellings, and it would false-red against a
# maintainer who hardens the fragment with `shopt -u nullglob`, which SATISFIES this invariant.
empty_fixture="$TMP/repo-empty"
mkdir -p "$empty_fixture/tests"
empty_runtime_log="$TMP/runtime-empty.log"
empty_execution_log="$TMP/execution-empty.log"

empty_status=0
(
  cd "$empty_fixture" || exit 1
  RUNTIME_LOG="$empty_runtime_log" EXECUTION_LOG="$empty_execution_log" FINALIZE_TEST_COMMAND= \
    /bin/bash -c "$contract"
) >/dev/null 2>&1 || empty_status=$?

assert "empty suite reports non-zero rather than certifying nothing" \
  '[ "$empty_status" -ne 0 ]'
assert "empty suite invokes the unexpanded glob" \
  '[ "$(cat "$empty_runtime_log" 2>/dev/null)" = "tests/test_*.sh" ]'

exit $fail

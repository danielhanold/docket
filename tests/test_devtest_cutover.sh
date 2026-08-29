#!/usr/bin/env bash
# tests/test_devtest_cutover.sh — the source-fidelity + docs + RC-workflow cutover guards
# (change 0318, Task 9). This is the "invoke an installed binary at a source-validation gate"
# MUTATION GUARD: the whole-suite gate command must be entered from SOURCE, `go run ./cmd/docket
# development test`, never a bare `docket …` that could select a stale installed binary.
#
# What it pins, all as pure text inspection over committed source (no Go build, no subprocess
# beyond grep/awk/sed/tr):
#   A  .docket.yml's finalize block carries test_command: go run ./cmd/docket development test,
#      and the value begins `go run ./cmd/docket` and NOT a bare `docket ` (the mutation guard).
#   B  .github/workflows/release-candidate.yml still DERIVES the command from .docket.yml
#      (finalize.test_command), executes it as a shell command line (sh -c "$test_cmd"), keeps no
#      `bash $test_cmd`-shaped wrap, and invokes scripts/run-tests.sh nowhere in its run blocks.
#   C  AGENTS.md and tests/README.md each bind the whole-suite claim to the go run command within a
#      bounded gap (whitespace collapsed first — learning phrase-grep-over-wrapped-prose), and
#      neither still presents scripts/run-tests.sh as THE whole-suite gate.
#   D  scripts/run-tests.md carries the frozen-oracle notice naming the canonical gate.
#
# Written FIRST and watched fail against the un-cut-over tree (TDD): before the flip, A/B/C/D all
# red because .docket.yml still says scripts/run-tests.sh, the workflow still wraps `bash $test_cmd`,
# the docs still lead with run-tests.sh, and run-tests.md has no frozen-oracle notice.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
# The assert helper is the tree's canonical one byte for byte (rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist).
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

DY="$REPO/.docket.yml"
WF="$REPO/.github/workflows/release-candidate.yml"
AG="$REPO/AGENTS.md"
RM="$REPO/tests/README.md"
RTM="$REPO/scripts/run-tests.md"

# --- A: .docket.yml finalize.test_command, source-entered ------------------------------------------
# Block-scoped awk over the first `finalize:` mapping (the tests/test_finalize_gate.sh idiom): never
# a bare whole-file grep, which docket's own change/spec prose would satisfy vacuously.
test_command_of(){
  awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f && /^[^[:space:]#]/{f=0}
    f && /^[[:space:]]+test_command[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*test_command[[:space:]]*:[[:space:]]*/,"",line);
      sub(/[[:space:]]+$/,"",line); print line; exit
    }' "$1" 2>/dev/null
}
TESTCMD="$(test_command_of "$DY")"

assert "the finalize.test_command is exactly the Go-native source entry" \
  '[ "$TESTCMD" = "go run ./cmd/docket development test" ]'

# The mutation guard: the value must begin `go run ./cmd/docket` so it compiles and runs the exact
# checkout under review — never a bare `docket …`, which selects whatever binary is installed.
assert "finalize.test_command begins with the source entry go run ./cmd/docket" \
  'case "$TESTCMD" in "go run ./cmd/docket"*) true ;; *) echo "  finalize.test_command: ${TESTCMD:-<unset>}" >&2; false ;; esac'
assert "finalize.test_command is NOT a bare docket invocation (no stale installed binary)" \
  'case "$TESTCMD" in "docket "*) echo "  finalize.test_command selects an installed binary: $TESTCMD" >&2; false ;; *) true ;; esac'

# --- B: the RC workflow's suite step -----------------------------------------------------------------
assert "the RC workflow still derives the suite command from .docket.yml finalize.test_command" \
  'grep -qF "finalize.test_command" "$WF" && grep -qF ".docket.yml" "$WF"'

# A multi-word `go run …` command is a shell command line, not a script path: it must be executed
# via `sh -c "$test_cmd"`, and no `bash $test_cmd`-shaped wrap may remain (that hands a many-word
# command to bash as a single filename).
assert "the RC workflow executes the resolved command as a shell command line (sh -c)" \
  'grep -qF "sh -c \"\$test_cmd\"" "$WF"'
assert "the RC workflow keeps no bash \$test_cmd script-path wrap" \
  '! grep -qE "bash[[:space:]]+\\\$test_cmd" "$WF"'

# No literal scripts/run-tests.sh INVOCATION in the workflow's run blocks. Comment mentions are
# fine (the Go runner emits run-tests.sh's vocabulary by construction); strip comments first so an
# executable revert to `bash scripts/run-tests.sh` is what this catches, not a prose reference.
WF_EXEC="$(sed 's/#.*//' "$WF")"
assert "the RC workflow invokes scripts/run-tests.sh in no run block (comments aside)" \
  '! grep -qF "run-tests.sh" <<<"$WF_EXEC"'

# --- C: contributor docs name the canonical gate, bound to the whole-suite claim --------------------
# Collapse whitespace first (learning phrase-grep-over-wrapped-prose): a wrapped bullet must still
# read as one line so a bounded-gap phrase grep binds the command to the claim, not a bare presence
# grep that a stray mention anywhere would satisfy.
AG_1LINE="$(tr -s '[:space:]' ' ' < "$AG")"
RM_1LINE="$(tr -s '[:space:]' ' ' < "$RM")"

assert "AGENTS.md binds the whole-suite gate to go run ./cmd/docket development test" \
  'grep -qE "whole suite.{0,255}go run \./cmd/docket development test" <<<"$AG_1LINE"'
assert "tests/README.md binds the whole-suite command to go run ./cmd/docket development test" \
  'grep -qE "whole suite.{0,255}go run \./cmd/docket development test" <<<"$RM_1LINE"'

# And neither still presents scripts/run-tests.sh AS the whole-suite gate: run-tests.sh may survive
# as the named frozen oracle / focused tool, but not immediately bound to the whole-suite claim.
assert "AGENTS.md no longer presents scripts/run-tests.sh as the whole-suite gate" \
  '! grep -qE "whole suite[^.]{0,80}scripts/run-tests\.sh" <<<"$AG_1LINE"'
assert "tests/README.md no longer presents scripts/run-tests.sh as the whole-suite gate" \
  '! grep -qE "whole suite[^.]{0,80}scripts/run-tests\.sh" <<<"$RM_1LINE"'

# --- D: the frozen oracle's own contract flags itself as the oracle ---------------------------------
assert "scripts/run-tests.md carries the frozen parity-oracle notice" \
  'grep -qiF "frozen parity oracle" "$RTM"'
assert "scripts/run-tests.md names the canonical whole-suite gate" \
  'grep -qF "go run ./cmd/docket development test" "$RTM"'

exit $fail

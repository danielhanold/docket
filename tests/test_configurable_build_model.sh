#!/usr/bin/env bash
# tests/test_configurable_build_model.sh — verifies change 0044 task 2: docket-implement-next's
# Step 5 build-dispatch rule wiring BUILD_IMPLEMENTER/BUILD_REVIEWER into SDD's model: field.
# Run: bash tests/test_configurable_build_model.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
RM="$REPO/README.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "implement-next names the build model surface" 'grep -q "BUILD_IMPLEMENTER" "$IMPL" && grep -q "BUILD_REVIEWER" "$IMPL"'
assert "build.implementer governs implementer + fix dispatches" 'grep -qiE "BUILD_IMPLEMENTER[^.]*(implementer|fix)|(implementer|fix)[^.]*BUILD_IMPLEMENTER" "$IMPL"'
assert "build.reviewer governs reviewer + final-review dispatches" 'grep -qiE "BUILD_REVIEWER[^.]*(review)|(review)[^.]*BUILD_REVIEWER" "$IMPL"'
assert "unset build role defers to SDD Model Selection" 'grep -qiE "unset[^.]*SDD|SDD.{0,40}Model Selection|defer to SDD" "$IMPL"'
assert "build wiring fills SDD model: field, no SDD fork" 'grep -qiE "model:" "$IMPL" && grep -qiE "no.{0,4}fork|already-required|SDD.s.{0,10}model" "$IMPL"'

assert "convention documents the build: surface" 'grep -q "build:" "$CONV" && grep -qE "implementer|reviewer" "$CONV"'
assert "convention notes build: takes direct model IDs / defers to SDD" 'grep -qiE "model id|direct model|defers to SDD|SDD.{0,20}selection" "$CONV"'
assert "README documents build:" 'grep -q "build:" "$RM" && grep -qiE "implementer|reviewer" "$RM"'

exit $fail

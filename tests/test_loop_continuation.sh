#!/usr/bin/env bash
# tests/test_loop_continuation.sh — guards change 0088 (loop continuation: docket-implement-next as
# a driver-agnostic re-invocation contract). Asserts the four-disposition terminal contract, the
# per-step-exit mappings, id-set scoping (SKILL.md), and the README /loop drain-pattern doc.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review; this test does not replace it.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

IMPL="$REPO/skills/docket-implement-next/SKILL.md"

# --- SKILL.md: the four-disposition terminal contract ---
assert "SKILL has a Terminal disposition section" 'grep -Eqi "Terminal disposition" "$IMPL"'
for d in advanced contended drained halted; do
  tok="\`$d\`"
  assert "SKILL names disposition $d (code-formatted)" 'grep -qF "$tok" "$IMPL"'
done
# The binary driver rule — both halves must be present (non-vacuous).
assert "SKILL states continue-on advanced/contended" 'grep -Eqi "continue on .{0,4}advanced" "$IMPL"'
assert "SKILL states stop-on drained/halted" 'grep -Eqi "stop on .{0,4}drained" "$IMPL"'
# Skipped-with-reasons enumeration.
assert "SKILL enumerates skipped-with-reason" 'grep -Eqi "skipped with (its|the) reason" "$IMPL"'

# --- SKILL.md: the closing obligation on the AGENT (change 0212) ---
# The pre-0212 section was guidance to a DRIVER only; nothing bound the running agent to declare a
# disposition, and the 0206 run closed with a step-scoped "build disposition" instead. These asserts
# pin the obligation itself, not the table it sits under.
assert "SKILL obliges the agent to declare before the run ends" \
  'grep -qF -- "the run does not end until" "$IMPL"'
assert "SKILL names a non-conforming vocabulary an aborted run" \
  'grep -qF -- "is by construction an aborted run" "$IMPL"'
# advanced is gated on Step 7 BY POINTER — 0212 must not define Step 7's postcondition (that is
# change 0203's surface), so assert the pointer and not any postcondition wording.
assert "SKILL gates advanced on Step 7's postcondition by pointer" \
  'grep -Eqi "advanced.{0,80}Step 7|Step 7.{0,80}advanced" "$IMPL"'

# --- SKILL.md: per-step-exit mappings ---
assert "SKILL ties a lost claim race to contended (Step 2)" 'grep -Eqi "claim (CAS|race)" "$IMPL"'
assert "SKILL ties the empty queue to drained (Step 1)" 'grep -Eqi "empty queue|no candidate|nothing .{0,20}build-ready" "$IMPL"'

# --- SKILL.md: id-set scoping ---
assert "SKILL documents an id allowlist" 'grep -Eqi "allowlist" "$IMPL"'
assert "SKILL shows the comma-separated id-set form" 'grep -Eq "docket-implement-next 90,92,94" "$IMPL"'
assert "SKILL states the allowlist is not a dependency override" 'grep -Eqi "never a dependency override" "$IMPL"'

README="$REPO/README.md"
wb='`/loop docket-implement-next`'

# --- README: the /loop drain-pattern doc ---
assert "README documents the /loop whole-backlog drain" 'grep -qF "$wb" "$README"'
assert "README documents the /loop id-set drain" 'grep -Eq "/loop docket-implement-next 90,92,94" "$README"'
assert "README states the driver never merges" 'grep -qiF "never merges** — the human merge gate is untouched" "$README"'
assert "README names all four dispositions" 'for d in advanced contended drained halted; do grep -qiF "$d" "$README" || exit 1; done'

# --- Non-vacuity / mutation proof: the code-formatted disposition grep actually bites. ---
probe="$(mktemp)"; printf 'plain advanced word, no code formatting\n' > "$probe"
assert "the code-formatted disposition grep is non-vacuous" '! grep -qF "\`advanced\`" "$probe"'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

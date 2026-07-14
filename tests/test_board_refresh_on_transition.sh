#!/usr/bin/env bash
# tests/test_board_refresh_on_transition.sh — verifies change 0004:
# BOARD.md is refreshed on every status transition, not only at Step 0.
# Extended by change 0059 (Task 3): every status-writing Board-pass caller must name the
# gated `board-refresh.sh` entry point at its Board site AND state the diff-only commit rule
# (only if BOARD.md changed), not just delegate to "docket-status's Board pass" prose.
# Run: bash tests/test_board_refresh_on_transition.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
# A. The board-refresh invariant lives in the convention (single-sourced in docket-convention).
assert "board-refresh invariant present in the convention" \
  'grep -q "Board refresh on status writes" skills/docket-convention/SKILL.md'

# B. docket-implement-next wires its three inline refreshes, best-effort.
assert "implement-next defines best-effort board refresh" \
  'grep -q "Best-effort board refresh" skills/docket-implement-next/SKILL.md'
assert "implement-next has 3 best-effort Board-pass site clauses (claim, reconcile-kill, implemented)" \
  '[ "$(grep -c "run the Board pass (best-effort" skills/docket-implement-next/SKILL.md)" -ge 3 ]'

# C. docket-new-change proposed-kill refreshes the board (must-land, not best-effort).
assert "new-change proposed-kill refreshes board (must-land Board pass)" \
  'grep -q "must-land Board pass" skills/docket-new-change/SKILL.md'

# D. terminal-publish stays board-agnostic — the kill gap is fixed at the SITES, not here.
assert "terminal-publish keeps the 'BOARD.md is never published' guarantee" \
  'grep -qF "is **never** published" skills/docket-finalize-change/SKILL.md'

# --- change 0059 Task 3, NARROWED by change 0071 -----------------------------------------------
# 0059 asserted that every Board-pass caller named `board-refresh.sh` and hand-stated the
# diff-only commit rule. 0071 collapses all 8 call sites into ONE facade call
# (`docket.sh docket-status --board-only`): the orchestrator now owns the render, the diff-only
# decision, the commit, and the push, and NO surfaces value crosses the skill/script boundary.
# The prose clauses 0059 anchored on are therefore gone BY DESIGN.
#
# This guard is NARROWED, never deleted (ADR-0031: deleting a sentinel is how the guarded hole
# reopens). The property that is still load-bearing: every status-writing skill routes its board
# write through the deterministic gated pipeline at its Board site — never a hand-render, never a
# raw redirect, never a bare "docket-status will get to it eventually" delegation.
#
# The diff-only rule 0059 asserted in PROSE is now asserted where it actually executes:
# tests/test_docket_status.sh ("board_pass second (clean) run reports clean") proves the
# orchestrator does not commit an unchanged board.
BOARD_PASS_CALL="docket.sh docket-status --board-only"

# E. The convention still names board-refresh.sh as the gated inline writer (a NOUN mention —
# permitted by ADR-0030, and load-bearing: it is what documents the single write choke point).
assert "convention names board-refresh.sh (the gated inline writer)" \
  'grep -q "board-refresh.sh" skills/docket-convention/SKILL.md'

# E2. The convention defines the ONE Board-pass call, and states the report-line contract that
# replaced the hand-rolled diff check.
assert "convention defines the single Board-pass facade call" \
  "grep -qF \"$BOARD_PASS_CALL\" skills/docket-convention/SKILL.md"
assert "convention states the stdout report-line contract (not an exit code)" \
  'grep -qF "never on the exit code" skills/docket-convention/SKILL.md'

CALLERS=(
  skills/docket-new-change/SKILL.md
  skills/docket-groom-next/SKILL.md
  skills/docket-auto-groom/SKILL.md
  skills/docket-finalize-change/SKILL.md
  skills/docket-implement-next/SKILL.md
)

for f in "${CALLERS[@]}"; do
  name="$(basename "$(dirname "$f")")"
  assert "$name routes its Board site through the single facade call" \
    "grep -qF \"$BOARD_PASS_CALL\" \"$f\""
  # The retired shapes must be GONE: a skill that still spells a surfaces value is a skill that
  # can still send an unresolved one.
  assert "$name no longer spells a surfaces value at its Board site" \
    "! grep -qE '\-\-surfaces|BOARD_SURFACES' \"$f\""
done

exit $fail

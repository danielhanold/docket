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

# --- change 0059 Task 3: every Board-pass caller must be explicit + gated, not a bare delegation ---
# The consistent, grep-able diff-only phrase every rewired site uses (see task-3 brief).
DIFF_ONLY_PHRASE="only if BOARD.md changed"
PORCELAIN_MENTION="git status --porcelain"

# E. The convention names board-refresh.sh as the gated inline entry point.
assert "convention names board-refresh.sh (inline entry point)" \
  'grep -q "board-refresh.sh" skills/docket-convention/SKILL.md'

CALLERS=(
  skills/docket-new-change/SKILL.md
  skills/docket-groom-next/SKILL.md
  skills/docket-auto-groom/SKILL.md
  skills/docket-finalize-change/SKILL.md
  skills/docket-implement-next/SKILL.md
)

for f in "${CALLERS[@]}"; do
  name="$(basename "$(dirname "$f")")"
  assert "$name names the board-refresh op (via the docket.sh facade) at a Board site" \
    "grep -q \"docket.sh board-refresh\" \"$f\""
  assert "$name states the diff-only commit rule ($DIFF_ONLY_PHRASE)" \
    "grep -qF \"$DIFF_ONLY_PHRASE\" \"$f\""
  assert "$name mentions the git status --porcelain diff check" \
    "grep -qF \"$PORCELAIN_MENTION\" \"$f\""
done

exit $fail

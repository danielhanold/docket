#!/usr/bin/env bash
# tests/test_board_refresh_on_transition.sh — verifies change 0004:
# BOARD.md is refreshed on every status transition, not only at Step 0.
# Run: bash tests/test_board_refresh_on_transition.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)

# A. The board-refresh invariant lives in the canonical convention → present in ALL five skills.
for s in "${SKILLS[@]}"; do
  assert "board-refresh invariant present in $s" \
    'grep -q "Board refresh on status writes" "skills/'"$s"'/SKILL.md"'
done
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'

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

# E. docket-status gains the board/source drift tripwire (a warning).
assert "docket-status has board/source drift health check" \
  'grep -q "Board/source drift" skills/docket-status/SKILL.md'

exit $fail

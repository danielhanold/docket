#!/usr/bin/env bash
# tests/test_dispatch_block_budget.sh — regrowth guard (change 0334) for the always-loaded
# AGENTS.md/CLAUDE.md docket dispatch block. The block rides every parent-facing harness's
# always-loaded context, so its word count is a cost paid on every turn; this guard makes the
# "compact the dispatch block" direction durable (learnings: size-target-is-direction).
# run: bash tests/test_dispatch_block_budget.sh
#
# WHAT CHANGED. Change 0334 dropped the 17-agent roster (and the interpolated shipped-harness list)
# from the block, replacing it with a compact registered-agent routing rule that defers to the
# harness's own agent registry. The block is one of TWO lockstep generators — the Go emitter
# internal/harness/dispatch.go and this shell mirror sync-agents.sh — and this guard measures the
# shell-emitted committed surface.
#
# RECORDED ACTUALS (the two numbers the "lowers the bound" acceptance is made of):
#   OLD actual (pre-0334, roster block, measured with `wc -w` on the block between its markers on
#              the pre-change tree): 1156 words.
#   NEW actual (compact routing rule + run-gate payload, this change): 352 words.
# BUDGET is the NEW actual rounded UP to the next multiple of 50 (352 -> 400), and the guard
# hard-asserts BUDGET < OLD actual — a block that regrows past the compaction reddens here, and the
# ceiling can only ever be moved DOWN, never back up toward the old roster.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

OLD_ACTUAL=1156   # pre-0334 roster block, recorded above; the ceiling must stay strictly below it.
BUDGET=400        # NEW measured actual (352) rounded up to the next multiple of 50.

# Direction, made durable: the ceiling is below the old actual, so the block cannot regrow back to
# the roster without reddening. This assert is what makes the number a DIRECTION, not a free knob.
assert "BUDGET ($BUDGET) is strictly below the recorded pre-0334 actual ($OLD_ACTUAL)" \
  '[ "$BUDGET" -lt "$OLD_ACTUAL" ]'

# Assemble the block in a hermetic fixture repo opted into a dispatch surface, exactly as a consumer
# repo would generate it, then measure the managed block BETWEEN its markers (markers excluded).
SBX="$(mktemp -d "${TMPDIR:-/tmp}/dispatchbudget.XXXXXX")"
git -C "$SBX" init --quiet
git -C "$SBX" config user.email t@t.test
git -C "$SBX" config user.name Test
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
A="$SBX/AGENTS.md"
assert "fixture: the dispatch block was generated" '[ -f "$A" ] && grep -qF "docket:dispatch:start" "$A"'

block_between_markers(){ awk '/docket:dispatch:start/{f=1;next} /docket:dispatch:end/{f=0} f' "$1"; }
WORDS="$(block_between_markers "$A" | wc -w | tr -d ' ')"
assert "the dispatch block is non-empty (fixture actually produced content)" '[ "$WORDS" -ge 1 ]'
assert "the dispatch block is within BUDGET ($WORDS <= $BUDGET words)" '[ "$WORDS" -le "$BUDGET" ]'
rm -rf "$SBX"

# Non-vacuity / mutation proof: the guard actually bites. A synthetic block one word over a 1-word
# budget must be caught by the SAME `-le` comparison the real assert uses (mirrors the self-check
# tests/test_skill_size_budgets.sh ends with).
probe="$(mktemp "${TMPDIR:-/tmp}/dispatchbudget-probe.XXXXXX")"; printf 'a b\n' > "$probe"
pW="$(wc -w < "$probe" | tr -d ' ')"
assert "the word-budget comparison is non-vacuous (2 > 1 is caught)" '[ ! "$pW" -le 1 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

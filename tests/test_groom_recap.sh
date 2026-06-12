#!/usr/bin/env bash
# tests/test_groom_recap.sh — guards change 0013 (groom-next stub recap):
#   - Step 3 opens with a recap of the selected stub, written for a zero-context reader,
#     BEFORE superpowers:brainstorming is invoked (phone / fresh-session grooming)
#   - the recap covers: what was selected and why, a PM-altitude summary, dependency
#     statuses (folded in from Step 1), and the open questions framed as the agenda
#   - the recap is an introduction, not a confirmation gate
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SKILL="$REPO/skills/docket-groom-next/SKILL.md"

assert "step 3 is recap-then-groom" \
  'grep -qF "### Step 3 — Recap, then groom with the human" "$SKILL"'
assert "recap is written for a zero-context reader" \
  'grep -qF "written for a reader with no prior context" "$SKILL"'
assert "recap states the selection: id, title, priority" \
  'grep -qF "id, title, priority" "$SKILL"'
assert "recap distills the stub at PM altitude" \
  'grep -qF "PM-altitude summary" "$SKILL"'
assert "step 1 folds dependency statuses into the recap" \
  'grep -qF "as part of the Step 3 recap" "$SKILL"'
assert "recap frames open questions as the agenda" \
  'grep -qF "the agenda the brainstorm will work through" "$SKILL"'
assert "recap is an introduction, not a confirmation gate" \
  'grep -qF "introduction, not a confirmation gate" "$SKILL"'

recap_line="$(grep -nF "recap of the selected stub" "$SKILL" | head -1 | cut -d: -f1)"
brainstorm_line="$(grep -nF "Then run \`superpowers:brainstorming\` WITH THE HUMAN" "$SKILL" | head -1 | cut -d: -f1)"
assert "recap comes before the brainstorm invocation" \
  '[ -n "$recap_line" ] && [ -n "$brainstorm_line" ] && [ "$recap_line" -lt "$brainstorm_line" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

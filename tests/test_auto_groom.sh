#!/usr/bin/env bash
# tests/test_auto_groom.sh — guards change 0014 (docket-auto-groom):
#   - convention defines the auto_groom knob, the tri-state auto_groomable field,
#     effective resolution, the autonomous-eligible queue, and the abstain rule
#   - the docket-auto-groom skill drains (loops), designer+critic gate every
#     build-ready exit, kill/defer are never autonomous, abstain flips the flag
#   - groom-next selection bands prefer stubs that need a human
#   - new-change can set the flag at create time; template documents it
#   - the board renders abstained stubs distinctly
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# --- convention: knob + field + shared definitions ---
assert "convention: .docket.yml example carries the auto_groom knob (default false)" \
  'grep -qE "^auto_groom: false" "$CONV"'
assert "convention: manifest carries tri-state auto_groomable" \
  'grep -qE "^auto_groomable:[[:space:]]+#" "$CONV"'
assert "convention: unset means inherit the repo default" \
  'grep -qF "unset ⇒ inherit" "$CONV"'
assert "convention: effective auto-groomable is defined" \
  'grep -qF "**effective auto-groomable**" "$CONV"'
assert "convention: autonomous-eligible queue is defined" \
  'grep -qF "**autonomous-eligible**" "$CONV"'
assert "convention: abstain rule is defined (flag flip + blocked section)" \
  'grep -qF "## Auto-groom blocked" "$CONV"'
assert "convention: body sections list the Auto-groom blocked section" \
  'grep -qF "\`## Auto-groom blocked\`" "$CONV"'
assert "convention: groom-next selection bands defined" \
  'grep -qF "selection bands" "$CONV"'

AG="$REPO/skills/docket-auto-groom/SKILL.md"

# --- the skill itself ---
assert "auto-groom: skill file exists" '[ -f "$AG" ]'
assert "auto-groom: drains until the queue is empty" \
  'grep -qF "until no autonomous-eligible stub remains" "$AG"'
assert "auto-groom: loads the convention first" \
  'grep -qF "docket-convention" "$AG"'
assert "auto-groom: rejects the simulated-human auto-answerer" \
  'grep -qiF "not invoke \`superpowers:brainstorming\`" "$AG"'
assert "auto-groom: designer records an Assumptions block" \
  'grep -qF "## Assumptions" "$AG"'
assert "auto-groom: designer reads the learnings ledger" \
  'grep -qF "LEARNINGS.md" "$AG"'
assert "auto-groom: critic is a fresh subagent, not the designer" \
  'grep -qF "fresh subagent" "$AG"'
assert "auto-groom: critic gates trivial verdicts too" \
  'grep -qF "trivial verdicts alike" "$AG"'
assert "auto-groom: kill and defer are never autonomous" \
  'grep -qF "Kill and defer are NEVER autonomous" "$AG"'
assert "auto-groom: abstain flips the flag and appends the blocked section" \
  'grep -qF "auto_groomable: false" "$AG" && grep -qF "## Auto-groom blocked" "$AG"'
assert "auto-groom: takes no claim, cites ADR-0004" \
  'grep -qF "ADR-0004" "$AG"'
assert "auto-groom: never implements (markdown only)" \
  'grep -qF "never branches, worktrees, or code" "$AG"'

# order: designer pass precedes critic pass precedes exits
designer_line="$(grep -nF "### Step 2 — Designer pass" "$AG" | head -1 | cut -d: -f1)"
critic_line="$(grep -nF "### Step 3 — Critic pass" "$AG" | head -1 | cut -d: -f1)"
exit_line="$(grep -nF "### Step 4 — Exit" "$AG" | head -1 | cut -d: -f1)"
assert "auto-groom: designer → critic → exit, in that order" \
  '[ -n "$designer_line" ] && [ -n "$critic_line" ] && [ -n "$exit_line" ] && [ "$designer_line" -lt "$critic_line" ] && [ "$critic_line" -lt "$exit_line" ]'

GN="$REPO/skills/docket-groom-next/SKILL.md"

# --- groom-next: auto-groom-aware bands ---
assert "groom-next: selection bands present" \
  'grep -qF "selection bands" "$GN"'
assert "groom-next: abstained stubs first" \
  'grep -qF "## Auto-groom blocked" "$GN"'
assert "groom-next: auto-groomable stubs flagged, not hidden" \
  'grep -qF "docket-auto-groom will handle it unless you want it now" "$GN"'
band1_off="$(grep -obF "abstained" "$GN" | head -1 | cut -d: -f1)"
band3_off="$(grep -obF "will handle it unless you want it now" "$GN" | head -1 | cut -d: -f1)"
assert "groom-next: abstained band stated before auto-groomable band" \
  '[ -n "$band1_off" ] && [ -n "$band3_off" ] && [ "$band1_off" -lt "$band3_off" ]'

NC="$REPO/skills/docket-new-change/SKILL.md"
TPL="$REPO/skills/docket-new-change/change-template.md"

# --- new-change: create-time flag ---
assert "new-change: create-time auto_groomable mention" \
  'grep -qF "auto_groomable: true" "$NC"'
assert "new-change: scan stubs leave the field unset (inherit)" \
  'grep -qF "leave \`auto_groomable\` unset" "$NC"'
assert "template: documents tri-state auto_groomable" \
  'grep -qF "auto_groomable:" "$TPL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

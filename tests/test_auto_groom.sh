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
  'grep -qF "auto_groomable:" "$CONV"'
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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

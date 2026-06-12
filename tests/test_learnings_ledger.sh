#!/usr/bin/env bash
# tests/test_learnings_ledger.sh — guards change 0006 (the learnings ledger):
#   - the convention carries the Learnings ledger contract (single source)
#   - the harvest procedure lives in docket-finalize-change; docket-status references it
#   - the readers (implement-next, groom-next) carry their read lines
#   - no operating skill restates the contract (sentinel scan)
# The ledger FILE lives on the docket branch only and is not testable here (see plan/results).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"
OPERATING=(docket-new-change docket-groom-next docket-implement-next docket-status docket-finalize-change docket-adr)

# (a) the convention contract
assert "convention has the Learnings ledger section" 'grep -qF "### Learnings ledger" "$CONV"'
assert "convention names the ledger path" 'grep -qF "LEARNINGS.md" "$CONV"'
assert "convention states the ~300-line soft cap" 'grep -qF "~300 lines" "$CONV"'
assert "directory layout lists LEARNINGS.md" 'grep -qF "LEARNINGS.md            # curated" "$CONV"'

# (b) the harvest procedure: single-sourced in finalize, referenced by status
assert "finalize carries the harvest step" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize has the idempotency probe" \
  'grep -qF "already cites" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "status sweep invokes the harvest by reference" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-status/SKILL.md" && grep -qF "docket-finalize-change" "$REPO/skills/docket-status/SKILL.md"'

# (c) the readers
assert "implement-next reads the ledger at plan time and review" \
  '[ "$(grep -cF "LEARNINGS.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "groom-next reads the ledger in scan-context" \
  'grep -qF "LEARNINGS.md" "$REPO/skills/docket-groom-next/SKILL.md"'

# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "compression, not destruction"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

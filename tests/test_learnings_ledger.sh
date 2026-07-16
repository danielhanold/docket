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

# (a) the convention contract — single source
assert "convention has the Learnings ledger section" 'grep -qF "### Learnings ledger" "$CONV"'
assert "convention names the findings directory" 'grep -qF "<changes_dir>/learnings/" "$CONV"'
assert "convention names the generated index as derived" \
  'grep -qF "is a **derived view**" "$CONV"'
assert "convention states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$CONV"'
assert "convention states the cap counts active findings" \
  'grep -qF "counts **active findings**" "$CONV"'
assert "convention states the off switch is a gate, not a purge" \
  'grep -qF "a no-op **read/write gate, never a" "$CONV"'
assert "convention pins the promotion_state enum" \
  'grep -qF "retained | candidate | promoted" "$CONV"'
assert "directory layout lists the learnings dir" \
  'grep -qE "^  learnings/ +# curated build-loop findings" "$CONV"'
assert "convention keeps the LEARNINGS.md stub pointer" 'grep -qF "remains as a pointer stub" "$CONV"'

# (b) the harvest procedure: single-sourced in finalize, referenced by status
assert "finalize carries the harvest step" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize's idempotency probe keys on the changes: list" \
  'grep -qF "already contains this change" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize gates the harvest on learnings.enabled" \
  'grep -qF "learnings disabled — harvest skipped" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize re-renders the index through the facade" \
  'grep -qF "docket.sh render-learnings-index" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "status sweep invokes the harvest by reference" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-status/SKILL.md" && grep -qF "docket-finalize-change" "$REPO/skills/docket-status/SKILL.md"'

# (c) the readers
assert "implement-next reads the ledger at plan time and review" \
  '[ "$(grep -cF "LEARNINGS.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "groom-next reads the ledger in scan-context" \
  'grep -qF "LEARNINGS.md" "$REPO/skills/docket-groom-next/SKILL.md"'

# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "will the agent know to search for this?"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

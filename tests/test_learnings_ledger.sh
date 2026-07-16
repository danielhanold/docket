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

# (b') change 0067 task 6: docket-status documents its own learnings pass (self-heal + advisories)
assert "status documents the learnings enable gate" \
  'grep -qF "learnings disabled" "$REPO/skills/docket-status/SKILL.md"'
assert "status documents the index self-heal as a derived view" \
  'grep -qF "render-learnings-index" "$REPO/skills/docket-status/SKILL.md"'
assert "status documents both needs-you advisories" \
  'grep -qF "over-cap" "$REPO/skills/docket-status/SKILL.md" && grep -qF "promotion-pending" "$REPO/skills/docket-status/SKILL.md"'

# (c) the readers — the two-step index-first read contract, at all three hot moments
assert "implement-next reads the index at plan time AND review" \
  '[ "$(grep -cF "learnings/README.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "implement-next gates its reads on learnings.enabled" \
  '[ "$(grep -cF "learnings.enabled" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "groom-next reads the index before the brainstorm" \
  'grep -qF "learnings/README.md" "$REPO/skills/docket-groom-next/SKILL.md"'
assert "groom-next gates its read on learnings.enabled" \
  'grep -qF "learnings.enabled" "$REPO/skills/docket-groom-next/SKILL.md"'
# No reader may still point at the retired single-file ledger as a READ target.
for sk in docket-implement-next docket-groom-next; do
  assert "$sk no longer reads the retired LEARNINGS.md" \
    '! grep -qEi "read .*LEARNINGS\.md" "$REPO/skills/$sk/SKILL.md"'
done

# (c') change 0067 plan-gap fix: the two sites Task 7's enumeration missed —
# docket-auto-groom's self-brainstorm scan and docket-brainstorm's consultant payload.
assert "auto-groom reads the learnings index before its self-brainstorm" \
  'grep -qF "learnings/README.md" "$REPO/skills/docket-auto-groom/SKILL.md"'
assert "auto-groom gates its learnings read on learnings.enabled" \
  'grep -qF "learnings.enabled" "$REPO/skills/docket-auto-groom/SKILL.md"'
assert "brainstorm's consultant payload references learnings findings/index, not the retired ledger" \
  'grep -qEi "learnings (findings|index)" "$REPO/skills/docket-brainstorm/SKILL.md"'
assert "convention's Readers line names docket-auto-groom" \
  'grep -E "^\*\*Readers:\*\*.*docket-auto-groom" "$CONV" >/dev/null'

# Completeness guard — SHAPE, not a hand-listed corpus. A hand-listed file set (like the
# `for sk in ...` loop above) is exactly the floor-not-the-set defect that let auto-groom
# and docket-brainstorm slip past Task 7's enumeration. Glob every live skill instead: any
# SKILL.md mentioning LEARNINGS.md WITHOUT qualifying it as the retired pointer stub is
# treated as a live read target and fails. docket-convention's directory-layout line and its
# "remains as a pointer stub" sentence are the only legitimate mentions, and both carry the
# phrase "pointer stub" on the same line — that phrase is the exemption, not a filename.
assert "no live skill still names LEARNINGS.md as a read target (glob corpus; convention's pointer-stub mentions exempt)" \
  '[ -z "$(grep -F "LEARNINGS.md" "$REPO"/skills/*/SKILL.md 2>/dev/null | grep -Fvi "pointer stub")" ]'

# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "will the agent know to search for this?"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done

# (e) end-to-end surfacing — LEARNINGS #49: a knob is not done when it merely works
assert "the sample .docket.yml carries the learnings block" \
  'grep -qE "^# learnings:$" "$REPO/.docket.yml"'
assert "the sample documents both keys" \
  'grep -qE "^#   enabled: true$" "$REPO/.docket.yml" && grep -qE "^#   cap: 300$" "$REPO/.docket.yml"'
assert "README presents learnings as a feature" 'grep -qF "## Learnings — the loop" "$REPO/README.md"'
assert "README points at the convention rather than restating mechanics" \
  'grep -qF "single source" "$REPO/README.md"'
assert "AGENTS.md exists as the promotion destination" '[ -f "$REPO/AGENTS.md" ]'
assert "AGENTS.md states the tiering criterion" \
  'grep -qF "will the agent know to search for this?" "$REPO/AGENTS.md"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"

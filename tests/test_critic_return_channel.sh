#!/usr/bin/env bash
# tests/test_critic_return_channel.sh — change 0281. The docket-auto-groom-critic verdict travels
# on exactly ONE channel: the critic's final report, read by the groom as the dispatch's return
# while it actively blocks. This guard binds the three prose contracts that make that true:
#
#   (a) agents/docket-auto-groom-critic.md — the DELIVERY half: the verdict IS the final report,
#       and the critic never addresses its dispatcher by name or via an agent-listing surface.
#   (b) skills/docket-auto-groom/SKILL.md Step 3 — the RECEIVING half, plus the bounded
#       no-verdict posture that terminates in the Tier B abstain.
#   (c) skills/docket-convention/SKILL.md *Composition* — the critic dispatch sits in the
#       in-context-return family, NOT inside the git-state-contract clause.
#
# Deliberate limits, named so a later reader does not over-trust this file:
#   * PHRASE-SCOPED. A contract reworded past these anchors escapes it. The anchors are verbatim
#     quoted clauses (ADR-0054), so drift is at least mechanically visible.
#   * The (c) assert is an ORDERING fact over one paragraph, not a parse of English antecedents.
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review.
# Run: bash tests/test_critic_return_channel.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Whitespace-collapse: these contracts are hard-wrapped prose, so every proximity match below runs
# against a flattened copy or it would fail on a line break that means nothing.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

CRITIC="$REPO/agents/docket-auto-groom-critic.md"
AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"

# --- (a) the critic's delivery clause -----------------------------------------------------------
# Non-vacuity anchor: the file must exist and be non-empty, or every assert below passes for
# reasons unrelated to the property.
assert "critic source exists and is non-empty" '[ -s "$CRITIC" ]'
# Non-vacuity anchor: a live PRESENCE assert through the same read, so a rename reddens here
# rather than silently greening the rest.
assert "critic source is the adversarial critic contract" 'grep -qi "adversarial critic" "$CRITIC"'

critic_flat="$(flat "$(cat "$CRITIC")")"

# The verdict is bound to the final report. Bounded gap ([^.]{0,80}) keeps the match inside one
# sentence, so a stray "verdict" and a stray "final report" in different clauses cannot satisfy it.
assert "critic: the verdict IS the critic's final report" \
  'grep -qE "verdict[^.]{0,80}final report|final report[^.]{0,80}verdict" <<<"$critic_flat"'

# The never-address-your-dispatcher clause, pinned on its two load-bearing halves: the prohibition
# itself, and the REASON (which is what stops a critic from inventing a workaround channel).
assert "critic: never message, address, or resolve the dispatcher" \
  'grep -qE "[Nn]ever[^.]{0,120}(message|address|resolve)[^.]{0,120}dispatcher" <<<"$critic_flat"'
assert "critic: states why no such address resolves" \
  'grep -qF -- "not registered under its skill name" <<<"$critic_flat"'
# The belief-changes-nothing clause: a critic that concludes the channel is broken must still write
# the verdict as its final report. Without this, a critic reasons its way into silence.
assert "critic: a believed-unavailable channel changes nothing about what it does" \
  'grep -qE "believe[^.]{0,160}changes nothing" <<<"$critic_flat"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
